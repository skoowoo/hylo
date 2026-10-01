package mate

import (
	"context"
	"testing"

	"github.com/hardhacker/hylo/internal/plugin"
)

func TestTranslateWechatMessage(t *testing.T) {
	called := false
	e := plugin.Event{
		Type:         plugin.EventWechatMessage,
		Path:         "/wechat/user-1",
		Content:      "hello",
		WechatUserID: "user-1",
		Reply: func(ctx context.Context, result plugin.ReplyResult) error {
			called = true
			if result.Text != "ok" || result.Status != "succeeded" {
				t.Fatalf("result = %+v", result)
			}
			return nil
		},
	}
	out := Translate(e)
	if len(out) != 1 {
		t.Fatalf("got %d events", len(out))
	}
	me := out[0]
	if me.Type != MateEventWechatMessage {
		t.Fatalf("type = %s", me.Type)
	}
	if me.Content != "hello" || me.WechatUserID != "user-1" {
		t.Fatalf("got %+v", me)
	}
	if me.Reply == nil {
		t.Fatal("reply callback not propagated")
	}
	if err := me.Reply(context.Background(), plugin.ReplyResult{Text: "ok", Status: "succeeded"}); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("reply not invoked")
	}
}

func TestRenderPromptWechatUserID(t *testing.T) {
	got := renderPrompt("from {WechatUserID}: {Content}", MateEvent{
		Type:         MateEventWechatMessage,
		WechatUserID: "wx-abc",
		Content:      "hi",
	})
	want := "from wx-abc: hi"
	if got != want {
		t.Fatalf("got %q", got)
	}
}
