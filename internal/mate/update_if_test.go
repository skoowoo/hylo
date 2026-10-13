package mate

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestUpdateMateIfRejectsStaleRead(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "vault"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	m := &Mate{Name: "A", AgentID: "claude", Enabled: true}
	if err := store.CreateMate(m); err != nil {
		t.Fatal(err)
	}
	a, _ := store.GetMate(m.ID)
	b, _ := store.GetMate(m.ID)

	a.Name = "first"
	if err := store.UpdateMateIf(a, a.UpdatedAt); err != nil {
		t.Fatal(err)
	}
	b.Name = "second"
	if err := store.UpdateMateIf(b, b.UpdatedAt); !errors.Is(err, ErrMateConflict) {
		t.Fatalf("stale update err = %v", err)
	}
	got, _ := store.GetMate(m.ID)
	if got.Name != "first" {
		t.Fatalf("name = %q", got.Name)
	}
}
