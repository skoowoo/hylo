package agent

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
)

// JSONEventStream parses json-event-stream (Codex / OpenCode / cursor-agent / Agy).
type JSONEventStream struct {
	kind   string
	buffer string
	on     func(map[string]any)
	// cursor state
	cursorStreamed     string // streamed text whose whole block hasn't been re-sent yet
	cursorResuming     bool   // true when suppressing session-history replay
	cursorCurrUser     string // composed text sent to agent; marks end of history replay
	openCodeTools      map[string]struct{}
	codexTools         map[string]struct{}
	codexErrEmitted    bool
	cursorToolsEmitted map[string]struct{}
	agyToolsEmitted    map[string]struct{}
}

func NewJSONEventStream(kind string, on func(map[string]any)) *JSONEventStream {
	return &JSONEventStream{
		kind:               kind,
		on:                 on,
		openCodeTools:      make(map[string]struct{}),
		codexTools:         make(map[string]struct{}),
		cursorToolsEmitted: make(map[string]struct{}),
		agyToolsEmitted:    make(map[string]struct{}),
	}
}

// EnableCursorResumeMode activates history-replay suppression for cursor-agent resumed sessions.
// cursor-agent replays all prior conversation turns when --resume is used; this suppresses
// those replayed assistant messages until the current user turn is detected in the stream.
// composedText is the exact prompt sent to the agent (used to identify the current user turn).
func (j *JSONEventStream) EnableCursorResumeMode(composedText string) {
	if j.kind == "cursor-agent" && composedText != "" {
		j.cursorResuming = true
		j.cursorCurrUser = composedText
	}
}

func (j *JSONEventStream) Feed(chunk string) {
	j.buffer += chunk
	for {
		nl := strings.IndexByte(j.buffer, '\n')
		if nl < 0 {
			return
		}
		line := strings.TrimSpace(j.buffer[:nl])
		j.buffer = j.buffer[nl+1:]
		j.line(line)
	}
}

func (j *JSONEventStream) Flush() {
	s := strings.TrimSpace(j.buffer)
	j.buffer = ""
	j.line(s)
}

// line relays one stdout line. Text that isn't JSON is the CLI talking to a
// person, so it shows; an event the protocol has but we don't use (approval
// handshakes, telemetry) is not output and is dropped.
func (j *JSONEventStream) line(s string) {
	if s == "" {
		return
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(s), &obj); err != nil {
		j.on(map[string]any{"type": "raw", "line": s})
		return
	}
	if !j.dispatch(obj, s) {
		slog.Debug("agent: unhandled event", "agent", j.kind, "type", obj["type"], "subtype", obj["subtype"])
	}
}

func (j *JSONEventStream) dispatch(obj map[string]any, raw string) bool {
	switch j.kind {
	case "opencode":
		return j.openCode(obj, raw)
	case "cursor-agent":
		return j.cursor(obj)
	case "codex":
		return j.codex(obj)
	case "agy":
		return j.agy(obj)
	default:
		return false
	}
}

func (j *JSONEventStream) openCode(obj map[string]any, raw string) bool {
	if sid := toString(obj["sessionID"]); sid != "" {
		j.on(sessionAgentEvent(sid))
	}
	typ, _ := obj["type"].(string)
	part, _ := obj["part"].(map[string]any)
	switch typ {
	case "step_start":
		j.on(map[string]any{"type": "status", "label": "running"})
		return true
	case "reasoning":
		if txt, _ := part["text"].(string); txt != "" {
			j.on(map[string]any{"type": "thinking_delta", "delta": txt})
		}
		return true
	case "text":
		txt, _ := part["text"].(string)
		if txt != "" {
			j.on(map[string]any{"type": "text_delta", "delta": txt})
		}
		return true
	case "tool_use":
		tool, _ := part["tool"].(string)
		callID, _ := part["id"].(string)
		if tool != "" && callID != "" {
			st, _ := part["state"].(map[string]any)
			key := toString(obj["sessionID"]) + ":" + callID
			if _, ok := j.openCodeTools[key]; !ok {
				j.openCodeTools[key] = struct{}{}
				j.on(map[string]any{"type": "tool_use", "id": callID, "name": tool, "input": st["input"]})
			}
			switch st["status"] {
			case "completed":
				j.on(map[string]any{"type": "tool_result", "toolUseId": callID, "content": stringifyContent(st["output"]), "isError": false})
			case "error":
				j.on(map[string]any{"type": "tool_result", "toolUseId": callID, "content": extractErr(st["error"], st["output"], "Tool error"), "isError": true})
			}
		}
		return true
	case "step_finish":
		j.on(map[string]any{"type": "usage", "usage": part["tokens"], "costUsd": part["cost"]})
		return true
	case "error":
		j.on(map[string]any{"type": "error", "message": extractErr(obj["error"], obj["message"], "OpenCode error"), "raw": raw})
		return true
	}
	return false
}

