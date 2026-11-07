package pulse

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/hardhacker/hylo/internal/plugin"
)

func TestChangesKeepContentChanges(t *testing.T) {
	var c changes
	now := at(15, 0)
	day := "/_shorts/2026-11-06.md"
	for _, e := range []plugin.Event{
		{Type: plugin.EventVaultWrite, Path: "/a.md", Time: at(14, 0)},
		{Type: plugin.EventVaultWrite, Path: "/a.md", Time: at(14, 1)}, // autosave
		{Type: plugin.EventVaultCreate, Path: "/b.md", Time: at(14, 2)},
		{Type: plugin.EventVaultWrite, Path: day, ShortID: "s1", ShortHash: "h", Time: at(14, 3)},
		{Type: plugin.EventVaultShortAppend, Path: day, Content: "an idea", Time: at(14, 3)},
		{Type: plugin.EventVaultWrite, Path: day, ShortID: "s0", ShortHash: "h", Time: at(14, 4)}, // an edit
		{Type: plugin.EventVaultWrite, Path: day, ShortID: "s0", ShortHash: "", Time: at(14, 5)},  // a delete
		{Type: plugin.EventVaultDelete, Path: "/c.md", Time: at(14, 6)},
		{Type: plugin.EventFSCreate, Path: "/moved.md", Moved: true, Time: at(14, 6)},
		{Type: plugin.EventVaultWrite, Path: "/_knowledge/_indexes/x.md", Time: at(14, 7)},
		{Type: plugin.EventVaultWrite, Path: "/_bots/_news/scratch.md", Time: at(14, 7)},
		{Type: plugin.EventVaultWrite, Path: "/_memory/profile.md", Time: at(14, 8)},
		{Type: plugin.EventVaultWrite, Path: "/_knowledge/Go.md", Time: at(14, 9)},
	} {
		c.add(e, now)
	}
	got := c.since(time.Time{})
	want := []change{
		{Path: "/a.md", At: at(14, 1)},
		{Path: "/b.md", Created: true, At: at(14, 2)},
		{Path: day, Short: "s1", New: true, Content: "an idea", At: at(14, 3)},
		{Path: day, Short: "s0", At: at(14, 4)},
		{Path: "/_memory/profile.md", At: at(14, 8)},
		{Path: "/_knowledge/Go.md", At: at(14, 9)},
	}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%d: got %+v, want %+v", i, got[i], want[i])
		}
	}
	later := at(14, 9).Add(digestWindow + time.Minute)
	c.add(plugin.Event{Type: plugin.EventVaultWrite, Path: "/d.md", Time: later}, later)
	if got := c.since(time.Time{}); len(got) != 1 {
		t.Fatalf("old changes not pruned: %+v", got)
	}
}

func TestChangesOutliveARestart(t *testing.T) {
	file := filepath.Join(t.TempDir(), "changes.json")
	var c changes
	if err := c.load(file, at(14, 0)); err != nil {
		t.Fatal(err)
	}
	c.add(plugin.Event{Type: plugin.EventVaultWrite, Path: "/a.md", Time: at(1, 45)}, at(1, 45))
	if err := c.save(); err != nil {
		t.Fatal(err)
	}
	var next changes
	if err := next.load(file, at(8, 5)); err != nil {
		t.Fatal(err)
	}
	if got := next.since(at(1, 30)); len(got) != 1 || got[0].Path != "/a.md" || !got[0].At.Equal(at(1, 45)) {
		t.Fatalf("after a restart: %+v", got)
	}
}
