package pulse

import (
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/hardhacker/hylo/internal/mate"
	"github.com/hardhacker/hylo/internal/storage"
)

// Nudges: things the bot raises on its own when the user has gone a while
// without them. Days count only when the user was around (at the computer or
// in hylo), so time away is never a lapse.

const (
	nudgePull = 0.2
	// nudgeAgain is how many days a nudge waits after a raise, doubled for
	// each further raise the user let pass without doing the thing.
	nudgeAgain = 3
	// After holdAfter such raises it holds until they age out of nudgeMemory.
	holdAfter   = 3
	nudgeMemory = 30 * 24 * time.Hour
	// habitWindow is how many days back a habit's rhythm is read.
	habitWindow = 30
)

// paced reports whether a nudge may be raised now, given its raises over
// nudgeMemory and when the user last did the thing: doing it starts over.
func paced(raises []mate.Nudge, done, now time.Time) (bool, string) {
	n := 0
	var last time.Time
	for _, r := range raises {
		if r.At.After(done) {
			n++
			last = maxTime(last, r.At)
		}
	}
	if n == 0 {
		return true, ""
	}
	if n >= holdAfter {
		return false, fmt.Sprintf("raised %d times since they last did it", n)
	}
	wait := nudgeAgain << (n - 1)
	if days := int(math.Round(dayStart(now).Sub(dayStart(last)).Hours() / 24)); days < wait {
		return false, fmt.Sprintf("raised %s; waits %s", last.Format("Mon 01-02"), count(wait, "day"))
	}
	return true, ""
}

// rhythm is how often the user does something, counted in days around.
type rhythm struct {
	last  time.Time // when they last did it; zero for not in the window
	usual int       // the median gap between two days they did it; 0 when unknown
	gap   int       // days around since they last did, today left out
	need  int       // the gap that counts as long
}

// measure reads a rhythm from the times something was done since from.
// Doing it today closes the gap.
func measure(done []time.Time, around map[string]bool, from, now time.Time, floor int) rhythm {
	var r rhythm
	doneOn := map[string]bool{}
	for _, t := range done {
		doneOn[t.Format(time.DateOnly)] = true
		if t.After(r.last) {
			r.last = t
		}
	}
	var gaps []int
	n, seen := 0, false
	today := dayStart(now)
	for d := dayStart(from); d.Before(today); d = d.AddDate(0, 0, 1) {
		key := d.Format(time.DateOnly)
		switch {
		case doneOn[key]:
			if seen {
				gaps = append(gaps, n+1)
			}
			seen, n = true, 0
		case around[key]:
			n++
		}
	}
	if doneOn[today.Format(time.DateOnly)] {
		n = 0
	}
	r.gap = n
	if len(gaps) > 0 {
		slices.Sort(gaps)
		r.usual = gaps[len(gaps)/2]
	}
	r.need = max(floor, 2*r.usual)
	return r
}

// habit is a nudge for something the user does now and then: it is due once
// they have gone well past their own rhythm, and never under floor days around.
func habit(name string, floor int, times func(*mate.Store, time.Time) ([]time.Time, error), did, didnt string) lead {
	return lead{
		name: name, every: 15 * time.Minute, nudge: true,
		look: func(s *scene) finding {
			from := dayStart(s.now).AddDate(0, 0, -habitWindow)
			// Before tracking began nothing was recorded, which isn't a lapse.
			first, ok, err := s.p.store.FirstNoteActivity()
			if err != nil || !ok {
				return finding{}
			}
			from = maxTime(from, dayStart(first))
			done, err := times(s.p.store, from)
			if err != nil {
				s.p.logger.Warn("pulse: habit times", "habit", name, "err", err)
				return finding{}
			}
			r := measure(done, s.aroundDays(), from, s.now, floor)
			f := finding{due: r.gap >= r.need, done: r.last}
			f.because = fmt.Sprintf("the user hasn't %s in %d days they were around", didnt, r.gap)
			if r.last.IsZero() {
				f.lines = append(f.lines, fmt.Sprintf("- Last %s: not since %s.", did, from.Format("Mon 2006-01-02")))
			} else {
				f.lines = append(f.lines, fmt.Sprintf("- Last %s: %s, %s.", did, r.last.Format("Mon 2006-01-02 15:04"), ago(s.now.Sub(r.last))))
			}
			f.lines = append(f.lines, fmt.Sprintf("- Since then: %d days at the computer or in hylo without it, today left out.", r.gap))
			if r.usual > 0 {
				f.lines = append(f.lines, fmt.Sprintf("- Their rhythm over the last %d days: about every %s around.", habitWindow, count(r.usual, "day")))
			}
			return f
		},
	}
}

var writingNudge = habit("writing", 2, (*mate.Store).WriteTimes, "wrote anything", "written anything")

var readingNudge = habit("reading", 4, (*mate.Store).ReadTimes, "read a note", "read a note")

// Open to-dos untouched for todoStale, up to todoGiveUp: older ones are
// likely settled some other way.
const (
	todoStale  = 14 * 24 * time.Hour
	todoGiveUp = 180 * 24 * time.Hour
	todoShown  = 5
	// todoRaised keeps a to-do the bot already raised out of later raises.
	todoRaised = 90 * 24 * time.Hour
)

var todosNudge = lead{
	name: "todos", every: time.Hour, nudge: true,
	look: func(s *scene) finding {
		if s.p.tasks == nil {
			return finding{}
		}
		tasks, err := s.p.tasks(s.now.Add(-todoGiveUp), s.now.Add(-todoStale))
		if err != nil {
			s.p.logger.Warn("pulse: open tasks", "err", err)
			return finding{}
		}
		raised := map[string]bool{}
		if past, err := s.p.store.Nudges(s.m.ID, "todos", s.now.Add(-todoRaised)); err == nil {
			for _, r := range past {
				for _, it := range r.Items {
					raised[it] = true
				}
			}
		}
		var f finding
		left := 0
		for _, t := range tasks {
			key := todoKey(t)
			if raised[key] {
				continue
			}
			left++
			if len(f.items) < todoShown {
				f.items = append(f.items, key)
				f.lines = append(f.lines, fmt.Sprintf("- %s, line %d (note last changed %s): %s", t.Path, t.Line, ago(s.now.Sub(t.Updated)), excerpt(t.Text)))
			}
		}
		if left > len(f.items) {
			f.lines = append(f.lines, fmt.Sprintf("- …%d more", left-len(f.items)))
		}
		f.due = left > 0
		f.because = "there are " + count(left, "open to-do") + " untouched for weeks"
		return f
	},
}

func todoKey(t storage.Task) string { return t.Path + ": " + t.Text }

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