func (j *JSONEventStream) cursor(obj map[string]any) bool {
	typ, _ := obj["type"].(string)
	switch typ {
	case "system":
		if obj["subtype"] == "init" {
			j.on(map[string]any{"type": "status", "label": "initializing", "model": obj["model"]})
			sid := toString(obj["session_id"])
			if sid == "" {
				sid = toString(obj["sessionId"])
			}
			if sid == "" {
				sid = toString(obj["chat_id"])
			}
			if sid != "" {
				j.on(sessionAgentEvent(sid))
			}
		}
		return true
	case "user":
		// cursor-agent echoes user messages; consume them to avoid cluttering console output.
		// When in resume-suppression mode, the current user turn signals end of history replay.
		msg, _ := obj["message"].(map[string]any)
		if j.cursorResuming {
			text := extractCursorText(msg)
			if text != "" && (text == j.cursorCurrUser || strings.Contains(text, j.cursorCurrUser)) {
				j.cursorResuming = false
				j.cursorStreamed = ""
			}
		} else {
			j.emitCursorToolResults(msg)
		}
		return true // always consume; user bubble already shows the user message
	case "assistant":
		if j.cursorResuming {
			return true // suppress replayed historical assistant messages
		}
		msg, _ := obj["message"].(map[string]any)
		j.emitCursorTools(msg)
		text := extractCursorText(msg)
		_, stamped := obj["timestamp_ms"]
		_, whole := obj["model_call_id"]
		if whole || !stamped {
			// A block re-sent whole after its deltas is consumed from what streamed,
			// wherever steps landed in between; only a block that never streamed
			// (no partial output) is new.
			if j.cursorStreamed == "" {
				if text != "" {
					j.on(map[string]any{"type": "text_delta", "delta": text})
				}
			} else if strings.HasPrefix(j.cursorStreamed, text) {
				j.cursorStreamed = j.cursorStreamed[len(text):]
			} else {
				j.cursorStreamed = ""
			}
			return true
		}
		if text != "" {
			j.cursorStreamed += text
			j.on(map[string]any{"type": "text_delta", "delta": text})
		}
		return true
	case "thinking":
		if j.cursorResuming {
			return true
		}
		if t := toString(obj["text"]); t != "" {
			j.on(map[string]any{"type": "thinking_delta", "delta": t})
		}
		return true
	case "tool_call":
		if j.cursorResuming {
			return true
		}
		j.cursorToolCall(obj)
		return true
	case "result":
		if isErr, _ := obj["is_error"].(bool); isErr {
			j.on(map[string]any{"type": "error", "message": extractErr(obj["error"], obj["result"], "Cursor error")})
		}
		j.on(map[string]any{"type": "usage", "usage": obj["usage"], "durationMs": obj["duration_ms"]})
		return true
	case "error":
		j.on(map[string]any{"type": "error", "message": extractErr(obj["message"], obj["error"], "Cursor error")})
		return true
	}
	return false
}

// cursorToolCall relays a tool_call event, shaped {"<kind>ToolCall": {args, result}}.
// MCP calls take Claude's mcp__<server>__<tool> name so both read the same.
func (j *JSONEventStream) cursorToolCall(obj map[string]any) {
	id := toString(obj["call_id"])
	tc, _ := obj["tool_call"].(map[string]any)
	if id == "" || tc == nil {
		return
	}
	var name string
	var call map[string]any
	for k, v := range tc {
		if m, ok := v.(map[string]any); ok && strings.HasSuffix(k, "ToolCall") {
			name, call = strings.TrimSuffix(k, "ToolCall"), m
			break
		}
	}
	if call == nil {
		return
	}
	args, _ := call["args"].(map[string]any)
	input := any(args)
	if name == "mcp" && args != nil {
		name = "mcp__" + toString(args["providerIdentifier"]) + "__" + toString(args["toolName"])
		input = args["args"]
	}
	switch obj["subtype"] {
	case "started":
		if _, seen := j.cursorToolsEmitted[id]; seen {
			return
		}
		j.cursorToolsEmitted[id] = struct{}{}
		j.on(map[string]any{"type": "tool_use", "id": id, "name": name, "input": input})
	case "completed":
		content, isErr := cursorToolResult(call["result"])
		j.on(map[string]any{"type": "tool_result", "toolUseId": id, "content": content, "isError": isErr})
	}
}

