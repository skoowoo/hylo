// Package editorbus lets server-side callers (MCP tools) drive the browser's live
// editor: an op is pushed to the page over SSE and the caller blocks until the page
// acks that typing finished.
package editorbus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

var (
	ErrNoEditor   = errors.New("no editor connected")
	ErrEditorGone = errors.New("editor disconnected")
)

// reconnectGrace is how long an op survives its page's SSE dropping; the page keeps
// typing meanwhile and EventSource reconnects with the same client id.
var reconnectGrace = 10 * time.Second

// maxAckBytes bounds an ack; a read carries the whole note.
const maxAckBytes = 16 << 20

// opTimeout bounds one op, queueing in the page included. 100ms per char stays
// above the slowest speed with its pauses.
func opTimeout(op Op) time.Duration {
	d := 2*time.Minute + time.Duration(utf8.RuneCountInString(op.Content))*100*time.Millisecond
	return min(d, time.Hour)
}

// Op kinds.
const (
	KindType    = "type"
	KindRewrite = "rewrite"
	KindRead    = "read"
)

// Op is one request against a note open in the editor.
type Op struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Path    string `json:"path"`
	Content string `json:"content,omitempty"`
	// At is "end" (default) or "cursor"; Heading, when set, wins and targets the end of that section.
	At      string `json:"at,omitempty"`
	Heading string `json:"heading,omitempty"`
	Speed   string `json:"speed,omitempty"`
	// Rewrite target: "selection" or a block id/range, which needs Expect; else FromText..ToText.
	Target   string `json:"target,omitempty"`
	Expect   string `json:"expect,omitempty"`
	FromText string `json:"from_text,omitempty"`
	ToText   string `json:"to_text,omitempty"`
	// Run is the agent run that issued the op, so cancelling that run stops it; "" when unknown.
	Run string `json:"-"`
}

// Result is the page's ack for an Op.
type Result struct {
	ID          string `json:"id"`
	OK          bool   `json:"ok"`
	Interrupted bool   `json:"interrupted,omitempty"`
	Error       string `json:"error,omitempty"`
	Chars       int    `json:"chars"`
	// Path is the note the op ran on, resolved by the page when the op had none.
	Path string `json:"path,omitempty"`
	// Data carries a read's payload.
	Data json.RawMessage `json:"data,omitempty"`
}

type event struct {
	name string
	data any
}

// sub is one SSE connection; client identifies the page across reconnects.
type sub struct {
	client string
	ch     chan event
}

type pending struct {
	op     Op
	client string
	res    chan Result
	gone   chan struct{}
	closed bool
}

type Bus struct {
	mu      sync.Mutex
	subs    []*sub
	pending map[string]*pending
	// active is the client whose window last gained focus.
	active string
	// cancels holds cancels for a client that was offline when they were issued.
	cancels map[string][]string
}

func New() *Bus {
	return &Bus{pending: map[string]*pending{}, cancels: map[string][]string{}}
}

// Connected reports whether any editor page is listening.
func (b *Bus) Connected() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.subs) > 0
}

// Busy reports whether any op is in flight.
func (b *Bus) Busy() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.pending) > 0
}

// subscribe registers a connection and replays what client may have missed while away:
// ops still awaiting its ack (the page drops ids it has seen) and cancels.
func (b *Bus) subscribe(client string) *sub {
	if client == "" {
		client = uuid.NewString()
	}
	s := &sub{client: client, ch: make(chan event, 64)}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subs = append(b.subs, s)
	for _, p := range b.pending {
		if p.client == client {
			s.send(event{"op", p.op})
		}
	}
	for _, id := range b.cancels[client] {
		s.send(event{"cancel", cancelData(id)})
	}
	delete(b.cancels, client)
	return s
}

func (b *Bus) unsubscribe(s *sub) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i, c := range b.subs {
		if c == s {
			b.subs = append(b.subs[:i], b.subs[i+1:]...)
			break
		}
	}
	if b.subFor(s.client) != nil {
		return
	}
	time.AfterFunc(reconnectGrace, func() { b.expire(s.client) })
}

// expire fails client's ops if it has not reconnected.
func (b *Bus) expire(client string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.subFor(client) != nil {
		return
	}
	delete(b.cancels, client)
	for _, p := range b.pending {
		if p.client == client && !p.closed {
			p.closed = true
			close(p.gone)
		}
	}
}

