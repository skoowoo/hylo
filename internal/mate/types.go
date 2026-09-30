package mate

import "time"

// ── Mate ─────────────────────────────────────────────────────────────────────

// Mate is a configured agent persona with its own agent, model, cwd, and triggers.
type Mate struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	Description   string        `json:"description"`
	AgentID       string        `json:"agentId"`
	Model         string        `json:"model"`
	Color         string        `json:"color"` // hex color, e.g. "#cc785c"
	Cwd           string        `json:"cwd"`
	SystemPrompt  string        `json:"systemPrompt"`
	TriggerConvID string        `json:"triggerConvId,omitempty"` // legacy; no longer used for new trigger runs
	Enabled       bool          `json:"enabled"`
	CreatedAt     time.Time     `json:"createdAt"`
	UpdatedAt     time.Time     `json:"updatedAt"`
	Triggers      []MateTrigger `json:"triggers,omitempty"`
	TriggerCount  int           `json:"triggerCount,omitempty"` // populated in list views
}

// MateTrigger fires a run when a matching mate event occurs.
type MateTrigger struct {
	ID           string    `json:"id"`
	MateID       string    `json:"mateId"`
	EventTypes   []string  `json:"eventTypes"`             // MateEventType values e.g. ["note_created", "note_updated"]
	PathPrefixes []string  `json:"pathPrefixes,omitempty"` // directory whitelist; empty = no filter (ignored for scheduled/wechat)
	Prompt       string    `json:"prompt"`                 // template: {{.Path}} {{.Name}} {{.Now}}
	Schedule     string    `json:"schedule,omitempty"`     // "every 1h", "daily 09:00", or "weekly mon,wed 09:00" when eventTypes includes scheduled
	LastFiredAt  time.Time `json:"lastFiredAt,omitempty"`  // set by runner; read-only for clients
	Enabled      bool      `json:"enabled"`
	CreatedAt    time.Time `json:"createdAt"`
}

// ── Chat ─────────────────────────────────────────────────────────────────────

// Conversation type constants.
const (
	ConvTypeChat         = "chat"          // user-initiated, multi-turn
	ConvTypeTrigger      = "trigger"       // background trigger runs, no reply, ephemeral session
	ConvTypeTriggerReply = "trigger_reply" // trigger with reply callback (e.g. wechat), multi-turn
)

// Conversation is one chat session bound to a specific mate.
type Conversation struct {
	ID                  string    `json:"id"`
	MateID              string    `json:"mateId"`
	Title               string    `json:"title"`
	Type                string    `json:"type"`              // ConvTypeChat | ConvTypeTrigger | ConvTypeTriggerReply
	UserKey             string    `json:"userKey,omitempty"` // external user id for trigger_reply (e.g. wechat user id)
	AgentSessionID      string    `json:"agentSessionId,omitempty"`
	AgentSessionAgentID string    `json:"agentSessionAgentId,omitempty"`
	CreatedAt           time.Time `json:"createdAt"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

// Message is one turn inside a conversation.
type Message struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversationId"`
	Role           string    `json:"role"`    // "user" | "assistant"
	Content        string    `json:"content"` // accumulated text; no thinking/tool detail
	AgentID        string    `json:"agentId"`
	MateID         string    `json:"mateId"`
	ModelID        string    `json:"modelId"`
	RunID          string    `json:"runId"`
	Status         string    `json:"status"`                 // "running" | "succeeded" | "failed" | "canceled" | ""
	TriggerEvent   string    `json:"triggerEvent,omitempty"` // mate event type when fired by trigger (assistant only)
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`

	// NoteAccess lists the vault notes this run touched (assistant messages only).
	// Populated on read via a join; not part of the chat_messages row itself.
	NoteAccess []NoteAccessEntry `json:"noteAccess,omitempty"`
}

// Note access actions.
const (
	NoteAccessRead  = "read"
	NoteAccessWrite = "write"
)

// NoteAccessEntry records one vault note an assistant run read or wrote,
// normalized across every agent backend (claude/codex/opencode/hermes/
// cursor-agent/copilot/pi) to the same shape regardless of which CLI tool
// produced it.
type NoteAccessEntry struct {
	Path   string    `json:"path"`   // vault-relative, e.g. "/journal/2026/april.md"
	Action string    `json:"action"` // NoteAccessRead | NoteAccessWrite
	Tool   string    `json:"tool"`   // originating tool/event name, for debugging
	At     time.Time `json:"at"`
}
