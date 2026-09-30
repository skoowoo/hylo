package agent

import "testing"

// Cursor-agent's stream-json format mirrors Claude's content-block shape
// (same "assistant"/message/content[] the existing text-diff logic already
// reads); tool_use/tool_result blocks arrive inline rather than as a
// distinct event type.
func TestJSONEventStream_CursorToolCalls(t *testing.T) {
	var events []map[string]any
	j := NewJSONEventStream("cursor-agent", func(m map[string]any) { events = append(events, m) })

	j.Feed(`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t1","name":"Read","input":{"file_path":"/vault/a.md"}}]}}` + "\n")
	j.Feed(`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"t1","content":"hi","is_error":false}]}}` + "\n")

	var gotUse, gotResult bool
	for _, e := range events {
		if e["type"] == "tool_use" && e["id"] == "t1" {
			gotUse = true
		}
		if e["type"] == "tool_result" && e["toolUseId"] == "t1" {
			gotResult = true
		}
	}
	if !gotUse {
		t.Fatalf("expected tool_use event, got %+v", events)
	}
	if !gotResult {
		t.Fatalf("expected tool_result event, got %+v", events)
	}
}

func TestJSONEventStream_CursorToolCallsDeduped(t *testing.T) {
	var events []map[string]any
	j := NewJSONEventStream("cursor-agent", func(m map[string]any) { events = append(events, m) })

	line := `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t1","name":"Read","input":{"file_path":"/vault/a.md"}}]}}` + "\n"
	j.Feed(line)
	j.Feed(line) // cursor-agent may resend the same snapshot mid-stream

	n := 0
	for _, e := range events {
		if e["type"] == "tool_use" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("expected exactly one tool_use event, got %d in %+v", n, events)
	}
}

func TestJSONEventStream_CodexFileChange(t *testing.T) {
	var events []map[string]any
	j := NewJSONEventStream("codex", func(m map[string]any) { events = append(events, m) })

	j.Feed(`{"type":"item.completed","item":{"id":"fc1","type":"file_change","changes":[{"path":"/vault/a.md","kind":"update"},{"path":"/vault/b.md","kind":"add"}]}}` + "\n")

	var use map[string]any
	for _, e := range events {
		if e["type"] == "tool_use" {
			use = e
		}
	}
	if use == nil {
		t.Fatalf("expected a tool_use event, got %+v", events)
	}
	input, _ := use["input"].(map[string]any)
	notes, _ := input["__notes"].([]any)
	if len(notes) != 2 {
		t.Fatalf("expected 2 notes, got %+v", notes)
	}
}
