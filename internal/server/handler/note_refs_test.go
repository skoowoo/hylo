package handler

import (
	"testing"
	"time"

	"github.com/hardhacker/hylo/internal/agent"
	"github.com/hardhacker/hylo/internal/storage"
)

func TestHyloNotesResolvesHyloToolCalls(t *testing.T) {
	v, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := v.WriteNote("/journal/april.md", []byte("x"), ""); err != nil {
		t.Fatal(err)
	}
	tr := agent.NewNoteAccessTracker(v.Root(), v.Root(), hyloNotes{v})
	for _, call := range []struct {
		name  string
		input map[string]any
	}{
		{"mcp__hylo__read", map[string]any{"path": "april"}},
		{"mcp__hylo__create", map[string]any{"path": "/ideas/new.md", "content": "x"}},
		{"mcp__hylo__read", map[string]any{"path": "journal/april"}},
		{"hylo_short_update", map[string]any{"id": "2026-10-03-091500", "content": "x"}},
		{"mcp__hylo__short_create", map[string]any{"content": "x"}},
		{"mcp__hylo__read", map[string]any{"path": "missing.md"}},
		{"mcp__hylo__search", map[string]any{"query": "april"}},
		{"Bash", map[string]any{"command": `hylo create -c "a; b /x.md" /ideas/q.md`}},
		{"Bash", map[string]any{"command": "hylo rename /journal/april.md may.md"}},
	} {
		tr.Feed(map[string]any{"type": "tool_use", "name": call.name, "input": call.input})
	}
	want := []agent.NoteAccess{
		{Path: "/journal/april.md", Action: agent.NoteActionRead},
		{Path: "/ideas/new.md", Action: agent.NoteActionWrite},
		{Path: "/_shorts/2026-10-03.md", Action: agent.NoteActionWrite},
		{Path: storage.ShortDayPath(time.Now()).String(), Action: agent.NoteActionWrite},
		{Path: "/ideas/q.md", Action: agent.NoteActionWrite},
		{Path: "/journal/may.md", Action: agent.NoteActionWrite},
	}
	got := tr.Entries()
	if len(got) != len(want) {
		t.Fatalf("entries = %+v", got)
	}
	for i := range want {
		if got[i].Path != want[i].Path || got[i].Action != want[i].Action {
			t.Errorf("entry %d = %s %s, want %s %s", i, got[i].Path, got[i].Action, want[i].Path, want[i].Action)
		}
	}
}
