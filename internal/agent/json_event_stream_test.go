package agent

import (
	"strings"
	"testing"
)

// Recorded from cursor-agent --stream-partial-output (model auto), ids shortened.
// interaction_query is its approval handshake: not output, so nothing reaches the chat.
const cursorTurn = `{"type":"system","subtype":"init","session_id":"s1","model":"Auto"}
{"type":"thinking","subtype":"delta","text":"Reading","session_id":"s1","timestamp_ms":1}
{"type":"thinking","subtype":"completed","session_id":"s1","timestamp_ms":2}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"先读"}]},"session_id":"s1","timestamp_ms":3}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":" a.md。"}]},"session_id":"s1","timestamp_ms":4}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"先读 a.md。"}]},"session_id":"s1","model_call_id":"m1","timestamp_ms":5}
{"type":"tool_call","subtype":"started","call_id":"c1","tool_call":{"readToolCall":{"args":{"path":"/w/a.md"}}},"session_id":"s1","timestamp_ms":6}
{"type":"tool_call","subtype":"completed","call_id":"c1","tool_call":{"readToolCall":{"args":{"path":"/w/a.md"},"result":{"success":{"content":"# Notes\n","totalLines":2}}}},"session_id":"s1","timestamp_ms":7}
{"type":"interaction_query","subtype":"request","query_type":"webFetchRequestQuery","query":{"id":0,"webFetchRequestQuery":{"args":{"url":"https://example.com"},"skipApproval":false}},"session_id":"s1","timestamp_ms":8}
{"type":"interaction_query","subtype":"response","query_type":"webFetchRequestQuery","response":{"id":0,"webFetchRequestResponse":{"approved":{}}},"session_id":"s1","timestamp_ms":8}
{"type":"tool_call","subtype":"started","call_id":"c2","tool_call":{"shellToolCall":{"args":{"command":"ls","description":"List files"}}},"session_id":"s1","timestamp_ms":8}
{"type":"tool_call","subtype":"completed","call_id":"c2","tool_call":{"shellToolCall":{"args":{"command":"ls"},"result":{"success":{"exitCode":0,"stdout":"a.md\n","stderr":""}}}},"session_id":"s1","timestamp_ms":9}
{"type":"tool_call","subtype":"started","call_id":"c3","tool_call":{"mcpToolCall":{"args":{"name":"hylo-read","args":{"path":"/_memory/p.md"},"toolName":"read","providerIdentifier":"hylo"}}},"session_id":"s1","timestamp_ms":10}
{"type":"tool_call","subtype":"completed","call_id":"c3","tool_call":{"mcpToolCall":{"args":{"toolName":"read","providerIdentifier":"hylo"},"result":{"error":{"error":"not found"}}}},"session_id":"s1","timestamp_ms":11}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"共 2 行。"}]},"session_id":"s1","timestamp_ms":12}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"共 2 行。"}]},"session_id":"s1"}
{"type":"result","subtype":"success","result":"先读 a.md。共 2 行。","session_id":"s1"}
`

func TestJSONEventStream_CursorTurn(t *testing.T) {
	var got []string
	var l TurnLog
	j := NewJSONEventStream("cursor-agent", func(m map[string]any) {
		l.Feed(m)
		switch m["type"] {
		case "text_delta", "thinking_delta":
			got = append(got, m["type"].(string)+":"+m["delta"].(string))
		case "tool_use":
			got = append(got, "use:"+m["name"].(string))
		case "tool_result":
			got = append(got, "result:"+m["content"].(string)+":"+map[bool]string{true: "err", false: "ok"}[m["isError"].(bool)])
		case "raw":
			t.Fatalf("unparsed line: %v", m["line"])
		}
	})
	j.Feed(cursorTurn)
	want := []string{
		"thinking_delta:Reading", "text_delta:先读", "text_delta: a.md。",
		"use:read", "result:# Notes\n:ok", "use:shell", "result:a.md:ok",
		"use:mcp__hylo__read", "result:not found:err", "text_delta:共 2 行。",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
	if l.Answer() != "共 2 行。" {
		t.Fatalf("answer %q", l.Answer())
	}
}

// Without partial output each block arrives whole, once.
func TestJSONEventStream_CursorWholeBlocks(t *testing.T) {
	var text string
	j := NewJSONEventStream("cursor-agent", func(m map[string]any) {
		if m["type"] == "text_delta" {
			text += m["delta"].(string)
		}
	})
	j.Feed(`{"type":"assistant","message":{"content":[{"type":"text","text":"Hello."}]},"session_id":"s1"}` + "\n")
	if text != "Hello." {
		t.Fatalf("%q", text)
	}
}

// A whole block re-sent after a step that landed between its deltas and the resend
// must not repeat the text.
func TestJSONEventStream_CursorWholeBlockAfterStep(t *testing.T) {
	var text string
	j := NewJSONEventStream("cursor-agent", func(m map[string]any) {
		if m["type"] == "text_delta" {
			text += m["delta"].(string)
		}
	})
	j.Feed(`{"type":"assistant","message":{"content":[{"type":"text","text":"Hi"}]},"timestamp_ms":1}
{"type":"tool_call","subtype":"started","call_id":"c1","tool_call":{"readToolCall":{"args":{"path":"/a"}}},"timestamp_ms":2}
{"type":"assistant","message":{"content":[{"type":"text","text":" there"}]},"timestamp_ms":3}
{"type":"assistant","message":{"content":[{"type":"text","text":"Hi"}]},"model_call_id":"m1","timestamp_ms":4}
{"type":"assistant","message":{"content":[{"type":"text","text":" there"}]},"model_call_id":"m2","timestamp_ms":5}
{"type":"assistant","message":{"content":[{"type":"text","text":"Next."}]}}
`)
	if text != "Hi there"+"Next." {
		t.Fatalf("%q", text)
	}
}

// Older cursor-agent builds sent tool_use/tool_result inline as content blocks.
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
