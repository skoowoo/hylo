package mate

import (
	"path/filepath"
	"testing"
	"time"
)

func TestStoreReportsChatChanges(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "vault"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	n := 0
	var last []string
	store.SetOnChat(func(ids []string) { n++; last = ids })

	m := &Mate{Name: "News", Enabled: true}
	if err := store.CreateMate(m); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatal("a bot edit is not a chat change")
	}
	conv, err := store.StartConversation(m.ID, "")
	if err != nil || n != 1 {
		t.Fatalf("new chat: %v, %d changes", err, n)
	}
	msg, err := store.InsertMessage(Message{ConversationID: conv.ID, Role: "user", Content: "hi"})
	if err != nil || n != 2 || len(last) != 1 || last[0] != msg.ID {
		t.Fatalf("insert: %v, %d changes, ids %v", err, n, last)
	}
	if err := store.UpdateMessageDone(msg.ID, "hello", "", "succeeded"); err != nil || n != 3 {
		t.Fatalf("done: %v, %d changes", err, n)
	}
	if err := store.DeleteMessages(msg.ID); err != nil || n != 4 {
		t.Fatalf("delete: %v, %d changes", err, n)
	}
	if err := store.SetActiveConversation(m.ID, ""); err != nil || n != 5 {
		t.Fatalf("new chat: %v, %d changes", err, n)
	}
}

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
