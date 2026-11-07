package mate

import (
	"fmt"
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
