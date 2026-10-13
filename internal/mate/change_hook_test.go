package mate

import (
	"path/filepath"
	"testing"
	"time"
)

func TestStoreReportsBotChanges(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "vault"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	n := 0
	store.SetOnChange(func() { n++ })

	m := &Mate{Name: "News", Enabled: true}
	if err := store.CreateMate(m); err != nil {
		t.Fatal(err)
	}
	m.Name = "Morning news"
	if err := store.UpdateMate(m); err != nil {
		t.Fatal(err)
	}
	if err := store.ReplaceTriggersForMate(m.ID, []MateTrigger{{EventTypes: []string{"scheduled"}, Schedule: "daily 08:00", Prompt: "p", Enabled: true}}); err != nil {
		t.Fatal(err)
	}
	if err := store.ReorderMates([]string{m.ID}); err != nil {
		t.Fatal(err)
	}
	if n != 4 {
		t.Fatalf("%d changes reported, want 4", n)
	}
	ts, _ := store.ListTriggers(m.ID)
	if err := store.UpdateTriggerLastFiredAt(ts[0].ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	if n != 4 {
		t.Fatal("a trigger firing is not a change to the bot")
	}
	if err := store.DeleteMate(m.ID); err != nil || n != 5 {
		t.Fatalf("delete: %v, %d changes", err, n)
	}
}
