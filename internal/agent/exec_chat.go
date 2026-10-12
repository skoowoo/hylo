package agent

import (
	"context"
	"io"
	"os/exec"
	"strings"
	"sync"
)

// sessionTracker collects the first agent-native session id from stream events.
type sessionTracker struct {
	mu sync.Mutex
	id string
}

func (t *sessionTracker) note(id string) {
	if id == "" {
		return
	}
	t.mu.Lock()
	if t.id == "" {
		t.id = id
	}
	t.mu.Unlock()
}

func (t *sessionTracker) get() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.id
}

func trackSessionFromAgentEvent(m map[string]any, tr *sessionTracker) {
	if typ, _ := m["type"].(string); typ != AgentEventSession {
		return
	}
	if id, _ := m["sessionId"].(string); id != "" {
		tr.note(id)
	}
}

// ChatProcess holds handles after ExecChat spawns a child.
type ChatProcess struct {
	Cmd *exec.Cmd
	ACP *ACPSession
	Pi  *PiSession
	// SessionID is the agent-native session id observed or assigned for this run.
	SessionID string
}

// ExecChat spawns the agent CLI and blocks until the process exits.
// sessionID/firstSession mirror BuildArgsContext session fields for protocol adapters.
// onStart, if non-nil, is called once the subprocess has been started (before blocking),
// allowing the caller to register the process for cancellation before it finishes.
func ExecChat(
	ctx context.Context,
	def *AgentDef,
	bin string,
	argv, env []string,
	cwd, composed, model string,
	sessionID string,
	firstSession bool,
	mcp []MCPServer,
	safeImagePaths []string,
	uploadRoot string,
	emit func(event string, data any),
	onStart func(*ChatProcess),
) (*ChatProcess, error) {
	cmd := exec.CommandContext(ctx, bin, argv...)
	cmd.Dir = cwd
	cmd.Env = env

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	cp := &ChatProcess{Cmd: cmd}
	if onStart != nil {
		onStart(cp)
	}
	if firstSession && sessionID != "" {
		cp.SessionID = sessionID
	}
	tracker := &sessionTracker{id: cp.SessionID}

	relay := func(m map[string]any) {
		trackSessionFromAgentEvent(m, tracker)
		emit("agent", m)
	}

	go func() {
		buf := make([]byte, 32*1024)
		for {
			n, err := stderr.Read(buf)
			if chunk := dropExpectedNoise(def.ID, string(buf[:n])); chunk != "" {
				emit("stderr", map[string]any{"chunk": chunk})
			}
			if err != nil {
				break
			}
		}
	}()

	streamFormat := def.StreamFormat
	if streamFormat == "" {
		streamFormat = StreamPlain
	}

	resumeACP := ""
	if streamFormat == StreamACPJSONRPC && sessionID != "" && !firstSession {
		resumeACP = sessionID
	}

	switch streamFormat {
	case StreamPiRPC:
		cp.Pi = AttachPiRPC(stdin, stdout, composed, model, safeImagePaths, uploadRoot, emit, nil)

	case StreamACPJSONRPC:
		cp.ACP = AttachACP(ctx, cmd, stdin, stdout, composed, cwd, model, resumeACP, mcp, emit)

	default:
		pr := streamFormat
		var pumpWg sync.WaitGroup
		pumpWg.Add(1)
		go func() {
			defer pumpWg.Done()
			switch pr {
			case StreamClaudeJSON:
				cs := NewClaudeStream(relay)
				_, _ = io.Copy(&writerFunc{func(p []byte) { cs.Feed(string(p)) }}, stdout)
				cs.Flush()
			case StreamJSONEvent:
				kind := def.EventParser
				if kind == "" {
					kind = def.ID
				}
				js := NewJSONEventStream(kind, relay)
				// For cursor-agent resumed sessions, suppress replayed history until
				// the current user message appears in the stream.
				if sessionID != "" && !firstSession {
					js.EnableCursorResumeMode(composed)
				}
				_, _ = io.Copy(&writerFunc{func(p []byte) { js.Feed(string(p)) }}, stdout)
				js.Flush()
			case StreamCopilotJSON:
				co := NewCopilotStream(relay)
				_, _ = io.Copy(&writerFunc{func(p []byte) { co.Feed(string(p)) }}, stdout)
				co.Flush()
			default:
				_, _ = io.Copy(&writerFunc{func(p []byte) {
					emit("stdout", map[string]any{"chunk": string(p)})
				}}, stdout)
			}
		}()
		if def.PromptViaStdin {
			_, _ = io.WriteString(stdin, composed)
		}
		_ = stdin.Close()
		pumpWg.Wait()
	}

	err = cmd.Wait()

	if cp.ACP != nil && cp.ACP.SessionID() != "" {
		cp.SessionID = cp.ACP.SessionID()
	} else if sid := tracker.get(); sid != "" {
		cp.SessionID = sid
	}

	return cp, err
}

type writerFunc struct{ f func([]byte) }

func (w *writerFunc) Write(p []byte) (int, error) {
	w.f(p)
	return len(p), nil
}

// dropExpectedNoise drops stderr lines that only echo what Hylo asked for: Pi warns
// on every first turn that the session id Hylo assigned is new.
func dropExpectedNoise(agentID, chunk string) string {
	if agentID != "pi" || !strings.Contains(chunk, "No project session found") {
		return chunk
	}
	var kept []string
	for _, line := range strings.SplitAfter(chunk, "\n") {
		if !strings.Contains(line, "No project session found") {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "")
}
