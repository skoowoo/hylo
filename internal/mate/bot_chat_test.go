package mate

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// A bot that reaches out leaves a wake task before each reply; pages count
// what a reader sees, and the cursor always moves.
func TestBotChatPagesSkipWakes(t *testing.T) {
	s, id := pulseStore(t)
	conv, _ := s.StartConversation(id, "")
	for i := 0; i < 6; i++ {
		s.InsertMessage(Message{ConversationID: conv.ID, Role: "wake", MateID: id, Content: "task", TriggerEvent: "scheduled"})
		time.Sleep(time.Millisecond)
		s.InsertMessage(Message{ConversationID: conv.ID, Role: "assistant", MateID: id, Status: "succeeded", Content: fmt.Sprint("news ", i), TriggerEvent: "scheduled"})
		time.Sleep(time.Millisecond)
	}
	_, lines, more, err := s.BotChat(id, "", 4)
	if err != nil || len(lines) != 4 || !more || lines[3].Text != "news 5" {
		t.Fatalf("first page = %d lines, more %v, %v: %+v", len(lines), more, err, lines)
	}
	var seen int
	before := ""
	for page := 0; page < 10; page++ {
		_, lines, more, err = s.BotChat(id, before, 1)
		if err != nil {
			t.Fatal(err)
		}
		if len(lines) == 0 {
			t.Fatalf("page %d is empty with more = %v: no cursor to go on", page, more)
		}
		seen += len(lines)
		before = lines[0].ID
		if !more {
			break
		}
	}
	if seen != 6 {
		t.Fatalf("paged through %d replies, want 6", seen)
	}
}

func TestBotChatHistoryAcrossConversations(t *testing.T) {
	s, id := pulseStore(t)
	other := &Mate{Name: "Other"}
	if err := s.CreateMate(other); err != nil {
		t.Fatal(err)
	}
	otherChat, _ := s.StartConversation(other.ID, "")
	s.InsertMessage(Message{ConversationID: otherChat.ID, Role: "user", Content: "private to other bot"})
	old, _ := s.StartConversation(id, "")
	insert := func(conv, role, status, content string, at int64) Message {
		t.Helper()
		m, err := s.InsertMessage(Message{ConversationID: conv, Role: role, Status: status, Content: content})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.db.Exec(`UPDATE chat_messages SET created_at = ? WHERE id = ?`, at, m.ID); err != nil {
			t.Fatal(err)
		}
		return m
	}
	insert(old.ID, "user", "", "too old", 99)
	long := strings.Repeat("长", 5000)
	first := insert(old.ID, "user", "", long, 100)
	second := insert(old.ID, "assistant", "succeeded", long, 100)
	insert(old.ID, "wake", "", "internal task", 100)
	insert(old.ID, "assistant", "failed", "unfinished", 100)
	insert(old.ID, "assistant", "running", "streaming", 100)
	current, _ := s.StartConversation(id, "")
	last := insert(current.ID, "user", "", "correction", 100)
	lines, more, err := s.BotChatHistory(id, time.UnixMilli(100), "", 2)
	if err != nil || !more || len(lines) != 2 {
		t.Fatalf("first page: %+v, %v, %v", lines, more, err)
	}
	if lines[0].ID != second.ID || lines[0].Text != strings.Repeat("长", chatLineMax)+"…" || !lines[0].Truncated || lines[0].ChatID != old.ID || lines[1].ID != last.ID {
		t.Fatalf("wrong history: %+v", lines)
	}
	next, more, err := s.BotChatHistory(id, time.UnixMilli(100), lines[0].ID, 2)
	if err != nil || more || len(next) != 1 || next[0].ID != first.ID || next[0].Text != long || next[0].Truncated {
		t.Fatalf("second page: %+v, %v, %v", next, more, err)
	}
	if err := s.DeleteConversation(old.ID); err != nil {
		t.Fatal(err)
	}
	lines, more, err = s.BotChatHistory(id, time.UnixMilli(100), "", 50)
	if err != nil || more || len(lines) != 1 || lines[0].ID != last.ID {
		t.Fatalf("after deletion: %+v, %v, %v", lines, more, err)
	}
}
