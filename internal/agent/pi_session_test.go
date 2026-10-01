package agent

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestHandlePiRecordStream(t *testing.T) {
	s := &PiSession{}
	var events []map[string]any
	emit := func(_ string, data any) {
		m, _ := data.(map[string]any)
		events = append(events, m)
	}
	started := time.Now()
	sent := false
	lines := []map[string]any{
		{"type": "response", "command": "prompt", "success": true, "data": map[string]any{"disposition": "started"}},
		{"type": "agent_start"},
		{"type": "turn_start"},
		{"type": "message_update", "assistantMessageEvent": map[string]any{"type": "text_delta", "contentIndex": 0, "delta": "p"}},
		{"type": "message_update", "assistantMessageEvent": map[string]any{"type": "text_delta", "contentIndex": 0, "delta": "ong"}},
		{"type": "message_end", "message": map[string]any{"role": "assistant", "stopReason": "stop", "content": []any{map[string]any{"type": "text", "text": "pong"}}}},
		{"type": "turn_end", "message": map[string]any{"role": "assistant", "usage": map[string]any{"input": 1, "output": 2}}, "toolResults": []any{}},
		{"type": "agent_end", "messages": []any{}, "willRetry": false},
		{"type": "agent_settled"},
	}
	var doneAt int
	for i, raw := range lines {
		if handlePiRecord(s, raw, nil, emit, started, &sent) {
			doneAt = i
			break
		}
	}
	if doneAt != len(lines)-1 {
		t.Fatalf("stopped at %d, want agent_settled index %d", doneAt, len(lines)-1)
	}
	var text strings.Builder
	var sawUsage bool
	for _, e := range events {
		if e["type"] == "text_delta" {
			text.WriteString(e["delta"].(string))
		}
		if e["type"] == "usage" {
			sawUsage = true
		}
	}
	if text.String() != "pong" {
		t.Fatalf("text = %q", text.String())
	}
	if !sawUsage {
		t.Fatal("missing usage from turn_end")
	}
	if s.HasFatalError() {
		t.Fatal("successful run marked fatal")
	}
}

func TestHandlePiRecordCommandError(t *testing.T) {
	s := &PiSession{}
	var msg string
	emit := func(_ string, data any) {
		m, _ := data.(map[string]any)
		if m["type"] == "error" {
			msg, _ = m["message"].(string)
		}
	}
	sent := false
	done := handlePiRecord(s, map[string]any{
		"type": "response", "command": "parse", "success": false,
		"error": `Failed to parse command: Unexpected token 'h', "hello" is not valid JSON`,
	}, nil, emit, time.Now(), &sent)
	if !done || !s.HasFatalError() {
		t.Fatalf("done=%v fatal=%v", done, s.HasFatalError())
	}
	if !strings.Contains(msg, "not valid JSON") {
		t.Fatalf("error = %q", msg)
	}
}

func TestHandlePiRecordToolAndDialog(t *testing.T) {
	s := &PiSession{}
	var events []map[string]any
	emit := func(_ string, data any) {
		m, _ := data.(map[string]any)
		events = append(events, m)
	}
	var stdin bytes.Buffer
	sent := false
	handlePiRecord(s, map[string]any{
		"type": "tool_execution_end", "toolCallId": "call_1", "toolName": "bash", "isError": false,
		"result": map[string]any{"content": []any{map[string]any{"type": "text", "text": "complete output"}}},
	}, &stdin, emit, time.Now(), &sent)
	if len(events) != 1 || events[0]["content"] != "complete output" || events[0]["isError"] != false {
		t.Fatalf("tool result = %#v", events)
	}
	done := handlePiRecord(s, map[string]any{
		"type": "extension_ui_request", "id": "dlg-1", "method": "confirm", "title": "Allow?",
	}, &stdin, emit, time.Now(), &sent)
	if done {
		t.Fatal("dialog should not end the run")
	}
	if !strings.Contains(stdin.String(), `"cancelled":true`) || !strings.Contains(stdin.String(), `"dlg-1"`) {
		t.Fatalf("dialog reply = %s", stdin.String())
	}
}

func TestHandlePiRecordAssistantErrorIsFatal(t *testing.T) {
	s := &PiSession{}
	sent := false
	emit := func(string, any) {}
	now := time.Now()
	handlePiRecord(s, map[string]any{
		"type": "message_end",
		"message": map[string]any{"role": "assistant", "stopReason": "error", "errorMessage": "boom"},
	}, nil, emit, now, &sent)
	if !handlePiRecord(s, map[string]any{"type": "agent_settled"}, nil, emit, now, &sent) || !s.HasFatalError() {
		t.Fatalf("fatal=%v", s.HasFatalError())
	}

	s = &PiSession{}
	handlePiRecord(s, map[string]any{
		"type": "message_end",
		"message": map[string]any{"role": "assistant", "stopReason": "error", "errorMessage": "boom"},
	}, nil, emit, now, &sent)
	handlePiRecord(s, map[string]any{
		"type": "message_end",
		"message": map[string]any{"role": "assistant", "stopReason": "stop"},
	}, nil, emit, now, &sent)
	handlePiRecord(s, map[string]any{"type": "agent_settled"}, nil, emit, now, &sent)
	if s.HasFatalError() {
		t.Fatal("retried success should clear the assistant error")
	}
}

func TestPiStreamErrorPrefersMessage(t *testing.T) {
	got := piStreamError(map[string]any{
		"type": "error", "reason": "error",
		"error": map[string]any{"errorMessage": "model rejected the request"},
	})
	if got != "model rejected the request" {
		t.Fatalf("got %q", got)
	}
}
