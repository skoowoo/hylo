package inbox

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// push fans a new message out to SSE subscribers, dropping it for slow ones.
func (s *Store) push(m Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for ch := range s.subs {
		select {
		case ch <- m:
		default:
		}
	}
}

func (s *Store) subscribe() chan Message {
	ch := make(chan Message, 16)
	s.mu.Lock()
	s.subs[ch] = struct{}{}
	s.mu.Unlock()
	return ch
}

func (s *Store) unsubscribe(ch chan Message) {
	s.mu.Lock()
	delete(s.subs, ch)
	s.mu.Unlock()
}

// StreamSSE serves GET /api/inbox/notifications: one "message" event per
// newly created inbox message.
func (s *Store) StreamSSE(w http.ResponseWriter, r *http.Request) {
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

	ch := s.subscribe()
	defer s.unsubscribe(ch)

	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()

	id := 0
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			if _, err := fmt.Fprintf(w, ": ping\n\n"); err != nil {
				return
			}
			fl.Flush()
		case m := <-ch:
			data, err := json.Marshal(m)
			if err != nil {
				continue
			}
			id++
			if _, err := fmt.Fprintf(w, "id: %d\nevent: message\ndata: %s\n\n", id, data); err != nil {
				return
			}
			fl.Flush()
		}
	}
}
