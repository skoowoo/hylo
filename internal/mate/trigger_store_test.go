package mate

import (
	"path/filepath"
	"testing"
	"time"
)

func TestScheduledTriggerPersistence(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(filepath.Join(dir, "vault"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	m := &Mate{Name: "Digest", AgentID: "claude", Enabled: true}
	if err := store.CreateMate(m); err != nil {
		t.Fatal(err)
	}

	last := time.Date(2026, 5, 22, 9, 0, 0, 0, time.UTC)
	triggers := []MateTrigger{{
		ID:         "tr-1",
		EventTypes: []string{string(MateEventScheduled)},
		Schedule:   "daily 09:00",
		Prompt:     "digest",
		Enabled:    true,
	}}
	if err := store.ReplaceTriggersForMate(m.ID, triggers); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateTriggerLastFiredAt("tr-1", last); err != nil {
		t.Fatal(err)
	}

	got, err := store.ListScheduledTriggers()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Schedule != "daily 09:00" {
		t.Fatalf("list scheduled: %+v", got)
	}

	// Same schedule preserves last_fired_at on replace.
	triggers[0].Prompt = "updated"
	if err := store.ReplaceTriggersForMate(m.ID, triggers); err != nil {
		t.Fatal(err)
	}
	reloaded, err := store.ListTriggers(m.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reloaded[0].LastFiredAt.Equal(last) {
		t.Fatalf("last_fired_at not preserved: got %v want %v", reloaded[0].LastFiredAt, last)
	}

	// A schedule change waits for its next slot rather than firing at once.
	triggers[0].Schedule = "every 1h"
	before := time.Now()
	if err := store.ReplaceTriggersForMate(m.ID, triggers); err != nil {
		t.Fatal(err)
	}
	reloaded, err = store.ListTriggers(m.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded[0].LastFiredAt.Before(before.Truncate(time.Millisecond)) {
		t.Fatalf("expected last_fired_at reset to now, got %v", reloaded[0].LastFiredAt)
	}

	now := time.Now()
	if err := store.UpdateTriggerLastFiredAt("tr-1", now); err != nil {
		t.Fatal(err)
	}
	reloaded, err = store.ListTriggers(m.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded[0].LastFiredAt.Unix() != now.Unix() {
		t.Fatalf("update last_fired_at: got %v want %v", reloaded[0].LastFiredAt, now)
	}
}

func TestNewDailyTriggerNotDueAfterTodaysSlot(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "vault"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	m := &Mate{Name: "News", AgentID: "claude", Enabled: true}
	if err := store.CreateMate(m); err != nil {
		t.Fatal(err)
	}
	// 00:00 is always behind us, the way 08:00 is when setup runs mid-morning.
	if err := store.ReplaceTriggersForMate(m.ID, []MateTrigger{{
		EventTypes: []string{string(MateEventScheduled)}, Schedule: "daily 00:00", Prompt: "news", Enabled: true,
	}}); err != nil {
		t.Fatal(err)
	}
	got, _ := store.ListScheduledTriggers()
	sched, _ := ParseSchedule(got[0].Schedule)
	if sched.Due(got[0].LastFiredAt, time.Now()) {
		t.Fatal("new trigger fired for a slot that passed before it existed")
	}
}

// Prompts keep their own copy of a skill's name, so a rename reaches into them, once.
func TestRenameSkill(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "vault"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	m := &Mate{Name: "Hylo", AgentID: "claude", Enabled: true, SystemPrompt: "Lean on hylo-recap for reviews; see [[hylo-recap-notes]] and notes/hylo-recaps.md."}
	if err := store.CreateMate(m); err != nil {
		t.Fatal(err)
	}
	if err := store.ReplaceTriggersForMate(m.ID, []MateTrigger{
		{ID: "a", EventTypes: []string{string(MateEventShortNoteCreated)}, Prompt: "Use the hylo-recap skill.", Enabled: true},
		{ID: "b", EventTypes: []string{string(MateEventShortNoteCreated)}, Prompt: "Use hylo-memory.", Enabled: true},
	}); err != nil {
		t.Fatal(err)
	}

	for range 2 {
		if err := store.RenameSkill("hylo-recap", "hylo-recall"); err != nil {
			t.Fatal(err)
		}
	}

	triggers, _ := store.ListTriggers(m.ID)
	if triggers[0].Prompt != "Use the hylo-recall skill." || triggers[1].Prompt != "Use hylo-memory." {
		t.Errorf("trigger prompts: %q, %q", triggers[0].Prompt, triggers[1].Prompt)
	}
	if got, _ := store.GetMate(m.ID); got.SystemPrompt != "Lean on hylo-recall for reviews; see [[hylo-recap-notes]] and notes/hylo-recaps.md." {
		t.Errorf("system prompt: %q", got.SystemPrompt)
	}
	if err := store.RenameSkill("hylo-x", "hylo-x2"); err == nil {
		t.Error("a rename that repeats on every run was accepted")
	}
}

func TestReplacePromptLeavesEditsAlone(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "vault"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	m := &Mate{Name: "Hylo", AgentID: "claude", Enabled: true}
	if err := store.CreateMate(m); err != nil {
		t.Fatal(err)
	}
	if err := store.ReplaceTriggersForMate(m.ID, []MateTrigger{
		{ID: "a", EventTypes: []string{string(MateEventShortNoteCreated)}, Prompt: "old text", Enabled: true},
		{ID: "b", EventTypes: []string{string(MateEventShortNoteCreated)}, Prompt: "old text, edited", Enabled: true},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.ReplacePrompt("old text", "new text"); err != nil {
		t.Fatal(err)
	}
	triggers, _ := store.ListTriggers(m.ID)
	if triggers[0].Prompt != "new text" || triggers[1].Prompt != "old text, edited" {
		t.Errorf("prompts: %q, %q", triggers[0].Prompt, triggers[1].Prompt)
	}
}
