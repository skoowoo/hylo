package mate

import (
	"time"

	"github.com/hardhacker/hylo/internal/plugin"
	"github.com/hardhacker/hylo/internal/prompt"
)

// MateEventType is a high-level semantic event exposed to trigger matching.
type MateEventType string

const (
	MateEventNoteCreated       MateEventType = "note_created"
	MateEventNoteUpdated       MateEventType = "note_updated"
	MateEventNoteDeleted       MateEventType = "note_deleted"
	MateEventShortNoteCreated  MateEventType = "short_note_created"
	MateEventScheduled         MateEventType = "scheduled"
	MateEventWechatMessage     MateEventType = "wechat_message"
	MateEventDiscordMessage    MateEventType = "discord_message"
	MateEventCompileRequested  MateEventType = "compile_requested"
	MateEventAgentRunCompleted MateEventType = "agent_run_completed"
)

// EventDef describes a built-in mate event for display in the UI.
type EventDef struct {
	Type        MateEventType `json:"type"`
	Label       string        `json:"label"`
	Description string        `json:"description"`
	// Vars are the tokens a trigger prompt on this event can use; see renderPrompt.
	Vars []prompt.Var `json:"vars"`
}

var (
	varPath  = prompt.Var{Token: "{Path}", Desc: "Full vault path of the affected note"}
	varName  = prompt.Var{Token: "{Name}", Desc: "Filename without extension"}
	varNow   = prompt.Var{Token: "{Now}", Desc: "Trigger time (RFC3339)"}
	noteVars = []prompt.Var{varPath, varName}
)

// BuiltinEvents is the authoritative list of mate events the system supports.
var BuiltinEvents = []EventDef{
	{MateEventNoteCreated, "Note Created", "Fires when any note is created in the vault", noteVars},
	{MateEventNoteUpdated, "Note Updated", "Fires when any existing note is modified", noteVars},
	{MateEventNoteDeleted, "Note Deleted", "Fires when any note is deleted", noteVars},
	{MateEventShortNoteCreated, "Short Note Created", "Fires each time a short note entry is appended", []prompt.Var{
		varPath, varName, {Token: "{Content}", Desc: "Appended short-note text"},
	}},
	{MateEventScheduled, "Scheduled", "Fires on a configured interval, daily time, or weekly time", []prompt.Var{
		varNow, {Token: "{Date}", Desc: "Date YYYY-MM-DD"}, {Token: "{Time}", Desc: "Time HH:MM"},
	}},
	{MateEventWechatMessage, "WeChat Message", "Fires when a WeChat direct message is received", []prompt.Var{
		{Token: "{Content}", Desc: "Incoming WeChat DM text"}, {Token: "{WechatUserID}", Desc: "Sender WeChat user ID"},
	}},
	{MateEventDiscordMessage, "Discord Message", "Fires when a Discord DM is received", []prompt.Var{
		{Token: "{Content}", Desc: "Incoming Discord DM text"},
		{Token: "{DiscordChannelID}", Desc: "DM channel ID"},
		{Token: "{DiscordUserID}", Desc: "Sender Discord user ID"},
	}},
	{MateEventCompileRequested, "Compile Requested", "Fires when the user manually triggers knowledge compilation for a note — path carries the note to compile", []prompt.Var{
		{Token: "{Path}", Desc: "Vault path of the note to compile"}, varName,
	}},
	{MateEventAgentRunCompleted, "Agent Run Completed", "Fires when another mate agent run succeeds — use source mate names to filter", []prompt.Var{
		{Token: "{Name}", Desc: "Name of the bot whose run just succeeded"},
		{Token: "{Content}", Desc: "Last assistant message from the completed run"},
		varNow,
	}},
}

// EventLabel returns the display label for a mate event type.
func EventLabel(t MateEventType) string {
	for _, e := range BuiltinEvents {
		if e.Type == t {
			return e.Label
		}
	}
	return string(t)
}

// MateEvent is the enriched semantic event passed to trigger matching and prompt rendering.
type MateEvent struct {
	Type MateEventType
	// Path holds the vault path for note/compile events.
	// For agent_run_completed it carries the source mate name; PathPrefixes then filters by mate name.
	Path         string
	Content      string    // populated for short_note_created and wechat_message
	FiredAt      time.Time // populated for scheduled: when the trigger fired
	WechatUserID string    // populated for wechat_message

	// Discord fields — populated for discord_message.
	DiscordChannelID string
	DiscordUserID    string
	DiscordMessageID string

	// Reply is propagated from plugin.Event; invoked after the trigger agent run finishes.
	Reply plugin.ReplyFunc

	// SourceMateID is set for agent_run_completed to prevent the source mate from triggering itself.
	SourceMateID string
}

// Translate converts a low-level plugin.Event to the matching set of MateEvents.
func Translate(e plugin.Event) []MateEvent {
	if e.IsDir {
		return nil
	}
	var out []MateEvent
	switch e.Type {
	case plugin.EventVaultCreate, plugin.EventFSCreate:
		if e.Moved {
			return nil
		}
		out = append(out, MateEvent{Type: MateEventNoteCreated, Path: e.Path})
	case plugin.EventVaultWrite, plugin.EventFSWrite:
		out = append(out, MateEvent{Type: MateEventNoteUpdated, Path: e.Path})
	case plugin.EventVaultDelete, plugin.EventFSDelete:
		out = append(out, MateEvent{Type: MateEventNoteDeleted, Path: e.Path})
	case plugin.EventVaultShortAppend:
		out = append(out, MateEvent{Type: MateEventShortNoteCreated, Path: e.Path, Content: e.Content})
	case plugin.EventWechatMessage:
		out = append(out, MateEvent{
			Type:         MateEventWechatMessage,
			Path:         e.Path,
			Content:      e.Content,
			WechatUserID: e.WechatUserID,
			Reply:        e.Reply,
		})
	case plugin.EventDiscordMessage:
		out = append(out, MateEvent{
			Type:             MateEventDiscordMessage,
			Path:             e.Path,
			Content:          e.Content,
			DiscordChannelID: e.DiscordChannelID,
			DiscordUserID:    e.DiscordUserID,
			DiscordMessageID: e.DiscordMessageID,
			Reply:            e.Reply,
		})
	case plugin.EventCompileRequested:
		out = append(out, MateEvent{
			Type:  MateEventCompileRequested,
			Path:  e.Path,
			Reply: e.Reply,
		})
	}
	return out
}
