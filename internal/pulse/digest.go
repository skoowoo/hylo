package pulse

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/hardhacker/hylo/internal/desktop"
	"github.com/hardhacker/hylo/internal/mate"
	"github.com/hardhacker/hylo/internal/prompt"
)

const (
	digestNotes  = 20
	digestChat   = 8
	digestRounds = 8
	chatWithin   = 48 * time.Hour
	// digestWindow is the furthest back "since then" reaches.
	digestWindow = 24 * time.Hour
	sentWithin   = 72 * time.Hour
	excerptLen   = 280
)

// digest is what a round reads: the moment, the agenda, what each lead found
// since the bot last looked and acted (since), and the nudges it may raise,
// in sections the round prompt refers to. last is when the previous round began.
func (p *Pulse) digest(m *mate.Mate, last, since, now time.Time) (string, []liveNudge) {
	s := p.scene(m, since, now)
	sv := p.survey(s)
	var b strings.Builder
	section := func(title string, lines []string, empty string) {
		fmt.Fprintf(&b, "\n## %s\n\n", title)
		if len(lines) == 0 {
			b.WriteString(empty + "\n")
			return
		}
		b.WriteString(strings.Join(lines, "\n") + "\n")
	}

	moment := []string{"- Local time: " + now.Format("Mon 2006-01-02 15:04") + "."}
	if last.IsZero() {
		moment = append(moment, "- This is your first round.")
	} else {
		moment = append(moment, "- Your last round: "+ago(now.Sub(last))+".")
	}
	if t, err := p.store.LastUserMessageAt(m.ID); err == nil && !t.IsZero() {
		moment = append(moment, "- The user last wrote to you "+ago(now.Sub(t))+".")
	}
	section("Now", moment, "")
	section("Where the user is", p.whereLines(m.ID, now), "You can't tell: the desktop app hasn't reported lately, and they haven't written from their phone.")

	var agenda []string
	if items, err := p.store.OpenAgenda(m.ID); err == nil {
		for _, it := range items {
			when := "no set time"
			if !it.Due.IsZero() {
				when = "due " + it.Due.Format("2006-01-02 15:04")
				if !it.Due.After(now) {
					when += ", now due"
				}
			}
			agenda = append(agenda, fmt.Sprintf("- `%s` %s (%s)", it.ID, it.Text, when))
		}
	}
	section("Your agenda", agenda, "Empty.")

	for _, sec := range sv.sections {
		section(sec.title, sec.lines, sec.empty)
	}
	if len(sv.nudges) > 0 {
		b.WriteString("\n## Nudges you could raise\n")
		for _, n := range sv.nudges {
			fmt.Fprintf(&b, "\n### %s\n\n%s\n\n%s\n", n.name, strings.Join(n.lines, "\n"), prompt.Nudge(n.name))
		}
	}
	section("Your chat lately", p.chatLines(m.ID, now), "No recent messages.")
	section("Your fellow bots", p.botLines(m.ID), "None; you are the user's only bot.")
	section("What you sent on your own lately", p.sentLines(m.ID, now), "Nothing in the last three days.")
	section("Your recent rounds", p.roundLines(m.ID, now), "None yet.")
	return strings.TrimSpace(b.String()), sv.nudges
}

// shortLines quotes new shorts and names edited ones, oldest first.
func shortLines(changed []change) []string {
	var out []string
	for _, c := range changed {
		switch {
		case c.Short == "":
		case c.New:
			out = append(out, fmt.Sprintf("- %s, new (`%s`): %s", c.At.Format("15:04"), c.Short, excerpt(c.Content)))
		default:
			out = append(out, fmt.Sprintf("- %s, edited `%s`", c.At.Format("15:04"), c.Short))
		}
	}
	if len(out) > digestNotes {
		out = append([]string{fmt.Sprintf("- …%d earlier ones left out", len(out)-digestNotes)}, out[len(out)-digestNotes:]...)
	}
	return out
}

// changedLines lists the notes that changed, newest first, each once.
func changedLines(changed []change) []string {
	type note struct {
		created bool
		at      time.Time
	}
	byPath := map[string]note{}
	for _, c := range changed {
		if c.Short != "" {
			continue
		}
		n := byPath[c.Path]
		n.created = n.created || c.Created
		n.at = c.At
		byPath[c.Path] = n
	}
	paths := make([]string, 0, len(byPath))
	for path := range byPath {
		paths = append(paths, path)
	}
	slices.SortFunc(paths, func(a, b string) int { return byPath[b].at.Compare(byPath[a].at) })
	var out []string
	for i, path := range paths {
		if i == digestNotes {
			out = append(out, fmt.Sprintf("- …and %d more", len(paths)-digestNotes))
			break
		}
		what := "changed"
		if byPath[path].created {
			what = "new"
		}
		out = append(out, fmt.Sprintf("- %s: %s, %s", path, what, byPath[path].at.Format("15:04")))
	}
	return out
}

