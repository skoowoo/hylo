package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/hardhacker/vaultr/internal/agent"
	"github.com/hardhacker/vaultr/internal/config"
	"github.com/hardhacker/vaultr/internal/mate"
	"github.com/hardhacker/vaultr/internal/plugin"
	"github.com/hardhacker/vaultr/internal/storage"
)

// AgentAPI serves Open Design–compatible /api/agents, /api/chat, and /api/runs.
type AgentAPI struct {
	logger     *slog.Logger
	cfg        *config.Config
	vault      *storage.Vault
	hub        *agent.Hub
	store      *mate.Store // nil when store is unavailable; handles both mate config and chat history
	agentCache *agent.AgentCache
	active     sync.Map // runID -> *agent.ChatProcess
	canceled   sync.Map // runID -> true; set by RunCancelPOST so executeChat can report "canceled" instead of "failed" for a user-initiated stop
	pathRuns   sync.Map // vault path -> runID; set by FireTriggerRun for path-bearing events
}

// NewAgentAPI constructs the agent HTTP surface.
func NewAgentAPI(logger *slog.Logger, cfg *config.Config, vault *storage.Vault, hub *agent.Hub, store *mate.Store, cache *agent.AgentCache) *AgentAPI {
	return &AgentAPI{logger: logger, cfg: cfg, vault: vault, hub: hub, store: store, agentCache: cache}
}

func (a *AgentAPI) uploadRoot() string {
	sub := strings.TrimSpace(a.cfg.Agent.UploadDir)
	if sub == "" {
		sub = "_agent_uploads"
	}
	clean := strings.TrimLeft(sub, "/")
	return filepath.Join(a.vault.Root(), clean)
}

type chatBody struct {
	AgentID            string            `json:"agentId"`
	MateID             string            `json:"mateId"` // when set, server resolves agent/model/cwd/systemPrompt from mate
	Message            string            `json:"message"`
	SystemPrompt       string            `json:"systemPrompt"`
	Model              string            `json:"model"`
	Reasoning          string            `json:"reasoning"`
	Cwd                string            `json:"cwd"`
	ImagePaths         []string          `json:"imagePaths"`
	ExtraAllowedDirs   []string          `json:"extraAllowedDirs"`
	MCPServers         []agent.MCPServer `json:"mcpServers"`
	AgentCliEnv        map[string]string `json:"agentCliEnv"`
	ProjectID          string            `json:"projectId"`
	ConversationID     string            `json:"conversationId"`
	UserMessageID      string            `json:"userMessageId"`      // pre-allocated by client (optional)
	AssistantMessageID string            `json:"assistantMessageId"` // pre-allocated by client (optional)
	TriggerRun         bool              `json:"triggerRun"`         // when true, skip persisting the user prompt message
	TriggerReply       bool              `json:"triggerReply"`       // when true, trigger run with reply callback — uses persistent agent session like chat
	TriggerEvent       string            `json:"triggerEvent"`       // mate event type label stored on assistant message
	eventReply         plugin.ReplyFunc  // optional; invoked when trigger run completes
}

// AgentsGET handles GET /api/agents.
// Accepts ?force=true to bypass the cache and re-detect synchronously.
func (a *AgentAPI) AgentsGET(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	force := r.URL.Query().Get("force") == "true"
	res := a.agentCache.Get(r.Context(), force)
	respondJSON(w, http.StatusOK, map[string]any{
		"agents":    res.Agents,
		"fromCache": res.FromCache,
		"stale":     res.Stale,
		"fetchedAt": res.FetchedAt.UnixMilli(),
	})
}

// ChatPOST handles POST /api/chat (SSE response).
func (a *AgentAPI) ChatPOST(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body chatBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	meta := map[string]string{
		"projectId": body.ProjectID, "conversationId": body.ConversationID, "agentId": body.AgentID,
	}
	run := a.hub.CreateRun(meta)
	run.AgentID = body.AgentID
	a.logger.Info("agent chat",
		slog.String("runId", run.ID),
		slog.String("agentId", body.AgentID),
		slog.String("streaming", "sse"))
	// Use WithoutCancel so the agent subprocess is not killed when the SSE
	// connection closes (e.g. proxy timeout, client reconnect). The run is
	// tracked in the hub and can be cancelled via POST /api/runs/:id/cancel.
	go a.executeChat(context.WithoutCancel(r.Context()), run, body)
	a.hub.StreamSSE(w, r, run)
}

