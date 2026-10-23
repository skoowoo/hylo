package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestStartParam(t *testing.T) {
	msg := func(text, chat string, bot bool) *Message {
		return &Message{From: &User{ID: 1, IsBot: bot}, Chat: Chat{ID: 1, Type: chat}, Text: text}
	}
	if arg, ok := StartParam(msg("/start abc", "private", false)); !ok || arg != "abc" {
		t.Fatalf("got %q %v", arg, ok)
	}
	if arg, ok := StartParam(msg("/start", "private", false)); !ok || arg != "" {
		t.Fatalf("bare /start: %q %v", arg, ok)
	}
	if _, ok := StartParam(msg("/start abc", "group", false)); ok {
		t.Fatal("a group /start must not count")
	}
	if _, ok := StartParam(msg("/start abc", "private", true)); ok {
		t.Fatal("a bot's /start must not count")
	}
	if _, ok := StartParam(msg("hello", "private", false)); ok {
		t.Fatal("not a /start")
	}
}

// fakeBotAPI serves one batch of updates, then empty polls.
func fakeBotAPI(t *testing.T, ups []Update) (*Client, *[]string) {
	var mu sync.Mutex
	var calls []string
	served := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
		mu.Lock()
		calls = append(calls, method)
		var result any = true
		if method == "getUpdates" {
			if served {
				result = []Update{}
			} else {
				result, served = ups, true
			}
		}
		mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": result})
	}))
	t.Cleanup(srv.Close)
	c, _ := NewClient("tok", "")
	c.base = srv.URL
	return c, &calls
}

func TestRunMonitorDeliversPrivateTextOnly(t *testing.T) {
	person := &User{ID: 7}
	c, _ := fakeBotAPI(t, []Update{
		{UpdateID: 1, Message: &Message{From: person, Chat: Chat{ID: 7, Type: "private"}, Text: "hi"}},
		{UpdateID: 2, Message: &Message{From: person, Chat: Chat{ID: -5, Type: "group"}, Text: "group"}},
		{UpdateID: 3, Message: &Message{From: &User{ID: 9, IsBot: true}, Chat: Chat{ID: 9, Type: "private"}, Text: "bot"}},
		{UpdateID: 4, Message: &Message{From: person, Chat: Chat{ID: 7, Type: "private"}}},
		{UpdateID: 5, Message: &Message{From: person, Chat: Chat{ID: 7, Type: "private"}, Photo: []PhotoSize{{FileID: "p"}}}},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	got := make(chan string, 4)
	go RunMonitor(ctx, MonitorOpts{Client: c, OnMessage: func(m *Message) {
		if len(m.Photo) > 0 {
			got <- "photo:" + m.Photo[0].FileID
			return
		}
		got <- m.Text
	}}) //nolint:errcheck
	select {
	case text := <-got:
		if text != "hi" {
			t.Fatalf("got %q", text)
		}
	case <-ctx.Done():
		t.Fatal("no message delivered")
	}
	select {
	case text := <-got:
		if text != "photo:p" {
			t.Fatalf("got %q", text)
		}
	case <-ctx.Done():
		t.Fatal("photo was dropped")
	}
	select {
	case text := <-got:
		t.Fatalf("unexpected %q", text)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestDownloadErrorOmitsToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusBadGateway)
	}))
	t.Cleanup(srv.Close)
	c, _ := NewClient("secret-token", "")
	c.SetBase(srv.URL)
	_, err := c.DownloadFile(context.Background(), "photos/a.png")
	if err == nil || strings.Contains(err.Error(), "secret-token") {
		t.Fatalf("err = %v", err)
	}
}

func TestAPIErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error_code": 409, "description": "Conflict"})
	}))
	t.Cleanup(srv.Close)
	c, _ := NewClient("tok", "")
	c.base = srv.URL
	_, err := c.GetUpdates(context.Background(), 0, 0)
	if !IsConflict(err) || IsUnauthorized(err) {
		t.Fatalf("err = %v", err)
	}
}
