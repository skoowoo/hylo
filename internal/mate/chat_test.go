package mate

import (
	"errors"
	"path/filepath"
	"slices"
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

	conv, err := store.CreateConversation("mate-1")
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

func TestCurrentConversation(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "vault"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	m := &Mate{Name: "Bot", AgentID: "claude", Enabled: true}
	if err := store.CreateMate(m); err != nil {
		t.Fatal(err)
	}

	id1, err := store.CurrentConversationID(m.ID)
	if err != nil {
		t.Fatal(err)
	}
	if id2, _ := store.CurrentConversationID(m.ID); id1 != id2 {
		t.Fatalf("expected same conv, got %s vs %s", id1, id2)
	}
	if _, err := store.InsertMessage(Message{ConversationID: id1, Role: "user", Content: "first ask"}); err != nil {
		t.Fatal(err)
	}

	// A new chat waits for its first message; nothing is stored until then.
	if err := store.SetActiveConversation(m.ID, ""); err != nil {
		t.Fatal(err)
	}
	if cur, _ := store.ActiveConversationID(m.ID); cur != "" {
		t.Fatalf("current %q after new chat", cur)
	}
	next, err := store.StartConversation(m.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.InsertMessage(Message{ConversationID: next.ID, Role: "user", Content: "second ask"}); err != nil {
		t.Fatal(err)
	}

	// Writing to the older chat moves it up the list but doesn't make it current.
	if _, err := store.db.Exec(`UPDATE conversations SET updated_at = updated_at - 1000 WHERE id = ?`, next.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.SetConversationAgentSession(id1, "claude", "/", "s1"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.InsertMessage(Message{ConversationID: id1, Role: "assistant", Content: "late"}); err != nil {
		t.Fatal(err)
	}
	if cur, _ := store.ActiveConversationID(m.ID); cur != next.ID {
		t.Fatalf("current %s, want %s", cur, next.ID)
	}
	convs, _ := store.ListConversations(m.ID)
	if len(convs) != 2 || convs[0].ID != id1 || convs[0].Title != "first ask" || convs[1].Title != "second ask" {
		t.Fatalf("list %+v", convs)
	}

	if err := store.SetActiveConversation(m.ID, id1); err != nil {
		t.Fatal(err)
	}
	if cur, _ := store.CurrentConversationID(m.ID); cur != id1 {
		t.Fatalf("continued %s, want %s", cur, id1)
	}
	if err := store.SetActiveConversation("other", id1); err == nil {
		t.Fatal("another bot's chat was made current")
	}
	if err := store.DeleteConversation(id1); err != nil {
		t.Fatal(err)
	}
	if cur, _ := store.ActiveConversationID(m.ID); cur != "" {
		t.Fatalf("current %q after deleting it", cur)
	}
}

func TestStartConversationKeepsTheNewest(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "vault"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	m := &Mate{Name: "Bot", AgentID: "claude", Enabled: true}
	if err := store.CreateMate(m); err != nil {
		t.Fatal(err)
	}
	var first string
	for i := 0; i < keepChats; i++ {
		conv, err := store.StartConversation(m.ID, "")
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first = conv.ID
		}
		if _, err := store.db.Exec(`UPDATE conversations SET updated_at = ? WHERE id = ?`, i, conv.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := store.InsertMessage(Message{ConversationID: conv.ID, Role: "user", Content: "hi"}); err != nil {
			t.Fatal(err)
		}
		if _, err := store.db.Exec(`UPDATE conversations SET updated_at = ? WHERE id = ?`, i, conv.ID); err != nil {
			t.Fatal(err)
		}
	}
	abandoned, _ := store.CreateConversation(m.ID)
	if _, err := store.StartConversation(m.ID, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetConversation(first); err == nil {
		t.Fatal("the oldest chat was kept")
	}
	if _, err := store.GetConversation(abandoned.ID); err == nil {
		t.Fatal("a chat without messages was kept")
	}
	if convs, _ := store.ListConversations(m.ID); len(convs) != keepChats-1 {
		t.Fatalf("%d chats with messages, want %d", len(convs), keepChats-1)
	}
}

func TestInsertMessageTriggerEvent(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(filepath.Join(dir, "vault"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	conv, err := store.CreateConversation("mate-1")
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

	conv, err := store.CreateConversation("mate-1")
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

	conv, err := store.CreateConversation("mate-1")
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

	conv, err := store.CreateConversation("mate-1")
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
	conv, err := store.CreateConversation("mate-1")
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
	conv, err := store.CreateConversation("mate-1")
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
	conv, err := store.CreateConversation(m.ID)
	if err != nil {
		t.Fatal(err)
	}
	add := func(msg Message) Message {
		msg.ConversationID, msg.MateID = conv.ID, m.ID
		out, err := store.InsertMessage(msg)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	unread := func() int {
		list, err := store.ListMates()
		if err != nil || len(list) != 1 {
			t.Fatal(err, list)
		}
		return list[0].Unread
	}
	reply := add(Message{Role: "assistant", Status: "succeeded"})
	add(Message{Role: "assistant", Status: "running", TriggerEvent: "scheduled"})
	add(Message{Role: "assistant", Status: "succeeded", App: "telegram"})
	time.Sleep(2 * time.Millisecond)
	wake := add(Message{Role: "assistant", Status: "succeeded", TriggerEvent: "scheduled"})
	time.Sleep(2 * time.Millisecond)
	last := add(Message{Role: "assistant", Status: "failed", TriggerEvent: "note_created"})
	if n := unread(); n != 3 {
		t.Fatalf("unread = %d, want the three finished replies, to the user or on its own, but not the one read in Telegram", n)
	}
	if n, err := store.MarkChatSeen(m.ID, []string{reply.ID, wake.ID, "gone"}); err != nil || n != 1 {
		t.Fatalf("seen up to the wake reply: unread = %d, %v; want the later one left", n, err)
	}
	if n, err := store.MarkChatSeen(m.ID, []string{last.ID}); err != nil || n != 0 {
		t.Fatalf("unread after seen = %d, %v", n, err)
	}
	if n, err := store.MarkChatSeen(m.ID, []string{reply.ID}); err != nil || n != 0 {
		t.Fatalf("an older row moved seen back: unread = %d, %v", n, err)
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
	conv, _ := store.CreateConversation(m.ID)
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

func TestNoteAccessLogsActivityThatFollowsRenamesAndOutlivesChat(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "vault"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	conv, err := store.CreateConversation("mate-1")
	if err != nil {
		t.Fatal(err)
	}
	msg, err := store.InsertMessage(Message{ConversationID: conv.ID, Role: "assistant", MateID: "mate-1"})
	if err != nil {
		t.Fatal(err)
	}
	at := time.UnixMilli(1_700_000_000_000)
	if err := store.InsertMessageNoteAccess(msg.ID, []NoteAccessEntry{
		{Path: "/a.md", Action: NoteAccessRead, Tool: "Read", At: at},
		{Path: "/b.md", Action: NoteAccessWrite, Tool: "Edit", At: at},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.RenameNotePath("/a.md", "/dir/a.md"); err != nil {
		t.Fatal(err)
	}

	access, err := store.ListMessageNoteAccess(msg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(access) != 2 || access[0].Path != "/dir/a.md" {
		t.Fatalf("note access = %+v", access)
	}

	if err := store.DeleteConversation(conv.ID); err != nil {
		t.Fatal(err)
	}
	rows, err := store.db.Query(`SELECT path, actor, action, at FROM note_activity ORDER BY path`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	type row struct {
		path, actor, action string
		at                  int64
	}
	var got []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.path, &r.actor, &r.action, &r.at); err != nil {
			t.Fatal(err)
		}
		got = append(got, r)
	}
	want := []row{
		{"/b.md", "mate-1", NoteAccessWrite, at.UnixMilli()},
		{"/dir/a.md", "mate-1", NoteAccessRead, at.UnixMilli()},
	}
	if len(got) != len(want) {
		t.Fatalf("activity = %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("activity[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestBotActivityKeepsOnlyReadsThatReadTheNote(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "vault"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	conv, err := store.CreateConversation("mate-1")
	if err != nil {
		t.Fatal(err)
	}
	asked, err := store.InsertMessage(Message{ConversationID: conv.ID, Role: "assistant", MateID: "mate-1"})
	if err != nil {
		t.Fatal(err)
	}
	wake, err := store.InsertMessage(Message{ConversationID: conv.ID, Role: "assistant", MateID: "mate-1", TriggerEvent: "wake"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.InsertMessageNoteAccess(asked.ID, []NoteAccessEntry{
		{Path: "/read.md", Action: NoteAccessRead},
		{Path: "/listed.md", Action: NoteAccessRead, Skim: true},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.InsertMessageNoteAccess(wake.ID, []NoteAccessEntry{
		{Path: "/swept.md", Action: NoteAccessRead},
		{Path: "/wrote.md", Action: NoteAccessWrite},
	}); err != nil {
		t.Fatal(err)
	}
	rows, err := store.ListNoteActivity(time.UnixMilli(0), time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range rows {
		got = append(got, r.Path)
	}
	slices.Sort(got)
	if want := []string{"/read.md", "/wrote.md"}; !slices.Equal(got, want) {
		t.Fatalf("activity = %v, want %v", got, want)
	}
	// The run's card still lists every note it touched.
	if access, _ := store.ListMessageNoteAccess(asked.ID); len(access) != 2 {
		t.Fatalf("card = %+v", access)
	}
}

func TestUpgradeMakesTheLatestChatCurrent(t *testing.T) {
	root := filepath.Join(t.TempDir(), "vault")
	store, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	m := &Mate{Name: "Bot", AgentID: "claude", Enabled: true}
	if err := store.CreateMate(m); err != nil {
		t.Fatal(err)
	}
	old, _ := store.CreateConversation(m.ID)
	latest, _ := store.CreateConversation(m.ID)
	empty, _ := store.CreateConversation(m.ID)
	for _, c := range []Conversation{old, latest} {
		if _, err := store.InsertMessage(Message{ConversationID: c.ID, Role: "user", Content: "hi"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.db.Exec(`UPDATE conversations SET updated_at = 1 WHERE id = ?`, old.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`ALTER TABLE mates DROP COLUMN active_conversation_id`); err != nil {
		t.Fatal(err)
	}
	_ = store.Close()

	store, err = Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if cur, _ := store.ActiveConversationID(m.ID); cur != latest.ID {
		t.Fatalf("current %s, want %s", cur, latest.ID)
	}
	if _, err := store.GetConversation(empty.ID); err == nil {
		t.Fatal("an empty chat survived the upgrade")
	}
}

func TestChatTitleFallsBackForChatsTheBotStarted(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "vault"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	m := &Mate{Name: "Bot", AgentID: "claude", Enabled: true}
	if err := store.CreateMate(m); err != nil {
		t.Fatal(err)
	}
	named := MateTrigger{Name: "Morning brief", EventTypes: []string{"scheduled"}, Schedule: "daily 09:00", Prompt: "brief", Enabled: true}
	unnamed := MateTrigger{EventTypes: []string{"scheduled"}, Schedule: "daily 10:00", Prompt: "recall", Enabled: true}
	triggers := []MateTrigger{named, unnamed}
	if err := store.ReplaceTriggersForMate(m.ID, triggers); err != nil {
		t.Fatal(err)
	}
	chat := func(msgs ...Message) string {
		conv, _ := store.CreateConversation(m.ID)
		for _, msg := range msgs {
			msg.ConversationID = conv.ID
			if _, err := store.InsertMessage(msg); err != nil {
				t.Fatal(err)
			}
		}
		return conv.ID
	}
	want := map[string]string{
		chat(Message{Role: "wake", Content: "brief", TriggerID: triggers[0].ID}, Message{Role: "user", Content: "thanks"}):                                               "thanks",
		chat(Message{Role: "wake", Content: "brief", TriggerID: triggers[0].ID}, Message{Role: "assistant", Content: "Today: …", TriggerID: triggers[0].ID}):             "Morning brief",
		chat(Message{Role: "wake", Content: "recall", TriggerID: triggers[1].ID}, Message{Role: "assistant", Content: "Two notes disagree.", TriggerID: triggers[1].ID}): "Two notes disagree.",
		chat(Message{Role: "assistant", Content: "Hi, I'm Bot."}):                                                                                                        "Hi, I'm Bot.",
	}
	convs, err := store.ListConversations(m.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range convs {
		if c.Title != want[c.ID] {
			t.Errorf("title %q, want %q", c.Title, want[c.ID])
		}
	}
	if len(convs) != len(want) {
		t.Fatalf("%d chats, want %d", len(convs), len(want))
	}
}

func TestListMatesRunning(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "vault"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	m := &Mate{Name: "Recall", Enabled: true}
	if err := store.CreateMate(m); err != nil {
		t.Fatal(err)
	}
	conv, err := store.CreateConversation(m.ID)
	if err != nil {
		t.Fatal(err)
	}
	running := func() bool {
		list, err := store.ListMates()
		if err != nil {
			t.Fatal(err)
		}
		return list[0].Running
	}
	reply, err := store.InsertMessage(Message{ConversationID: conv.ID, MateID: m.ID, Role: "assistant", Status: "running", App: "telegram"})
	if err != nil {
		t.Fatal(err)
	}
	if !running() {
		t.Fatal("a reply on its way, from any app, should show the bot at work")
	}
	if err := store.UpdateMessageDone(reply.ID, "done", "", "succeeded"); err != nil {
		t.Fatal(err)
	}
	if running() {
		t.Fatal("still at work after its reply finished")
	}
}