// RunsPOST handles POST /api/runs — returns 202 and runId, starts chat in background.
func (a *AgentAPI) RunsPOST(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body chatBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	meta := map[string]string{
		"projectId": body.ProjectID, "conversationId": body.ConversationID, "agentId": body.AgentID,
	}
	run := a.hub.CreateRun(meta)
	run.AgentID = body.AgentID
	a.logger.Info("agent run queued",
		slog.String("runId", run.ID),
		slog.String("agentId", body.AgentID))
	go a.executeChat(r.Context(), run, body)
	respondJSON(w, http.StatusAccepted, map[string]any{"runId": run.ID})
}

// RunEventsGET handles GET /api/runs/:id/events (SSE).
func (a *AgentAPI) RunEventsGET(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	run := a.hub.Get(id)
	if run == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	a.hub.StreamSSE(w, r, run)
}

// RunGET handles GET /api/runs/:id (JSON status).
func (a *AgentAPI) RunGET(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	run := a.hub.Get(id)
	if run == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	respondJSON(w, http.StatusOK, run.StatusJSON())
}

// RunsActiveGET handles GET /api/runs/active — returns the count of currently running agent processes.
func (a *AgentAPI) RunsActiveGET(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"count": a.hub.ActiveCount()})
}

// RunCancelPOST handles POST /api/runs/:id/cancel.
func (a *AgentAPI) RunCancelPOST(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if a.hub.Get(id) == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	// Marked before the kill so executeChat's post-exit status check
	// (a.canceled.LoadAndDelete) can tell a user-initiated stop apart from a
	// genuine crash and report "canceled" instead of "failed".
	a.canceled.Store(id, true)
	if v, ok := a.active.Load(id); ok {
		if cp, ok := v.(*agent.ChatProcess); ok && cp.Cmd != nil && cp.Cmd.Process != nil {
			_ = cp.Cmd.Process.Kill()
		}
	}
	a.logger.Info("agent run cancel", slog.String("runId", id))
	respondJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// RunByRefGET handles GET /api/runs/by-ref?path=VAULT_PATH.
// Returns the status of the most-recent run registered for a vault path, or
// {"status":"idle"} when no run is found. Callers can poll this endpoint after
// POST /api/compile/trigger to track progress without an SSE connection.
func (a *AgentAPI) RunByRefGET(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "path required", http.StatusBadRequest)
		return
	}
	v, ok := a.pathRuns.Load(path)
	if !ok {
		respondJSON(w, http.StatusOK, map[string]any{"status": "idle"})
		return
	}
	runID, _ := v.(string)
	run := a.hub.Get(runID)
	if run == nil {
		respondJSON(w, http.StatusOK, map[string]any{"status": "idle"})
		return
	}
	respondJSON(w, http.StatusOK, run.StatusJSON())
}

// FireTriggerRun is called by the mate runner to launch a background agent run.
// Messages are appended to the mate's active chat conversation; each run uses a fresh agent session.
// When ev.Reply is set it is invoked once after the run reaches a terminal state.
// onDone is called exactly once when the run reaches a terminal state.
func (a *AgentAPI) FireTriggerRun(ctx context.Context, m *mate.Mate, convID, prompt string, ev mate.MateEvent, onDone func(mate.RunResult)) {
	meta := map[string]string{
		"mateId": m.ID, "conversationId": convID,
	}
	run := a.hub.CreateRun(meta)
	run.AgentID = m.AgentID
	if ev.Path != "" {
		a.pathRuns.Store(ev.Path, run.ID)
	}
	body := chatBody{
		MateID:         m.ID,
		AgentID:        m.AgentID,
		Message:        prompt,
		SystemPrompt:   m.SystemPrompt,
		Model:          m.Model,
		Cwd:            m.Cwd,
		ConversationID: convID,
		TriggerRun:     true,
		TriggerReply:   ev.Reply != nil,
		TriggerEvent:   string(ev.Type),
		eventReply:     ev.Reply,
	}
	go func() {
		result := a.executeChat(ctx, run, body)
		if onDone != nil {
			onDone(result)
		}
	}()
}

