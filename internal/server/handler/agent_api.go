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

	"github.com/hardhacker/hylo/internal/agent"
	"github.com/hardhacker/hylo/internal/config"
	"github.com/hardhacker/hylo/internal/editorbus"
	"github.com/hardhacker/hylo/internal/mate"
	"github.com/hardhacker/hylo/internal/prompt"
	"github.com/hardhacker/hylo/internal/storage"
	"github.com/hardhacker/hylo/internal/util"
)

// AgentAPI serves Open Design–compatible /api/agents, /api/chat, and /api/runs.
type AgentAPI struct {
	logger     *slog.Logger
	cfg        *config.Config
	vault      *storage.Vault
	hub        *agent.Hub
	store      *mate.Store // nil when store is unavailable; handles both mate config and chat history
	agentCache *agent.AgentCache
	active     sync.Map        // runID -> *agent.ChatProcess
	canceled   sync.Map        // runID -> true; set by RunCancelPOST so executeChat can report "canceled" instead of "failed" for a user-initiated stop
	pathRuns   sync.Map        // vault path -> runID; set by FireTriggerRun for path-bearing events
	runner     *mate.Runner    // optional; set via SetMateRunner once the mate runner exists, so TriggerRetryPOST can re-fire through its onDone pipeline
	editorBus  *editorbus.Bus  // optional; cancelled alongside a run so the editor stops typing
	setupMu    sync.Mutex      // one first-bot setup at a time, so two can't both find no bots
	chatSlots  sync.Map        // mateID -> chan struct{} (cap 1); one chat run at a time, desktop and channels together
	queued     sync.Map        // runID -> context.CancelFunc; a run still waiting for its bot's chat
	life       context.Context // detached runs end with it; nil leaves them to their own lifetime
	runs       sync.WaitGroup
}

// NewAgentAPI constructs the agent HTTP surface.
func NewAgentAPI(logger *slog.Logger, cfg *config.Config, vault *storage.Vault, hub *agent.Hub, store *mate.Store, cache *agent.AgentCache) *AgentAPI {
	return &AgentAPI{logger: logger, cfg: cfg, vault: vault, hub: hub, store: store, agentCache: cache}
}

// SetMateRunner wires in the mate runner so TriggerRetryPOST can re-fire a
// trigger run through the same onDone pipeline (inbox, agent_run_completed
// fan-out) an automatic trigger fire uses.
func (a *AgentAPI) SetMateRunner(r *mate.Runner) {
	a.runner = r
}

// SetEditorBus lets a run cancel stop the editor from typing what the agent already sent.
func (a *AgentAPI) SetEditorBus(b *editorbus.Bus) { a.editorBus = b }

// SetLifetime ends detached runs, and their agent processes, when ctx ends.
func (a *AgentAPI) SetLifetime(ctx context.Context) { a.life = ctx }

// Wait returns once every detached run has finished.
func (a *AgentAPI) Wait() { a.runs.Wait() }

// detach runs fn past its request or caller, until fn returns or the server stops.
func (a *AgentAPI) detach(ctx context.Context, fn func(context.Context)) {
	ctx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	stop := func() bool { return false }
	if a.life != nil {
		stop = context.AfterFunc(a.life, cancel)
	}
	a.runs.Add(1)
	go func() {
		defer a.runs.Done()
		defer stop()
		defer cancel()
		fn(ctx)
	}()
}

func (a *AgentAPI) uploadRoot() string {
	return filepath.Join(a.vault.Root(), a.cfg.Agent.ResolvedUploadDir())
}

