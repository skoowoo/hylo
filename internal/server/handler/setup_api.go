package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/hardhacker/hylo/internal/mate"
)

// Setting up the first bot: the desktop onboarding and the web's empty
// Agent Bots page both run detect → hello → create through these.

const (
	hyloBotName        = "Hylo"
	hyloBotDescription = "Your personal AI agent: chat here, on WeChat or on Discord."
	// Opens every message a chat trigger hands the bot, so it can tell where a
	// message came from; a message without one was typed in Hylo's own chat.
	wechatTag  = "[Message via WeChat]"
	discordTag = "[Message via Discord]"

	compilePrompt = "Compile note `{Path}` into structured knowledge under `/_knowledge`. Steps:\n\n" +
		"1. Use the `hylo-compile-note` skill to compile the note\n" +
		"2. Once compiled, use the `hylo-index-knowledge` skill to create or update the index\n" +
		"3. Finally, if the `humanizer` skill is available, use it to polish the wording of the newly generated knowledge unit(s) — you must preserve the `frontmatter`, `link`, `wiki link`, and paragraph structure."
	memoryPrompt         = "Please update my personal memory from my recent notes."
	recapShortNotePrompt = "Use the hylo-recap skill to cross-check this short note against my knowledge base — anything related or conflicting?\n\nShort note content: `{Content}`"
	recapWeeklyPrompt    = "Use the hylo-recap skill to review everything I wrote this past week."
)

// hyloBotPrompt sits under the global agent prompt, which already covers the
// knowledge base, memory and wiki links; this one gives the main bot its job.
var hyloBotPrompt = `You are Hylo, the user's personal AI agent, powered by their notes. They just write; you learn from their notes who they are and what they care about, and work for them with that in mind. Organizing, connecting and recalling notes is your job, not theirs.

- Use what you know of them in everything you do, not only note tasks: when you find, recommend or summarize anything, fit it to their interests and current work.
- Before answering anything about their own work, plans or past thinking, search the vault and the knowledge base, and cite the notes you used.
- Know them: draw on their personal memory, and when they share something lasting about themselves, offer to update it.
- When they ask you to remember or save something, write it into the vault: a quick thought goes into today's short note; anything longer becomes its own note in a fitting top-level folder. Never create nested folders, and never ask them where something should go.
- When they ask to organize notes or turn them into knowledge, compile them into the knowledge base and link them up.
- Be brief and direct.
- A message that starts with ` + wechatTag + ` or ` + discordTag + ` was sent from that app, usually on a phone: answer it in plain text, a few sentences, no markdown, and leave the tag out of your reply. Any other message is the user chatting with you in Hylo.
- Reply in the language the user writes in.`

// hyloBot answers wherever the user talks to Hylo (the chat triggers stay idle
// until WeChat or Discord is connected), and compiles, keeps memory and recaps.
// The two schedules are apart so they don't run at once on Mondays.
func hyloBot(agentID, model string) (*mate.Mate, []mate.MateTrigger) {
	if model == "" {
		model = "default"
	}
	on := func(name string, event mate.MateEventType, prompt string) mate.MateTrigger {
		return mate.MateTrigger{Name: name, EventTypes: []string{string(event)}, Prompt: prompt, Enabled: true}
	}
	scheduled := func(name, schedule, prompt string) mate.MateTrigger {
		t := on(name, mate.MateEventScheduled, prompt)
		t.Schedule = schedule
		return t
	}
	webClips := on("Compiler", mate.MateEventNoteCreated, compilePrompt)
	webClips.PathPrefixes = []string{"/Web Clips/"}
	m := &mate.Mate{
		Name:         hyloBotName,
		Description:  hyloBotDescription,
		AgentID:      agentID,
		Model:        model,
		SystemPrompt: hyloBotPrompt,
		Enabled:      true,
	}
	return m, []mate.MateTrigger{
		on("WeChat", mate.MateEventWechatMessage, wechatTag+"\n{Content}"),
		on("Discord", mate.MateEventDiscordMessage, discordTag+"\n{Content}"),
		on("Compiler", mate.MateEventCompileRequested, compilePrompt),
		webClips,
		scheduled("Memory", "daily 09:00", memoryPrompt),
		on("Recap", mate.MateEventShortNoteCreated, recapShortNotePrompt),
		scheduled("Recap", "weekly mon 09:05", recapWeeklyPrompt),
	}
}

var helloNotesAngles = []string{
	"Notice a theme running through their note titles and name it, like a small revelation.",
	"Pick one note title and link it to another they may not have connected yet.",
	"Pick one note title and ask one curious, specific question about it they'd want to answer.",
	"Offer one concrete thing you could do with these exact notes right now, naming them.",
	"Make a light, perceptive guess at what they've been working on lately, from what changed most recently.",
	"Guess one thing about them as a person, from what they choose to write about, and say it lightly.",
}

var helloEmptyAngles = []string{
	"Suggest one tiny, specific first note that fits their time of day.",
	"Make a playful, perceptive remark about a blank page, then invite one specific first thought.",
	"Ask one question whose answer would make a great first note.",
	"Tell them, concretely, what you'll do once they've written a few notes, like learning what they care about and finding news that fits.",
}

func quoted(items []string) string {
	q := make([]string, len(items))
	for i, s := range items {
		q[i] = fmt.Sprintf("%q", s)
	}
	return strings.Join(q, ", ")
}

