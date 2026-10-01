package wechatplugin

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hardhacker/hylo/internal/config"
	"github.com/hardhacker/hylo/internal/plugin"
	"github.com/hardhacker/hylo/internal/wechat"
)

func TestReplyTypingLifecycle(t *testing.T) {
	var mu sync.Mutex
	var typingStatuses []int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "getconfig"):
			_ = json.NewEncoder(w).Encode(map[string]any{"typing_ticket": "ticket-1"})
		case strings.Contains(r.URL.Path, "sendtyping"):
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			status, _ := body["status"].(float64)
			mu.Lock()
			typingStatuses = append(typingStatuses, int(status))
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ret":0}`))
		case strings.Contains(r.URL.Path, "sendmessage"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ret":0}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	p := New(config.WechatConfig{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	p.client = wechat.NewClient()
	p.token = &wechat.TokenData{BaseURL: srv.URL, Token: "t"}

	reply := p.makeReplyFunc("u1", "ctx")
	time.Sleep(20 * time.Millisecond)

	if err := reply(context.Background(), plugin.ReplyResult{Text: "hello", Status: "succeeded"}); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(typingStatuses) < 2 {
		t.Fatalf("typing calls = %v, want at least start + cancel", typingStatuses)
	}
	if typingStatuses[0] != wechat.TypingStatusTyping {
		t.Fatalf("first status = %d, want typing", typingStatuses[0])
	}
	if typingStatuses[len(typingStatuses)-1] != wechat.TypingStatusCancel {
		t.Fatalf("last status = %d, want cancel", typingStatuses[len(typingStatuses)-1])
	}
}

func TestGetTypingTicketCachesPerUser(t *testing.T) {
	configCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "getconfig") {
			http.NotFound(w, r)
			return
		}
		configCalls++
		_ = json.NewEncoder(w).Encode(map[string]any{"typing_ticket": "ticket-1"})
	}))
	t.Cleanup(srv.Close)

	p := New(config.WechatConfig{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	p.client = wechat.NewClient()
	p.token = &wechat.TokenData{BaseURL: srv.URL, Token: "t"}

	ctx := context.Background()
	ticket, ok := p.getTypingTicket(ctx, "u1", "ctx")
	if !ok || ticket != "ticket-1" {
		t.Fatalf("first ticket = %q ok=%v", ticket, ok)
	}
	ticket, ok = p.getTypingTicket(ctx, "u1", "ctx")
	if !ok || ticket != "ticket-1" {
		t.Fatalf("cached ticket = %q ok=%v", ticket, ok)
	}
	if configCalls != 1 {
		t.Fatalf("getconfig calls = %d, want 1", configCalls)
	}
}