type chatBody struct {
	AgentID            string            `json:"agentId"`
	MateID             string            `json:"mateId"` // when set, server resolves agent/model/cwd/systemPrompt from mate
	Message            string            `json:"message"`
	SystemPrompt       string            `json:"systemPrompt"` // the bot layer of the system prompt; see prompt.Run.Bot
	Model              string            `json:"model"`
	Reasoning          string            `json:"reasoning"`
	ImagePaths         []string          `json:"imagePaths"`
	Attachments        []mate.Attachment `json:"attachments"`
	ExtraAllowedDirs   []string          `json:"extraAllowedDirs"`
	AgentCliEnv        map[string]string `json:"agentCliEnv"`
	ProjectID          string            `json:"projectId"`
	ConversationID     string            `json:"conversationId"`
	UserMessageID      string            `json:"userMessageId"`      // pre-allocated by client (optional)
	AssistantMessageID string            `json:"assistantMessageId"` // pre-allocated by client (optional)
	TriggerRun         bool              `json:"triggerRun"`         // when true, skip persisting the user prompt message
	TriggerEvent       string            `json:"triggerEvent"`       // mate event type label stored on assistant message
	TriggerID          string            `json:"triggerId"`          // trigger that fired the run, stored on assistant message
	RetryOf            string            `json:"retryOf"`            // failed/stopped assistant message this run replaces
	Intro              bool              `json:"intro"`              // first chat after onboarding; see prompt.Intro
	IntroLang          string            `json:"introLang"`          // language the onboarding hello picked
	oneShot            bool              // self-contained internal ask; see prompt.OneShot
	app                string            // wechat, discord or telegram; the stored message stays the raw text
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
	// The agent subprocess outlives the SSE connection (proxy timeout, client
	// reconnect); the run is tracked in the hub and can be cancelled via
	// POST /api/runs/:id/cancel.
	a.detach(r.Context(), func(ctx context.Context) { a.executeChat(ctx, run, body) })
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
	// The request ends as soon as the runId is sent; the run must outlive it.
	a.detach(r.Context(), func(ctx context.Context) { a.executeChat(ctx, run, body) })
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
	if stop, ok := a.queued.Load(id); ok {
		stop.(context.CancelFunc)()
	}
	if v, ok := a.active.Load(id); ok {
		if cp, ok := v.(*agent.ChatProcess); ok && cp.Cmd != nil && cp.Cmd.Process != nil {
			_ = cp.Cmd.Process.Kill()
		}
	}
	if a.editorBus != nil {
		a.editorBus.CancelRun(id)
	}
	a.logger.Info("agent run cancel", slog.String("runId", id))
	respondJSON(w, http.StatusOK, map[string]any{"ok": true})
}

type triggerRetryBody struct {
	MateID             string `json:"mateId"`
	ConversationID     string `json:"conversationId"`
	Prompt             string `json:"prompt"`
	TriggerEvent       string `json:"triggerEvent"`
	TriggerID          string `json:"triggerId"`
	RetryOf            string `json:"retryOf"`
	UserMessageID      string `json:"userMessageId"`
	AssistantMessageID string `json:"assistantMessageId"`
}

// RunsRetryTriggerPOST handles POST /api/runs/retry-trigger — re-fires a
// trigger-originated message the UI is retrying (a failed/canceled
// assistant message with triggerEvent set) through mate.Runner.RetryRun,
// instead of the plain /api/chat path a normal chat retry uses. That
// distinction matters: /api/chat's executeChat call has no onDone hook, so
// it never reaches the mate runner's RunDoneHook — which is what creates the
// inbox message on completion. Routing trigger retries through RetryRun
// keeps that behavior identical to an automatic trigger fire.
// Returns 202 + runId; the client watches it the same way it already
// recovers from a dropped SSE stream (poll GET /api/runs/:id, then resync
// the conversation to pick up the persisted content).
func (a *AgentAPI) RunsRetryTriggerPOST(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if a.runner == nil {
		http.Error(w, "trigger retry unavailable", http.StatusServiceUnavailable)
		return
	}
	var body triggerRetryBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if body.MateID == "" || body.ConversationID == "" || strings.TrimSpace(body.Prompt) == "" {
		http.Error(w, "mateId, conversationId and prompt are required", http.StatusBadRequest)
		return
	}
	runID, err := a.runner.RetryRun(body.MateID, body.ConversationID, body.Prompt, body.UserMessageID, body.AssistantMessageID, mate.MateEventType(body.TriggerEvent), body.TriggerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	// Only once the retry is under way, so a refused one keeps the failed attempt.
	a.dropRetried(body.ConversationID, body.RetryOf, body.Prompt)
	respondJSON(w, http.StatusAccepted, map[string]any{"runId": runID})
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

func (a *AgentAPI) dropRetried(convID, retryOf, prompt string) {
	if a.store == nil || convID == "" || retryOf == "" {
		return
	}
	if err := a.store.DeleteRetried(convID, retryOf, prompt); err != nil {
		a.logger.Warn("chat: drop retried message", slog.String("err", err.Error()))
	}
}

// FireTriggerRun is called by the mate runner to launch a background agent run.
// Messages are appended to the mate's active chat conversation; each run uses a fresh agent session.
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
		ConversationID: convID,
		TriggerRun:     true,
		TriggerEvent:   string(ev.Type),
		TriggerID:      ev.TriggerID,
	}
	a.detach(ctx, func(ctx context.Context) {
		result := a.executeChat(ctx, run, body)
		if onDone != nil {
			onDone(result)
		}
	})
}

