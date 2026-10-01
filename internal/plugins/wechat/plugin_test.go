package wechatplugin

import (
	"io"
	"log/slog"
	"sync"
	"testing"

	"github.com/hardhacker/hylo/internal/config"
	"github.com/hardhacker/hylo/internal/plugin"
	"github.com/hardhacker/hylo/internal/wechat"
)

func TestDispatchWechatMessageCarriesReply(t *testing.T) {
	p := New(config.WechatConfig{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	p.token = &wechat.TokenData{BaseURL: "https://example.com", Token: "t"}

	var wg sync.WaitGroup
	wg.Add(1)
	var got plugin.Event
	p.SetDispatch(func(e plugin.Event) {
		got = e
		wg.Done()
	})

	p.dispatchWechatMessage(queuedMsg{userID: "u1", contextToken: "ctx", text: "ping"})
	wg.Wait()

	if got.Type != plugin.EventWechatMessage {
		t.Fatalf("type = %s", got.Type)
	}
	if got.Content != "ping" || got.WechatUserID != "u1" {
		t.Fatalf("got %+v", got)
	}
	if got.Reply == nil {
		t.Fatal("missing reply callback")
	}
}
