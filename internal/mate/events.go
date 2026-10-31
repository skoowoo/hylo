package mate

import (
	"time"

	"github.com/hardhacker/hylo/internal/plugin"
	"github.com/hardhacker/hylo/internal/prompt"
)

// MateEventType is a high-level semantic event exposed to trigger matching.
type MateEventType string

const (
	MateEventNoteCreated      MateEventType = "note_created"
	MateEventNoteUpdated      MateEventType = "note_updated"
	MateEventNoteDeleted      MateEventType = "note_deleted"
	MateEventShortNoteCreated MateEventType = "short_note_created"
	MateEventScheduled        MateEventType = "scheduled"
	MateEventCompileRequested MateEventType = "compile_requested"
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
	{MateEventCompileRequested, "Compile Requested", "Fires when the user manually triggers knowledge compilation for a note — path carries the note to compile", []prompt.Var{
		{Token: "{Path}", Desc: "Vault path of the note to compile"}, varName,
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
	Path    string
	Content string    // populated for short_note_created
	FiredAt time.Time // populated for scheduled: when the trigger fired
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
	case plugin.EventCompileRequested:
		out = append(out, MateEvent{
			Type: MateEventCompileRequested,
			Path: e.Path,
		})
	}
	return out
}
