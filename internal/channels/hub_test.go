package channels

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hardhacker/hylo/internal/config"
	"github.com/hardhacker/hylo/internal/mate"
)

func TestPlanRestartsWhenTheTokenChanges(t *testing.T) {
	old := mate.Channel{ID: "1", Kind: mate.ChannelTelegram, Token: "a", OwnerUserID: "9", ExternalID: "b"}
	have := map[string]string{old.ID: fingerprint(old)}
	next := old
	next.Token = "b"
	stop, start := plan(have, []mate.Channel{next})
	if len(stop) != 1 || stop[0] != "1" || len(start) != 1 || start[0].Token != "b" {
		t.Fatalf("stop %v start %+v", stop, start)
	}
	stop, start = plan(map[string]string{old.ID: fingerprint(old)}, []mate.Channel{old})
	if len(stop) != 0 || len(start) != 0 {
		t.Fatalf("unchanged: stop %v start %+v", stop, start)
	}
}

type fakeChat struct {
	mu     sync.Mutex
	calls  int
	text   string
	images []string
}

func (f *fakeChat) Deliver(_ context.Context, _, _, text string, images []string) (string, string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.text = text
	f.images = append([]string(nil), images...)
	return "Hello **there**", "succeeded"
}

func (f *fakeChat) state() (calls int, text string, images []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls, f.text, append([]string(nil), f.images...)
}

type fakeOut struct {
	sent []string
}

func (f *fakeOut) SendText(_ context.Context, _, text string) error {
	f.sent = append(f.sent, text)
	return nil
}

func (f *fakeOut) Typing(context.Context, string) {}

func TestHandleJoinsTheCurrentChat(t *testing.T) {
	store, err := mate.Open(filepath.Join(t.TempDir(), "vault"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	m := &mate.Mate{Name: "A", AgentID: "pi", Enabled: true}
	if err := store.CreateMate(m); err != nil {
		t.Fatal(err)
	}
	conv, err := store.CreateConversation(m.ID, "desk")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.InsertMessage(mate.Message{ConversationID: conv.ID, Role: "user", Content: "hi"}); err != nil {
		t.Fatal(err)
	}

	chat := &fakeChat{}
	h := NewHub(store, t.TempDir(), "", nil, config.Proxy{})
	h.SetChatter(chat)
	ch := mate.Channel{MateID: m.ID, Kind: mate.ChannelWechat, OwnerUserID: "owner"}
	out := &fakeOut{}

	h.handle(context.Background(), ch, "stranger", "stranger", "nope", nil, out)
	if chat.calls != 0 || len(out.sent) != 0 {
		t.Fatal("stranger was answered")
	}

	h.handle(context.Background(), ch, "owner", "owner", "ping", nil, out)
	if chat.calls != 1 || chat.text != "ping" {
		t.Fatalf("deliver %+v", chat)
	}
	if len(out.sent) != 1 || strings.Contains(out.sent[0], "**") {
		t.Fatalf("sent %#v", out.sent)
	}
	id, err := store.GetOrCreateActiveChatConvID(m.ID)
	if err != nil || id != conv.ID {
		t.Fatalf("conversation %s, want %s (%v)", id, conv.ID, err)
	}

	h.handle(context.Background(), ch, "owner", "owner", "/new", nil, out)
	if chat.calls != 1 {
		t.Fatal("/new ran the agent")
	}
	id, err = store.GetOrCreateActiveChatConvID(m.ID)
	if err != nil || id == conv.ID {
		t.Fatalf("new conversation %s", id)
	}
	if out.sent[len(out.sent)-1] != "新对话已开启" {
		t.Fatalf("reset reply %#v", out.sent)
	}
}

func TestSweepKeepsOnlyBoundState(t *testing.T) {
	root := filepath.Join(t.TempDir(), "vault")
	store, err := mate.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	m := &mate.Mate{Name: "A", AgentID: "pi"}
	if err := store.CreateMate(m); err != nil {
		t.Fatal(err)
	}
	ch, err := store.SaveChannel(mate.Channel{MateID: m.ID, Kind: mate.ChannelWechat, Token: "t", OwnerUserID: "o", ExternalID: "x"})
	if err != nil {
		t.Fatal(err)
	}
	h := NewHub(store, root, "", nil, config.Proxy{})
	for _, id := range []string{ch.ID, "gone"} {
		if err := os.MkdirAll(h.stateDir(id), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	h.sweep()
	if _, err := os.Stat(h.stateDir(ch.ID)); err != nil {
		t.Fatalf("a disabled bot's binding lost its state: %v", err)
	}
	if _, err := os.Stat(h.stateDir("gone")); !os.IsNotExist(err) {
		t.Fatalf("stale state kept: %v", err)
	}
}

type fakeLink struct {
	chat string
	sent chan string
}

func (f *fakeLink) SendText(_ context.Context, chatID, text string) error {
	f.sent <- chatID + ":" + text
	return nil
}
func (f *fakeLink) Typing(context.Context, string) {}
func (f *fakeLink) Run(ctx context.Context) error  { <-ctx.Done(); return nil }

type fakePushLink struct{ fakeLink }

func (f *fakePushLink) OwnerChat(context.Context) (string, error) { return f.chat, nil }

func TestPushWakeReachesOnlyThisBotsPushableApps(t *testing.T) {
	h := NewHub(nil, t.TempDir(), "", nil, config.Proxy{})
	tg := &fakePushLink{fakeLink{chat: "tg", sent: make(chan string, 4)}}
	wx := &fakeLink{chat: "wx", sent: make(chan string, 4)}
	other := &fakePushLink{fakeLink{chat: "other", sent: make(chan string, 4)}}
	add := func(id, mateID, kind string, link runner) {
		h.running[id] = &live{ch: mate.Channel{ID: id, MateID: mateID, Kind: kind}, link: link, ctx: t.Context()}
	}
	add("1", "a", mate.ChannelTelegram, tg)
	add("2", "a", mate.ChannelWechat, wx)
	add("3", "b", mate.ChannelDiscord, other)

	h.PushWake("a", "Time to **review**")
	if got := <-tg.sent; got != "tg:Time to review" {
		t.Fatalf("telegram got %q", got)
	}
	h.PushWake("a", "  ")
	select {
	case got := <-wx.sent:
		t.Fatalf("wechat can't push, got %q", got)
	case got := <-other.sent:
		t.Fatalf("another bot's app got %q", got)
	case got := <-tg.sent:
		t.Fatalf("blank reply pushed: %q", got)
	case <-time.After(50 * time.Millisecond):
	}
}
