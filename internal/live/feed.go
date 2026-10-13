// Package live pushes server-side changes, from any source, to open pages,
// so pages refresh from it rather than announcing their own changes.
package live

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Topics a change belongs to.
const (
	TopicNotes  = "notes"
	TopicBots   = "bots"
	TopicSkills = "skills"
	TopicInbox  = "inbox"
)

// Change is one thing that changed. For notes, Op is create, write, delete,
// rename, short, pin or image, and Path the note (or folder, or image);
// Inbox creates carry the new message in Data; other topics only say that
// something in them changed.
type Change struct {
	Topic   string `json:"topic"`
	Op      string `json:"op"`
	Path    string `json:"path,omitempty"`
	OldPath string `json:"oldPath,omitempty"`
	IsDir   bool   `json:"isDir,omitempty"`
	Data    any    `json:"data,omitempty"`
}

const (
	subBuffer = 256
	heartbeat = 25 * time.Second
)

type subscriber struct {
	ch       chan Change
	overflow chan struct{}
}

// Feed fans changes out to every connected page.
type Feed struct {
	mu   sync.Mutex
	subs map[*subscriber]struct{}
}

func NewFeed() *Feed {
	return &Feed{subs: map[*subscriber]struct{}{}}
}

// Publish never blocks: a page too slow to keep up is told to resync instead.
func (f *Feed) Publish(c Change) {
	if f == nil {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for s := range f.subs {
		select {
		case s.ch <- c:
		default:
			select {
			case s.overflow <- struct{}{}:
			default:
			}
		}
	}
}

// Notify returns a hook for a store that only says its topic changed.
func (f *Feed) Notify(topic string) func() {
	return func() { f.Publish(Change{Topic: topic, Op: "change"}) }
}

func (f *Feed) subscribe() *subscriber {
	s := &subscriber{ch: make(chan Change, subBuffer), overflow: make(chan struct{}, 1)}
	f.mu.Lock()
	f.subs[s] = struct{}{}
	f.mu.Unlock()
	return s
}

func (f *Feed) unsubscribe(s *subscriber) {
	f.mu.Lock()
	delete(f.subs, s)
	f.mu.Unlock()
}

// ServeHTTP streams changes as SSE "change" events. There's no replay: a page
// that reconnects, or falls behind, gets a "resync" and reloads what it shows.
func (f *Feed) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	// The stream outlives the server's write timeout.
	_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})
	s := f.subscribe()
	defer f.unsubscribe(s)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, ": live\n\n")
	flusher.Flush()

	tick := time.NewTicker(heartbeat)
	defer tick.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case c := <-s.ch:
			data, _ := json.Marshal(c)
			fmt.Fprintf(w, "event: change\ndata: %s\n\n", data)
		case <-s.overflow:
			for len(s.ch) > 0 {
				<-s.ch
			}
			fmt.Fprint(w, "event: resync\ndata: {}\n\n")
		case <-tick.C:
			fmt.Fprint(w, ": ping\n\n")
		}
		flusher.Flush()
	}
}
