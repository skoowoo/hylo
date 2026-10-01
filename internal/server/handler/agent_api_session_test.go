package handler

import (
	"testing"

	"github.com/hardhacker/hylo/internal/agent"
)

func TestComposePromptResumeSession(t *testing.T) {
	body := chatBody{
		SystemPrompt: "You are helpful.",
		Message:      "Follow up question",
	}
	full := composePrompt(body, "/vault", false)
	if full == body.Message {
		t.Fatal("expected full composed prompt on first turn")
	}
	if resume := composePrompt(body, "/vault", true); resume != body.Message {
		t.Fatalf("resume prompt = %q, want user message only", resume)
	}
}

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
