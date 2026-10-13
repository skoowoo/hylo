// Package prompt owns every prompt Hylo hands an agent: the layered system
// prompt, the first-turn message envelope, trigger templates and the built-in
// texts under text/.
package prompt

import "strings"

// Kind is where a run comes from; sections use it to decide whether they apply.
type Kind int

const (
	// Chat is the user talking in Hylo's own chat UI.
	Chat Kind = iota
	// Trigger is a bot run fired by an event, including WeChat/Discord replies.
	Trigger
	// OneShot is an internal single-shot ask, like the setup hello: its
	// message is self-contained and must not read the vault or use tools.
	OneShot
)

// Run is everything the system prompt depends on. Callers gather it; sections
// stay pure functions of it.
type Run struct {
	Kind Kind
	// Bot is the bot's own system prompt.
	Bot string
	// MCP is set when Hylo's MCP tools are injected into this run.
	MCP   bool
	Intro *Intro
	// Overrides replaces a section's built-in text by section ID, e.g. the
	// user's agent.system_prompt for Global. Blank values are ignored.
	Overrides map[string]string
}

// Section is one layer of the system prompt.
type Section struct {
	ID      string
	Applies func(Run) bool
	Text    func(Run) string
}

// Part is a rendered section.
type Part struct {
	ID   string
	Text string
}

const divider = "\n\n---\n\n"

// Parts renders the sections that apply to r, in order.
func Parts(r Run) []Part {
	var out []Part
	for _, s := range sections {
		if s.Applies != nil && !s.Applies(r) {
			continue
		}
		t := strings.TrimSpace(r.Overrides[s.ID])
		if t == "" {
			t = strings.TrimSpace(s.Text(r))
		}
		if t != "" {
			out = append(out, Part{s.ID, t})
		}
	}
	return out
}

// System is the full system prompt for r.
func System(r Run) string {
	parts := Parts(r)
	texts := make([]string, len(parts))
	for i, p := range parts {
		texts[i] = p.Text
	}
	return strings.Join(texts, divider)
}

// Message is what the agent CLI receives as its prompt. Agents take no
// separate system prompt, so the first turn carries it inline; a resumed
// session already has it. workdir is named only when the agent runs in the
// vault rather than a bot's own cwd.
func Message(system, request, workdir string, resume bool) string {
	if resume {
		return request
	}
	var sb strings.Builder
	if t := strings.TrimSpace(system); t != "" {
		sb.WriteString("# Instructions\n\n")
		sb.WriteString(t)
		sb.WriteString("\n\n---\n")
	}
	if workdir != "" {
		sb.WriteString("\nYour working directory (vault): ")
		sb.WriteString(workdir)
		sb.WriteString("\n\n")
	}
	sb.WriteString("# User request\n\n")
	sb.WriteString(request)
	return sb.String()
}