// FireTriggerRetry is the mate.RetryFunc registered with the mate runner —
// it re-fires a trigger run the UI is retrying. It mirrors FireTriggerRun
// (same fresh-session, TriggerRun execution path) except the message IDs
// come from the caller instead of being generated fresh, so the UI's
// optimistic placeholders land on the exact rows this run persists.
func (a *AgentAPI) FireTriggerRetry(ctx context.Context, m *mate.Mate, convID, prompt, userMsgID, assistantMsgID string, ev mate.MateEvent, onDone func(mate.RunResult)) string {
	meta := map[string]string{
		"mateId": m.ID, "conversationId": convID,
	}
	run := a.hub.CreateRun(meta)
	run.AgentID = m.AgentID
	body := chatBody{
		MateID:             m.ID,
		AgentID:            m.AgentID,
		Message:            prompt,
		SystemPrompt:       m.SystemPrompt,
		Model:              m.Model,
		ConversationID:     convID,
		UserMessageID:      userMsgID,
		AssistantMessageID: assistantMsgID,
		TriggerRun:         true,
		TriggerEvent:       string(ev.Type),
		TriggerID:          ev.TriggerID,
	}
	a.detach(ctx, func(ctx context.Context) {
		result := a.executeChat(ctx, run, body)
		if onDone != nil {
			onDone(result)
		}
	})
	return run.ID
}

// queueChat waits until the bot's chat is free. A cancel, or the caller
// going away, ends the wait before the run writes anything.
func (a *AgentAPI) queueChat(ctx context.Context, runID, mateID string) (release func(), ok bool) {
	slot := a.chatSlot(mateID)
	ctx, stop := context.WithCancel(ctx)
	defer stop()
	a.queued.Store(runID, stop)
	defer a.queued.Delete(runID)
	if _, c := a.canceled.Load(runID); c {
		return nil, false
	}
	select {
	case slot <- struct{}{}:
	case <-ctx.Done():
		return nil, false
	}
	if _, c := a.canceled.Load(runID); c {
		<-slot
		return nil, false
	}
	return func() { <-slot }, true
}

func (a *AgentAPI) chatSlot(mateID string) chan struct{} {
	v, _ := a.chatSlots.LoadOrStore(mateID, make(chan struct{}, 1))
	return v.(chan struct{})
}

// Deliver runs one user turn in the bot's current chat — the same conversation
// the desktop composer writes to — and returns the reply and run status.
func (a *AgentAPI) Deliver(ctx context.Context, mateID, app, text string, images []string) (string, string) {
	if a.store == nil || a.hub == nil {
		return "", "failed"
	}
	m, err := a.store.GetMate(mateID)
	if err != nil || m == nil || !m.Enabled {
		return "", "failed"
	}
	// Pick the chat only once it is free, so a /new or a desktop new chat made
	// while this turn waited is the one it lands in.
	slot := a.chatSlot(mateID)
	select {
	case slot <- struct{}{}:
	case <-ctx.Done():
		return "", "canceled"
	}
	defer func() { <-slot }()
	convID, err := a.store.GetOrCreateActiveChatConvID(mateID)
	if err != nil {
		a.logger.Warn("channel chat: conversation", slog.String("err", err.Error()))
		return "", "failed"
	}
	run := a.hub.CreateRun(map[string]string{"conversationId": convID})
	result := a.executeChat(ctx, run, chatBody{MateID: mateID, Message: text, ConversationID: convID, app: app, ImagePaths: images})
	status, _ := run.StatusJSON()["status"].(string)
	return result.LastMessage, status
}