// subFor returns client's newest connection. Caller holds mu.
func (b *Bus) subFor(client string) *sub {
	for i := len(b.subs) - 1; i >= 0; i-- {
		if b.subs[i].client == client {
			return b.subs[i]
		}
	}
	return nil
}

func (s *sub) send(e event) bool {
	select {
	case s.ch <- e:
		return true
	default:
		return false
	}
}

func cancelData(id string) map[string]string { return map[string]string{"id": id} }

// cancel tells the op's page to stop, now or when it reconnects.
func (b *Bus) cancel(id string, p *pending) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if s := b.subFor(p.client); s != nil && s.send(event{"cancel", cancelData(id)}) {
		return
	}
	b.cancels[p.client] = append(b.cancels[p.client], id)
}

// Focus marks client as the window the user is in; it gets the next ops.
func (b *Bus) Focus(client string) {
	b.mu.Lock()
	b.active = client
	b.mu.Unlock()
}

// Dispatch sends op to the focused editor (else the newest) and waits for its ack.
// Typing into two windows of the same note would fight over the file, so only one gets it.
func (b *Bus) Dispatch(ctx context.Context, op Op) (Result, error) {
	b.mu.Lock()
	if len(b.subs) == 0 {
		b.mu.Unlock()
		return Result{}, ErrNoEditor
	}
	s := b.subFor(b.active)
	if s == nil {
		s = b.subs[len(b.subs)-1]
	}
	op.ID = uuid.NewString()
	p := &pending{op: op, client: s.client, res: make(chan Result, 1), gone: make(chan struct{})}
	b.pending[op.ID] = p
	ok := s.send(event{"op", op})
	b.mu.Unlock()

	defer func() {
		b.mu.Lock()
		delete(b.pending, op.ID)
		b.mu.Unlock()
	}()
	if !ok {
		return Result{}, fmt.Errorf("editor queue full")
	}

	timer := time.NewTimer(opTimeout(op))
	defer timer.Stop()
	select {
	case r := <-p.res:
		return r, nil
	case <-ctx.Done():
		b.cancel(op.ID, p)
		return Result{}, ctx.Err()
	case <-p.gone:
		return Result{}, ErrEditorGone
	case <-timer.C:
		b.cancel(op.ID, p)
		return Result{}, errors.New("editor did not finish in time")
	}
}

// Ack completes the op with the page's result; unknown ids are ignored.
func (b *Bus) Ack(r Result) {
	b.mu.Lock()
	p := b.pending[r.ID]
	b.mu.Unlock()
	if p == nil {
		return
	}
	select {
	case p.res <- r:
	default:
	}
}

// CancelRun stops the in-flight ops issued by run, plus those whose run is unknown,
// when the user cancels that run. The MCP SDK does not cancel a tool call when its
// HTTP connection drops, so callers cannot rely on Dispatch's ctx for this.
func (b *Bus) CancelRun(run string) {
	b.mu.Lock()
	var hit []*pending
	for _, p := range b.pending {
		if p.op.Run == run || p.op.Run == "" {
			hit = append(hit, p)
		}
	}
	b.mu.Unlock()
	for _, p := range hit {
		b.cancel(p.op.ID, p)
		select {
		case p.res <- Result{ID: p.op.ID, Interrupted: true, Error: "canceled"}:
		default:
		}
	}
}

// StreamSSE serves GET /api/editor/agent?client=<page id>.
func (b *Bus) StreamSSE(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	fl.Flush()

	s := b.subscribe(r.URL.Query().Get("client"))
	defer b.unsubscribe(s)

	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			fl.Flush()
		case e := <-s.ch:
			data, err := json.Marshal(e.data)
			if err != nil {
				continue
			}
			if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", e.name, data); err != nil {
				return
			}
			fl.Flush()
		}
	}
}

// AckPOST serves POST /api/editor/agent/ack.
func (b *Bus) AckPOST(w http.ResponseWriter, r *http.Request) {
	var res Result
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxAckBytes)).Decode(&res); err != nil || res.ID == "" {
		http.Error(w, "invalid ack body", http.StatusBadRequest)
		return
	}
	b.Ack(res)
	w.WriteHeader(http.StatusNoContent)
}

// FocusPOST serves POST /api/editor/agent/focus.
func (b *Bus) FocusPOST(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Client string `json:"client"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&body); err != nil || body.Client == "" {
		http.Error(w, "invalid focus body", http.StatusBadRequest)
		return
	}
	b.Focus(body.Client)
	w.WriteHeader(http.StatusNoContent)
}
