package handler

import (
	"errors"

	"github.com/hardhacker/hylo/internal/agent"
	hylomcp "github.com/hardhacker/hylo/internal/mcp"
	"github.com/hardhacker/hylo/internal/storage"
)

// hyloNotes resolves calls to hylo's own MCP tools and CLI to the vault notes
// they touch (agent.KnownNotes), looking notes up in the vault the way the
// tools do over HTTP (notes.NoteLookup).
type hyloNotes struct {
	vault *storage.Vault
}

func (h hyloNotes) Tool(name string, input map[string]any) ([]agent.NoteAccess, bool) {
	tool, ok := hylomcp.ToolName(name)
	if !ok {
		return nil, false
	}
	accs, ok := hylomcp.NotesTouched(h, tool, input)
	return toAgentNoteAccess(accs), ok
}

func (h hyloNotes) Command(args []string) ([]agent.NoteAccess, bool) {
	accs, ok := hylomcp.CommandNotesTouched(h, args)
	return toAgentNoteAccess(accs), ok
}

// StatNote matches POST /api/vault/stat.
func (h hyloNotes) StatNote(raw string) (storage.Note, error) {
	p, ok := statPath(raw)
	if !ok {
		return storage.Note{}, errors.New("path must be absolute")
	}
	return h.vault.StatNote(p)
}

// NotesByName matches POST /api/notes/resolve.
func (h hyloNotes) NotesByName(name string) ([]storage.Note, error) {
	return h.vault.GetNotesByName(name)
}

func toAgentNoteAccess(accs []hylomcp.NoteAccess) []agent.NoteAccess {
	out := make([]agent.NoteAccess, 0, len(accs))
	for _, a := range accs {
		action := agent.NoteActionRead
		if a.Write {
			action = agent.NoteActionWrite
		}
		out = append(out, agent.NoteAccess{Path: a.Path, Action: action})
	}
	return out
}
