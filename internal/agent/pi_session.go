package agent

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

// PiSession wraps `pi --mode rpc` JSONL: commands on stdin, events on stdout.
type PiSession struct {
	fatal   atomic.Bool
	errText string
}

func (p *PiSession) HasFatalError() bool { return p.fatal.Load() }

const (
	piMaxImages     = 10
	piMaxImageBytes = 20 * 1024 * 1024
)

var piImageExt = map[string]string{
	".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg",
	".gif": "image/gif", ".webp": "image/webp",
}

// AttachPiRPC sends a `prompt` command over Pi's JSONL protocol and maps stdout events.
// A raw text prompt is rejected (`Unexpected token ... is not valid JSON`).
func AttachPiRPC(
	stdin io.WriteCloser,
	stdout io.Reader,
	prompt, model string,
	imagePaths []string,
	uploadRoot string,
	emit func(event string, data any),
	onDone func(),
) *PiSession {
	s := &PiSession{}
	emit("agent", map[string]any{"type": "status", "label": "initializing", "model": model})

	var images []map[string]any
	var total int
	for _, pth := range imagePaths {
		if len(images) >= piMaxImages {
			break
		}
		pth = strings.TrimSpace(pth)
		if pth == "" {
			continue
		}
		real, err := filepath.EvalSymlinks(pth)
		if err != nil {
			real = pth
		}
		st, err := os.Stat(real)
		if err != nil || st.IsDir() || !st.Mode().IsRegular() {
			continue
		}
		if uploadRoot != "" {
			root, _ := filepath.EvalSymlinks(uploadRoot)
			if root != "" {
				rp, _ := filepath.EvalSymlinks(real)
				if !strings.HasPrefix(rp, root+string(filepath.Separator)) && rp != root {
					continue
				}
			}
		}
		ext := strings.ToLower(filepath.Ext(real))
		mime, ok := piImageExt[ext]
		if !ok {
			continue
		}
		if total+int(st.Size()) > piMaxImageBytes {
			continue
		}
		data, err := os.ReadFile(real)
		if err != nil {
			continue
		}
		total += len(data)
		images = append(images, map[string]any{
			"type": "image", "data": base64.StdEncoding.EncodeToString(data), "mimeType": mime,
		})
	}

	msg := map[string]any{"id": "1", "type": "prompt", "message": prompt}
	if len(images) > 0 {
		msg["images"] = images
	}
	b, _ := json.Marshal(msg)
	_, _ = stdin.Write(append(b, '\n'))

	started := time.Now()
	sentFirst := false

	go func() {
		defer func() {
			_ = stdin.Close()
			if onDone != nil {
				onDone()
			}
		}()
		sc := bufio.NewScanner(stdout)
		buf := make([]byte, 0, 64*1024)
		sc.Buffer(buf, 8<<20)
		for sc.Scan() {
			line := sc.Bytes()
			var raw map[string]any
			if err := json.Unmarshal(line, &raw); err != nil {
				continue
			}
			if handlePiRecord(s, raw, stdin, emit, started, &sentFirst) {
				return
			}
		}
	}()
	return s
}

// handlePiRecord maps one RPC stdout record. done means the reader should close stdin.
func handlePiRecord(s *PiSession, raw map[string]any, stdin io.Writer, emit func(string, any), started time.Time, sentFirst *bool) (done bool) {
	t, _ := raw["type"].(string)
	switch t {
	case "response":
		if success, _ := raw["success"].(bool); success {
			return false
		}
		s.fatal.Store(true)
		msg, _ := raw["error"].(string)
		if msg == "" {
			msg = "pi command failed"
		}
		emit("agent", map[string]any{"type": "error", "message": msg})
		return true
	case "agent_settled":
		// agent_end can still be followed by retries, compaction, or queued prompts.
		if s.errText != "" {
			s.fatal.Store(true)
		}
		return true
	case "extension_ui_request":
		// Dialog methods block until the client answers. This host has no UI for them.
		replyPiDialog(stdin, raw, emit)
		return false
	default:
		if t == "message_end" {
			notePiAssistant(s, raw)
		}
		mapPi(raw, emit, started, sentFirst)
		return false
	}
}

