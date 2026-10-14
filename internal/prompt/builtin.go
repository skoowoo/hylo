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

// Hello scenes: what the first words should do for someone in that spot.
const (
	// HelloSeen shows we read their notes, from one detail only a reader knows.
	HelloSeen = "seen"
	// HelloObsidian is for someone who keeps an Obsidian vault but started
	// Hylo on an empty folder, likely trying it before trusting it.
	HelloObsidian = "obsidian"
	// HelloOtherApp is for an empty vault whose notes likely live in another app.
	HelloOtherApp = "other-app"
	// HelloFresh is for someone with no notes anywhere we can see.
	HelloFresh = "fresh"
)

// Hello is a glimpse of the user's notes and note apps for the agent's first words.
type Hello struct {
	Vault    string
	Notes    int
	Obsidian bool // the vault is also an Obsidian vault
	Folders  []string
	Recent   []HelloNote
	Older    []HelloNote
	// WritingTime names when they usually write, e.g. "late at night"; empty when unclear.
	WritingTime string
	// OtherVaults are Obsidian vaults they didn't pick.
	OtherVaults []HelloVault
	NoteApps    []string
	Lang        string
	Now         string
}

type HelloNote struct {
	Title   string
	Excerpt string
	Age     string // e.g. "3 days ago"
}

type HelloVault struct {
	Name       string
	LastOpened string // e.g. "opened yesterday"; may be empty
}

// Scene picks what the first words should do; it's a fact about the user, not a guess for the agent.
func (h Hello) Scene() string {
	switch {
	case h.Notes > 0:
		return HelloSeen
	case len(h.OtherVaults) > 0:
		return HelloObsidian
	case len(h.NoteApps) > 0:
		return HelloOtherApp
	}
	return HelloFresh
}

// HelloMessage asks for the agent's first words, something only an AI that
// sees these notes, or this user's setup, could say.
func HelloMessage(h Hello) string {
	var known []string
	if h.Now != "" {
		known = append(known, fmt.Sprintf("- Their local time: %s.", h.Now))
	}
	if h.Notes == 0 {
		known = append(known, fmt.Sprintf("- The notes folder they gave Hylo, %q, is brand new and empty.", h.Vault))
	} else {
		plural := "notes"
		if h.Notes == 1 {
			plural = "note"
		}
		line := fmt.Sprintf("- The notes folder they gave Hylo, %q, holds %d %s", h.Vault, h.Notes, plural)
		if h.Obsidian {
			line += "; it is also their Obsidian vault"
		}
		if len(h.Folders) > 0 {
			line += ", mostly in " + quoted(h.Folders)
		}
		known = append(known, line+".")
		if len(h.Recent) > 0 {
			known = append(known, "- Most recently edited:"+notes(h.Recent))
		}
		if len(h.Older) > 0 {
			known = append(known, "- Some older notes:"+notes(h.Older))
		}
		if h.WritingTime != "" {
			known = append(known, "- They mostly write "+h.WritingTime+".")
		}
	}
	if len(h.OtherVaults) > 0 {
		var vs []string
		for _, v := range h.OtherVaults {
			if v.LastOpened != "" {
				vs = append(vs, fmt.Sprintf("%q (%s)", v.Name, v.LastOpened))
			} else {
				vs = append(vs, fmt.Sprintf("%q", v.Name))
			}
		}
		known = append(known, "- Obsidian vaults they have but didn't give Hylo: "+strings.Join(vs, ", ")+".")
	}
	if len(h.NoteApps) > 0 {
		known = append(known, "- Note apps installed on this computer: "+strings.Join(h.NoteApps, ", ")+".")
	}
	lang := h.Lang
	if lang == "" {
		lang = "English"
	}
	return execute(helloTmpl, struct{ Known, Scene, Lang string }{strings.Join(known, "\n"), h.Scene(), lang})
}

func notes(ns []HelloNote) string {
	var sb strings.Builder
	for _, n := range ns {
		fmt.Fprintf(&sb, "\n  - %q", n.Title)
		if n.Age != "" {
			sb.WriteString(", " + n.Age)
		}
		if n.Excerpt != "" {
			fmt.Fprintf(&sb, ": %q", n.Excerpt)
		}
	}
	return sb.String()
}

func quoted(items []string) string {
	q := make([]string, len(items))
	for i, s := range items {
		q[i] = fmt.Sprintf("%q", s)
	}
	return strings.Join(q, ", ")
}
