package mate

import (
	"path/filepath"
	"testing"
	"time"
)

func TestWriteAndReadTimesAreTheUsers(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "vault"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	conv, err := store.CreateConversation("mate-1")
	if err != nil {
		t.Fatal(err)
	}
	asked, _ := store.InsertMessage(Message{ConversationID: conv.ID, Role: "assistant", MateID: "mate-1"})
	wake, _ := store.InsertMessage(Message{ConversationID: conv.ID, Role: "assistant", MateID: "mate-1", TriggerEvent: "wake"})
	now := time.Now()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(store.RecordUserVisit("/mine.md", NoteAccessWrite, now))
	must(store.InsertMessageNoteAccess(asked.ID, []NoteAccessEntry{
		{Path: "/asked.md", Action: NoteAccessWrite, At: now},
		{Path: "/_memory/state.md", Action: NoteAccessWrite, At: now},
		{Path: "/looked.md", Action: NoteAccessRead, At: now},
	}))
	must(store.InsertMessageNoteAccess(wake.ID, []NoteAccessEntry{{Path: "/own.md", Action: NoteAccessWrite, At: now}}))

	since := now.Add(-time.Hour)
	if w, err := store.WriteTimes(since); err != nil || len(w) != 2 {
		t.Fatalf("writes = %v %v, want the user's and the one they asked for", w, err)
	}
	if r, err := store.ReadTimes(since); err != nil || len(r) != 1 {
		t.Fatalf("reads = %v %v", r, err)
	}
	days, err := store.AroundDays(since)
	if err != nil || !days[now.Format(time.DateOnly)] || len(days) != 1 {
		t.Fatalf("around = %v %v", days, err)
	}
}
