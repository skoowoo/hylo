package mate

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestChannelBelongsToOneBot(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "vault"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	a := &Mate{Name: "A", AgentID: "pi", Enabled: true}
	b := &Mate{Name: "B", AgentID: "pi", Enabled: true}
	if err := store.CreateMate(a); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateMate(b); err != nil {
		t.Fatal(err)
	}

	first, err := store.SaveChannel(Channel{
		MateID: a.ID, Kind: ChannelTelegram, Token: "tok", OwnerUserID: "9", ExternalID: "bot-1", Display: "@a",
	})
	if err != nil || first.ID == "" {
		t.Fatal(err)
	}
	again, err := store.SaveChannel(Channel{
		MateID: a.ID, Kind: ChannelTelegram, Token: "tok-2", OwnerUserID: "9", ExternalID: "bot-1", Display: "@a",
	})
	if err != nil {
		t.Fatal(err)
	}
	if again.ID == first.ID {
		t.Fatal("a replaced binding kept its id")
	}
	if left, _ := store.ListMateChannels(a.ID); len(left) != 1 || left[0].Token != "tok-2" {
		t.Fatalf("after replace %+v", left)
	}
	_, err = store.SaveChannel(Channel{
		MateID: b.ID, Kind: ChannelTelegram, Token: "other", OwnerUserID: "8", ExternalID: "bot-1",
	})
	if !errors.Is(err, ErrChannelTaken) {
		t.Fatalf("second bot: %v", err)
	}
	// A different WeChat contact can belong to the other bot.
	if _, err := store.SaveChannel(Channel{
		MateID: b.ID, Kind: ChannelWechat, Token: "wx", OwnerUserID: "u@im.wechat", ExternalID: "b@im.bot",
	}); err != nil {
		t.Fatal(err)
	}
	active, err := store.ListActiveChannels()
	if err != nil || len(active) != 2 {
		t.Fatalf("active %+v %v", active, err)
	}
	// A different WeChat account can be connected at the same time.
	if _, err := store.SaveChannel(Channel{
		MateID: a.ID, Kind: ChannelWechat, Token: "wx-other", OwnerUserID: "v@im.wechat", ExternalID: "v@im.bot",
	}); err != nil {
		t.Fatal(err)
	}
	// The same WeChat account scanning onto another bot drops the previous one.
	if _, err := store.SaveChannel(Channel{
		MateID: a.ID, Kind: ChannelWechat, Token: "wx-2", OwnerUserID: "u@im.wechat", ExternalID: "a@im.bot",
	}); err != nil {
		t.Fatal(err)
	}
	active, err = store.ListActiveChannels()
	if err != nil {
		t.Fatal(err)
	}
	var wechatMate string
	for _, c := range active {
		if c.Kind != ChannelWechat {
			continue
		}
		if wechatMate != "" {
			t.Fatalf("wechat still on two bots: %+v", active)
		}
		wechatMate = c.MateID
		if c.ExternalID != "a@im.bot" {
			t.Fatalf("kept %s, want the new contact", c.ExternalID)
		}
	}
	if wechatMate != a.ID {
		t.Fatalf("wechat left on %s", wechatMate)
	}
	if err := store.DeleteMate(b.ID); err != nil {
		t.Fatal(err)
	}
	left, err := store.ListChannels()
	if err != nil || len(left) != 2 {
		t.Fatalf("after delete %+v %v", left, err)
	}
	for _, c := range left {
		if c.MateID != a.ID {
			t.Fatalf("channel left on deleted bot: %+v", c)
		}
	}
}

func TestNewChatReusingAnEmptyOneMakesItCurrent(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "vault"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	m := &Mate{Name: "A", AgentID: "pi", Enabled: true}
	if err := store.CreateMate(m); err != nil {
		t.Fatal(err)
	}
	old, _ := store.CreateConversation(m.ID, "old")
	if _, err := store.InsertMessage(Message{ConversationID: old.ID, Role: "user", Content: "hi"}); err != nil {
		t.Fatal(err)
	}
	empty, _ := store.CreateConversation(m.ID, "empty")
	// The user went back to the old chat after opening the empty one.
	if _, err := store.db.Exec(`UPDATE conversations SET created_at = created_at - 90000, updated_at = updated_at - 30000 WHERE id = ?`, old.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`UPDATE conversations SET updated_at = updated_at - 60000 WHERE id = ?`, empty.ID); err != nil {
		t.Fatal(err)
	}
	conv, err := store.EnsureNewConv(m.ID, ConvTypeChat, "", 10)
	if err != nil || conv.ID != empty.ID {
		t.Fatalf("new chat %s %v, want the empty one", conv.ID, err)
	}
	if cur, _ := store.GetOrCreateActiveChatConvID(m.ID); cur != empty.ID {
		t.Fatalf("current chat %s, want %s", cur, empty.ID)
	}
}
