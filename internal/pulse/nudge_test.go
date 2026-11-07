package pulse

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/hardhacker/hylo/internal/mate"
	"github.com/hardhacker/hylo/internal/prompt"
	"github.com/hardhacker/hylo/internal/storage"
)

func daysAgo(now time.Time, n int) time.Time {
	return dayStart(now).AddDate(0, 0, -n).Add(12 * time.Hour)
}

func aroundOn(now time.Time, days ...int) map[string]bool {
	m := map[string]bool{}
	for _, d := range days {
		m[daysAgo(now, d).Format(time.DateOnly)] = true
	}
	return m
}

func TestMeasureCountsDaysAround(t *testing.T) {
	now := at(10, 0)
	from := daysAgo(now, 20)
	around := aroundOn(now, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 0)
	done := []time.Time{daysAgo(now, 12), daysAgo(now, 11), daysAgo(now, 10), daysAgo(now, 8), daysAgo(now, 6)}
	r := measure(done, around, from, now, 2)
	if r.usual != 2 || r.gap != 5 || r.need != 4 || !r.last.Equal(daysAgo(now, 6)) {
		t.Fatalf("rhythm = %+v", r)
	}
	// Days away don't count, and doing it today closes the gap.
	r = measure(done, aroundOn(now, 12, 11, 10, 9, 8, 7, 6, 1), from, now, 2)
	if r.gap != 1 {
		t.Fatalf("gap counted days away: %+v", r)
	}
	r = measure(append(done, now), around, from, now, 2)
	if r.gap != 0 {
		t.Fatalf("today's write left a gap: %+v", r)
	}
	if r := measure(nil, around, from, now, 4); r.gap != 12 || r.usual != 0 || r.need != 4 {
		t.Fatalf("never done = %+v", r)
	}
}

func TestPacedBacksOffAndHolds(t *testing.T) {
	now := at(10, 0)
	raise := func(days int) mate.Nudge { return mate.Nudge{At: daysAgo(now, days)} }
	if ok, _ := paced(nil, time.Time{}, now); !ok {
		t.Fatal("never raised, held")
	}
	if ok, _ := paced([]mate.Nudge{raise(2)}, time.Time{}, now); ok {
		t.Fatal("raised again before its wait")
	}
	if ok, _ := paced([]mate.Nudge{raise(3)}, time.Time{}, now); !ok {
		t.Fatal("held past its wait")
	}
	// A second raise the user let pass doubles the wait to 6 days.
	if ok, _ := paced([]mate.Nudge{raise(9), raise(5)}, time.Time{}, now); ok {
		t.Fatal("second raise didn't double the wait")
	}
	three := []mate.Nudge{raise(20), raise(15), raise(8)}
	if ok, why := paced(three, time.Time{}, now); ok || why != "raised 3 times since they last did it" {
		t.Fatalf("three raises: %v %q", ok, why)
	}
	// Doing the thing starts the pacing over.
	if ok, _ := paced(three, daysAgo(now, 7), now); !ok {
		t.Fatal("done since, still held")
	}
}

func TestNudgedLine(t *testing.T) {
	live := []liveNudge{{name: "writing"}, {name: "todos", items: []string{"/a.md: call"}}}
	brief, got := nudged("Ask about the trip.\n\n[nudged: todos]\n", live)
	if brief != "Ask about the trip." || len(got) != 1 || got["todos"][0] != "/a.md: call" {
		t.Fatalf("brief %q nudged %v", brief, got)
	}
	if brief, got := nudged("Tell them.", live); brief != "Tell them." || got != nil {
		t.Fatalf("no line: %q %v", brief, got)
	}
	if _, got := nudged("Tell them.\n[nudged: reading]", live); got != nil {
		t.Fatalf("recorded a nudge that wasn't live: %v", got)
	}
}

func TestEveryNudgeHasItsText(t *testing.T) {
	for _, l := range leads {
		if l.nudge && strings.TrimSpace(prompt.Nudge(l.name)) == "" {
			t.Errorf("nudge %s has no text", l.name)
		}
	}
}

func (r *rig) nudges() []NudgeState {
	settled, _ := r.store.PulseSettled(r.bot.ID)
	return r.survey(r.scene(r.bot, settled, r.clock)).states
}

