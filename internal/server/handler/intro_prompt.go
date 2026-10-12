package handler

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/hardhacker/hylo/internal/storage"
)

const introExcerpt = 120

// vaultOverview is what the intro guide is built from. It goes into the prompt
// so the agent starts typing without exploring the vault first.
func vaultOverview(v *storage.Vault) string {
	if v == nil {
		return ""
	}
	o, err := buildVaultOverview(v)
	if err != nil {
		return ""
	}
	if o.Notes == 0 {
		return "Their vault is empty: they are starting fresh."
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "- %d notes", o.Notes)
	var names []string
	for _, f := range o.Folders {
		names = append(names, fmt.Sprintf("%s (%d)", f.Name, f.Count))
	}
	if len(names) > 0 {
		sb.WriteString(", in folders " + strings.Join(names, ", "))
	}
	sb.WriteString(".\n- Most recently edited:")
	for _, n := range o.Recent {
		fmt.Fprintf(&sb, "\n  - %s", n.Path)
		if n.Preview != "" {
			fmt.Fprintf(&sb, ": %q", n.Preview)
		}
	}
	return sb.String()
}

func excerpt(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) <= introExcerpt {
		return s
	}
	return string([]rune(s)[:introExcerpt]) + "…"
}

// introPrompt steers the first chat after onboarding: the bot writes the user a
// guide built from their own vault, live in the editor, so its first answer
// shows what it can do rather than listing it.
func introPrompt(greeting, overview string) string {
	var sb strings.Builder
	sb.WriteString("This is the user's very first chat with you, right after they set up Hylo. Their message asks what you can do and how to start.")
	if g := strings.TrimSpace(greeting); g != "" {
		fmt.Fprintf(&sb, " You already opened this chat with: %q", g)
	}
	if overview != "" {
		sb.WriteString("\n\nTheir vault right now:\n" + overview)
	}
	sb.WriteString(`

Answer by writing them a short guide to Hylo live in the editor, made from their own notes, so they watch it being written. Be quick to start: everything you need is here, so don't read skill files or explore the vault first.

Hylo is a personal AI agent powered by their notes: they just write, you learn from their notes who they are, and you work for them around the clock, on their notes and well beyond them. The guide shows that promise through their own notes.

1. With editor_type, type the guide into a new note named "Getting started with Hylo" (in the user's language), one section per call, at normal speed. If that name is taken, add " 2".
   - A "# " title, and one sentence on what you, their personal agent, will do for them, tied to their notes above.
   - You write: short notes for passing thoughts, clipped web pages, and [[wikilinks]] between notes. No sorting or tagging, and keep folders flat. Link one or two of their real notes by name if they have any.
   - I get to know you: you compile their notes into a knowledge base, a linked wiki they can browse as a knowledge graph, and distill it into personal memory: who they are, what they're working on, what they care about. They can compile a note from its menu and ask you to update their memory; every chat after starts out knowing them.
   - I work for you: because you know them, your work fits them, not just their notes. Agent bots (Settings → Agent Bots) run on a schedule or when notes change: a morning news digest picked for their interests, a weekly review of what they wrote, a daily memory update. Give two or three example requests built from their own notes, at least one beyond the notes themselves (news, recommendations, plans); for an empty vault, make them fit their first week of notes.
   - Always on: ask here in chat, or from WeChat or Discord once connected. Run Hylo on a home or cloud server and you stay online even when this computer is off: bots keep running and you answer anytime. Setup: https://github.com/skoowoo/hylo/blob/main/docs/remote-deployment.md, then add it from the server switcher → Manage servers… → Add a remote server.
   - A small mindmap of how their notes and Hylo fit together, as a fenced block:
     ` + "```mindmap" + `
     # Hylo
     ## You write
     - quick thoughts
     ## I know you
     - memory
     ## I work for you
     - news digest
     ## Always on
     - WeChat
     ` + "```" + `
     Every node is a heading or a list item. Keep labels to a few plain words; [[links]] don't work inside a mindmap.
   Keep the guide under 350 words, or 700 characters in Chinese or Japanese. No frontmatter.
2. Then reply here in one or two sentences with a [[link]] to the guide. Don't repeat the guide in the chat.

If the editor tools are missing or fail, don't write the note another way: give a brief version of the guide in the chat instead.
Write in the language the user writes in.`)
	return sb.String()
}
