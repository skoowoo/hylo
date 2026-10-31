package mate

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
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

	if err := store.SetConversationAgentSession(conv.ID, "claude", "/v", "sess-abc"); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetConversation(conv.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.AgentSessionID != "sess-abc" || got.AgentSessionAgentID != "claude" || got.AgentSessionCwd != "/v" {
		t.Fatalf("got %+v", got)
	}

	if err := store.ClearConversationAgentSession(conv.ID); err != nil {
		t.Fatal(err)
	}
	got, err = store.GetConversation(conv.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.AgentSessionID != "" || got.AgentSessionAgentID != "" || got.AgentSessionCwd != "" {
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

func TestMessageAttachmentsRoundTrip(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "vault"))
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
		Role:           "user",
		Content:        "look",
		Attachments:    []Attachment{{ID: "11111111-1111-1111-1111-111111111111.png", MIME: "image/png", Name: "shot.png"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	msgs, err := store.ListMessages(conv.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 || len(msgs[0].Attachments) != 1 || msgs[0].Attachments[0].Name != "shot.png" {
		t.Fatalf("got %+v", msgs)
	}
	page, _, err := store.ListMessagesPage(conv.ID, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 1 || page[0].Attachments[0].ID != msgs[0].Attachments[0].ID {
		t.Fatalf("page %+v", page)
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

func TestMessageTriggerID(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "vault"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	conv, err := store.CreateConversation("mate-1", "Auto runs")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.InsertMessage(Message{ConversationID: conv.ID, Role: "assistant", TriggerEvent: "scheduled", TriggerID: "trig-1"}); err != nil {
		t.Fatal(err)
	}
	msgs, _, err := store.ListMessagesPage(conv.ID, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 || msgs[0].TriggerID != "trig-1" {
		t.Fatalf("got %+v", msgs)
	}
}

func TestDeleteRetried(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "vault"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	conv, err := store.CreateConversation("mate-1", "Chat")
	if err != nil {
		t.Fatal(err)
	}
	add := func(role, content, status string) Message {
		m, err := store.InsertMessage(Message{ConversationID: conv.ID, Role: role, Content: content, Status: status})
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(2 * time.Millisecond)
		return m
	}
	add("user", "first", "")
	ok := add("assistant", "done", "succeeded")
	add("user", "again", "")
	failed := add("assistant", "", "failed")

	if err := store.DeleteRetried(conv.ID, ok.ID, "first"); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteRetried(conv.ID, failed.ID, "again"); err != nil {
		t.Fatal(err)
	}
	msgs, err := store.ListMessages(conv.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 || msgs[1].ID != ok.ID {
		t.Fatalf("want only the first exchange left, got %+v", msgs)
	}
}

func TestDeleteRetriedTakesTheWake(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "vault"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	conv, err := store.CreateConversation("mate-1", "Chat")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.InsertMessage(Message{ConversationID: conv.ID, Role: "wake", Content: "task", TriggerEvent: "scheduled"}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Millisecond)
	failed, err := store.InsertMessage(Message{ConversationID: conv.ID, Role: "assistant", Status: "failed", TriggerEvent: "scheduled"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteRetried(conv.ID, failed.ID, "task"); err != nil {
		t.Fatal(err)
	}
	if msgs, _ := store.ListMessages(conv.ID); len(msgs) != 0 {
		t.Fatalf("a retried wake should replace both rows, got %+v", msgs)
	}
}

func TestUnreadCountsFinishedWakeReplies(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "vault"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	m := &Mate{Name: "Recall", Enabled: true}
	if err := store.CreateMate(m); err != nil {
		t.Fatal(err)
	}
	conv, err := store.CreateConversation(m.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	add := func(msg Message) {
		msg.ConversationID, msg.MateID = conv.ID, m.ID
		if _, err := store.InsertMessage(msg); err != nil {
			t.Fatal(err)
		}
	}
	unread := func() int {
		list, err := store.ListMates()
		if err != nil || len(list) != 1 {
			t.Fatal(err, list)
		}
		return list[0].Unread
	}
	add(Message{Role: "assistant", Status: "succeeded"})
	add(Message{Role: "assistant", Status: "running", TriggerEvent: "scheduled"})
	add(Message{Role: "assistant", Status: "succeeded", TriggerEvent: "scheduled"})
	add(Message{Role: "assistant", Status: "failed", TriggerEvent: "note_created"})
	if n := unread(); n != 2 {
		t.Fatalf("unread = %d, want the two finished wake replies", n)
	}
	time.Sleep(2 * time.Millisecond)
	if err := store.MarkChatSeen(m.ID); err != nil {
		t.Fatal(err)
	}
	if n := unread(); n != 0 {
		t.Fatalf("unread after seen = %d", n)
	}
}

func TestWakesCarryTheirTriggerName(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "vault"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	m := &Mate{Name: "Recall", Enabled: true}
	if err := store.CreateMate(m); err != nil {
		t.Fatal(err)
	}
	if err := store.ReplaceTriggersForMate(m.ID, []MateTrigger{{Name: "Look back", EventTypes: []string{"scheduled"}, Schedule: "daily 15:00", Prompt: "p", Enabled: true}}); err != nil {
		t.Fatal(err)
	}
	ts, _ := store.ListTriggers(m.ID)
	conv, _ := store.CreateConversation(m.ID, "")
	for _, msg := range []Message{
		{Role: "wake", Content: "p", TriggerEvent: "scheduled", TriggerID: ts[0].ID},
		{Role: "assistant", Status: "succeeded", TriggerEvent: "scheduled", TriggerID: ts[0].ID},
	} {
		msg.ConversationID = conv.ID
		if _, err := store.InsertMessage(msg); err != nil {
			t.Fatal(err)
		}
	}
	msgs, _, err := store.ListMessagesPage(conv.ID, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if msgs[0].TriggerName != "Look back" || msgs[0].TriggerSchedule != "daily 15:00" {
		t.Fatalf("wake = %+v", msgs[0])
	}
	if msgs[1].TriggerName != "Look back" {
		t.Fatal("the reply carries the trigger's name too")
	}
}

func TestTurnBotRefusesAWakeWhoseSourceWentAway(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "vault"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	m := &Mate{Name: "Recall", Enabled: true}
	if err := store.CreateMate(m); err != nil {
		t.Fatal(err)
	}
	trig := MateTrigger{EventTypes: []string{"scheduled"}, Schedule: "daily 15:00", Prompt: "p", Enabled: true}
	if err := store.ReplaceTriggersForMate(m.ID, []MateTrigger{trig}); err != nil {
		t.Fatal(err)
	}
	ts, _ := store.ListTriggers(m.ID)
	if _, err := store.TurnBot(m.ID, ts[0].ID); err != nil {
		t.Fatalf("a live wake: %v", err)
	}

	trig.ID, trig.Enabled = ts[0].ID, false
	if err := store.ReplaceTriggersForMate(m.ID, []MateTrigger{trig}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.TurnBot(m.ID, ts[0].ID); !errors.Is(err, ErrTriggerDisabled) {
		t.Fatalf("paused trigger: %v", err)
	}
	if _, err := store.TurnBot(m.ID, "gone"); !errors.Is(err, ErrTriggerDisabled) {
		t.Fatalf("deleted trigger: %v", err)
	}

	m.Enabled = false
	if err := store.UpdateMate(m); err != nil {
		t.Fatal(err)
	}
	if _, err := store.TurnBot(m.ID, ""); !errors.Is(err, ErrTriggerDisabled) {
		t.Fatalf("disabled bot: %v", err)
	}
	if err := store.DeleteMate(m.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.TurnBot(m.ID, ""); !errors.Is(err, ErrTriggerDisabled) {
		t.Fatalf("deleted bot: %v", err)
	}
}
