package prompt

import (
	"fmt"
	"strings"
)

// Chat triggers open every message with a tag so the bot can tell where it
// came from; a message without one was typed in Hylo's own chat.
const (
	WechatTag  = "[Message via WeChat]"
	DiscordTag = "[Message via Discord]"
)

// HyloBot sits under the global prompt, which already covers the knowledge
// base, memory and wiki links; this one gives the main bot its job.
var HyloBot = execute(parse("bot_hylo"), struct{ WechatTag, DiscordTag string }{WechatTag, DiscordTag})

// Trigger templates of the Hylo bot.
var (
	TriggerCompile     = load("trigger_compile")
	TriggerMemory      = load("trigger_memory")
	TriggerRecapShort  = load("trigger_recap_short")
	TriggerRecapWeekly = load("trigger_recap_weekly")
	TriggerWechat      = WechatTag + "\n{Content}"
	TriggerDiscord     = DiscordTag + "\n{Content}"
)

var helloTmpl = parse("hello")

var HelloNotesAngles = []string{
	"Notice a theme running through their note titles and name it, like a small revelation.",
	"Pick one note title and link it to another they may not have connected yet.",
	"Pick one note title and ask one curious, specific question about it they'd want to answer.",
	"Offer one concrete thing you could do with these exact notes right now, naming them.",
	"Make a light, perceptive guess at what they've been working on lately, from what changed most recently.",
	"Guess one thing about them as a person, from what they choose to write about, and say it lightly.",
}

var HelloEmptyAngles = []string{
	"Suggest one tiny, specific first note that fits their time of day.",
	"Make a playful, perceptive remark about a blank page, then invite one specific first thought.",
	"Ask one question whose answer would make a great first note.",
	"Tell them, concretely, what you'll do once they've written a few notes, like learning what they care about and finding news that fits.",
}

// Hello is a glimpse of the vault for the agent's first words to the user.
type Hello struct {
	Vault   string
	Notes   int
	Folders []string
	Recent  []string
	Lang    string
	Now     string
	// Seed rotates the angle so the same vault gets different words each time.
	Seed int
}

// HelloMessage asks for the agent's first words, something only an AI that
// sees these notes could say.
func HelloMessage(h Hello) string {
	var known []string
	if h.Now != "" {
		known = append(known, fmt.Sprintf("- Their local time: %s.", h.Now))
	}
	var angle string
	if h.Notes == 0 {
		known = append(known, fmt.Sprintf("- Their notes folder %q is brand new and empty.", h.Vault))
		angle = HelloEmptyAngles[h.Seed%len(HelloEmptyAngles)]
	} else {
		plural := "notes"
		if h.Notes == 1 {
			plural = "note"
		}
		line := fmt.Sprintf("- Their notes folder %q holds %d %s", h.Vault, h.Notes, plural)
		if len(h.Folders) > 0 {
			line += ", mostly in " + quoted(h.Folders)
		}
		known = append(known, line+".")
		if len(h.Recent) > 0 {
			known = append(known, "- Most recently edited: "+quoted(h.Recent)+".")
		}
		angle = HelloNotesAngles[h.Seed%len(HelloNotesAngles)]
	}
	lang := h.Lang
	if lang == "" {
		lang = "English"
	}
	return execute(helloTmpl, struct{ Known, Angle, Lang string }{strings.Join(known, "\n"), angle, lang})
}

func quoted(items []string) string {
	q := make([]string, len(items))
	for i, s := range items {
		q[i] = fmt.Sprintf("%q", s)
	}
	return strings.Join(q, ", ")
}