func (a *AgentAPI) executeChat(ctx context.Context, run *agent.Run, body chatBody) mate.RunResult {
	// When mateId is provided, mate config in DB is authoritative for agent/model.
	// cwd and systemPrompt can still be overridden by the caller.
	if body.MateID != "" && a.store != nil {
		if m, err := a.store.GetMate(body.MateID); err == nil && m != nil {
			body.AgentID = m.AgentID
			body.Model = m.Model
			run.AgentID = m.AgentID
			if body.Cwd == "" {
				body.Cwd = m.Cwd
			}
			if body.SystemPrompt == "" {
				body.SystemPrompt = m.SystemPrompt
			}
		}
	}
	body.SystemPrompt = mergeSystemPrompts(a.cfg.Agent.EffectiveSystemPrompt(), body.SystemPrompt)

	// Persist user message and create assistant placeholder when a conversation
	// is active. assistantMsgID is carried through to the completion handler.
	var assistantMsgID string
	if a.store != nil && body.ConversationID != "" && body.Message != "" {
		if _, err := a.store.InsertMessage(mate.Message{
			ID:             body.UserMessageID,
			ConversationID: body.ConversationID,
			Role:           "user",
			Content:        body.Message,
			AgentID:        body.AgentID,
			MateID:         body.MateID,
			ModelID:        body.Model,
		}); err != nil {
			a.logger.Warn("chat: insert user message", slog.String("err", err.Error()))
		}
		if am, err := a.store.InsertMessage(mate.Message{
			ID:             body.AssistantMessageID,
			ConversationID: body.ConversationID,
			Role:           "assistant",
			AgentID:        body.AgentID,
			MateID:         body.MateID,
			ModelID:        body.Model,
			RunID:          run.ID,
			Status:         "running",
			TriggerEvent:   body.TriggerEvent,
		}); err != nil {
			a.logger.Warn("chat: insert assistant placeholder", slog.String("err", err.Error()))
		} else {
			assistantMsgID = am.ID
		}
	}

	def := agent.GetAgentDef(body.AgentID)
	if def == nil {
		a.logger.Warn("agent aborted",
			slog.String("runId", run.ID),
			slog.String("agentId", body.AgentID),
			slog.String("reason", "unknown_agent"))
		a.hub.Emit(run, "error", map[string]any{"code": "AGENT_UNAVAILABLE", "message": "unknown agent"})
		a.hub.Finish(run, "failed", map[string]any{})
		a.invokeEventReply(ctx, body, "", "failed")
		if assistantMsgID != "" && a.store != nil {
			_ = a.store.UpdateMessageDone(assistantMsgID, "", "failed")
		}
		return mate.RunResult{Success: false}
	}

	var sessionID string
	var firstSession, resumeTurn bool
	if body.TriggerRun && !body.TriggerReply {
		sessionID, firstSession = resolveTriggerSession(def)
	} else {
		sessionID, firstSession, resumeTurn = a.resolveAgentSession(body.ConversationID, def)
	}
	composed := composePrompt(body, a.vault.Root(), resumeTurn)
	if e := agent.CheckPromptArgvBudget(def, composed); e != nil {
		a.logger.Warn("agent aborted",
			slog.String("runId", run.ID),
			slog.String("agentId", def.ID),
			slog.String("reason", e.Code))
		a.hub.Emit(run, "error", map[string]any{"code": e.Code, "message": e.Message})
		a.hub.Finish(run, "failed", map[string]any{})
		a.invokeEventReply(ctx, body, "", "failed")
		if assistantMsgID != "" && a.store != nil {
			_ = a.store.UpdateMessageDone(assistantMsgID, "", "failed")
		}
		return mate.RunResult{Success: false}
	}
	if body.Message == "" && len(body.ImagePaths) == 0 {
		a.logger.Warn("agent aborted",
			slog.String("runId", run.ID),
			slog.String("agentId", def.ID),
			slog.String("reason", "message_required"))
		a.hub.Emit(run, "error", map[string]any{"code": "BAD_REQUEST", "message": "message required"})
		a.hub.Finish(run, "failed", map[string]any{})
		a.invokeEventReply(ctx, body, "", "failed")
		if assistantMsgID != "" && a.store != nil {
			_ = a.store.UpdateMessageDone(assistantMsgID, "", "failed")
		}
		return mate.RunResult{Success: false}
	}

	safeModel := pickModel(def, body.Model)
	safeReasoning := pickReasoning(def, body.Reasoning)

	cwd := body.Cwd
	if cwd == "" {
		cwd = a.vault.Root()
	}
	if rp, err := filepath.Abs(cwd); err == nil {
		cwd = rp
	}

	envCfg := body.AgentCliEnv
	if envCfg == nil {
		envCfg = map[string]string{}
	}
	bin := agent.ResolveAgentExecutable(def, envCfg)
	if bin == "" {
		a.logger.Warn("agent aborted",
			slog.String("runId", run.ID),
			slog.String("agentId", def.ID),
			slog.String("reason", "binary_not_found"))
		a.hub.Emit(run, "error", map[string]any{
			"code":    "AGENT_UNAVAILABLE",
			"message": def.Name + " binary not found on PATH",
		})
		a.hub.Finish(run, "failed", map[string]any{})
		a.invokeEventReply(ctx, body, "", "failed")
		if assistantMsgID != "" && a.store != nil {
			_ = a.store.UpdateMessageDone(assistantMsgID, "", "failed")
		}
		return mate.RunResult{Success: false}
	}

	argCtx := agent.BuildArgsContext{
		Prompt: composed, ImagePaths: body.ImagePaths, ExtraAllowedDirs: body.ExtraAllowedDirs,
		Model: safeModel, Reasoning: safeReasoning, Cwd: cwd,
		SessionID: sessionID, FirstSession: firstSession,
	}
	argv := agent.BuildInvocationArgs(def, argCtx)
	if e := agent.CheckWindowsCmdShimCommandLineBudget(def, bin, argv); e != nil {
		a.logger.Warn("agent aborted",
			slog.String("runId", run.ID),
			slog.String("agentId", def.ID),
			slog.String("reason", e.Code))
		a.hub.Emit(run, "error", map[string]any{"code": e.Code, "message": e.Message})
		a.hub.Finish(run, "failed", map[string]any{})
		a.invokeEventReply(ctx, body, "", "failed")
		if assistantMsgID != "" && a.store != nil {
			_ = a.store.UpdateMessageDone(assistantMsgID, "", "failed")
		}
		return mate.RunResult{Success: false}
	}
	if e := agent.CheckWindowsDirectExeCommandLineBudget(def, bin, argv); e != nil {
		a.logger.Warn("agent aborted",
			slog.String("runId", run.ID),
			slog.String("agentId", def.ID),
			slog.String("reason", e.Code))
		a.hub.Emit(run, "error", map[string]any{"code": e.Code, "message": e.Message})
		a.hub.Finish(run, "failed", map[string]any{})
		a.invokeEventReply(ctx, body, "", "failed")
		if assistantMsgID != "" && a.store != nil {
			_ = a.store.UpdateMessageDone(assistantMsgID, "", "failed")
		}
		return mate.RunResult{Success: false}
	}

	env := agent.SpawnEnvForAgent(def.ID, agent.ShellEnv(), envCfg)
	for k, v := range def.StaticEnv {
		env = mergeEnvKey(env, k, v)
	}
	// Keep $PWD in sync with cmd.Dir. Some agents (e.g. opencode) use $PWD
	// rather than getcwd() to locate the project root; without this they inherit
	// the server's CWD and treat the intended workspace as an external directory.
	env = mergeEnvKey(env, "PWD", cwd)

	up := a.uploadRoot()
	safeImages := filterImagePaths(body.ImagePaths, up)

	run.Start()
	a.hub.Emit(run, "start", map[string]any{
		"runId": run.ID, "agentId": def.ID, "bin": bin,
		"streamFormat": def.StreamFormat, "cwd": cwd,
		"model": safeModel, "reasoning": safeReasoning,
	})
	spawnCmd := exec.Command(bin, argv...)
	logSpawn := []any{
		slog.String("runId", run.ID),
		slog.String("agentId", def.ID),
		slog.String("streamFormat", string(def.StreamFormat)),
		slog.String("cwd", cwd),
		slog.String("cmd", spawnCmd.String()),
	}
	if safeModel != "" {
		logSpawn = append(logSpawn, slog.String("model", safeModel))
	}
	a.logger.Info("agent spawn", logSpawn...)

	// Accumulate text_delta/text_replace events so we can persist the assistant response.
	// text_snapshot is an internal-only event (not forwarded to the SSE client):
	// it carries the final clean text from agents like cursor-agent and overrides
	// any duplicated/reformatted deltas that may have accumulated during streaming.
	// text_replace IS forwarded: it signals the client to replace (not append) text,
	// preventing duplicate display when cursor-agent reformats mid-stream.
	var textAcc strings.Builder
	// Tracks which vault notes this run's tool calls touched, normalized across
	// every backend's tool_use shape. See agent.NoteAccessTracker.
	noteTracker := agent.NewNoteAccessTracker(cwd, a.vault.Root())
	emit := func(ev string, data any) {
		if ev == "agent" {
			if m, ok := data.(map[string]any); ok {
				switch m["type"] {
				case "text_snapshot":
					// Internal only: reset textAcc to the final authoritative text.
					if t, ok := m["text"].(string); ok {
						textAcc.Reset()
						textAcc.WriteString(t)
					}
					return // do not forward to SSE clients
				case "text_replace":
					// Reset accumulator to the replacement text; forward to clients.
					if t, ok := m["text"].(string); ok {
						textAcc.Reset()
						textAcc.WriteString(t)
					}
				case "text_delta":
					if d, ok := m["delta"].(string); ok {
						textAcc.WriteString(d)
					}
				case "tool_use":
					for _, na := range noteTracker.Feed(m) {
						a.hub.Emit(run, "agent", map[string]any{
							"type": "note_access", "path": na.Path, "action": na.Action, "tool": na.Tool,
						})
					}
				}
			}
		}
		a.hub.Emit(run, ev, data)
	}

	cp, err := agent.ExecChat(ctx, def, bin, argv, env, cwd, composed, safeModel, sessionID, firstSession, body.MCPServers, safeImages, up, emit, func(started *agent.ChatProcess) {
		a.active.Store(run.ID, started)
	})
	defer a.active.Delete(run.ID)

	code := 0
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			code = ee.ExitCode()
		} else {
			a.canceled.Delete(run.ID)
			a.logger.Warn("agent exec failed",
				slog.String("runId", run.ID),
				slog.String("agentId", def.ID),
				slog.String("err", truncateStr(err.Error(), 256)))
			a.hub.Emit(run, "error", map[string]any{"message": err.Error()})
			if assistantMsgID != "" && a.store != nil {
				_ = a.store.UpdateMessageDone(assistantMsgID, textAcc.String(), "failed")
				_ = a.store.InsertMessageNoteAccess(assistantMsgID, toMateNoteAccess(noteTracker.Entries()))
			}
			a.hub.Finish(run, "failed", map[string]any{})
			a.invokeEventReply(ctx, body, textAcc.String(), "failed")
			return mate.RunResult{Success: false, LastMessage: textAcc.String()}
		}
	}
	st := "succeeded"
	if _, wasCanceled := a.canceled.LoadAndDelete(run.ID); wasCanceled {
		// A killed process surfaces as a non-zero/-1 exit code indistinguishable
		// from a real crash, so RunCancelPOST's flag is what tells them apart.
		st = "canceled"
	} else if code != 0 || (cp.ACP != nil && cp.ACP.HasFatalError()) {
		st = "failed"
	}
	if assistantMsgID != "" && a.store != nil {
		if err := a.store.UpdateMessageDone(assistantMsgID, textAcc.String(), st); err != nil {
			a.logger.Warn("chat: update assistant message", slog.String("err", err.Error()))
		}
		if err := a.store.InsertMessageNoteAccess(assistantMsgID, toMateNoteAccess(noteTracker.Entries())); err != nil {
			a.logger.Warn("chat: persist note access", slog.String("err", err.Error()))
		}
	}
	if (!body.TriggerRun || body.TriggerReply) && st == "succeeded" && cp.SessionID != "" && body.ConversationID != "" && def.SupportsNativeSession && a.store != nil {
		if err := a.store.SetConversationAgentSession(body.ConversationID, def.ID, cp.SessionID); err != nil {
			a.logger.Warn("chat: persist agent session", slog.String("err", err.Error()))
		}
	}
	a.logger.Info("agent done",
		slog.String("runId", run.ID),
		slog.String("agentId", def.ID),
		slog.String("status", st),
		slog.Int("exitCode", code))
	a.hub.Finish(run, st, map[string]any{"exitCode": code})
	a.invokeEventReply(ctx, body, textAcc.String(), st)
	return mate.RunResult{Success: st == "succeeded", LastMessage: textAcc.String()}
}

