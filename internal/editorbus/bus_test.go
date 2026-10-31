package editorbus

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func recv(t *testing.T, s *sub) event {
	t.Helper()
	select {
	case e := <-s.ch:
		return e
	case <-time.After(time.Second):
		t.Fatal("no event")
		return event{}
	}
}

func TestDispatchNoEditor(t *testing.T) {
	_, err := New().Dispatch(context.Background(), Op{Path: "/a.md"})
	if !errors.Is(err, ErrNoEditor) {
		t.Fatalf("err = %v", err)
	}
}

func TestUnwatchedRunNeverReachesTheEditor(t *testing.T) {
	b := New()
	s := b.subscribe("")
	done := b.Unwatched("wake-run")
	if _, err := b.Dispatch(context.Background(), Op{Path: "/a.md", Content: "x", Run: "wake-run"}); !errors.Is(err, ErrUnwatched) {
		t.Fatalf("err = %v", err)
	}
	select {
	case e := <-s.ch:
		t.Fatalf("the editor got %+v", e)
	default:
	}
	done()
	go func() {
		op := (<-s.ch).data.(Op)
		b.Ack(Result{ID: op.ID, OK: true})
	}()
	if _, err := b.Dispatch(context.Background(), Op{Path: "/a.md", Run: "wake-run"}); err != nil {
		t.Fatalf("once done, err = %v", err)
	}
}

func TestDispatchAck(t *testing.T) {
	b := New()
	s := b.subscribe("")
	go func() {
		e := <-s.ch
		op := e.data.(Op)
		b.Ack(Result{ID: op.ID, OK: true, Chars: len(op.Content)})
	}()
	r, err := b.Dispatch(context.Background(), Op{Path: "/a.md", Content: "hello"})
	if err != nil || !r.OK || r.Chars != 5 {
		t.Fatalf("r=%+v err=%v", r, err)
	}
}

func TestDispatchTargetsLatestEditor(t *testing.T) {
	b := New()
	first, second := b.subscribe(""), b.subscribe("")
	go func() { b.Ack(Result{ID: (<-second.ch).data.(Op).ID, OK: true}) }()
	if _, err := b.Dispatch(context.Background(), Op{}); err != nil {
		t.Fatal(err)
	}
	if len(first.ch) != 0 {
		t.Fatal("older editor received the op")
	}
}

func TestDispatchPrefersFocusedEditor(t *testing.T) {
	b := New()
	first, second := b.subscribe("a"), b.subscribe("b")
	b.Focus("a")
	go func() { b.Ack(Result{ID: (<-first.ch).data.(Op).ID, OK: true}) }()
	if _, err := b.Dispatch(context.Background(), Op{}); err != nil {
		t.Fatal(err)
	}
	if len(second.ch) != 0 {
		t.Fatal("unfocused editor received the op")
	}
}