func (p *Pulse) chatLines(mateID string, now time.Time) []string {
	_, lines, _, err := p.store.BotChat(mateID, "", digestChat)
	if err != nil {
		return nil
	}
	var out []string
	for _, l := range lines {
		if l.Text == "" || now.Sub(l.At) > chatWithin {
			continue
		}
		who := "user"
		switch {
		case l.OnItsOwn:
			who = "you, on your own"
		case l.Role == "assistant":
			who = "you"
		case l.App != "":
			who = "user, from " + appName(l.App)
		}
		out = append(out, fmt.Sprintf("- %s, %s: %s", who, l.At.Format("Mon 15:04"), excerpt(l.Text)))
	}
	return out
}

// whereLines says where the user is now, whether a message would reach their
// phone, and where their day at the computer went.
func (p *Pulse) whereLines(mateID string, now time.Time) []string {
	st := p.desk.Now(now)
	var out []string
	switch {
	case !st.Known:
	case st.Present:
		out = append(out, fmt.Sprintf("- Now: in %s for %s.", st.App, span(now.Sub(st.AppSince))))
		if !st.BackAt.IsZero() && now.Sub(st.BackAt) <= 2*time.Hour && st.AwayFor >= desktop.ReturnAfter {
			out = append(out, fmt.Sprintf("- Back at %s after %s away.", st.BackAt.Format("15:04"), span(st.AwayFor)))
		}
	default:
		out = append(out, "- Away since "+st.AwaySince.Format("15:04")+": locked, idle, or the computer asleep.")
	}
	if l := p.reach(mateID, st, now).line(now); l != "" {
		out = append(out, l)
	}
	if !st.Known {
		return out
	}
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if ss, err := p.desk.Sessions(midnight, now); err == nil && len(ss) > 0 {
		apps := desktop.Totals(ss)
		var parts []string
		for _, a := range apps[:min(len(apps), 5)] {
			parts = append(parts, a.App+" "+span(a.Time))
		}
		out = append(out, "- Today: "+strings.Join(parts, ", ")+".")
	}
	if ss, err := p.desk.Sessions(now.Add(-2*time.Hour), now); err == nil && len(ss) > 0 {
		var parts []string
		for _, s := range desktop.Timeline(ss, 8) {
			parts = append(parts, fmt.Sprintf("%s %s–%s", s.App, s.Start.Format("15:04"), s.End.Format("15:04")))
		}
		out = append(out, "- Lately: "+strings.Join(parts, ", ")+".")
	}
	return out
}

// botLines is one line per other enabled bot: where to start looking.
func (p *Pulse) botLines(self string) []string {
	bots, err := p.store.ListMates()
	if err != nil {
		return nil
	}
	var out []string
	for _, b := range bots {
		if b.ID == self || !b.Enabled {
			continue
		}
		line := fmt.Sprintf("- %s (`%s`)", b.Name, b.ID)
		if b.Description != "" {
			line += ", " + excerpt(b.Description)
		}
		var state []string
		if b.Unread > 0 {
			state = append(state, fmt.Sprintf("%d unread", b.Unread))
		}
		if !b.LastActiveAt.IsZero() {
			state = append(state, "last active "+b.LastActiveAt.Format("Mon 15:04"))
		}
		if len(state) > 0 {
			line += ": " + strings.Join(state, ", ")
		}
		out = append(out, line)
	}
	return out
}

func (p *Pulse) sentLines(mateID string, now time.Time) []string {
	sent, err := p.store.SentWakes(mateID, now.Add(-sentWithin), now)
	if err != nil {
		return nil
	}
	var out []string
	for _, w := range sent {
		from := "your rounds"
		if w.Event != string(mate.MateEventPulse) {
			from = "a trigger"
			if w.TriggerName != "" {
				from = fmt.Sprintf("trigger %q", w.TriggerName)
			}
		}
		took := "not answered"
		switch {
		case w.Replied:
			took = "answered"
		case w.ReplyPending:
			took = "no answer yet"
		}
		out = append(out, fmt.Sprintf("- %s, from %s (%s): %s", w.At.Format("Mon 15:04"), from, took, excerpt(w.Text)))
	}
	return out
}

func (p *Pulse) roundLines(mateID string, now time.Time) []string {
	rounds, err := p.store.ListPulses(mateID, digestRounds+1)
	if err != nil {
		return nil
	}
	var out []string
	for _, r := range rounds {
		if r.Decision == "" {
			continue // this one
		}
		line := fmt.Sprintf("- %s, %s", ago(now.Sub(r.At)), r.Decision)
		switch r.Decision {
		case mate.PulseSilent:
			line += ": " + excerpt(r.Summary)
		case mate.PulseSpeak:
			line += ": " + excerpt(r.Brief)
			if len(r.Nudged) > 0 {
				line += " (nudged " + strings.Join(slices.Sorted(maps.Keys(r.Nudged)), ", ") + ")"
			}
			if r.Delivery != "" {
				line += " → " + r.Delivery
			}
		}
		out = append(out, line)
	}
	return out
}

func excerpt(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > excerptLen {
		return string(r[:excerptLen]) + "…"
	}
	return s
}

func ago(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh %dm ago", int(d.Hours()), int(d.Minutes())%60)
	}
	return fmt.Sprintf("%d days ago", int(d.Hours()/24))
}
