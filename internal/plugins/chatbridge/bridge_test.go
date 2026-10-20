package chatbridge

import (
	"context"
	"io"
	"log/slog"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/hardhacker/hylo/internal/plugin"
)

func TestSplitText(t *testing.T) {
	cases := []struct {
		text  string
		limit int
		want  []string
	}{
		{"aaaa\nbbbb", 5, []string{"aaaa", "bbbb"}},
		{"short", 10, []string{"short"}},
		{"abcdef", 4, []string{"abcd", "ef"}},
		// CJK must not be cut mid-rune.
		{"你好世界再见", 4, []string{"你好世界", "再见"}},
		// An emoji takes two UTF-16 units.
		{"ab😀cd", 3, []string{"ab", "😀c", "d"}},
	}
	for _, c := range cases {
		if got := SplitText(c.text, c.limit); !reflect.DeepEqual(got, c.want) {
			t.Errorf("SplitText(%q, %d) = %q, want %q", c.text, c.limit, got, c.want)
		}
	}
}

type fakeApp struct {
	mu   sync.Mutex
	sent []string
}

func (f *fakeApp) SendText(_ context.Context, chatID, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, chatID+":"+text)
	return nil
}

func (f *fakeApp) Format(text string) string { return text }

func (f *fakeApp) Typing(ctx context.Context, _ string) { <-ctx.Done() }

func newTestBridge(owner string) (*Bridge, *fakeApp, *[]plugin.Event) {
	app := &fakeApp{}
	b := New("telegram", "Telegram", 10, owner, app, slog.New(slog.NewTextHandler(io.Discard, nil)))
	var got []plugin.Event
	b.SetDispatch(func(e plugin.Event) { got = append(got, e) })
	return b, app, &got
}

func TestReceiveOnlyFromOwner(t *testing.T) {
	b, _, got := newTestBridge("42")
	b.Receive("42", "7", "hi")
	if len(*got) != 0 {
		t.Fatal("a stranger's message reached the bots")
	}
	b.Receive("42", "42", " ping ")
	if len(*got) != 1 {
		t.Fatal("the owner's message was dropped")
	}
	e := (*got)[0]
	if e.Type != plugin.EventChatMessage || e.Channel != "telegram" || e.Content != "ping" || e.ChatID != "42" || e.UserID != "42" || e.Reply == nil {
		t.Fatalf("event = %+v", e)
	}
}

func TestReceiveWithoutOwnerDropsAll(t *testing.T) {
	b, _, got := newTestBridge("")
	b.Receive("1", "", "hi")
	b.Receive("1", "1", "hi")
	if len(*got) != 0 {
		t.Fatal("no owner must mean no one is answered")
	}
}

func TestReplySplitsAndExplainsUnhandled(t *testing.T) {
	b, app, got := newTestBridge("42")
	b.Receive("42", "42", "a")
	b.Receive("42", "42", "b")
	reply := (*got)[0].Reply
	if err := reply(context.Background(), plugin.ReplyResult{Text: "0123456789abc", Status: "succeeded"}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(app.sent, []string{"42:0123456789", "42:abc"}) {
		t.Fatalf("sent = %q", app.sent)
	}
	app.sent = nil
	b.limit = 200
	(*got)[1].Reply(context.Background(), plugin.ReplyResult{Status: "unhandled"}) //nolint:errcheck
	if len(app.sent) != 1 || !strings.Contains(app.sent[0], "telegram_message") {
		t.Fatalf("sent = %q", app.sent)
	}
}

func TestPlainText(t *testing.T) {
	got := PlainText("**bold** and [link](https://x.com)")
	if strings.Contains(got, "**") || !strings.Contains(got, "link (https://x.com)") {
		t.Fatalf("got %q", got)
	}
}