// A user who wrote every other day and then stopped, but kept reading.
func (r *rig) quietWriter(t *testing.T) {
	t.Helper()
	for d := 12; d >= 1; d-- {
		if err := r.store.RecordUserVisit("/journal.md", mate.NoteAccessRead, daysAgo(r.clock, d)); err != nil {
			t.Fatal(err)
		}
	}
	for _, d := range []int{12, 10, 8} {
		if err := r.store.RecordUserVisit("/journal.md", mate.NoteAccessWrite, daysAgo(r.clock, d)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestWritingNudgeIsRaisedOnce(t *testing.T) {
	r := newRig(t)
	r.quietWriter(t)
	r.odds = 0
	why, _ := r.weighs(t)
	if !strings.Contains(why, "the user hasn't written anything in 7 days they were around") {
		t.Fatalf("why = %q, nudges %+v", why, r.nudges())
	}
	d, live := r.digest(r.bot, time.Time{}, time.Time{}, r.clock)
	if len(live) != 1 || !strings.Contains(d, "### writing") || !strings.Contains(d, prompt.Nudge("writing")) {
		t.Fatalf("digest lacks the nudge (%v):\n%s", live, d)
	}

	r.reply = mate.RunResult{Success: true, LastMessage: "Ask how the launch went; keep the answer as a short.\n[nudged: writing]"}
	r.run(context.Background(), r.bot, "test", nil)
	ps, _ := r.store.ListPulses(r.bot.ID, 1)
	if _, ok := ps[0].Nudged["writing"]; !ok || strings.Contains(ps[0].Brief, "[nudged") {
		t.Fatalf("round = %+v", ps[0])
	}
	if !strings.Contains(r.woken[0].Prompt, "end on the one question") {
		t.Fatalf("wake doesn't let it ask: %s", r.woken[0].Prompt)
	}
	r.WakeEnded(r.woken[0].Ref, "succeeded", false)

	r.clock = r.clock.Add(time.Hour)
	if st := r.nudges()[0]; st.Live || st.State != "a nudge already went out today" {
		t.Fatalf("raised twice in a day: %+v", r.nudges())
	}
	r.clock = r.clock.Add(24 * time.Hour)
	if st := r.nudges()[0]; st.Live || !strings.HasPrefix(st.State, "raised ") {
		t.Fatalf("raised again the next day: %+v", r.nudges())
	}
}

func TestMutedNudge(t *testing.T) {
	r := newRig(t)
	r.quietWriter(t)
	r.cfg.Mute = []string{"writing"}
	r.odds = 0
	if why, _ := r.weighs(t); strings.Contains(why, "written") || r.nudges()[0].State != "muted" {
		t.Fatalf("muted nudge raised: %q %+v", why, r.nudges())
	}
}

func TestTodosNudgeSkipsRaisedOnes(t *testing.T) {
	r := newRig(t)
	stale := r.clock.AddDate(0, 0, -30)
	r.tasks = func(from, to time.Time) ([]storage.Task, error) {
		return []storage.Task{
			{Path: "/trip.md", Line: 3, Text: "book the train", Updated: stale},
			{Path: "/house.md", Line: 8, Text: "fix the tap", Updated: stale},
		}, nil
	}
	d, live := r.digest(r.bot, time.Time{}, time.Time{}, r.clock)
	if len(live) != 1 || live[0].name != "todos" || len(live[0].items) != 2 || !strings.Contains(d, "/trip.md, line 3") {
		t.Fatalf("todos = %+v\n%s", live, d)
	}
	pr, _ := r.store.StartPulse(r.bot.ID, "x", r.clock.AddDate(0, 0, -10))
	pr.Decision, pr.Nudged = mate.PulseSpeak, map[string][]string{"todos": {"/trip.md: book the train"}}
	if err := r.store.EndPulse(pr); err != nil {
		t.Fatal(err)
	}
	_ = r.store.SetPulseDelivery(pr.ID, "sent")
	r.looked = map[string]looked{}
	f := todosNudge.look(r.scene(r.bot, time.Time{}, r.clock))
	if len(f.items) != 1 || f.items[0] != "/house.md: fix the tap" {
		t.Fatalf("raised to-do came back: %v", f.items)
	}
}