// resolveTriggerSession returns an ephemeral agent session for a one-shot trigger run.
// The session is never persisted on the conversation.
func resolveTriggerSession(def *agent.AgentDef) (sessionID string, firstSession bool) {
	if def == nil || !def.SupportsNativeSession {
		return "", false
	}
	if agent.HostAssignsSessionID(def.ID) {
		return agent.NewSessionID(), true
	}
	return "", false
}

// resolveAgentSession loads or creates the agent-native session id bound to a mate conversation.
func (a *AgentAPI) resolveAgentSession(conversationID string, def *agent.AgentDef) (sessionID string, firstSession, resumeTurn bool) {
	if a.store == nil || conversationID == "" || def == nil || !def.SupportsNativeSession {
		return "", false, false
	}
	conv, err := a.store.GetConversation(conversationID)
	if err != nil {
		a.logger.Warn("chat: load conversation for session", slog.String("err", err.Error()))
		return "", false, false
	}
	if conv.AgentSessionID != "" {
		if conv.AgentSessionAgentID != "" && conv.AgentSessionAgentID != def.ID {
			if err := a.store.ClearConversationAgentSession(conversationID); err != nil {
				a.logger.Warn("chat: clear stale agent session", slog.String("err", err.Error()))
			}
		} else {
			return conv.AgentSessionID, false, true
		}
	}
	if agent.HostAssignsSessionID(def.ID) {
		sid := agent.NewSessionID()
		if err := a.store.SetConversationAgentSession(conversationID, def.ID, sid); err != nil {
			a.logger.Warn("chat: assign agent session", slog.String("err", err.Error()))
			return "", false, false
		}
		return sid, true, false
	}
	return "", false, false
}

