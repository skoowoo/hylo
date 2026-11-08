package mate

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func pulseStore(t *testing.T) (*Store, string) {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "vault"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	m := &Mate{Name: "Hylo", Main: true, Enabled: true}
	if err := s.CreateMate(m); err != nil {
		t.Fatal(err)
	}
	return s, m.ID
}

func TestAgendaDueFiresOnce(t *testing.T) {
	s, id := pulseStore(t)
	now := time.Now()
	due, err := s.AddAgenda(id, "ask how the talk went", now.Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddAgenda(id, "later", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddAgenda(id, "someday", time.Time{}); err != nil {
		t.Fatal(err)
	}
	got, _ := s.DueAgenda(id, now)
	if len(got) != 1 || got[0].ID != due.ID {
		t.Fatalf("due = %+v", got)
	}
	if err := s.MarkAgendaFired([]string{due.ID}, now); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.DueAgenda(id, now); len(got) != 0 {
		t.Fatalf("a fired item woke the bot again: %+v", got)
	}
	open, _ := s.OpenAgenda(id)
	if len(open) != 3 || open[0].ID != due.ID || open[2].Text != "someday" {
		t.Fatalf("open = %+v", open)
	}
	if err := s.FinishAgenda(id, due.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.FinishAgenda(id, due.ID); !errors.Is(err, ErrAgendaItemNotFound) {
		t.Fatalf("finishing twice: %v", err)
	}
	if err := s.FinishAgenda("other-bot", open[1].ID); !errors.Is(err, ErrAgendaItemNotFound) {
		t.Fatalf("another bot finished it: %v", err)
	}
}

func TestPulseJournal(t *testing.T) {
	s, id := pulseStore(t)
	t0 := time.Now().Add(-time.Hour)
	p, err := s.StartPulse(id, "chance", t0)
	if err != nil {
		t.Fatal(err)
	}
	p.Decision, p.Brief, p.EndedAt, p.Wrote = PulseSpeak, "tell them", t0.Add(time.Minute), []string{"/a.md"}
	if err := s.EndPulse(p); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPulseDelivery(p.ID, "sent"); err != nil {
		t.Fatal(err)
	}
	if settled, _ := s.PulseSettled(id); time.Since(settled) > time.Minute {
		t.Fatalf("settled at %v, not when the brief's chat turn ended", settled)
	}
	n, last, err := s.PulseCount(id, t0)
	if err != nil || n != 1 || !last.Equal(time.UnixMilli(t0.UnixMilli())) {
		t.Fatalf("count = %d, %v, %v", n, last, err)
	}
	ps, _ := s.ListPulses(id, 5)
	if len(ps) != 1 || ps[0].Delivery != "sent" || ps[0].Brief != "tell them" || len(ps[0].Wrote) != 1 {
		t.Fatalf("journal = %+v", ps)
	}
	acts, _ := s.ListNoteActivity(t0, time.Now())
	if len(acts) != 1 || acts[0].Actor != id || acts[0].Action != NoteAccessWrite {
		t.Fatalf("a round's write is not the bot's activity: %+v", acts)
	}
}

func TestSentWakesReplied(t *testing.T) {
	s, id := pulseStore(t)
	conv, _ := s.CreateConversation(id)
	a, _ := s.InsertMessage(Message{ConversationID: conv.ID, Role: "assistant", MateID: id, Status: "succeeded", Content: "found something", TriggerEvent: string(MateEventPulse)})
	time.Sleep(2 * time.Millisecond)
	if _, err := s.InsertMessage(Message{ConversationID: conv.ID, Role: "user", MateID: id, Content: "thanks"}); err != nil {
		t.Fatal(err)
	}
	ws, err := s.SentWakes(id, time.Now().Add(-time.Hour), time.Now())
	if err != nil || len(ws) != 1 || !ws[0].Replied || ws[0].Event != "pulse" {
		t.Fatalf("sent = %+v, %v", ws, err)
	}
	if last, _ := s.LastUserMessageAt(id); last.Before(a.CreatedAt) {
		t.Fatalf("last user message %v", last)
	}
}

func TestBotChatMarksUnread(t *testing.T) {
	s, id := pulseStore(t)
	conv, _ := s.StartConversation(id, "")
	s.InsertMessage(Message{ConversationID: conv.ID, Role: "user", MateID: id, Content: "hi"})
	hello, _ := s.InsertMessage(Message{ConversationID: conv.ID, Role: "assistant", MateID: id, Status: "succeeded", Content: "hello"})
	if _, err := s.MarkChatSeen(id, []string{hello.ID}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Millisecond)
	s.InsertMessage(Message{ConversationID: conv.ID, Role: "wake", MateID: id, Content: "task", TriggerEvent: "scheduled"})
	s.InsertMessage(Message{ConversationID: conv.ID, Role: "assistant", MateID: id, Status: "succeeded", Content: "news", TriggerEvent: "scheduled"})
	chat, lines, more, err := s.BotChat(id, "", 10)
	if err != nil || chat != conv.ID || more || len(lines) != 3 {
		t.Fatalf("chat = %q %+v %v %v", chat, lines, more, err)
	}
	if lines[1].Unread || !lines[2].Unread || !lines[2].OnItsOwn || lines[2].Source != "Scheduled" {
		t.Fatalf("lines = %+v", lines)
	}
}