// channelRequest is what the agent sees for a channel turn; the saved message
// stays the raw text, so the desktop shows the user's own words.
func channelRequest(app, text string) string {
	var name string
	switch app {
	case mate.ChannelWechat:
		name = "WeChat"
	case mate.ChannelDiscord:
		name = "Discord"
	case mate.ChannelTelegram:
		name = "Telegram"
	default:
		return text
	}
	return "Sent from " + name + ", usually on a phone. Reply in plain text, a few sentences, no markdown.\n\n" + text
}

func (a *AgentAPI) executeChat(ctx context.Context, run *agent.Run, body chatBody) mate.RunResult {
	// Desktop chat and a connected app share this bot's chat session, so only
	// one of them runs at a time. Trigger runs keep their own session; a
	// channel turn already holds the slot (Deliver).
	if body.MateID != "" && !body.TriggerRun && body.app == "" {
		release, ok := a.queueChat(ctx, run.ID, body.MateID)
		if !ok {
			a.canceled.Delete(run.ID)
			a.hub.Finish(run, "canceled", map[string]any{})
			return mate.RunResult{}
		}
		defer release()
	}
	// When mateId is provided, mate config in DB is authoritative for agent/model/cwd.
	// systemPrompt can still be overridden by the caller.
	dir := "/"
	main := false
	if body.MateID != "" && a.store != nil {
		if m, err := a.store.GetMate(body.MateID); err == nil && m != nil {
			body.AgentID = m.AgentID
			body.Model = m.Model
			run.AgentID = m.AgentID
			dir = m.Cwd
			main = m.Main
			if body.SystemPrompt == "" {
				body.SystemPrompt = m.SystemPrompt
			}
		}
	}
	pr := prompt.Run{
		Bot:       body.SystemPrompt,
		Main:      main,
		MCP:       a.mcpURL(body.AgentID) != "",
		Overrides: map[string]string{prompt.SectionGlobal: a.cfg.Agent.SystemPrompt},
	}
	switch {
	case body.oneShot:
		pr.Kind = prompt.OneShot
	case body.TriggerRun:
		pr.Kind = prompt.Trigger
	}
	if body.Intro {
		// The user is watching an empty editor; drafting fast matters more than depth.
		if body.Reasoning == "" {
			body.Reasoning = "low"
		}
		pr.Intro = &prompt.Intro{Greeting: a.openingWords(body.ConversationID), Overview: vaultOverview(a.vault), Lang: body.IntroLang}
	}

	// Images are checked before a retry drops the failed turn, so a bad image never loses it.
	atts, paths, imgErr := a.prepareChatImages(body.Attachments, body.ImagePaths)
	if imgErr != nil {
		a.hub.Emit(run, "error", map[string]any{"code": "BAD_REQUEST", "message": imgErr.Error()})
		a.hub.Finish(run, "failed", map[string]any{})
		return mate.RunResult{Success: false}
	}
	body.Attachments = atts
	body.ImagePaths = paths

	a.dropRetried(body.ConversationID, body.RetryOf, body.Message)

	// Persist user message and create assistant placeholder when a conversation
	// is active. assistantMsgID is carried through to the completion handler.
	// An image with no caption is still a turn; the caption stays the raw text.
	var assistantMsgID string
	if a.store != nil && body.ConversationID != "" && (body.Message != "" || len(body.ImagePaths) > 0) {
		if _, err := a.store.InsertMessage(mate.Message{
			ID:             body.UserMessageID,
			ConversationID: body.ConversationID,
			Role:           "user",
			Content:        body.Message,
			Attachments:    body.Attachments,
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
			TriggerID:      body.TriggerID,
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
		if assistantMsgID != "" && a.store != nil {
			_ = a.store.UpdateMessageDone(assistantMsgID, "", "", "failed")
		}
		return mate.RunResult{Success: false}
	}

	dir, cwd, err := a.workdir(dir)
	if err != nil {
		a.logger.Warn("agent aborted",
			slog.String("runId", run.ID),
			slog.String("agentId", def.ID),
			slog.String("reason", "bad_cwd"),
			slog.String("err", err.Error()))
		a.hub.Emit(run, "error", map[string]any{"code": "BAD_REQUEST", "message": err.Error()})
		a.hub.Finish(run, "failed", map[string]any{})
		if assistantMsgID != "" && a.store != nil {
			_ = a.store.UpdateMessageDone(assistantMsgID, "", "", "failed")
		}
		return mate.RunResult{Success: false}
	}

	var sessionID string
	var firstSession, resumeTurn bool
	if body.TriggerRun {
		sessionID, firstSession = resolveTriggerSession(def)
	} else {
		sessionID, firstSession, resumeTurn = a.resolveAgentSession(body.ConversationID, def, cwd)
	}
	up := a.uploadRoot()
	safeImages := filterImagePaths(body.ImagePaths, up)
	request := channelRequest(body.app, agent.TurnText(def.ID, body.Message, safeImages))
	composed := prompt.Message(prompt.System(pr), request, cwd, dir, resumeTurn)
	if e := agent.CheckPromptArgvBudget(def, composed); e != nil {
		a.logger.Warn("agent aborted",
			slog.String("runId", run.ID),
			slog.String("agentId", def.ID),
			slog.String("reason", e.Code))
		a.hub.Emit(run, "error", map[string]any{"code": e.Code, "message": e.Message})
		a.hub.Finish(run, "failed", map[string]any{})
		if assistantMsgID != "" && a.store != nil {
			_ = a.store.UpdateMessageDone(assistantMsgID, "", "", "failed")
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
		if assistantMsgID != "" && a.store != nil {
			_ = a.store.UpdateMessageDone(assistantMsgID, "", "", "failed")
		}
		return mate.RunResult{Success: false}
	}

	safeModel := pickModel(def, body.Model)
	safeReasoning := pickReasoning(def, body.Reasoning)

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
		if assistantMsgID != "" && a.store != nil {
			_ = a.store.UpdateMessageDone(assistantMsgID, "", "", "failed")
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

	argCtx := agent.BuildArgsContext{
		Prompt: composed, ImagePaths: safeImages, ExtraAllowedDirs: body.ExtraAllowedDirs,
		Model: safeModel, Reasoning: safeReasoning, Cwd: cwd,
		SessionID: sessionID, FirstSession: firstSession,
	}
	argv := agent.BuildInvocationArgs(def, argCtx)
	argv, env = a.injectMCP(def.ID, run.ID, cwd, argv, env)
	if e := agent.CheckWindowsCmdShimCommandLineBudget(def, bin, argv); e != nil {
		a.logger.Warn("agent aborted",
			slog.String("runId", run.ID),
			slog.String("agentId", def.ID),
			slog.String("reason", e.Code))
		a.hub.Emit(run, "error", map[string]any{"code": e.Code, "message": e.Message})
		a.hub.Finish(run, "failed", map[string]any{})
		if assistantMsgID != "" && a.store != nil {
			_ = a.store.UpdateMessageDone(assistantMsgID, "", "", "failed")
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
		if assistantMsgID != "" && a.store != nil {
			_ = a.store.UpdateMessageDone(assistantMsgID, "", "", "failed")
		}
		return mate.RunResult{Success: false}
	}

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

	var turn agent.TurnLog
	// Tracks which vault notes this run's tool calls touched, normalized across
	// every backend's tool_use shape. See agent.NoteAccessTracker.
	noteTracker := agent.NewNoteAccessTracker(cwd, a.vault.Root())
	emit := func(ev string, data any) {
		if ev == "agent" {
			if m, ok := data.(map[string]any); ok {
				turn.Feed(m)
				if m["type"] == "tool_use" {
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

	cp, err := agent.ExecChat(ctx, def, bin, argv, env, cwd, composed, safeModel, sessionID, firstSession, safeImages, up, emit, func(started *agent.ChatProcess) {
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
				_ = a.store.UpdateMessageDone(assistantMsgID, turn.Answer(), turn.JSON(), "failed")
				_ = a.store.InsertMessageNoteAccess(assistantMsgID, toMateNoteAccess(noteTracker.Entries()))
			}
			a.hub.Finish(run, "failed", map[string]any{})
			return mate.RunResult{Success: false, LastMessage: turn.Answer()}
		}
	}
	st := "succeeded"
	if _, wasCanceled := a.canceled.LoadAndDelete(run.ID); wasCanceled {
		// A killed process surfaces as a non-zero/-1 exit code indistinguishable
		// from a real crash, so RunCancelPOST's flag is what tells them apart.
		st = "canceled"
	} else if code != 0 || (cp.Pi != nil && cp.Pi.HasFatalError()) {
		st = "failed"
	}
	if assistantMsgID != "" && a.store != nil {
		if err := a.store.UpdateMessageDone(assistantMsgID, turn.Answer(), turn.JSON(), st); err != nil {
			a.logger.Warn("chat: update assistant message", slog.String("err", err.Error()))
		}
		if err := a.store.InsertMessageNoteAccess(assistantMsgID, toMateNoteAccess(noteTracker.Entries())); err != nil {
			a.logger.Warn("chat: persist note access", slog.String("err", err.Error()))
		}
	}
	if !body.TriggerRun && st == "succeeded" && cp.SessionID != "" && body.ConversationID != "" && def.SupportsNativeSession && a.store != nil {
		if err := a.store.SetConversationAgentSession(body.ConversationID, def.ID, cwd, cp.SessionID); err != nil {
			a.logger.Warn("chat: persist agent session", slog.String("err", err.Error()))
		}
	}
	a.logger.Info("agent done",
		slog.String("runId", run.ID),
		slog.String("agentId", def.ID),
		slog.String("status", st),
		slog.Int("exitCode", code))
	a.hub.Finish(run, st, map[string]any{"exitCode": code})
	return mate.RunResult{Success: st == "succeeded", LastMessage: turn.Answer()}
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

// workdir resolves a bot's vault directory to the filesystem path its agent runs in,
// creating it if it went missing.
func (a *AgentAPI) workdir(dir string) (string, string, error) {
	dir, err := util.CleanVaultDir(dir)
	if err != nil {
		return "", "", err
	}
	cwd, err := filepath.Abs(filepath.Join(a.vault.Root(), filepath.FromSlash(dir)))
	if err != nil {
		return "", "", err
	}
	return dir, cwd, os.MkdirAll(cwd, 0755)
}

// resolveAgentSession loads or creates the agent-native session id bound to a mate conversation.
// Agents keep sessions per directory, so one started under another cwd can't be resumed.
func (a *AgentAPI) resolveAgentSession(conversationID string, def *agent.AgentDef, cwd string) (sessionID string, firstSession, resumeTurn bool) {
	if a.store == nil || conversationID == "" || def == nil || !def.SupportsNativeSession {
		return "", false, false
	}
	conv, err := a.store.GetConversation(conversationID)
	if err != nil {
		a.logger.Warn("chat: load conversation for session", slog.String("err", err.Error()))
		return "", false, false
	}
	if conv.AgentSessionID != "" {
		if (conv.AgentSessionAgentID != "" && conv.AgentSessionAgentID != def.ID) || (conv.AgentSessionCwd != "" && conv.AgentSessionCwd != cwd) {
			if err := a.store.ClearConversationAgentSession(conversationID); err != nil {
				a.logger.Warn("chat: clear stale agent session", slog.String("err", err.Error()))
			}
		} else {
			return conv.AgentSessionID, false, true
		}
	}
	if agent.HostAssignsSessionID(def.ID) {
		sid := agent.NewSessionID()
		if err := a.store.SetConversationAgentSession(conversationID, def.ID, cwd, sid); err != nil {
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

// openingWords is what the bot already said in a conversation before the user
// wrote anything: onboarding seeds it with the hello.
func (a *AgentAPI) openingWords(conversationID string) string {
	if a.store == nil || conversationID == "" {
		return ""
	}
	msgs, err := a.store.ListMessages(conversationID)
	if err != nil {
		return ""
	}
	var said []string
	for _, m := range msgs {
		if m.Role != "assistant" {
			break
		}
		said = append(said, m.Content)
	}
	return strings.Join(said, "\n")
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
