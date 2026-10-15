package agent

import (
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Run is one agent chat execution (mirrors open-design daemon run shape).
type Run struct {
	mu             sync.Mutex
	ID             string
	ProjectID      string
	ConversationID string
	AgentID        string
	Status         string
	CreatedAt      int64
	UpdatedAt      int64
	Events         []RecordedEvent
	NextEventID    int
	subs           []chan RecordedEvent
}

// RecordedEvent is replay + live SSE payload.
type RecordedEvent struct {
	ID    int             `json:"id"`
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data"`
}

// Hub is the in-memory run registry (subset of apps/daemon/src/runs.ts).
type Hub struct {
	mu        sync.Mutex
	runs      map[string]*Run
	maxEvents int
	ttl       time.Duration
}

// NewHub constructs a hub with Open Design–like defaults.
func NewHub() *Hub {
	return &Hub{
		runs:      make(map[string]*Run),
		maxEvents: 2000,
		ttl:       2 * time.Hour,
	}
}

// CreateRun allocates a queued run.
func (h *Hub) CreateRun(meta map[string]string) *Run {
	h.mu.Lock()
	defer h.mu.Unlock()
	id := uuid.NewString()
	now := time.Now().UnixMilli()
	r := &Run{ID: id, Status: "queued", CreatedAt: now, UpdatedAt: now}
	if meta != nil {
		r.ProjectID = meta["projectId"]
		r.ConversationID = meta["conversationId"]
		r.AgentID = meta["agentId"]
	}
	h.runs[id] = r
	return r
}

// Get returns a run.
func (h *Hub) Get(id string) *Run {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.runs[id]
}

// ActiveCount returns the number of runs currently in "running" state.
func (h *Hub) ActiveCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for _, r := range h.runs {
		r.mu.Lock()
		if r.Status == "running" {
			n++
		}
		r.mu.Unlock()
	}
	return n
}

// IsTerminalStatus mirrors TERMINAL_RUN_STATUSES.
func IsTerminalStatus(s string) bool {
	switch s {
	case "succeeded", "failed", "canceled":
		return true
	default:
		return false
	}
}

// Emit records and broadcasts one event.
func (h *Hub) Emit(r *Run, event string, data any) {
	b, err := json.Marshal(data)
	if err != nil {
		b = []byte(`{"error":"encode"}`)
	}
	r.mu.Lock()
	r.NextEventID++
	id := r.NextEventID
	if len(r.Events) >= h.maxEvents {
		// drop oldest
		r.Events = append([]RecordedEvent(nil), r.Events[len(r.Events)-h.maxEvents+1:]...)
	}
	rec := RecordedEvent{ID: id, Event: event, Data: json.RawMessage(b)}
	r.Events = append(r.Events, rec)
	r.UpdatedAt = time.Now().UnixMilli()
	subs := append([]chan RecordedEvent(nil), r.subs...)
	r.mu.Unlock()
	for _, ch := range subs {
		select {
		case ch <- rec:
		default:
			// Non-blocking: when the SSE consumer falls behind on writes to the HTTP
			// response, drops here prevented backpressure—but can lose the terminal
			// "end" event. StreamSSE reconciles against run.Events after ch closes.
		}
	}
}

// Start sets running.
func (r *Run) Start() {
	r.mu.Lock()
	r.Status = "running"
	r.UpdatedAt = time.Now().UnixMilli()
	r.mu.Unlock()
}

// Finish marks terminal state and closes subscriber channels.
func (h *Hub) Finish(r *Run, status string, payload map[string]any) {
	if payload == nil {
		payload = map[string]any{}
	}
	payload["status"] = status
	h.Emit(r, "end", payload)
	r.mu.Lock()
	r.Status = status
	r.UpdatedAt = time.Now().UnixMilli()
	for _, ch := range r.subs {
		close(ch)
	}
	r.subs = nil
	r.mu.Unlock()
	go func() {
		time.Sleep(h.ttl)
		h.mu.Lock()
		delete(h.runs, r.ID)
		h.mu.Unlock()
	}()
}

