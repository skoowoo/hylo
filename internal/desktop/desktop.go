// Package desktop keeps what the desktop app reports about the user at the
// computer: whether they are there, and which app is in front, by name only.
// Samples arrive every 30s; stretches of one app become sessions in the store.
package desktop

import (
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/hardhacker/hylo/internal/mate"
)

// Sample is one report from the desktop app.
type Sample struct {
	App    string `json:"app"`
	Idle   int    `json:"idle"` // seconds since the last keyboard or mouse input
	Locked bool   `json:"locked"`
}

const (
	// idleAway is how long without input counts as away.
	idleAway = 5 * time.Minute
	// sampleGap is past the 30s cadence: a longer silence is the app gone or the computer asleep.
	sampleGap = 90 * time.Second
	// ReturnAfter is the shortest absence that counts as coming back: a meal
	// or a meeting, not a dropped sample or a short lock. A shorter gap only
	// splits the session.
	ReturnAfter = time.Hour
	// knownFor bounds how long a silent desktop still reads as the user away, not unknown.
	knownFor = 12 * time.Hour
)

// Tracker follows the user at the computer.
type Tracker struct {
	store   *mate.Store
	enabled bool

	mu sync.Mutex
	seen
}

// seen is what the tracker made of the samples so far.
type seen struct {
	lastSample  time.Time
	present     bool
	app         string
	appSince    time.Time
	lastPresent time.Time
	backAt      time.Time
	awayFor     time.Duration
}

// New makes a Tracker. A disabled one drops every report and knows nothing.
func New(store *mate.Store, enabled bool) *Tracker {
	t := &Tracker{store: store, enabled: enabled}
	if enabled {
		t.lastPresent, _ = store.LastDesktop()
	}
	return t
}

// Enabled reports whether reports are kept.
func (t *Tracker) Enabled() bool { return t != nil && t.enabled }

// Report takes a sample taken at now.
func (t *Tracker) Report(s Sample, now time.Time) error {
	if !t.Enabled() {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.lastSample = now
	if s.Locked || time.Duration(s.Idle)*time.Second >= idleAway || s.App == "" {
		t.present = false
		return nil
	}
	// A short gap is a broken session, not a return, so it must not replace one.
	if gone := now.Sub(t.lastPresent); !t.lastPresent.IsZero() && gone >= ReturnAfter {
		t.backAt, t.awayFor = now, gone
	}
	if !t.present || s.App != t.app || now.Sub(t.lastPresent) > sampleGap {
		t.appSince = now
	}
	t.present, t.lastPresent = true, now
	t.app = s.App
	// Under the lock, so a Clear can't land between the state and its session.
	return t.store.TouchDesktop(s.App, now, sampleGap)
}

// Clear forgets every session and what the tracker made of them.
func (t *Tracker) Clear() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if err := t.store.ClearDesktop(); err != nil {
		return err
	}
	t.seen = seen{}
	return nil
}

// State is the user at the computer now.
type State struct {
	Known     bool          `json:"known"` // the desktop app has reported lately
	Present   bool          `json:"present"`
	App       string        `json:"app,omitempty"`
	AppSince  time.Time     `json:"appSince,omitzero"`
	AwaySince time.Time     `json:"awaySince,omitzero"` // when not present: last seen
	BackAt    time.Time     `json:"backAt,omitzero"`    // the latest return after ReturnAfter or more away
	AwayFor   time.Duration `json:"-"`                  // how long that absence was
}

// Now reports the state at now.
func (t *Tracker) Now(now time.Time) State {
	if !t.Enabled() {
		return State{}
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	st := State{BackAt: t.backAt, AwayFor: t.awayFor}
	if t.present && now.Sub(t.lastSample) <= sampleGap {
		st.Known, st.Present = true, true
		st.App, st.AppSince = t.app, t.appSince
		return st
	}
	if !t.lastPresent.IsZero() && now.Sub(t.lastPresent) <= knownFor {
		st.Known, st.AwaySince = true, t.lastPresent
	}
	return st
}

// Sessions lists the sessions overlapping [from, to], clipped to it.
func (t *Tracker) Sessions(from, to time.Time) ([]mate.DesktopSession, error) {
	if !t.Enabled() {
		return nil, nil
	}
	ss, err := t.store.DesktopSessions(from, to)
	for i := range ss {
		ss[i].Start = maxTime(ss[i].Start, from)
		ss[i].End = minTime(ss[i].End, to)
	}
	return ss, err
}

// Total is time spent in one app.
type Total struct {
	App     string        `json:"app"`
	Time    time.Duration `json:"-"`
	Minutes int           `json:"minutes"`
}

// Totals sums sessions by app, longest first.
func Totals(ss []mate.DesktopSession) []Total {
	apps := map[string]time.Duration{}
	for _, s := range ss {
		apps[s.App] += s.End.Sub(s.Start)
	}
	out := make([]Total, 0, len(apps))
	for app, d := range apps {
		out = append(out, Total{App: app, Time: d, Minutes: int(d.Minutes())})
	}
	slices.SortFunc(out, func(a, b Total) int {
		if a.Time != b.Time {
			return int(b.Time - a.Time)
		}
		return strings.Compare(a.App, b.App)
	})
	return out
}

// ScreenNow is the user at the computer now, as a tool reads it.
type ScreenNow struct {
	Known        bool      `json:"known"`
	Present      bool      `json:"present"`
	App          string    `json:"app,omitempty"`
	InAppMinutes int       `json:"in_app_minutes,omitempty"`
	AwaySince    time.Time `json:"away_since,omitzero"`
	BackAt       time.Time `json:"back_at,omitzero"`
	AwayMinutes  int       `json:"away_minutes,omitempty"` // how long the absence before back_at was
}

// ScreenTime is where the user's time at the computer went.
type ScreenTime struct {
	Enabled  bool                  `json:"enabled"`
	Now      ScreenNow             `json:"now"`
	Apps     []Total               `json:"apps"`
	Timeline []mate.DesktopSession `json:"timeline"`
}

// Timeline joins back-to-back sessions of one app and keeps the last n,
// oldest first: a glance at what the user did.
func Timeline(ss []mate.DesktopSession, n int) []mate.DesktopSession {
	var out []mate.DesktopSession
	for _, s := range ss {
		if k := len(out); k > 0 && out[k-1].App == s.App && s.Start.Sub(out[k-1].End) <= sampleGap {
			out[k-1].End = s.End
			continue
		}
		out = append(out, s)
	}
	if len(out) > n {
		out = out[len(out)-n:]
	}
	return out
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}
