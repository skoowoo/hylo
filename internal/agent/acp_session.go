package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"time"
)

// ACPSession tracks ACP JSON-RPC attachment.
type ACPSession struct {
	fatal     bool
	sessionID string
}

func (s *ACPSession) HasFatalError() bool { return s.fatal }

// SessionID returns the agent-native session id for this ACP run.
func (s *ACPSession) SessionID() string { return s.sessionID }

// AttachACP runs the ACP client protocol (aligned with open-design acp.ts attachAcpSession).
func AttachACP(
	ctx context.Context,
	cmd *exec.Cmd,
	stdin io.WriteCloser,
	stdout io.Reader,
	prompt, cwd, model string,
	resumeSessionID string,
	mcpServers []MCPServer,
	emit func(event string, data any),
) *ACPSession {
	sess := &ACPSession{sessionID: resumeSessionID}
	abs, _ := filepath.Abs(cwd)
	if abs == "" {
		abs, _ = filepath.Abs(".")
	}

	fail := func(msg string) {
		sess.fatal = true
		emit("error", map[string]any{"message": msg})
		_ = cmd.Process.Kill()
	}

	writeLine := func(v any) {
		b, err := json.Marshal(v)
		if err != nil {
			fail(err.Error())
			return
		}
		b = append(b, '\n')
		if _, err := stdin.Write(b); err != nil {
			fail(err.Error())
		}
	}

	sendRPC := func(id float64, method string, params map[string]any) {
		writeLine(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
	}
	sendResult := func(id float64, result map[string]any) {
		writeLine(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
	}

	expectedID := 1
	nextID := float64(2)
	var promptReqID *float64
	var setModelReqID *float64
	var sessionID string
	if resumeSessionID != "" {
		sessionID = resumeSessionID
	}
	emTh, emTok := false, false
	t0 := time.Now()
	toolKinds := map[string]string{}

	go func() {
		sc := bufio.NewScanner(stdout)
		buf := make([]byte, 0, 64*1024)
		sc.Buffer(buf, 8<<20)
		for sc.Scan() {
			line := sc.Bytes()
			var env struct {
				ID     float64         `json:"id"`
				Method string          `json:"method"`
				Params json.RawMessage `json:"params"`
				Result json.RawMessage `json:"result"`
				Error  json.RawMessage `json:"error"`
			}
			if err := json.Unmarshal(line, &env); err != nil {
				continue
			}
			if env.Error != nil {
				fail(fmt.Sprintf("acp rpc error: %s", string(env.Error)))
				return
			}
			if env.Method == "session/request_permission" {
				var p struct {
					Options []map[string]any `json:"options"`
				}
				_ = json.Unmarshal(env.Params, &p)
				var optList []any
				for _, o := range p.Options {
					optList = append(optList, o)
				}
				oid := choosePermissionOutcome(optList)
				if oid == "" {
					fail("permission request")
					return
				}
				sendResult(env.ID, map[string]any{"outcome": map[string]any{"outcome": "selected", "optionId": oid}})
				continue
			}
			if env.Method == "session/update" {
				var p struct {
					Update struct {
						SessionUpdate string `json:"sessionUpdate"`
						Content       struct {
							Text string `json:"text"`
						} `json:"content"`
						ToolCallID string `json:"toolCallId"`
						Kind       string `json:"kind"`
						Status     string `json:"status"`
						Locations  []struct {
							Path string `json:"path"`
						} `json:"locations"`
					} `json:"update"`
				}
				_ = json.Unmarshal(env.Params, &p)
				switch p.Update.SessionUpdate {
				case "agent_thought_chunk":
					if p.Update.Content.Text != "" {
						if !emTh {
							emTh = true
							emit("agent", map[string]any{"type": "thinking_start"})
						}
						emit("agent", map[string]any{"type": "thinking_delta", "delta": p.Update.Content.Text})
					}
				case "agent_message_chunk":
					if p.Update.Content.Text != "" {
						if !emTok {
							emTok = true
							emit("agent", map[string]any{"type": "status", "label": "streaming", "ttftMs": time.Since(t0).Milliseconds()})
						}
						emit("agent", map[string]any{"type": "text_delta", "delta": p.Update.Content.Text})
					}
				case "tool_call", "tool_call_update":
					id := p.Update.ToolCallID
					if id == "" {
						continue
					}
					if p.Update.Kind != "" {
						toolKinds[id] = p.Update.Kind
					}
					if len(p.Update.Locations) > 0 {
						notes := make([]any, 0, len(p.Update.Locations))
						for _, loc := range p.Update.Locations {
							if loc.Path == "" {
								continue
							}
							notes = append(notes, map[string]any{"path": loc.Path, "action": acpNoteAction(toolKinds[id])})
						}
						if len(notes) > 0 {
							emit("agent", map[string]any{
								"type": "tool_use", "id": id, "name": "acp_" + toolKinds[id],
								"input": map[string]any{"__notes": notes},
							})
						}
					}
					if p.Update.Status == "completed" || p.Update.Status == "failed" {
						emit("agent", map[string]any{"type": "tool_result", "toolUseId": id, "isError": p.Update.Status == "failed"})
					}
				}
				continue
			}

			if env.Result == nil {
				continue
			}
			if int(env.ID) != expectedID {
				continue
			}

			if expectedID == 1 {
				if resumeSessionID != "" {
					sendRPC(2, "session/resume", map[string]any{
						"sessionId":  resumeSessionID,
						"cwd":        abs,
						"mcpServers": toACPStdio(mcpServers),
					})
				} else {
					sendRPC(2, "session/new", map[string]any{
						"cwd":        abs,
						"mcpServers": toACPStdio(mcpServers),
					})
				}
				expectedID = 2
				continue
			}
			if expectedID == 2 {
				var sn struct {
					SessionID string `json:"sessionId"`
					Models    struct {
						CurrentModelID string `json:"currentModelId"`
					} `json:"models"`
				}
				if err := json.Unmarshal(env.Result, &sn); err != nil {
					if resumeSessionID != "" {
						fail("session/resume decode")
					} else {
						fail("session/new decode")
					}
					return
				}
				if sn.SessionID != "" {
					sessionID = sn.SessionID
					sess.sessionID = sessionID
					emit("agent", sessionAgentEvent(sessionID))
				}
				if sessionID == "" {
					fail("empty sessionId")
					return
				}
				if sn.Models.CurrentModelID != "" {
					emit("agent", map[string]any{"type": "status", "label": "model", "model": sn.Models.CurrentModelID})
				}
				if model != "" && model != "default" {
					rid := nextID
					nextID++
					setModelReqID = &rid
					expectedID = int(rid)
					sendRPC(rid, "session/set_model", map[string]any{
						"sessionId": sessionID,
						"modelId":   model,
					})
					continue
				}
				pid := nextID
				nextID++
				promptReqID = &pid
				expectedID = int(pid)
				sendRPC(pid, "session/prompt", map[string]any{
					"sessionId": sessionID,
					"prompt":    []any{map[string]any{"type": "text", "text": prompt}},
				})
				continue
			}

			if setModelReqID != nil && env.ID == *setModelReqID {
				setModelReqID = nil
				emit("agent", map[string]any{"type": "status", "label": "model", "model": model})
				pid := nextID
				nextID++
				promptReqID = &pid
				expectedID = int(pid)
				sendRPC(pid, "session/prompt", map[string]any{
					"sessionId": sessionID,
					"prompt":    []any{map[string]any{"type": "text", "text": prompt}},
				})
				continue
			}

			if promptReqID != nil && env.ID == *promptReqID {
				var pr struct {
					Usage json.RawMessage `json:"usage"`
				}
				_ = json.Unmarshal(env.Result, &pr)
				emit("agent", map[string]any{"type": "usage", "usage": pr.Usage, "durationMs": time.Since(t0).Milliseconds()})
				_ = stdin.Close()
				return
			}
		}
		if err := sc.Err(); err != nil {
			fail(err.Error())
		}
	}()

	sendRPC(1, "initialize", map[string]any{
		"protocolVersion": acpProtocolVersion,
		"clientCapabilities": map[string]any{
			"terminal": false,
		},
		"clientInfo": map[string]string{"name": "vaultr", "version": "agent"},
	})

	return sess
}

// acpNoteAction maps an ACP tool_call "kind" (per the Agent Client Protocol
// spec: read/edit/delete/move/search/execute/think/fetch/other) to our
// read/write action.
func acpNoteAction(kind string) string {
	switch kind {
	case "edit", "delete", "move":
		return NoteActionWrite
	default:
		return NoteActionRead
	}
}

func toACPStdio(servers []MCPServer) []map[string]any {
	var out []map[string]any
	for _, s := range servers {
		typ := s.Type
		if typ == "" {
			typ = "stdio"
		}
		out = append(out, map[string]any{
			"type": typ, "name": s.Name, "command": s.Command,
			"args": s.Args, "env": s.Env,
		})
	}
	return out
}

func choosePermissionOutcome(options []any) string {
	for _, o := range options {
		m, ok := o.(map[string]any)
		if !ok {
			continue
		}
		if m["optionId"] == "approve_for_session" {
			return "approve_for_session"
		}
	}
	for _, o := range options {
		m, ok := o.(map[string]any)
		if !ok {
			continue
		}
		if m["kind"] == "allow_always" {
			if s, ok := m["optionId"].(string); ok {
				return s
			}
		}
	}
	for _, o := range options {
		m, ok := o.(map[string]any)
		if !ok {
			continue
		}
		if m["kind"] == "allow_once" {
			if s, ok := m["optionId"].(string); ok {
				return s
			}
		}
	}
	return ""
}
