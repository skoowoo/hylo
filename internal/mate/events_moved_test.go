package mate

import (
	"testing"

	"github.com/hardhacker/hylo/internal/plugin"
)

func TestTranslateSkipsMovedCreate(t *testing.T) {
	if out := Translate(plugin.Event{Type: plugin.EventFSCreate, Path: "/a/n.md", Moved: true}); len(out) != 0 {
		t.Fatalf("moved note fired %v", out)
	}
	out := Translate(plugin.Event{Type: plugin.EventFSCreate, Path: "/a/n.md"})
	if len(out) != 1 || out[0].Type != MateEventNoteCreated {
		t.Fatalf("new note = %v", out)
	}
}
