package pulse

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/hardhacker/hylo/internal/desktop"
	"github.com/hardhacker/hylo/internal/mate"
)

// A lead is one kind of thing rounds keep an eye on. Every minute each lead
// looks at the moment: what it finds can raise the odds of a round, says why
// the round woke, and gives the round a section of its digest. A new lead is
// a look func and an entry in leads; a nudge also needs its text in
// prompt/text/nudge_<name>.md.
type lead struct {
	name  string
	title string        // its digest section; empty for none
	empty string        // the section when nothing turned up
	every time.Duration // how long a look holds; 0 looks every minute
	look  func(s *scene) finding
	// nudge marks something the bot raises on its own, with nothing new to
	// prompt it: worth saying once in a while, never nagging (nudge.go).
	nudge bool
}

type finding struct {
	pull    float64  // added to the odds' boost
	because string   // why a round woke, as a clause
	lines   []string // the digest section
	// Nudges only.
	due   bool      // worth raising, pacing aside
	items []string  // what a raise covers, so it isn't raised again
	done  time.Time // when the user last did the thing: pacing starts over
}

var leads = []lead{shortsLead, notesLead, readsLead, returnLead, writingNudge, readingNudge, todosNudge}

// scene is the moment the leads look at, gathered once.
type scene struct {
	p     *Pulse
	m     *mate.Mate
	now   time.Time
	since time.Time // when the bot last settled: what came after is news
	desk  desktop.State
	reach reach
	// changed and user cover sceneLookback.
	changed  []change
	user     []mate.NoteActivity
	lastChat time.Time

	around map[string]bool // see aroundDays
}

// sceneLookback reaches as far as the signals and the digest both need.
const sceneLookback = max(signalLookback, digestWindow)

func (p *Pulse) scene(m *mate.Mate, since, now time.Time) *scene {
	s := &scene{p: p, m: m, now: now, since: since, desk: p.desk.Now(now)}
	s.reach = p.reach(m.ID, s.desk, now)
	s.changed = p.changes.since(now.Add(-sceneLookback))
	var err error
	if s.user, err = p.store.RecentNoteActivity(now.Add(-sceneLookback), now.Add(time.Second), mate.ActorUser, 0); err != nil {
		p.logger.Warn("pulse: user activity", "err", err)
	}
	s.lastChat, _ = p.store.LastUserMessageAt(m.ID)
	return s
}

// fresh reports whether t is news that still pulls: after the bot settled,
// within signalWindow.
func (s *scene) fresh(t time.Time) bool { return t.After(s.since) && s.now.Sub(t) <= signalWindow }

// digestSince is where the digest's "since then" starts.
func (s *scene) digestSince() time.Time {
	if s.since.IsZero() || s.now.Sub(s.since) > digestWindow {
		return s.now.Add(-digestWindow)
	}
	return s.since
}

// aroundDays are the local days, over habitWindow, the user was around.
func (s *scene) aroundDays() map[string]bool {
	if s.around == nil {
		var err error
		if s.around, err = s.p.store.AroundDays(dayStart(s.now).AddDate(0, 0, -habitWindow)); err != nil {
			s.p.logger.Warn("pulse: days around", "err", err)
			s.around = map[string]bool{}
		}
	}
	return s.around
}

// survey is what every lead found at one moment.
type survey struct {
	boost    float64
	because  []string
	sections []section
	nudges   []liveNudge // nudges a round may raise now
	states   []NudgeState
}

type section struct {
	title, empty string
	lines        []string
}

type liveNudge struct {
	name  string
	lines []string
	items []string
}

// NudgeState is where one nudge stands, for the dev page.
type NudgeState struct {
	Name  string `json:"name"`
	Live  bool   `json:"live"`
	State string `json:"state"`
}

type looked struct {
	at time.Time
	f  finding
}

func (p *Pulse) survey(s *scene) survey {
	var sv survey
	nudgedToday, err := p.store.NudgedSince(s.m.ID, dayStart(s.now))
	if err != nil {
		p.logger.Warn("pulse: nudged today", "err", err)
	}
	for _, l := range leads {
		if !l.nudge {
			f := p.look(l, s)
			sv.boost += f.pull
			if f.because != "" {
				sv.because = append(sv.because, f.because)
			}
			if l.title != "" {
				sv.sections = append(sv.sections, section{l.title, l.empty, f.lines})
			}
			continue
		}
		st := NudgeState{Name: l.name}
		if slices.Contains(p.cfg.Mute, l.name) {
			st.State = "muted"
			sv.states = append(sv.states, st)
			continue
		}
		f := p.look(l, s)
		switch {
		case !f.due:
			st.State = "nothing to raise"
		case nudgedToday:
			st.State = "a nudge already went out today"
		default:
			raises, err := p.store.Nudges(s.m.ID, l.name, s.now.Add(-nudgeMemory))
			if err != nil {
				p.logger.Warn("pulse: nudges", "nudge", l.name, "err", err)
				st.State = "can't tell when it was last raised"
				break
			}
			if ok, why := paced(raises, f.done, s.now); !ok {
				st.State = why
				break
			}
			st.Live, st.State = true, f.because
			sv.boost += nudgePull
			sv.because = append(sv.because, f.because)
			sv.nudges = append(sv.nudges, liveNudge{name: l.name, lines: f.lines, items: f.items})
		}
		sv.states = append(sv.states, st)
	}
	sv.boost = min(sv.boost, maxBoost)
	return sv
}

// look is l's finding now, or a recent one while it holds.
func (p *Pulse) look(l lead, s *scene) finding {
	if l.every > 0 {
		p.mu.Lock()
		got, ok := p.looked[l.name]
		p.mu.Unlock()
		if ok && s.now.Sub(got.at) < l.every && !s.now.Before(got.at) {
			return got.f
		}
	}
	f := l.look(s)
	if l.every > 0 {
		p.mu.Lock()
		p.looked[l.name] = looked{s.now, f}
		p.mu.Unlock()
	}
	return f
}

// why says what woke a round, from what the leads found.
func (sv survey) why() string {
	if len(sv.because) == 0 {
		return ""
	}
	return listed(sv.because)
}

// listed joins clauses the way a sentence does: "a, b and c".
func listed(parts []string) string {
	if len(parts) == 1 {
		return parts[0]
	}
	last := len(parts) - 1
	return strings.Join(parts[:last], ", ") + " and " + parts[last]
}

func dayStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func count(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
