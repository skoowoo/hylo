package agent

import (
	"encoding/json"
	"strings"
)

// JSONEventStream parses json-event-stream (Codex / OpenCode / cursor-agent).
type JSONEventStream struct {
	kind   string
	buffer string
	on     func(map[string]any)
	// cursor state
	cursorText         string
	cursorResuming     bool   // true when suppressing session-history replay
	cursorCurrUser     string // composed text sent to agent; marks end of history replay
	openCodeTools      map[string]struct{}
	codexTools         map[string]struct{}
	codexErrEmitted    bool
	cursorToolsEmitted map[string]struct{}
}

func NewJSONEventStream(kind string, on func(map[string]any)) *JSONEventStream {
	return &JSONEventStream{
		kind:               kind,
		on:                 on,
		openCodeTools:      make(map[string]struct{}),
		codexTools:         make(map[string]struct{}),
		cursorToolsEmitted: make(map[string]struct{}),
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
		if line == "" {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			j.on(map[string]any{"type": "raw", "line": line})
			continue
		}
		if !j.dispatch(obj, line) {
			j.on(map[string]any{"type": "raw", "line": line})
		}
	}
}

func (j *JSONEventStream) Flush() {
	s := strings.TrimSpace(j.buffer)
	j.buffer = ""
	if s == "" {
		return
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(s), &obj); err != nil {
		j.on(map[string]any{"type": "raw", "line": s})
		return
	}
	if !j.dispatch(obj, s) {
		j.on(map[string]any{"type": "raw", "line": s})
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
	case "text":
		txt, _ := part["text"].(string)
		if txt != "" {
			j.on(map[string]any{"type": "text_delta", "delta": txt})
		}
		return true
	case "tool_use":
		tool, _ := part["tool"].(string)
		callID, _ := part["callID"].(string)
		if tool != "" && callID != "" {
			key := toString(obj["sessionID"]) + ":" + callID
			if _, ok := j.openCodeTools[key]; !ok {
				j.openCodeTools[key] = struct{}{}
				st, _ := part["state"].(map[string]any)
				j.on(map[string]any{"type": "tool_use", "id": callID, "name": tool, "input": st["input"]})
			}
			st, _ := part["state"].(map[string]any)
			if st != nil && st["status"] == "completed" {
				j.on(map[string]any{"type": "tool_result", "toolUseId": callID, "content": stringifyContent(st["output"]), "isError": false})
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
				j.cursorText = "" // reset so the new response starts fresh
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
		if text == "" {
			return true
		}
		if j.cursorText == "" {
			j.cursorText = text
			j.on(map[string]any{"type": "text_delta", "delta": text})
			return true
		}
		if strings.HasPrefix(text, j.cursorText) {
			d := text[len(j.cursorText):]
			j.cursorText = text
			if d != "" {
				j.on(map[string]any{"type": "text_delta", "delta": d})
			}
			return true
		}
		// New message is not a continuation of the current accumulated text.
		// Two sub-cases:
		//
		// (a) Regression: j.cursorText starts with the incoming text — the cursor
		//     stream rewound (e.g. mid-stream reformat). Hold off; j.cursorText stays
		//     at the longer value so the prefix-diff keeps working once the stream
		//     advances past what was already emitted.
		//
		// (b) Divergence: the snapshot diverges from what was accumulated so far —
		//     cursor reformatted earlier content mid-stream. Emit the full text as a
		//     delta so the client sees it in real time. DB duplication is corrected
		//     later by the text_snapshot emitted on "result".
		if strings.HasPrefix(j.cursorText, text) {
			// Regression: hold off — do not update j.cursorText.
			return true
		}
		// Divergence: cursor reformatted earlier content. Emit a text_replace event so
		// the client replaces (rather than appends) the displayed text, preventing
		// duplication. DB persistence is authoritative via text_snapshot on "result".
		j.cursorText = text
		j.on(map[string]any{"type": "text_replace", "text": text})
		return true
	case "result":
		// Emit the final complete text as a snapshot so the caller can use it for
		// persistent storage, overriding any duplicated/partial deltas that may
		// have accumulated during streaming reformats.
		if j.cursorText != "" {
			j.on(map[string]any{"type": "text_snapshot", "text": j.cursorText})
		}
		j.on(map[string]any{"type": "usage", "usage": obj["usage"], "durationMs": obj["duration_ms"]})
		return true
	}
	return false
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
		if s, ok := p.(string); ok && s != "" {
			return s
		}
	}
	return "error"
}

func stringifyContent(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return jsonStringify(v)
}
