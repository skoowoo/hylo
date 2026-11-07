package pulse

import (
	"testing"
	"time"

	"github.com/hardhacker/hylo/internal/desktop"
	"github.com/hardhacker/hylo/internal/mate"
)

func at(h, m int) time.Time { return time.Date(2026, 11, 6, h, m, 0, 0, time.Local) }

func TestWindow(t *testing.T) {
	day := day{start: 8 * 60, end: 24 * 60}
	if _, _, ok := day.window(at(7, 59)); ok {
		t.Fatal("awake before 08:00")
	}
	from, to, ok := day.window(at(23, 30))
	if !ok || !from.Equal(at(8, 0)) || !to.Equal(at(8, 0).Add(16*time.Hour)) {
		t.Fatalf("23:30 = %v %v %v", from, to, ok)
	}
	late := day2(7*60, 60)
	from, _, ok = late.window(at(0, 30))
	if !ok || !from.Equal(at(7, 0).AddDate(0, 0, -1)) {
		t.Fatalf("past midnight belongs to the day before: %v %v", from, ok)
	}
	if _, _, ok := late.window(at(3, 0)); ok {
		t.Fatal("awake at 03:00")
	}
}

func day2(start, end int) day { return day{start, end} }

func TestChanceSpreadsWhatIsLeft(t *testing.T) {
	if c := chance(16, 16*time.Hour, 0); c < 0.0166 || c > 0.0167 {
		t.Fatalf("16 left over 16h = %v, want one an hour", c)
	}
	if chance(0, time.Hour, 3) != 0 {
		t.Fatal("rounds past the budget")
	}
	if chance(5, 30*time.Second, 0) != 1 {
		t.Fatal("chance above 1")
	}
	if a, b := chance(4, 4*time.Hour, 0), chance(4, 4*time.Hour, 1); b != 2*a {
		t.Fatalf("boost 1 doubles the odds: %v %v", a, b)
	}
}

// signals is what the leads find in a moment, as weigh sees it.
func signals(t *testing.T, changed []change, user []mate.NoteActivity, desk desktop.State, rc reach, lastChat, settled, now time.Time) (float64, string) {
	t.Helper()
	r := newRig(t)
	sv := r.survey(&scene{p: r.Pulse, m: r.bot, now: now, since: settled, desk: desk, reach: rc, changed: changed, user: user, lastChat: lastChat})
	return sv.boost, sv.why()
}

func TestSignals(t *testing.T) {
	now := at(15, 0)
	settled := at(14, 0)
	changed := []change{
		{Path: "/old.md", At: at(13, 0)}, // the bot's own doing, before it settled
		{Path: "/a.md", At: at(14, 50)},
		{Path: "/_shorts/2026-11-06.md", Short: "s1", New: true, At: at(14, 55)},
		{Path: "/_shorts/2026-11-06.md", Short: "s2", New: true, At: at(14, 56)},
	}
	user := []mate.NoteActivity{
		{Path: "/earlier.md", Action: mate.NoteAccessRead, At: at(12, 0)},
		{Path: "/a.md", Action: mate.NoteAccessRead, At: at(14, 40)}, // changed too: counted once
		{Path: "/b.md", Action: mate.NoteAccessRead, At: at(14, 58)},
	}
	boost, why := signals(t, changed, user, desktop.State{}, reach{}, time.Time{}, settled, now)
	if why != "2 short notes were written, 1 note changed and the user read 1 note" {
		t.Fatalf("why = %q", why)
	}
	if boost < 2*shortWeight || boost > 2*shortWeight+writeWeight+readWeight {
		t.Fatalf("boost = %v", boost)
	}
	if b, why := signals(t, changed[:1], nil, desktop.State{}, reach{}, time.Time{}, settled, now); b != 0 || why != "" {
		t.Fatalf("its own writes woke it: %v %q", b, why)
	}
}

func TestSignalsBackAfterAway(t *testing.T) {
	now := at(15, 0)
	back := []mate.NoteActivity{{Path: "/a.md", Action: mate.NoteAccessRead, At: at(14, 50)}}
	_, why := signals(t, nil, back, desktop.State{}, reach{}, at(9, 0), at(12, 0), now)
	if why != "the user read 1 note and the user is back after a while away" {
		t.Fatalf("why = %q", why)
	}
	steady := append([]mate.NoteActivity{{Path: "/c.md", Action: mate.NoteAccessRead, At: at(13, 0)}}, back...)
	if _, why := signals(t, nil, steady, desktop.State{}, reach{}, time.Time{}, at(12, 0), now); why != "the user read 2 notes" {
		t.Fatalf("steady why = %q", why)
	}
	if _, why := signals(t, nil, back, desktop.State{}, reach{}, time.Time{}, at(14, 55), now); why != "" {
		t.Fatalf("seen return why = %q", why)
	}
}

func TestSignalsFromTheDesktop(t *testing.T) {
	now := at(15, 0)
	back := desktop.State{Known: true, Present: true, BackAt: at(14, 50), AwayFor: 2*time.Hour + 10*time.Minute}
	if _, why := signals(t, nil, nil, back, reach{}, time.Time{}, at(12, 0), now); why != "the user is back at the computer after 2h 10m away" {
		t.Fatalf("why = %q", why)
	}
	coffee := back
	coffee.AwayFor = 10 * time.Minute
	if _, why := signals(t, nil, nil, coffee, reach{}, time.Time{}, at(12, 0), now); why != "" {
		t.Fatalf("a short break counted: %q", why)
	}
	// With the desktop known, a gap in note activity is no return.
	notes := []mate.NoteActivity{{Path: "/a.md", Action: mate.NoteAccessRead, At: at(14, 50)}}
	if _, why := signals(t, nil, notes, desktop.State{Known: true, Present: true}, reach{}, time.Time{}, at(12, 0), now); why != "the user read 1 note" {
		t.Fatalf("why = %q", why)
	}
}