func truncateStr(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func mergeSystemPrompts(global, mate string) string {
	g := strings.TrimSpace(global)
	m := strings.TrimSpace(mate)
	if g == "" {
		return m
	}
	if m == "" {
		return g
	}
	return g + "\n\n---\n\n" + m
}

func composePrompt(body chatBody, vaultRoot string, resumeSession bool) string {
	if resumeSession {
		return body.Message
	}
	var sb strings.Builder
	if t := strings.TrimSpace(body.SystemPrompt); t != "" {
		sb.WriteString("# Instructions\n\n")
		sb.WriteString(t)
		sb.WriteString("\n\n---\n")
	}
	if body.Cwd == "" {
		sb.WriteString("\nYour working directory (vault): ")
		sb.WriteString(vaultRoot)
		sb.WriteString("\n\n")
	}
	sb.WriteString("# User request\n\n")
	sb.WriteString(body.Message)
	return sb.String()
}

func pickModel(def *agent.AgentDef, model string) string {
	if model == "" {
		return ""
	}
	if agent.IsKnownModel(def, model) {
		return model
	}
	if s := agent.SanitizeCustomModel(model); s != "" {
		return s
	}
	return ""
}

func pickReasoning(def *agent.AgentDef, r string) string {
	if r == "" || r == "default" {
		return ""
	}
	for _, x := range def.ReasoningOptions {
		if x.ID == r {
			return r
		}
	}
	return ""
}

func filterImagePaths(paths []string, uploadRoot string) []string {
	root := filepath.Clean(uploadRoot) + string(filepath.Separator)
	var out []string
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		ap, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		if strings.HasPrefix(ap, root) {
			if st, err := os.Stat(ap); err == nil && !st.IsDir() {
				out = append(out, ap)
			}
		}
	}
	return out
}