func TestDispatchCancelNotifiesEditor(t *testing.T) {
	b := New()
	s := b.subscribe("")
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-s.ch
		cancel()
	}()
	if _, err := b.Dispatch(ctx, Op{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	if e := recv(t, s); e.name != "cancel" {
		t.Fatalf("event = %s", e.name)
	}
}

func TestDispatchEditorGone(t *testing.T) {
	defer func(g time.Duration) { reconnectGrace = g }(reconnectGrace)
	reconnectGrace = 10 * time.Millisecond
	b := New()
	s := b.subscribe("")
	go func() {
		<-s.ch
		b.unsubscribe(s)
	}()
	if _, err := b.Dispatch(context.Background(), Op{}); !errors.Is(err, ErrEditorGone) {
		t.Fatalf("err = %v", err)
	}
}

// A dropped SSE connection is not a lost page: the op is replayed on reconnect and
// the page's ack still completes it.
func TestReloadedFocusedPageTakesOver(t *testing.T) {
	b := New()
	other := b.subscribe("other")
	old := b.subscribe("old")
	b.Focus("old")
	b.unsubscribe(old)
	reloaded := b.subscribe("new")
	go func() { b.Ack(Result{ID: (<-reloaded.ch).data.(Op).ID, OK: true}) }()
	if _, err := b.Dispatch(context.Background(), Op{}); err != nil {
		t.Fatal(err)
	}
	if len(other.ch) != 0 {
		t.Fatal("op went to another window")
	}
}

func TestDispatchSurvivesReconnect(t *testing.T) {
	defer func(g time.Duration) { reconnectGrace = g }(reconnectGrace)
	reconnectGrace = 200 * time.Millisecond
	b := New()
	s := b.subscribe("page")
	done := make(chan struct{})
	go func() {
		defer close(done)
		r, err := b.Dispatch(context.Background(), Op{Content: "x"})
		if err != nil || !r.OK {
			t.Errorf("r=%+v err=%v", r, err)
		}
	}()
	id := recv(t, s).data.(Op).ID
	b.unsubscribe(s)
	s2 := b.subscribe("page")
	if e := recv(t, s2); e.name != "op" || e.data.(Op).ID != id {
		t.Fatalf("replayed %s %+v", e.name, e.data)
	}
	time.Sleep(300 * time.Millisecond)
	b.Ack(Result{ID: id, OK: true})
	<-done
}

func TestCancelWhileOfflineIsDeliveredOnReconnect(t *testing.T) {
	b := New()
	s := b.subscribe("page")
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-s.ch
		b.unsubscribe(s)
		cancel()
	}()
	if _, err := b.Dispatch(ctx, Op{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	if e := recv(t, b.subscribe("page")); e.name != "cancel" {
		t.Fatalf("event = %s", e.name)
	}
}

func TestCancelRun(t *testing.T) {
	b := New()
	s := b.subscribe("")
	results := make(chan Result, 2)
	for _, run := range []string{"r1", "r2"} {
		go func() {
			r, _ := b.Dispatch(context.Background(), Op{Run: run})
			results <- r
		}()
	}
	ops := map[string]string{}
	for range 2 {
		op := recv(t, s).data.(Op)
		ops[op.Run] = op.ID
	}
	b.CancelRun("r1")
	if r := <-results; !r.Interrupted || r.ID != ops["r1"] {
		t.Fatalf("r=%+v", r)
	}
	if e := recv(t, s); e.name != "cancel" || e.data.(map[string]string)["id"] != ops["r1"] {
		t.Fatalf("event = %s %v", e.name, e.data)
	}
	b.Ack(Result{ID: ops["r2"], OK: true})
	if r := <-results; !r.OK {
		t.Fatalf("r2 = %+v", r)
	}
}

func TestAckAcceptsLargeRead(t *testing.T) {
	b := New()
	body := `{"id":"x","ok":true,"data":"` + strings.Repeat("字", 1<<17) + `"}`
	rec := httptest.NewRecorder()
	b.AckPOST(rec, httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body)))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("code = %d", rec.Code)
	}
}

// An op issued while the page's SSE is reconnecting waits for it instead of failing.
func TestDispatchDuringReconnectIsDeliveredOnReconnect(t *testing.T) {
	b := New()
	s := b.subscribe("page")
	b.unsubscribe(s)
	if !b.Connected() {
		t.Fatal("a reconnecting page counts as connected")
	}
	done := make(chan Result, 1)
	go func() {
		r, _ := b.Dispatch(context.Background(), Op{Content: "x"})
		done <- r
	}()
	time.Sleep(20 * time.Millisecond)
	op := recv(t, b.subscribe("page")).data.(Op)
	if op.BudgetMS <= 0 || op.BudgetMS > replyBudget.Milliseconds() {
		t.Fatalf("budget = %d", op.BudgetMS)
	}
	b.Ack(Result{ID: op.ID, OK: true})
	if r := <-done; !r.OK {
		t.Fatalf("r = %+v", r)
	}
}

// Nothing reached the page, so the caller may safely write the note another way.
func TestUndeliveredOpReportsNoEditor(t *testing.T) {
	defer func(g time.Duration) { reconnectGrace = g }(reconnectGrace)
	reconnectGrace = 20 * time.Millisecond
	b := New()
	b.unsubscribe(b.subscribe("page"))
	if _, err := b.Dispatch(context.Background(), Op{}); !errors.Is(err, ErrNoEditor) {
		t.Fatalf("err = %v", err)
	}
}

func TestDispatchStopsAStuckEditor(t *testing.T) {
	defer func(r, a time.Duration) { replyBudget, ackGrace = r, a }(replyBudget, ackGrace)
	replyBudget, ackGrace = 20*time.Millisecond, 10*time.Millisecond
	b := New()
	s := b.subscribe("")
	if _, err := b.Dispatch(context.Background(), Op{}); !errors.Is(err, ErrEditorStuck) {
		t.Fatalf("err = %v", err)
	}
	recv(t, s)
	if e := recv(t, s); e.name != "cancel" {
		t.Fatalf("event = %s", e.name)
	}
}
