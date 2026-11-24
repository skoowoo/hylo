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

func TestJSONEventStream_OpenCodeV2Turn(t *testing.T) {
	stream := `{"type":"step_start","sessionID":"ses_1","part":{"type":"step-start"}}
{"type":"reasoning","sessionID":"ses_1","part":{"type":"reasoning","text":"Checking directory"}}
{"type":"tool_use","sessionID":"ses_1","part":{"partID":"prt_1","type":"tool","id":"call_1","tool":"shell","state":{"status":"completed","input":{"command":"pwd"},"output":"/vault\n"}}}
{"type":"tool_use","sessionID":"ses_1","part":{"partID":"prt_2","type":"tool","id":"call_2","tool":"read","state":{"status":"error","input":{"path":"/missing.md"},"error":"File not found: /missing.md"}}}
{"type":"text","sessionID":"ses_1","part":{"type":"text","text":"Done."}}
{"type":"step_finish","sessionID":"ses_1","part":{"type":"step-finish","cost":0.01,"tokens":{"input":10,"output":5}}}
{"type":"error","sessionID":"ses_1","error":{"type":"provider.no-route","message":"Model unavailable: bad/model"}}
`
	var got []string
	var sessionID string
	j := NewJSONEventStream("opencode", func(m map[string]any) {
		switch m["type"] {
		case AgentEventSession:
			sessionID = m["sessionId"].(string)
		case "thinking_delta", "text_delta":
			got = append(got, m["type"].(string)+":"+m["delta"].(string))
		case "tool_use":
			got = append(got, "use:"+m["id"].(string)+":"+m["name"].(string))
		case "tool_result":
			got = append(got, "result:"+m["toolUseId"].(string)+":"+m["content"].(string)+":"+map[bool]string{true: "err", false: "ok"}[m["isError"].(bool)])
		case "error":
			got = append(got, "error:"+m["message"].(string))
		case "raw":
			t.Fatalf("unparsed line: %v", m["line"])
		}
	})
	j.Feed(stream)
	if sessionID != "ses_1" {
		t.Fatalf("sessionID = %q", sessionID)
	}
	want := []string{
		"thinking_delta:Checking directory",
		"use:call_1:shell",
		"result:call_1:/vault\n:ok",
		"use:call_2:read",
		"result:call_2:File not found: /missing.md:err",
		"text_delta:Done.",
		"error:Model unavailable: bad/model",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
}

func TestAgyEvents(t *testing.T) {
	stream := strings.Join([]string{
		`{"event":"init","conversation_id":"conv-1","init":{"cwd":"/vault","model":"gemini-4-neon-medium","permission_mode":"always-proceed"}}`,
		`{"event":"step_update","step_update":{"conversation_id":"conv-1","step_index":0,"state":"DONE","step_type":"user_input"}}`,
		`{"event":"step_update","step_update":{"conversation_id":"conv-1","step_index":1,"state":"ACTIVE","step_type":"tool","tool_name":"run_command","tool_info":{"name":"run_command","parameters":{"CommandLine":"echo hi"}}}}`,
		`{"event":"step_update","step_update":{"conversation_id":"conv-1","step_index":1,"state":"DONE","step_type":"tool","tool_name":"run_command","tool_info":{"name":"run_command","parameters":{"CommandLine":"echo hi"},"output":"hi\n"}}}`,
		`{"event":"step_update","step_update":{"conversation_id":"conv-1","step_index":2,"state":"DONE","step_type":"tool","tool_name":"view_file","tool_info":{"name":"view_file","parameters":{"AbsolutePath":"/missing.md"},"error":{"type":"not_found","message":"file missing"}}}}`,
		`{"event":"step_update","step_update":{"conversation_id":"conv-1","step_index":3,"state":"ACTIVE","step_type":"tool","tool_name":"call_mcp_tool","tool_info":{"name":"call_mcp_tool","parameters":{"ServerName":"hylo","ToolName":"read","Arguments":{"path":"/a.md"}}}}}`,
		`{"event":"step_update","step_update":{"conversation_id":"conv-1","step_index":3,"state":"DONE","step_type":"tool","tool_name":"call_mcp_tool","tool_info":{"name":"call_mcp_tool","parameters":{"ServerName":"hylo","ToolName":"read","Arguments":{"path":"/a.md"}},"output":"ok"}}}`,
		`{"event":"step_update","step_update":{"conversation_id":"conv-1","step_index":4,"state":"ACTIVE","step_type":"agent_response","text_delta":"Hel"}}`,
		`{"event":"step_update","step_update":{"conversation_id":"conv-1","step_index":4,"state":"DONE","step_type":"agent_response","text_delta":"lo"}}`,
		`{"event":"result","result":{"conversation_id":"conv-1","status":"SUCCESS","response":"Hello","duration_seconds":1.5,"num_turns":1,"usage":{"input_tokens":10,"output_tokens":2}}}`,
	}, "\n") + "\n"

	var sessionID string
	var got []string
	j := NewJSONEventStream("agy", func(m map[string]any) {
		switch m["type"] {
		case AgentEventSession:
			sessionID = m["sessionId"].(string)
		case "status":
			got = append(got, "status:"+m["label"].(string))
		case "text_delta":
			got = append(got, "text:"+m["delta"].(string))
		case "tool_use":
			got = append(got, "use:"+m["id"].(string)+":"+m["name"].(string))
		case "tool_result":
			got = append(got, "result:"+m["toolUseId"].(string)+":"+m["content"].(string)+":"+map[bool]string{true: "err", false: "ok"}[m["isError"].(bool)])
		case "usage":
			got = append(got, "usage")
		case "error":
			got = append(got, "error:"+m["message"].(string))
		case "raw":
			t.Fatalf("unparsed line: %v", m["line"])
		}
	})
	j.Feed(stream)
	if sessionID != "conv-1" {
		t.Fatalf("sessionID = %q", sessionID)
	}
	want := []string{
		"status:initializing",
		"use:step-1:run_command",
		"result:step-1:hi\n:ok",
		"use:step-2:view_file",
		"result:step-2:file missing:err",
		"use:step-3:mcp__hylo__read",
		"result:step-3:ok:ok",
		"text:Hel",
		"text:lo",
		"usage",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
}
