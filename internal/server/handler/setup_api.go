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
	"github.com/hardhacker/hylo/internal/prompt"
)

// Setting up the first bot: the desktop onboarding and the web's empty
// Agent Bots page both run detect → hello → create through these.

const (
	hyloBotName        = "Hylo"
	hyloBotDescription = "Your personal AI agent. Connect WeChat, Discord or Telegram on this bot to chat there too."
)

// hyloBot compiles notes and keeps memory. Chat apps are connected on the bot
// itself and join its chat. Reviews are a Recall bot's job, which hylo-setup offers.
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
	webClips := on("Compiler", mate.MateEventNoteCreated, prompt.TriggerCompile)
	webClips.PathPrefixes = []string{"/Web Clips/"}
	m := &mate.Mate{
		Name:         hyloBotName,
		Description:  hyloBotDescription,
		AgentID:      agentID,
		Model:        model,
		Main:         true,
		SystemPrompt: prompt.HyloBot,
		Enabled:      true,
	}
	return m, []mate.MateTrigger{
		on("Compiler", mate.MateEventCompileRequested, prompt.TriggerCompile),
		webClips,
		scheduled("Memory", "daily 09:00", prompt.TriggerMemory),
	}
}

// helloApps is what the desktop app sees of the user's other notes.
type helloApps struct {
	ObsidianVaults []struct {
		Name       string `json:"name"`
		LastOpened int64  `json:"lastOpened"` // Unix ms; 0 when unknown
	} `json:"obsidianVaults"`
	NoteApps []string `json:"noteApps"`
}

func helloOf(o VaultOverview, apps helloApps, lang, now string) prompt.Hello {
	h := prompt.Hello{Vault: o.Name, Notes: o.Notes, Obsidian: o.Obsidian, WritingTime: o.WritingTime, Lang: lang, Now: now}
	for _, f := range o.Folders {
		h.Folders = append(h.Folders, f.Name)
	}
	t := time.Now()
	for _, n := range o.Recent {
		h.Recent = append(h.Recent, prompt.HelloNote{Title: n.Title, Excerpt: n.Preview, Age: ago(n.Updated, t)})
	}
	for _, n := range o.Older {
		h.Older = append(h.Older, prompt.HelloNote{Title: n.Title, Excerpt: n.Preview, Age: ago(n.Updated, t)})
	}
	for _, v := range apps.ObsidianVaults {
		if len(h.OtherVaults) == 3 {
			break
		}
		hv := prompt.HelloVault{Name: v.Name}
		if v.LastOpened > 0 {
			hv.LastOpened = "opened " + ago(time.UnixMilli(v.LastOpened), t)
		}
		h.OtherVaults = append(h.OtherVaults, hv)
	}
	h.NoteApps = apps.NoteApps[:min(len(apps.NoteApps), 5)]
	return h
}

func ago(at, now time.Time) string {
	days := int(now.Sub(at).Hours() / 24)
	switch {
	case days < 1:
		return "today"
	case days < 2:
		return "yesterday"
	case days < 14:
		return fmt.Sprintf("%d days ago", days)
	case days < 60:
		return fmt.Sprintf("%d weeks ago", days/7)
	case days < 730:
		return fmt.Sprintf("%d months ago", days/30)
	}
	return fmt.Sprintf("%d years ago", days/365)
}

// SetupHelloPOST handles POST /api/setup/hello: starts the agent's first words
// as a run and returns its runId, streamed from /api/runs/{id}/events.
func (a *AgentAPI) SetupHelloPOST(w http.ResponseWriter, r *http.Request) {
	var body struct {
		AgentID     string      `json:"agentId"`
		Model       string      `json:"model"`
		Lang        string      `json:"lang"`
		LangSignals langSignals `json:"langSignals"`
		Now         string      `json:"now"`
		helloApps
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.AgentID) == "" {
		http.Error(w, "agentId required", http.StatusBadRequest)
		return
	}
	o, err := buildVaultOverview(a.vault, time.Now().Nanosecond())
	if err != nil {
		writeVaultError(w, err)
		return
	}
	lang := userLang(body.LangSignals)
	if lang == "" {
		lang = body.Lang
	}
	chat := chatBody{
		AgentID:    body.AgentID,
		Model:      body.Model,
		Message:    prompt.HelloMessage(helloOf(o, body.helloApps, lang, body.Now)),
		TriggerRun: true,
		oneShot:    true,
	}
	run := a.hub.CreateRun(map[string]string{"agentId": body.AgentID})
	run.AgentID = body.AgentID
	a.detach(r.Context(), func(ctx context.Context) { a.executeChat(ctx, run, chat) })
	// The intro chat that follows writes in the same language.
	respondJSON(w, http.StatusAccepted, map[string]any{"runId": run.ID, "lang": lang})
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
		if conv, err := a.store.EnsureNewConv(m.ID, mate.ConvTypeChat, "", 10); err == nil {
			msg := mate.Message{ConversationID: conv.ID, Role: "assistant", Content: g, MateID: m.ID, AgentID: m.AgentID, Status: "succeeded"}
			if _, err := a.store.InsertMessage(msg); err != nil {
				a.logger.Warn("setup: greeting", slog.String("err", err.Error()))
			}
		}
	}
	respondJSON(w, http.StatusCreated, map[string]any{"mateId": m.ID})
}
