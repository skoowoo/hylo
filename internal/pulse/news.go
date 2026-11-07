package pulse

import (
	"fmt"
	"slices"
	"time"

	"github.com/hardhacker/hylo/internal/desktop"
	"github.com/hardhacker/hylo/internal/mate"
)

// News leads: what happened since the bot last looked and acted.

var shortsLead = lead{
	name: "shorts", title: "Short notes written since then", empty: "None.",
	look: func(s *scene) finding {
		var f finding
		n := 0
		for _, c := range s.changed {
			if c.Short != "" && s.fresh(c.At) {
				f.pull += fade(shortWeight, c.At, s.now)
				n++
			}
		}
		if n > 0 {
			was := " was written"
			if n > 1 {
				was = " were written"
			}
			f.because = count(n, "short note") + was
		}
		f.lines = shortLines(changedSince(s.changed, s.digestSince()))
		return f
	},
}

var notesLead = lead{
	name: "notes", title: "Notes changed since then", empty: "None.",
	look: func(s *scene) finding {
		var f finding
		notes := freshNotes(s)
		for _, at := range notes {
			f.pull += fade(writeWeight, at, s.now)
		}
		if len(notes) > 0 {
			f.because = count(len(notes), "note") + " changed"
		}
		f.lines = changedLines(changedSince(s.changed, s.digestSince()))
		return f
	},
}

// readsLead is the notes the user read that didn't change; a change says more.
var readsLead = lead{
	name: "reads", title: "Notes the user read since then", empty: "None.",
	look: func(s *scene) finding {
		var f finding
		notes := freshNotes(s)
		reads := map[string]time.Time{}
		for _, a := range s.user {
			if a.Action == mate.NoteAccessRead && s.fresh(a.At) && notes[a.Path].IsZero() && a.At.After(reads[a.Path]) {
				reads[a.Path] = a.At
			}
		}
		for _, at := range reads {
			f.pull += fade(readWeight, at, s.now)
		}
		if len(reads) > 0 {
			f.because = "the user read " + count(len(reads), "note")
		}
		f.lines = readLines(s)
		return f
	},
}

// returnLead is the user coming back. The desktop tells a return exactly;
// without it, a long gap in activity stands in. Picking up the phone while
// away is a return of its own.
var returnLead = lead{
	name: "return",
	look: func(s *scene) finding {
		r, desk, now := s.reach, s.desk, s.now
		switch {
		case r.Where == reachPhone && r.AppAt.After(s.since):
			return finding{pull: returnWeight, because: "the user is on their phone, writing from " + appName(r.App)}
		case desk.Known:
			if desk.Present && now.Sub(desk.BackAt) <= backWithin && desk.BackAt.After(s.since) && desk.AwayFor >= desktop.ReturnAfter {
				return finding{pull: returnWeight, because: "the user is back at the computer after " + span(desk.AwayFor) + " away"}
			}
			return finding{}
		}
		var times []time.Time
		for _, a := range s.user {
			if now.Sub(a.At) <= signalLookback {
				times = append(times, a.At)
			}
		}
		if now.Sub(s.lastChat) <= signalLookback {
			times = append(times, s.lastChat)
		}
		slices.SortFunc(times, time.Time.Compare)
		if back := sessionStart(times); !back.IsZero() && now.Sub(back) <= backWithin && back.After(s.since) {
			return finding{pull: returnWeight, because: "the user is back after a while away"}
		}
		return finding{}
	},
}

// freshNotes are the notes (not shorts) that changed as news that still pulls.
func freshNotes(s *scene) map[string]time.Time {
	notes := map[string]time.Time{}
	for _, c := range s.changed {
		if c.Short == "" && s.fresh(c.At) {
			notes[c.Path] = c.At
		}
	}
	return notes
}

func changedSince(changed []change, since time.Time) []change {
	var out []change
	for _, c := range changed {
		if c.At.After(since) {
			out = append(out, c)
		}
	}
	return out
}

// sessionStart is when the user's current stretch of activity began: the first
// time after a gap of awayGap, or the first time seen. times is sorted.
func sessionStart(times []time.Time) time.Time {
	if len(times) == 0 {
		return time.Time{}
	}
	start := times[0]
	for i := 1; i < len(times); i++ {
		if times[i].Sub(times[i-1]) >= awayGap {
			start = times[i]
		}
	}
	return start
}

// readLines lists the notes the user read since then and that didn't change, newest first.
func readLines(s *scene) []string {
	since := s.digestSince()
	skip := map[string]bool{}
	for _, c := range changedSince(s.changed, since) {
		skip[c.Path] = true
	}
	latest := map[string]time.Time{}
	var paths []string
	for _, a := range s.user {
		if a.Action == mate.NoteAccessRead && a.At.After(since) && !skip[a.Path] && latest[a.Path].IsZero() {
			latest[a.Path] = a.At
			paths = append(paths, a.Path)
		}
	}
	var out []string
	for i, path := range paths {
		if i == digestNotes {
			out = append(out, fmt.Sprintf("- …and %d more", len(paths)-digestNotes))
			break
		}
		out = append(out, fmt.Sprintf("- %s, %s", path, latest[path].Format("15:04")))
	}
	return out
}
