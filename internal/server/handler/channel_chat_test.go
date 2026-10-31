package handler

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/hardhacker/hylo/internal/mate"
)

func TestChannelRequestLeavesTheSavedTextAlone(t *testing.T) {
	if got := channelRequest("", "ping"); got != "ping" {
		t.Fatalf("desktop turn changed: %q", got)
	}
	got := channelRequest(mate.ChannelWechat, "ping")
	if !strings.HasSuffix(got, "\n\nping") || !strings.Contains(got, "WeChat") {
		t.Fatalf("wechat turn: %q", got)
	}
}

func TestQueuedChatRunCanBeCanceled(t *testing.T) {
	a := &AgentAPI{}
	release, ok := a.queueChat(context.Background(), "first", "bot", nil)
	if !ok {
		t.Fatal("free chat was refused")
	}
	done := make(chan bool)
	go func() {
		_, ok := a.queueChat(context.Background(), "second", "bot", nil)
		done <- ok
	}()
	for {
		if stop, ok := a.queued.Load("second"); ok {
			a.canceled.Store("second", true)
			stop.(context.CancelFunc)()
			break
		}
		time.Sleep(time.Millisecond)
	}
	if <-done {
		t.Fatal("a canceled run still got the chat")
	}
	release()
	if _, ok := a.queueChat(context.Background(), "third", "bot", nil); !ok {
		t.Fatal("chat stayed busy after release")
	}
}
