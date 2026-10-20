package mate

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hardhacker/hylo/internal/plugin"
)

func TestTranslateChatMessage(t *testing.T) {
	called := false
	e := plugin.Event{
		Type:    plugin.EventChatMessage,
		Channel: "telegram",
		Path:    "/telegram/42",
		Content: "hello",
		UserID:  "42",
		ChatID:  "42",
		Reply: func(ctx context.Context, result plugin.ReplyResult) error {
			called = true
			return nil
		},
	}
	out := Translate(e)
	if len(out) != 1 {
		t.Fatalf("got %d events", len(out))
	}
	me := out[0]
	if me.Type != MateEventTelegramMessage || me.Content != "hello" || me.UserID != "42" || me.ChatID != "42" {
		t.Fatalf("got %+v", me)
	}
	if me.Reply == nil {
		t.Fatal("reply callback not propagated")
	}
	me.Reply(context.Background(), plugin.ReplyResult{}) //nolint:errcheck
	if !called {
		t.Fatal("reply not invoked")
	}
	if out := Translate(plugin.Event{Type: plugin.EventChatMessage, Channel: "irc"}); len(out) != 0 {
		t.Fatal("an unknown channel must not fire anything")
	}
}

func TestRenderPromptChatIDs(t *testing.T) {
	me := MateEvent{Type: MateEventDiscordMessage, UserID: "u1", ChatID: "c1", Content: "hi"}
	if got := renderPrompt("{DiscordUserID}/{DiscordChannelID}: {Content}", me); got != "u1/c1: hi" {
		t.Fatalf("got %q", got)
	}
	if got := renderPrompt("{WechatUserID} {TelegramUserID}", me); got != "u1 u1" {
		t.Fatalf("got %q", got)
	}
}

func TestEventDefsMarkChatEvents(t *testing.T) {
	b, err := json.Marshal(BuiltinEvents)
	if err != nil {
		t.Fatal(err)
	}
	var defs []struct {
		Type string `json:"type"`
		Chat bool   `json:"chat"`
	}
	if err := json.Unmarshal(b, &defs); err != nil {
		t.Fatal(err)
	}
	for _, d := range defs {
		if want := strings.HasSuffix(d.Type, "_message"); d.Chat != want {
			t.Errorf("%s: chat = %v", d.Type, d.Chat)
		}
	}
}

func TestUnansweredChatMessageHearsBack(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "vault"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	r := NewRunner(store, slog.New(slog.NewTextHandler(io.Discard, nil)))

	var got plugin.ReplyResult
	r.dispatch(plugin.Event{
		Type:    plugin.EventChatMessage,
		Channel: "telegram",
		Content: "hi",
		Reply: func(_ context.Context, res plugin.ReplyResult) error {
			got = res
			return nil
		},
	})
	if got.Status != "unhandled" {
		t.Fatalf("reply = %+v, want unhandled", got)
	}
}

func TestChatMessageHearsBackWhenTheRunCannotStart(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "vault"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	r := NewRunner(store, slog.New(slog.NewTextHandler(io.Discard, nil)))
	r.SetRunFunc(func(context.Context, *Mate, string, string, MateEvent, func(RunResult)) {
		t.Error("a trigger without a bot must not run")
	})

	got := make(chan plugin.ReplyResult, 1)
	r.fireTrigger(MateTrigger{ID: "t1", MateID: "gone"}, MateEvent{
		Type:    MateEventTelegramMessage,
		Content: "hi",
		Reply: func(_ context.Context, res plugin.ReplyResult) error {
			got <- res
			return nil
		},
	})
	select {
	case res := <-got:
		if res.Status != "failed" {
			t.Fatalf("reply = %+v, want failed", res)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the chat app never heard back")
	}
}
