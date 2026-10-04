package mate

import (
	"path/filepath"
	"testing"
)

func TestConversationAgentSession(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(filepath.Join(dir, "vault"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	conv, err := store.CreateConversation("mate-1", "Test")
	if err != nil {
		t.Fatal(err)
	}
	if conv.AgentSessionID != "" {
		t.Fatalf("new conversation should have empty agent session")
	}

	if err := store.SetConversationAgentSession(conv.ID, "claude", "sess-abc"); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetConversation(conv.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.AgentSessionID != "sess-abc" || got.AgentSessionAgentID != "claude" {
		t.Fatalf("got %+v", got)
	}

	if err := store.ClearConversationAgentSession(conv.ID); err != nil {
		t.Fatal(err)
	}
	got, err = store.GetConversation(conv.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.AgentSessionID != "" || got.AgentSessionAgentID != "" {
		t.Fatalf("expected cleared session, got %+v", got)
	}
}

func TestGetOrCreateActiveChatConvID(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(filepath.Join(dir, "vault"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	m := &Mate{Name: "Bot", AgentID: "claude", Enabled: true}
	if err := store.CreateMate(m); err != nil {
		t.Fatal(err)
	}

	id1, err := store.GetOrCreateActiveChatConvID(m.ID)
	if err != nil {
		t.Fatal(err)
	}
	id2, err := store.GetOrCreateActiveChatConvID(m.ID)
	if err != nil {
		t.Fatal(err)
	}
	if id1 != id2 {
		t.Fatalf("expected same conv, got %s vs %s", id1, id2)
	}

	conv, err := store.GetConversation(id1)
	if err != nil {
		t.Fatal(err)
	}
	if conv.Type != "chat" || conv.MateID != m.ID {
		t.Fatalf("got %+v", conv)
	}
}

func TestInsertMessageTriggerEvent(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(filepath.Join(dir, "vault"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	conv, err := store.CreateConversation("mate-1", "Test")
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.InsertMessage(Message{
		ConversationID: conv.ID,
		Role:           "assistant",
		Content:        "done",
		TriggerEvent:   "note_created",
		Status:         "succeeded",
	})
	if err != nil {
		t.Fatal(err)
	}
	msgs, err := store.ListMessages(conv.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 || msgs[0].TriggerEvent != "note_created" {
		t.Fatalf("got %+v", msgs)
	}
}

func TestListMessagesPage(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "vault"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	conv, err := store.CreateConversation("mate-1", "Test")
	if err != nil {
		t.Fatal(err)
	}
	// Same-millisecond inserts are common, so the cursor must not rely on created_at alone.
	var ids []string
	for i := 0; i < 7; i++ {
		m, err := store.InsertMessage(Message{ConversationID: conv.ID, Role: "user", Content: "m"})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, m.ID)
	}

	var got []string
	before := ""
	for {
		page, more, err := store.ListMessagesPage(conv.ID, before, 3)
		if err != nil {
			t.Fatal(err)
		}
		var pageIDs []string
		for _, m := range page {
			pageIDs = append(pageIDs, m.ID)
		}
		got = append(pageIDs, got...)
		if !more {
			break
		}
		before = page[0].ID
	}
	if len(got) != len(ids) {
		t.Fatalf("got %d messages, want %d", len(got), len(ids))
	}
	for i := range ids {
		if got[i] != ids[i] {
			t.Fatalf("order mismatch at %d: got %v want %v", i, got, ids)
		}
	}
}
