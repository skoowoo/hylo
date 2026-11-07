package pulse

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hardhacker/hylo/internal/config"
	"github.com/hardhacker/hylo/internal/desktop"
	"github.com/hardhacker/hylo/internal/mate"
	"github.com/hardhacker/hylo/internal/plugin"
	"github.com/hardhacker/hylo/internal/prompt"
)

type rig struct {
	*Pulse
	desk    *desktop.Tracker
	push    []string
	store   *mate.Store
	bot     *mate.Mate
	clock   time.Time
	odds    float64
	busy    bool
	reply   mate.RunResult
	asked   []string
	woken   []mate.Wake
	wakeErr error
}

func newRig(t *testing.T) *rig {
	t.Helper()
	s, err := mate.Open(filepath.Join(t.TempDir(), "vault"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	bot := &mate.Mate{Name: "Hylo", Main: true, Enabled: true}
	if err := s.CreateMate(bot); err != nil {
		t.Fatal(err)
	}
	r := &rig{store: s, bot: bot, clock: at(10, 0), odds: 0.99}
	cfg := config.ProactiveConfig{Enabled: true, WakesPerDay: 24, ActiveHours: "08:00-24:00", MinGap: "20m"}
	r.desk = desktop.New(s, true)
	p, err := New(Deps{
		Store: s, Desk: r.desk,
		Round: func(_ context.Context, _ *mate.Mate, req, _ string) mate.RunResult {
			r.asked = append(r.asked, req)
			return r.reply
		},
		Wake:     func(_ *mate.Mate, w mate.Wake) error { r.woken = append(r.woken, w); return r.wakeErr },
		Idle:     func(string) bool { return !r.busy },
		Pushable: func(string) []string { return r.push },
	}, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	p.now = func() time.Time { return r.clock }
	p.roll = func() float64 { return r.odds }
	r.Pulse = p
	return r
}

func (r *rig) weighs(t *testing.T) (string, []mate.AgendaItem) {
	t.Helper()
	m, why, due := r.weigh()
	if m == nil {
		return "", nil
	}
	return why, due
}

func (r *rig) round(t *testing.T) mate.PulseRun {
	t.Helper()
	r.run(context.Background(), r.bot, "test", nil)
	ps, err := r.store.ListPulses(r.bot.ID, 1)
	if err != nil || len(ps) != 1 {
		t.Fatalf("journal: %v %v", ps, err)
	}
	return ps[0]
}

func TestWeighOdds(t *testing.T) {
	r := newRig(t)
	if why, _ := r.weighs(t); why != "" {
		t.Fatalf("a 0.99 roll started a round: %q", why)
	}
	r.odds = 0
	if why, _ := r.weighs(t); why == "" {
		t.Fatal("a 0 roll started no round")
	}
	if r.last.Chance <= 0 || r.last.Skip != "" {
		t.Fatalf("odds = %+v", r.last)
	}

	r.clock = at(6, 0)
	if why, _ := r.weighs(t); why != "" || r.last.Skip != "asleep" {
		t.Fatalf("woke at night: %q %+v", why, r.last)
	}
	r.clock = at(10, 0)
	r.busy = true
	if why, _ := r.weighs(t); why != "" || r.last.Skip != "the chat is in use" {
		t.Fatalf("woke over the chat: %q %+v", why, r.last)
	}
}

func TestWeighGapAndBudget(t *testing.T) {
	r := newRig(t)
	r.odds = 0
	if _, err := r.store.StartPulse(r.bot.ID, "x", r.clock.Add(-10*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if why, _ := r.weighs(t); why != "" || !strings.HasPrefix(r.last.Skip, "last round") {
		t.Fatalf("inside the gap: %q %+v", why, r.last)
	}
	r.cfg.WakesPerDay = 1
	r.clock = r.clock.Add(time.Hour)
	if why, _ := r.weighs(t); why != "" || r.last.Skip != "no rounds left today" {
		t.Fatalf("past the budget: %q %+v", why, r.last)
	}
}

func TestDueAgendaWakesOnceOutsideTheOdds(t *testing.T) {
	r := newRig(t)
	r.cfg.WakesPerDay = 0
	if _, err := r.store.AddAgenda(r.bot.ID, "ask about the trip", r.clock.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	why, due := r.weighs(t)
	if len(due) != 1 || !strings.Contains(why, "ask about the trip") {
		t.Fatalf("due item did not wake the bot: %q %v", why, due)
	}
	r.reply = mate.RunResult{Success: true, LastMessage: "Asked already.\n[silent]"}
	r.run(context.Background(), r.bot, why, due)
	if why, due := r.weighs(t); len(due) != 0 || why != "" {
		t.Fatalf("woke twice for one item: %q %v", why, due)
	}
}

func TestDueAgendaOutlivesAFailedRound(t *testing.T) {
	r := newRig(t)
	r.cfg.WakesPerDay = 0
	if _, err := r.store.AddAgenda(r.bot.ID, "remind them at six", r.clock.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	why, due := r.weighs(t)
	r.run(context.Background(), r.bot, why, due) // r.reply fails
	if _, due := r.weighs(t); len(due) != 0 || !strings.HasPrefix(r.last.Skip, "the last round for due agenda items failed") {
		t.Fatalf("retried at once: %v %+v", due, r.last)
	}
	r.clock = r.clock.Add(dueRetry)
	if _, due := r.weighs(t); len(due) != 1 {
		t.Fatalf("a failed round lost the item: %v %+v", due, r.last)
	}
}

func TestRoundSilent(t *testing.T) {
	r := newRig(t)
	r.reply = mate.RunResult{Success: true, LastMessage: "Looked at the new note; nothing new.\n[silent]"}
	p := r.round(t)
	if p.Decision != mate.PulseSilent || p.Summary != "Looked at the new note; nothing new." || len(r.woken) != 0 {
		t.Fatalf("round = %+v, woken %v", p, r.woken)
	}
	if !strings.Contains(r.asked[0], "## Your agenda") {
		t.Fatalf("no digest in the request:\n%s", r.asked[0])
	}
}

func TestRoundSpeaks(t *testing.T) {
	r := newRig(t)
	r.reply = mate.RunResult{Success: true, LastMessage: "Tell them the draft contradicts /x.md."}
	p := r.round(t)
	if p.Decision != mate.PulseSpeak || len(r.woken) != 1 {
		t.Fatalf("round = %+v, woken %v", p, r.woken)
	}
	w := r.woken[0]
	if w.Event != mate.MateEventPulse || !strings.Contains(w.Prompt, "contradicts /x.md") || !strings.Contains(w.Prompt, prompt.SilentReply) {
		t.Fatalf("wake = %+v", w)
	}
	if w.Key != "" || w.Ref != p.ID {
		t.Fatalf("brief can be dropped for another, or not told back: %+v", w)
	}
	r.WakeEnded(w.Ref, "succeeded", false)
	if ps, _ := r.store.ListPulses(r.bot.ID, 1); ps[0].Delivery != "sent" {
		t.Fatalf("delivery = %q", ps[0].Delivery)
	}
}

func TestRoundThatWroteMustTell(t *testing.T) {
	r := newRig(t)
	r.reply = mate.RunResult{Success: true, LastMessage: "Fixed a link.\n[silent]", Wrote: []string{"/a.md"}}
	p := r.round(t)
	if p.Decision != mate.PulseSpeak || len(r.woken) != 1 {
		t.Fatalf("a round that wrote stayed silent: %+v", p)
	}
	w := r.woken[0].Prompt
	if !strings.Contains(w, `"/a.md"`) || strings.Contains(w, prompt.SilentReply) {
		t.Fatalf("its chat turn may stay silent:\n%s", w)
	}
}

func TestRoundFailedAndDropped(t *testing.T) {
	r := newRig(t)
	if p := r.round(t); p.Decision != mate.PulseFailed || len(r.woken) != 0 {
		t.Fatalf("failed round = %+v", p)
	}
	r.clock = r.clock.Add(time.Hour)
	r.reply = mate.RunResult{Success: true, LastMessage: "brief"}
	r.wakeErr = mate.ErrBotBusy
	if p := r.round(t); p.Delivery != "dropped" {
		t.Fatalf("dropped brief = %+v", p)
	}
}

func TestDigestShowsWhatChanged(t *testing.T) {
	r := newRig(t)
	now := time.Now()
	r.clock = now
	day := "/_shorts/" + now.Format("2006-01-02") + ".md"
	for _, e := range []plugin.Event{
		{Type: plugin.EventVaultWrite, Path: "/before.md", Time: now.Add(-2 * time.Hour)},
		{Type: plugin.EventVaultWrite, Path: day, ShortID: "s1", ShortHash: "h", Time: now.Add(-5 * time.Minute)},
		{Type: plugin.EventVaultShortAppend, Path: day, Content: "the launch slips a week", Time: now.Add(-5 * time.Minute)},
		{Type: plugin.EventVaultCreate, Path: "/_knowledge/Launch.md", Time: now.Add(-4 * time.Minute)},
	} {
		r.Notify(e)
	}
	if err := r.store.RecordUserVisit("/plan.md", mate.NoteAccessRead, now.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	d, _ := r.digest(r.bot, now.Add(-time.Hour), now.Add(-time.Hour), now)
	for _, want := range []string{"new (`s1`): the launch slips a week", "/_knowledge/Launch.md: new", "/plan.md"} {
		if !strings.Contains(d, want) {
			t.Errorf("digest lacks %q:\n%s", want, d)
		}
	}
	if strings.Contains(d, "/before.md") || strings.Contains(d, day+":") {
		t.Errorf("digest has what it already saw, or the shorts file as a note:\n%s", d)
	}
}

func TestOwnChatTurnDoesNotWakeIt(t *testing.T) {
	r := newRig(t)
	r.clock = time.Now().Add(-time.Hour)
	r.reply = mate.RunResult{Success: true, LastMessage: "Tidy the plan and tell them."}
	r.round(t)
	// The brief's chat turn writes a note, then ends.
	r.Notify(plugin.Event{Type: plugin.EventVaultWrite, Path: "/plan.md", Time: time.Now().Add(-time.Second)})
	r.WakeEnded(r.woken[0].Ref, "succeeded", false)
	r.clock = time.Now().Add(time.Minute)
	r.odds = 1
	r.weighs(t)
	if r.last.Signals != "" {
		t.Fatalf("its own write counted: %+v", r.last)
	}
	r.Notify(plugin.Event{Type: plugin.EventVaultWrite, Path: "/later.md", Time: r.clock})
	r.weighs(t)
	if r.last.Signals != "1 note changed" {
		t.Fatalf("a later change did not count: %+v", r.last)
	}
}

func TestRoundTimesOut(t *testing.T) {
	r := newRig(t)
	old := roundTimeout
	roundTimeout = 20 * time.Millisecond
	t.Cleanup(func() { roundTimeout = old })
	r.Pulse.round = func(ctx context.Context, _ *mate.Mate, _, _ string) mate.RunResult {
		<-ctx.Done()
		return mate.RunResult{LastMessage: "half a thought"}
	}
	if p := r.round(t); p.Decision != mate.PulseFailed || !strings.HasPrefix(p.Summary, "Timed out") || len(r.woken) != 0 {
		t.Fatalf("timed-out round = %+v", p)
	}
	r.clock = r.clock.Add(time.Hour)
	r.Pulse.round = func(ctx context.Context, _ *mate.Mate, _, _ string) mate.RunResult {
		<-ctx.Done()
		return mate.RunResult{LastMessage: "[silent]", Wrote: []string{"/a.md"}}
	}
	if p := r.round(t); p.Decision != mate.PulseSpeak || len(r.woken) != 1 {
		t.Fatalf("a timed-out round that wrote stayed silent: %+v", p)
	}
}

func TestDigestPointsAtFellowBots(t *testing.T) {
	r := newRig(t)
	news := &mate.Mate{Name: "News", Description: "Morning digests", Enabled: true}
	off := &mate.Mate{Name: "Old", Enabled: false}
	for _, m := range []*mate.Mate{news, off} {
		if err := r.store.CreateMate(m); err != nil {
			t.Fatal(err)
		}
	}
	conv, _ := r.store.StartConversation(news.ID, "")
	r.store.InsertMessage(mate.Message{ConversationID: conv.ID, Role: "assistant", MateID: news.ID, Status: "succeeded", Content: "today's news", TriggerEvent: "scheduled"})
	d, _ := r.digest(r.bot, time.Time{}, time.Time{}, time.Now())
	if !strings.Contains(d, "- News (`"+news.ID+"`), Morning digests: 1 unread, last active") {
		t.Errorf("no line for News:\n%s", d)
	}
	if strings.Contains(d, "Old") || strings.Contains(d, "- Hylo") {
		t.Errorf("a disabled bot, or itself, is listed:\n%s", d)
	}
}

func TestAwayFromTheComputerHalvesTheOdds(t *testing.T) {
	r := newRig(t)
	r.clock = time.Now()
	r.odds = 1
	r.weighs(t)
	base := r.last.Chance
	r.desk.Report(desktop.Sample{App: "Xcode", Locked: true}, r.clock)
	if err := r.store.TouchDesktop("Xcode", r.clock.Add(-10*time.Minute), time.Minute); err != nil {
		t.Fatal(err)
	}
	r.desk = desktop.New(r.store, true) // knows when they were last seen
	r.Pulse.desk = r.desk
	r.weighs(t)
	if r.last.Reach != reachNone || r.last.Chance != base*reachOdds[reachNone] {
		t.Fatalf("away odds = %+v, base %v", r.last, base)
	}
	r.push = []string{mate.ChannelTelegram}
	r.weighs(t)
	if r.last.Reach != reachPush || r.last.Chance != base*reachOdds[reachPush] {
		t.Fatalf("away but reachable odds = %+v, base %v", r.last, base)
	}
}

func TestPhoneInHand(t *testing.T) {
	r := newRig(t)
	r.clock = time.Now()
	r.odds = 1
	r.push = []string{mate.ChannelTelegram}
	r.desk.Report(desktop.Sample{App: "Xcode", Locked: true}, r.clock)
	if err := r.store.TouchDesktop("Xcode", r.clock.Add(-time.Hour), time.Minute); err != nil {
		t.Fatal(err)
	}
	r.desk = desktop.New(r.store, true)
	r.Pulse.desk = r.desk
	conv, _ := r.store.StartConversation(r.bot.ID, "")
	if _, err := r.store.InsertMessage(mate.Message{ConversationID: conv.ID, Role: "user", MateID: r.bot.ID, Content: "on the train", App: mate.ChannelTelegram}); err != nil {
		t.Fatal(err)
	}
	r.weighs(t)
	if r.last.Reach != reachPhone || !strings.Contains(r.last.Signals, "on their phone, writing from Telegram") {
		t.Fatalf("odds = %+v", r.last)
	}
	d, _ := r.digest(r.bot, time.Time{}, time.Time{}, r.clock)
	if !strings.Contains(d, "- On their phone: wrote to you from Telegram just now.") {
		t.Errorf("digest:\n%s", d)
	}
	r.reply = mate.RunResult{Success: true, LastMessage: "Tell them the train timetable changed."}
	r.round(t)
	if w := r.woken[0].Prompt; !strings.Contains(w, "no markdown") {
		t.Fatalf("a message for the phone isn't told so:\n%s", w)
	}
}

func TestDigestShowsTheUserAtTheComputer(t *testing.T) {
	r := newRig(t)
	now := at(12, 0) // clear of midnight, which splits "Today"
	xcode := desktop.Sample{App: "Xcode"}
	for i := 0; i < 5; i++ {
		r.desk.Report(xcode, now.Add(-time.Duration(4-i)*30*time.Second))
	}
	d, _ := r.digest(r.bot, time.Time{}, time.Time{}, now)
	for _, want := range []string{"## Where the user is", "- Now: in Xcode for 2m.", "- Today: Xcode 2m.", "- Lately: Xcode "} {
		if !strings.Contains(d, want) {
			t.Errorf("digest lacks %q:\n%s", want, d)
		}
	}
}