// StreamSSE serves GET /api/runs/:id/events.
func (h *Hub) StreamSSE(w http.ResponseWriter, req *http.Request, run *Run) {
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	// SSE is an open-ended stream; the server-level WriteTimeout (default 60 s)
	// would abruptly close the TCP connection mid-stream and cause
	// io.ErrUnexpectedEOF on the client. Clear the per-response write deadline
	// so long agent runs are not cut off.
	_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	last := 0
	if s := req.Header.Get("Last-Event-ID"); s != "" {
		last, _ = strconv.Atoi(s)
	}

	// Snapshot events and subscribe under lock, but do I/O outside the lock
	// to avoid blocking Emit (which also acquires run.mu) during HTTP writes.
	run.mu.Lock()
	var toReplay []RecordedEvent
	for _, ev := range run.Events {
		if ev.ID > last {
			toReplay = append(toReplay, ev)
		}
	}
	terminal := IsTerminalStatus(run.Status)
	var ch chan RecordedEvent
	if !terminal {
		ch = make(chan RecordedEvent, 128)
		run.subs = append(run.subs, ch)
	}
	run.mu.Unlock()

	lastSent := last
	for _, ev := range toReplay {
		if err := writeSSE(w, fl, ev); err != nil {
			return
		}
		lastSent = ev.ID
	}
	if terminal {
		return
	}

	ctx := req.Context()
	defer func() {
		run.mu.Lock()
		var kept []chan RecordedEvent
		for _, c := range run.subs {
			if c != ch {
				kept = append(kept, c)
			}
		}
		run.subs = kept
		run.mu.Unlock()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-ch:
			if !ok {
				flushSSEventsAfterSubscriptionDrop(w, fl, run, lastSent)
				return
			}
			if err := writeSSE(w, fl, ev); err != nil {
				return
			}
			if ev.ID > lastSent {
				lastSent = ev.ID
			}
			if ev.Event == "end" {
				return
			}
		}
	}
}

// flushSSEventsAfterSubscriptionDrop writes any hub events newer than lastSentId that
// were never delivered live (Emit uses a non-blocking send and can drop frames when
// the subscriber channel fills—especially the terminal "end" after a long Cursor run).
func flushSSEventsAfterSubscriptionDrop(w http.ResponseWriter, fl http.Flusher, run *Run, lastSentID int) {
	run.mu.Lock()
	events := append([]RecordedEvent(nil), run.Events...)
	run.mu.Unlock()
	for _, ev := range events {
		if ev.ID > lastSentID {
			if writeSSE(w, fl, ev) != nil {
				return
			}
			lastSentID = ev.ID
			if ev.Event == "end" {
				return
			}
		}
	}
}

func writeSSE(w http.ResponseWriter, fl http.Flusher, ev RecordedEvent) error {
	if _, err := w.Write([]byte("id: " + strconv.Itoa(ev.ID) + "\n")); err != nil {
		return err
	}
	if _, err := w.Write([]byte("event: " + ev.Event + "\n")); err != nil {
		return err
	}
	if _, err := w.Write([]byte("data: ")); err != nil {
		return err
	}
	if _, err := w.Write(ev.Data); err != nil {
		return err
	}
	if _, err := w.Write([]byte("\n\n")); err != nil {
		return err
	}
	fl.Flush()
	return nil
}

// StatusJSON for GET /api/runs/:id.
func (r *Run) StatusJSON() map[string]any {
	r.mu.Lock()
	defer r.mu.Unlock()
	return map[string]any{
		"id":             r.ID,
		"projectId":      r.ProjectID,
		"conversationId": r.ConversationID,
		"agentId":        r.AgentID,
		"status":         r.Status,
		"createdAt":      r.CreatedAt,
		"updatedAt":      r.UpdatedAt,
	}
}
