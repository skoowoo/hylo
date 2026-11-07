package handler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hardhacker/hylo/internal/agent"
	"github.com/hardhacker/hylo/internal/mate"
)

func handed(t *ticket) bool {
	select {
	case <-t.ch:
		return true
	default:
		return false
	}
}

func quietFor(t *testing.T, d time.Duration) {
	old, gap := wakeQuiet, wakeGap
	wakeQuiet, wakeGap = d, 0
	t.Cleanup(func() { wakeQuiet, wakeGap = old, gap })
}

func TestLaneServesTheUserBeforeAWake(t *testing.T) {
	quietFor(t, 0)
	var l chatLane
	first, _ := l.join(false, "")
	wake, _ := l.join(true, "k")
	user, _ := l.join(false, "")
	if !handed(first) || handed(wake) || handed(user) {
		t.Fatal("only the first turn should hold the chat")
	}
	l.leave(first)
	if !handed(user) || handed(wake) {
		t.Fatal("the user's turn goes ahead of the waiting wake")
	}
	l.leave(user)
	if !handed(wake) {
		t.Fatal("with no user waiting and no quiet to keep, the wake runs")
	}
}

func TestWakeWaitsForTheUserToGoQuiet(t *testing.T) {
	quietFor(t, 80*time.Millisecond)
	var l chatLane
	user, _ := l.join(false, "")
	l.leave(user)
	wake, _ := l.join(true, "k")
	time.Sleep(20 * time.Millisecond)
	if handed(wake) {
		t.Fatal("a wake started right after the user's turn")
	}
	again, _ := l.join(false, "")
	l.leave(again)
	time.Sleep(70 * time.Millisecond)
	if handed(wake) {
		t.Fatal("another user turn should restart the quiet")
	}
	select {
	case <-wake.ch:
	case <-time.After(time.Second):
		t.Fatal("the wake never ran once the user went quiet")
	}
}

func TestLaneDropsDuplicateAndExtraWakes(t *testing.T) {
	quietFor(t, time.Hour)
	var l chatLane
	l.lastUser = time.Now()
	if w, err := l.join(true, "same"); w == nil || err != nil {
		t.Fatal("first wake refused")
	}
	if w, err := l.join(true, "same"); w != nil || err != nil {
		t.Fatalf("a duplicate wake got %v %v", w, err)
	}
	for i := 1; i < maxWakeWaits; i++ {
		if w, _ := l.join(true, string(rune('a'+i))); w == nil {
			t.Fatalf("wake %d refused under the cap", i)
		}
	}
	if _, err := l.join(true, "over"); !errors.Is(err, mate.ErrBotBusy) {
		t.Fatalf("a full lane took one more: %v", err)
	}
}

func TestAbandonedTurnPassesTheChatOn(t *testing.T) {
	quietFor(t, 0)
	var l chatLane
	first, _ := l.join(false, "")
	second, _ := l.join(false, "")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if l.wait(ctx, second) {
		t.Fatal("a canceled wait got the chat")
	}
	third, _ := l.join(false, "")
	l.abandon(first)
	if !handed(third) {
		t.Fatal("the chat stuck with a turn that gave up")
	}
}

func TestWakesNeverRunBackToBack(t *testing.T) {
	quietFor(t, 0)
	wakeGap = 80 * time.Millisecond
	var l chatLane
	first, _ := l.join(true, "a")
	second, _ := l.join(true, "b")
	if !handed(first) {
		t.Fatal("an idle lane held back the first wake")
	}
	l.leave(first)
	user, _ := l.join(false, "")
	if !handed(user) {
		t.Fatal("the gap between wakes held back the user")
	}
	l.leave(user)
	time.Sleep(40 * time.Millisecond)
	if handed(second) {
		t.Fatal("the next wake started right after the last one")
	}
	select {
	case <-second.ch:
	case <-time.After(time.Second):
		t.Fatal("the next wake never ran after the gap")
	}
}

func TestLaneIdleWaitsLikeAWake(t *testing.T) {
	quietFor(t, 50*time.Millisecond)
	var l chatLane
	if !l.idle() {
		t.Fatal("a fresh chat is idle")
	}
	user, _ := l.join(false, "")
	if l.idle() {
		t.Fatal("idle while the user's turn holds the chat")
	}
	l.leave(user)
	if l.idle() {
		t.Fatal("idle right after the user's turn")
	}
	time.Sleep(60 * time.Millisecond)
	if !l.idle() {
		t.Fatal("not idle once the user went quiet")
	}
	wake, _ := l.join(true, "k")
	if l.idle() {
		t.Fatal("idle while a wake holds the chat")
	}
	l.leave(wake)
}

func TestWakeThatNeverReachesTheChatStillEnds(t *testing.T) {
	quietFor(t, 0)
	a := testAgentAPI(t)
	store, err := mate.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	a.store, a.hub = store, agent.NewHub()
	bot := &mate.Mate{Name: "Hylo", Main: true, Enabled: false}
	if err := store.CreateMate(bot); err != nil {
		t.Fatal(err)
	}
	ended := make(chan WakeEnd, 1)
	a.SetWakeEnd(func(e WakeEnd) { ended <- e })
	if err := a.Wake(bot, mate.Wake{Event: mate.MateEventPulse, Prompt: "brief", Ref: "round-1"}); err != nil {
		t.Fatal(err)
	}
	select {
	case e := <-ended:
		if e.Ref != "round-1" || e.Status != "failed" {
			t.Fatalf("end = %+v", e)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("a wake turned away by its bot never ended")
	}
	a.Wait()
}