// helloPrompt asks for the agent's first words to the user. It gets a glimpse
// of the vault so it can say something only an AI that sees these notes could;
// the angle rotates so the same vault gets different words each time.
func helloPrompt(o VaultOverview, lang, now string, seed int) string {
	var known []string
	if now != "" {
		known = append(known, fmt.Sprintf("- Their local time: %s.", now))
	}
	var angle string
	if o.Notes == 0 {
		known = append(known, fmt.Sprintf("- Their notes folder %q is brand new and empty.", o.Name))
		angle = helloEmptyAngles[seed%len(helloEmptyAngles)]
	} else {
		plural := "notes"
		if o.Notes == 1 {
			plural = "note"
		}
		line := fmt.Sprintf("- Their notes folder %q holds %d %s", o.Name, o.Notes, plural)
		if len(o.Folders) > 0 {
			names := make([]string, len(o.Folders))
			for i, f := range o.Folders {
				names[i] = f.Name
			}
			line += ", mostly in " + quoted(names)
		}
		known = append(known, line+".")
		if len(o.Recent) > 0 {
			titles := make([]string, len(o.Recent))
			for i, n := range o.Recent {
				titles[i] = n.Title
			}
			known = append(known, "- Most recently edited: "+quoted(titles)+".")
		}
		angle = helloNotesAngles[seed%len(helloNotesAngles)]
	}
	if lang == "" {
		lang = "English"
	}
	return fmt.Sprintf(`You are Hylo, a personal AI agent powered by someone's notes: they just write, and from their notes you learn what matters to them, then work for them around the clock, on their notes and beyond. They just set you up, and this is the first thing you'll ever say to them. It appears in a small card on the setup screen, so make it count.

What you know:
%s

Write one or two short sentences: 25 words at most, or 50 characters in Chinese or Japanese. %s
Sound like a sharp, warm friend who already noticed something, not like a product. Never say you're ready to help, never ask how you can help, no greeting formula, no emoji, no markdown, no quotation marks around your reply. Speak as Hylo: never name the AI model, agent or company behind you. Don't use tools or open files; use only what's above.
Reply in the language their note titles are mostly written in; if there are none, use the language of the locale %s.`,
		strings.Join(known, "\n"), angle, lang)
}

// SetupHelloPOST handles POST /api/setup/hello: starts the agent's first words
// as a run and returns its runId, streamed from /api/runs/{id}/events.
func (a *AgentAPI) SetupHelloPOST(w http.ResponseWriter, r *http.Request) {
	var body struct {
		AgentID string `json:"agentId"`
		Model   string `json:"model"`
		Lang    string `json:"lang"`
		Now     string `json:"now"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.AgentID) == "" {
		http.Error(w, "agentId required", http.StatusBadRequest)
		return
	}
	o, err := buildVaultOverview(a.vault)
	if err != nil {
		writeVaultError(w, err)
		return
	}
	chat := chatBody{
		AgentID:    body.AgentID,
		Model:      body.Model,
		Message:    helloPrompt(o, body.Lang, body.Now, time.Now().Nanosecond()),
		TriggerRun: true,
	}
	run := a.hub.CreateRun(map[string]string{"agentId": body.AgentID})
	run.AgentID = body.AgentID
	go a.executeChat(context.WithoutCancel(r.Context()), run, chat)
	respondJSON(w, http.StatusAccepted, map[string]any{"runId": run.ID})
}

// SetupBotPOST handles POST /api/setup/bot: creates the Hylo bot, its chat
// opening with the hello the user just read. Only an empty vault gets one,
// so a repeat is harmless; mateId is empty then.
func (a *AgentAPI) SetupBotPOST(w http.ResponseWriter, r *http.Request) {
	if a.store == nil {
		http.Error(w, "bots unavailable", http.StatusServiceUnavailable)
		return
	}
	var body struct {
		AgentID  string `json:"agentId"`
		Model    string `json:"model"`
		Greeting string `json:"greeting"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.AgentID) == "" {
		http.Error(w, "agentId required", http.StatusBadRequest)
		return
	}
	a.setupMu.Lock()
	defer a.setupMu.Unlock()
	existing, err := a.store.ListMates()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(existing) > 0 {
		respondJSON(w, http.StatusOK, map[string]any{"mateId": ""})
		return
	}
	m, triggers := hyloBot(body.AgentID, strings.TrimSpace(body.Model))
	if err := mate.ValidateTriggers(triggers); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := a.store.CreateMate(m); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// A bot without its triggers would block every retry, so undo it.
	if err := a.store.ReplaceTriggersForMate(m.ID, triggers); err != nil {
		if derr := a.store.DeleteMate(m.ID); derr != nil {
			a.logger.Warn("setup: undo bot", slog.String("err", derr.Error()))
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// The bot works without its greeting, so a failure here doesn't fail setup.
	if g := strings.TrimSpace(body.Greeting); g != "" {
		if conv, err := a.store.EnsureNewConv(m.ID, mate.ConvTypeChat, "", "", 10); err == nil {
			msg := mate.Message{ConversationID: conv.ID, Role: "assistant", Content: g, MateID: m.ID, AgentID: m.AgentID, Status: "succeeded"}
			if _, err := a.store.InsertMessage(msg); err != nil {
				a.logger.Warn("setup: greeting", slog.String("err", err.Error()))
			}
		}
	}
	respondJSON(w, http.StatusCreated, map[string]any{"mateId": m.ID})
}
