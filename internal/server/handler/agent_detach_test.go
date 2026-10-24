package handler

import (
	"context"
	"testing"
	"time"
)

func TestDetachedRunEndsWithServer(t *testing.T) {
	life, end := context.WithCancel(context.Background())
	a := &AgentAPI{}
	a.SetLifetime(life)

	req, cancelReq := context.WithCancel(context.Background())
	started := make(chan struct{})
	a.detach(req, func(ctx context.Context) {
		close(started)
		<-ctx.Done()
	})
	<-started
	cancelReq()

	waited := make(chan struct{})
	go func() {
		a.Wait()
		close(waited)
	}()
	select {
	case <-waited:
		t.Fatal("run ended with its request")
	case <-time.After(50 * time.Millisecond):
	}

	end()
	select {
	case <-waited:
	case <-time.After(2 * time.Second):
		t.Fatal("run outlived the server")
	}
}