func cursorToolResult(v any) (string, bool) {
	r, _ := v.(map[string]any)
	if e, ok := r["error"]; ok {
		if m, ok := e.(map[string]any); ok {
			if s := firstNonEmpty(toString(m["error"]), toString(m["message"]), toString(m["stderr"])); s != "" {
				return s, true
			}
		}
		return stringifyToolResult(e), true
	}
	ok, _ := r["success"].(map[string]any)
	if ok == nil {
		if r == nil {
			return "", false
		}
		return stringifyToolResult(r), false
	}
	if s := toString(ok["content"]); s != "" {
		return s, false
	}
	if out := strings.TrimRight(toString(ok["stdout"])+toString(ok["stderr"]), "\n"); out != "" {
		return out, false
	}
	if c, ok := ok["content"].([]any); ok {
		return stringifyToolResult(c), false
	}
	return stringifyToolResult(ok), false
}

// emitCursorTools relays tool_use blocks from a cursor-agent assistant message.
// cursor-agent's stream-json message shape mirrors Claude's content-block
// format (the same "assistant"/message/content[] the existing text handling
// above already reads), so tool_use blocks arrive inline with text blocks
// rather than as a separate event type.
func (j *JSONEventStream) emitCursorTools(msg map[string]any) {
	arr, _ := msg["content"].([]any)
	for _, x := range arr {
		m, ok := x.(map[string]any)
		if !ok || m["type"] != "tool_use" {
			continue
		}
		id, _ := m["id"].(string)
		if id == "" {
			continue
		}
		if _, seen := j.cursorToolsEmitted[id]; seen {
			continue
		}
		j.cursorToolsEmitted[id] = struct{}{}
		j.on(map[string]any{"type": "tool_use", "id": id, "name": m["name"], "input": m["input"]})
	}
}

// emitCursorToolResults relays tool_result blocks from a cursor-agent user message.
func (j *JSONEventStream) emitCursorToolResults(msg map[string]any) {
	arr, _ := msg["content"].([]any)
	for _, x := range arr {
		m, ok := x.(map[string]any)
		if !ok || m["type"] != "tool_result" {
			continue
		}
		j.on(map[string]any{
			"type": "tool_result", "toolUseId": m["tool_use_id"],
			"content": stringifyToolResult(m["content"]), "isError": m["is_error"],
		})
	}
}

func extractCursorText(msg map[string]any) string {
	arr, _ := msg["content"].([]any)
	var b strings.Builder
	for _, x := range arr {
		m, ok := x.(map[string]any)
		if !ok || m["type"] != "text" {
			continue
		}
		b.WriteString(toString(m["text"]))
	}
	return b.String()
}

func (j *JSONEventStream) codex(obj map[string]any) bool {
	typ, _ := obj["type"].(string)
	switch typ {
	case "error":
		if !j.codexErrEmitted {
			j.codexErrEmitted = true
			j.on(map[string]any{"type": "error", "message": extractErr(obj["message"], obj["error"], "Codex error")})
		}
		return true
	case "turn.failed":
		if !j.codexErrEmitted {
			j.codexErrEmitted = true
			j.on(map[string]any{"type": "error", "message": extractErr(obj["error"], obj["message"], "Codex turn failed")})
		}
		return true
	case "thread.started":
		j.on(map[string]any{"type": "status", "label": "initializing"})
		sid := toString(obj["thread_id"])
		if sid == "" {
			sid = toString(obj["threadId"])
		}
		if sid != "" {
			j.on(sessionAgentEvent(sid))
		}
		return true
	case "turn.started":
		j.on(map[string]any{"type": "status", "label": "running"})
		return true
	case "item.started":
		it, _ := obj["item"].(map[string]any)
		if it["type"] == "command_execution" {
			id, _ := it["id"].(string)
			if id != "" {
				if _, ok := j.codexTools[id]; !ok {
					j.codexTools[id] = struct{}{}
					cmd, _ := it["command"].(string)
					j.on(map[string]any{"type": "tool_use", "id": id, "name": "Bash", "input": map[string]any{"command": cmd}})
				}
			}
		}
		return true
	case "item.completed":
		it, _ := obj["item"].(map[string]any)
		if it["type"] == "command_execution" {
			id, _ := it["id"].(string)
			if id != "" {
				j.on(map[string]any{
					"type": "tool_result", "toolUseId": id,
					"content": stringifyContent(it["aggregated_output"]),
					"isError": toFloat(it["exit_code"]) != 0,
				})
			}
			return true
		}
		if it["type"] == "file_change" {
			id, _ := it["id"].(string)
			changes, _ := it["changes"].([]any)
			notes := make([]any, 0, len(changes))
			for _, c := range changes {
				cm, ok := c.(map[string]any)
				if !ok {
					continue
				}
				path, _ := cm["path"].(string)
				if path == "" {
					continue
				}
				notes = append(notes, map[string]any{"path": path, "action": NoteActionWrite})
			}
			if len(notes) > 0 {
				j.on(map[string]any{"type": "tool_use", "id": id, "name": "codex_file_change", "input": map[string]any{"__notes": notes}})
				j.on(map[string]any{"type": "tool_result", "toolUseId": id, "content": "", "isError": false})
			}
			return true
		}
		if it["type"] == "agent_message" {
			if txt, ok := it["text"].(string); ok && txt != "" {
				j.on(map[string]any{"type": "text_delta", "delta": txt})
			}
		}
		return true
	case "turn.completed":
		j.on(map[string]any{"type": "usage", "usage": obj["usage"]})
		return true
	}
	return false
}

