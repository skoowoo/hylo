package handler

import (
	"log/slog"
	"testing"

	"github.com/hardhacker/hylo/internal/agent"
	"github.com/hardhacker/hylo/internal/mate"
)

func TestResolveTriggerSession(t *testing.T) {
	sid, first := resolveTriggerSession(agent.GetAgentDef("claude"))
	if sid == "" || !first {
		t.Fatalf("claude trigger session = %q first=%v", sid, first)
	}
	sid2, first2 := resolveTriggerSession(agent.GetAgentDef("codex"))
	if sid2 != "" || first2 {
		t.Fatalf("codex trigger session should be empty, got %q first=%v", sid2, first2)
	}
}

func TestHostAssignsSessionID(t *testing.T) {
	if !agent.HostAssignsSessionID("claude") {
		t.Fatal("claude should host-assign session id")
	}
	if agent.HostAssignsSessionID("codex") {
		t.Fatal("codex should capture session id from stream")
	}
}

func TestAgentSessionIsBoundToCwd(t *testing.T) {
	store, err := mate.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	a := &AgentAPI{store: store, logger: slog.Default()}
	conv, err := store.CreateConversation("m", "")
	if err != nil {
		t.Fatal(err)
	}
	def := agent.GetAgentDef("claude")
	sid, first, _ := a.resolveAgentSession(conv.ID, def, "/v")
	if sid == "" || !first {
		t.Fatalf("first session = %q first=%v", sid, first)
	}
	if again, _, resume := a.resolveAgentSession(conv.ID, def, "/v"); again != sid || !resume {
		t.Fatalf("same cwd should resume %q, got %q resume=%v", sid, again, resume)
	}
	if moved, first, _ := a.resolveAgentSession(conv.ID, def, "/v/_bots/_news"); moved == sid || !first {
		t.Fatalf("another cwd should start a new session, got %q first=%v", moved, first)
	}
}