func notePiAssistant(s *PiSession, raw map[string]any) {
	msg, _ := raw["message"].(map[string]any)
	if msg["role"] != "assistant" {
		return
	}
	if em, _ := msg["errorMessage"].(string); em != "" {
		s.errText = em
		return
	}
	s.errText = ""
}

func replyPiDialog(stdin io.Writer, raw map[string]any, emit func(string, any)) {
	method, _ := raw["method"].(string)
	switch method {
	case "notify":
		if raw["notifyType"] == "error" {
			if msg, _ := raw["message"].(string); msg != "" {
				emit("agent", map[string]any{"type": "error", "message": msg})
			}
		}
		return
	case "select", "confirm", "input", "editor":
	default:
		return
	}
	id, _ := raw["id"].(string)
	if id == "" || stdin == nil {
		return
	}
	b, err := json.Marshal(map[string]any{
		"type": "extension_ui_response", "id": id, "cancelled": true,
	})
	if err != nil {
		return
	}
	_, _ = stdin.Write(append(b, '\n'))
}

func mapPi(raw map[string]any, emit func(string, any), started time.Time, sentFirst *bool) {
	t, _ := raw["type"].(string)
	switch t {
	case "agent_start":
		emit("agent", map[string]any{"type": "status", "label": "working"})
	case "turn_start":
		emit("agent", map[string]any{"type": "status", "label": "thinking"})
	case "turn_end":
		if msg, ok := raw["message"].(map[string]any); ok {
			if u, ok := msg["usage"].(map[string]any); ok {
				emit("agent", map[string]any{"type": "usage", "usage": u, "durationMs": time.Since(started).Milliseconds()})
			}
		}
	case "message_end":
		msg, _ := raw["message"].(map[string]any)
		if msg["role"] != "assistant" {
			return
		}
		if em, _ := msg["errorMessage"].(string); em != "" {
			emit("agent", map[string]any{"type": "error", "message": em})
		}
	case "message_update":
		ev, _ := raw["assistantMessageEvent"].(map[string]any)
		if ev == nil {
			return
		}
		et, _ := ev["type"].(string)
		switch et {
		case "text_delta":
			if d, ok := ev["delta"].(string); ok && d != "" {
				if !*sentFirst {
					*sentFirst = true
					emit("agent", map[string]any{"type": "status", "label": "streaming", "ttftMs": time.Since(started).Milliseconds()})
				}
				emit("agent", map[string]any{"type": "text_delta", "delta": d})
			}
		case "thinking_delta":
			if d, ok := ev["delta"].(string); ok {
				emit("agent", map[string]any{"type": "thinking_delta", "delta": d})
			}
		case "thinking_start":
			emit("agent", map[string]any{"type": "thinking_start"})
		case "error":
			emit("agent", map[string]any{"type": "error", "message": piStreamError(ev)})
		}
	case "tool_execution_start":
		emit("agent", map[string]any{"type": "tool_use", "id": raw["toolCallId"], "name": raw["toolName"], "input": raw["args"]})
	case "tool_execution_end":
		emit("agent", map[string]any{
			"type": "tool_result", "toolUseId": raw["toolCallId"],
			"content": piResultText(raw["result"]), "isError": raw["isError"] == true,
		})
	case "extension_error":
		emit("agent", map[string]any{"type": "error", "message": raw["error"]})
	}
}

func piStreamError(ev map[string]any) string {
	if errObj, ok := ev["error"].(map[string]any); ok {
		if m, _ := errObj["errorMessage"].(string); m != "" {
			return m
		}
	}
	if m, _ := ev["reason"].(string); m != "" && m != "error" {
		return m
	}
	return "pi stream error"
}

func piResultText(result any) string {
	m, ok := result.(map[string]any)
	if !ok || m == nil {
		return ""
	}
	content, ok := m["content"].([]any)
	if !ok {
		return ""
	}
	var parts []string
	for _, c := range content {
		cm, ok := c.(map[string]any)
		if !ok {
			continue
		}
		if text, _ := cm["text"].(string); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n")
}