func (j *JSONEventStream) agy(obj map[string]any) bool {
	if sid := toString(obj["conversation_id"]); sid != "" {
		j.on(sessionAgentEvent(sid))
	}
	ev, _ := obj["event"].(string)
	switch ev {
	case "init":
		initObj, _ := obj["init"].(map[string]any)
		var model any
		if initObj != nil {
			model = initObj["model"]
		}
		j.on(map[string]any{"type": "status", "label": "initializing", "model": model})
		return true
	case "step_update":
		su, _ := obj["step_update"].(map[string]any)
		if su == nil {
			return true
		}
		if sid := toString(su["conversation_id"]); sid != "" {
			j.on(sessionAgentEvent(sid))
		}
		switch toString(su["step_type"]) {
		case "agent_response":
			if td := toString(su["thinking_delta"]); td != "" {
				j.on(map[string]any{"type": "thinking_delta", "delta": td})
			}
			if td := toString(su["text_delta"]); td != "" {
				j.on(map[string]any{"type": "text_delta", "delta": td})
			}
		case "tool":
			j.agyToolStep(su)
		}
		return true
	case "result":
		res, _ := obj["result"].(map[string]any)
		if res == nil {
			return true
		}
		if sid := toString(res["conversation_id"]); sid != "" {
			j.on(sessionAgentEvent(sid))
		}
		status := toString(res["status"])
		if (status != "" && status != "SUCCESS") || toString(res["error"]) != "" {
			j.on(map[string]any{"type": "error", "message": extractErr(res["error"], status, "Antigravity error")})
		}
		usageEv := map[string]any{"type": "usage", "usage": res["usage"]}
		if dur := toFloat(res["duration_seconds"]); dur > 0 {
			usageEv["durationMs"] = dur * 1000
		}
		j.on(usageEv)
		return true
	}
	return false
}

func (j *JSONEventStream) agyToolStep(su map[string]any) {
	info, _ := su["tool_info"].(map[string]any)
	name := firstNonEmpty(toString(su["tool_name"]), toString(info["name"]))
	if name == "" {
		return
	}
	var input any
	if info != nil {
		input = info["parameters"]
	}
	if name == "call_mcp_tool" {
		if params, ok := input.(map[string]any); ok {
			srv := firstNonEmpty(toString(params["ServerName"]), toString(params["server_name"]))
			tool := firstNonEmpty(toString(params["ToolName"]), toString(params["tool_name"]))
			if srv != "" && tool != "" {
				name = "mcp__" + srv + "__" + tool
				if args, ok := params["Arguments"]; ok {
					input = args
				} else if args, ok := params["arguments"]; ok {
					input = args
				}
			}
		}
	}
	id := fmt.Sprintf("step-%.0f", toFloat(su["step_index"]))
	if _, seen := j.agyToolsEmitted[id]; !seen {
		j.agyToolsEmitted[id] = struct{}{}
		j.on(map[string]any{"type": "tool_use", "id": id, "name": name, "input": input})
	}
	state := toString(su["state"])
	if state == "DONE" || state == "ERROR" {
		isErr := state == "ERROR" || (info != nil && info["error"] != nil)
		var content string
		if isErr && info != nil {
			content = extractErr(info["error"], info["output"], "Tool error")
		} else if info != nil {
			content = stringifyToolResult(info["output"])
		}
		j.on(map[string]any{"type": "tool_result", "toolUseId": id, "content": content, "isError": isErr})
	}
}

func toFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	default:
		return 0
	}
}

func extractErr(parts ...any) string {
	for _, p := range parts {
		if s := extractErrMsg(p); s != "" {
			return s
		}
	}
	return "error"
}

func extractErrMsg(p any) string {
	switch v := p.(type) {
	case string:
		return v
	case map[string]any:
		for _, k := range []string{"message", "error", "data"} {
			if s := extractErrMsg(v[k]); s != "" {
				return s
			}
		}
	}
	return ""
}

func stringifyContent(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return jsonStringify(v)
}