// toMateNoteAccess converts the agent package's run-scoped note list to the
// mate package's persisted shape. Kept as a conversion at this boundary so
// mate (storage) doesn't need to import agent (CLI orchestration).
func toMateNoteAccess(entries []agent.NoteAccess) []mate.NoteAccessEntry {
	if len(entries) == 0 {
		return nil
	}
	out := make([]mate.NoteAccessEntry, len(entries))
	for i, e := range entries {
		out[i] = mate.NoteAccessEntry{Path: e.Path, Action: e.Action, Tool: e.Tool, At: e.At}
	}
	return out
}

func mergeEnvKey(env []string, k, v string) []string {
	m := map[string]string{}
	for _, kv := range env {
		if i := strings.IndexByte(kv, '='); i > 0 {
			m[kv[:i]] = kv[i+1:]
		}
	}
	m[k] = v
	var out []string
	for key, val := range m {
		out = append(out, key+"="+val)
	}
	return out
}

func (a *AgentAPI) invokeEventReply(ctx context.Context, body chatBody, text, status string) {
	if body.eventReply == nil {
		return
	}
	if err := body.eventReply(ctx, plugin.ReplyResult{Text: text, Status: status}); err != nil {
		a.logger.Warn("mate event reply failed", slog.String("err", err.Error()))
	}
}
