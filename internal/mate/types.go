package mate

import (
	"encoding/json"
	"time"
)

// ── Mate ─────────────────────────────────────────────────────────────────────

// Mate is a configured agent persona with its own agent, model, cwd, and triggers.
type Mate struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Description  string        `json:"description"`
	AgentID      string        `json:"agentId"`
	Model        string        `json:"model"`
	Color        string        `json:"color"` // hex color, e.g. "#cc785c"
	Cwd          string        `json:"cwd"`   // vault dir, set by CreateMate and fixed after
	Main         bool          `json:"main"`  // the one bot at the vault root, made by onboarding; can't be deleted
	SystemPrompt string        `json:"systemPrompt"`
	Enabled      bool          `json:"enabled"`
	CreatedAt    time.Time     `json:"createdAt"`
	UpdatedAt    time.Time     `json:"updatedAt"`
	Triggers     []MateTrigger `json:"triggers,omitempty"`
	TriggerCount int           `json:"triggerCount,omitempty"` // populated in list views
	// Channels lists connected app kinds. Filled for the bots list, not stored on the row.
	Channels []string `json:"channels,omitempty"`
}

// MateTrigger fires a run when a matching mate event occurs.
type MateTrigger struct {
	ID           string    `json:"id"`
	MateID       string    `json:"mateId"`
	Name         string    `json:"name,omitempty"`         // optional label shown in place of "Trigger N"
	EventTypes   []string  `json:"eventTypes"`             // MateEventType values e.g. ["note_created", "note_updated"]
	PathPrefixes []string  `json:"pathPrefixes,omitempty"` // directory whitelist; empty = no filter (scheduled and chat triggers have none)
	Prompt       string    `json:"prompt"`                 // template: {Path} {Name} {Now}…, per EventDef.Vars
	Schedule     string    `json:"schedule,omitempty"`     // "every 1h", "daily 09:00", or "weekly mon,wed 09:00" when eventTypes includes scheduled
	LastFiredAt  time.Time `json:"lastFiredAt,omitempty"`  // last fire, or when the schedule was set; read-only for clients
	Enabled      bool      `json:"enabled"`
	CreatedAt    time.Time `json:"createdAt"`
}

// ── Chat ─────────────────────────────────────────────────────────────────────

// Conversation type constants.
const (
	ConvTypeChat    = "chat"    // user-initiated, multi-turn
	ConvTypeTrigger = "trigger" // background trigger runs, no reply, ephemeral session
)

// Conversation is one chat session bound to a specific mate.
type Conversation struct {
	ID                  string    `json:"id"`
	MateID              string    `json:"mateId"`
	Title               string    `json:"title"`
	Type                string    `json:"type"` // ConvTypeChat | ConvTypeTrigger
	AgentSessionID      string    `json:"agentSessionId,omitempty"`
	AgentSessionAgentID string    `json:"agentSessionAgentId,omitempty"`
	AgentSessionCwd     string    `json:"agentSessionCwd,omitempty"`
	CreatedAt           time.Time `json:"createdAt"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

// Attachment is one image on a user message. The file lives in the vault's
// upload directory; ID is its filename there.
type Attachment struct {
	ID   string `json:"id"`
	MIME string `json:"mime"`
	Name string `json:"name"`
}

// Message is one turn inside a conversation.
type Message struct {
	ID             string       `json:"id"`
	ConversationID string       `json:"conversationId"`
	Role           string       `json:"role"`    // "user" | "assistant"
	Content        string       `json:"content"` // the reply: the run's last text, without narration
	Attachments    []Attachment `json:"attachments,omitempty"`
	AgentID        string       `json:"agentId"`
	MateID         string       `json:"mateId"`
	ModelID        string       `json:"modelId"`
	RunID          string       `json:"runId"`
	Status         string       `json:"status"`                 // "running" | "succeeded" | "failed" | "canceled" | ""
	TriggerEvent   string       `json:"triggerEvent,omitempty"` // mate event type when fired by trigger (assistant only)
	TriggerID      string       `json:"triggerId,omitempty"`    // the MateTrigger that fired this run (assistant only)
	CreatedAt      time.Time    `json:"createdAt"`
	UpdatedAt      time.Time    `json:"updatedAt"`

	// NoteAccess lists the vault notes this run touched (assistant messages only).
	// Populated on read via a join; not part of the chat_messages row itself.
	NoteAccess []NoteAccessEntry `json:"noteAccess,omitempty"`

	// Trace is the run's agent events as a JSON array (agent.TurnLog).
	Trace json.RawMessage `json:"trace,omitempty"`
}

// Note access actions.
const (
	NoteAccessRead  = "read"
	NoteAccessWrite = "write"
)

// NoteAccessEntry records one vault note an assistant run read or wrote,
// normalized across every agent backend (claude/codex/opencode/
// cursor-agent/copilot/pi) to the same shape regardless of which CLI tool
// produced it.
type NoteAccessEntry struct {
	Path   string    `json:"path"`   // vault-relative, e.g. "/journal/2026/april.md"
	Action string    `json:"action"` // NoteAccessRead | NoteAccessWrite
	Tool   string    `json:"tool"`   // originating tool/event name, for debugging
	At     time.Time `json:"at"`
}
