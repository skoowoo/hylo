package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/hardhacker/hylo/internal/mate"
)

func exploreStore(t *testing.T) (*mate.Store, *mate.Mate) {
	t.Helper()
	s, err := mate.Open(filepath.Join(t.TempDir(), "vault"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	news := &mate.Mate{Name: "News", Enabled: true}
	if err := s.CreateMate(news); err != nil {
		t.Fatal(err)
	}
	return s, news
}

func TestActivityNamesItsActors(t *testing.T) {
	s, news := exploreStore(t)
	now := time.Now()
	if err := s.RecordUserVisit("/a.md", mate.NoteAccessRead, now.Add(-2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordUserVisit("/old.md", mate.NoteAccessRead, now.Add(-48*time.Hour)); err != nil {
		t.Fatal(err)
	}
	p := mate.PulseRun{MateID: news.ID, EndedAt: now.Add(-time.Hour), Wrote: []string{"/news.md"}}
	if pr, err := s.StartPulse(news.ID, "x", now.Add(-time.Hour)); err == nil {
		p.ID = pr.ID
	}
	if err := s.EndPulse(p); err != nil {
		t.Fatal(err)
	}
	get := func(q string) []ActivityEntry {
		w := httptest.NewRecorder()
		NewActivityAPI(s).ActivityGET(w, httptest.NewRequest(http.MethodGet, "/api/activity?"+q, nil))
		if w.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", q, w.Code, w.Body)
		}
		var out struct{ Entries []ActivityEntry }
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		return out.Entries
	}
	all := get("")
	if len(all) != 2 || all[0].Actor != "News" || all[0].BotID != news.ID || all[1].Actor != "user" {
		t.Fatalf("a day back = %+v", all)
	}
	if got := get("since=3d&actor=user"); len(got) != 2 {
		t.Fatalf("three days of the user = %+v", got)
	}
	w := httptest.NewRecorder()
	NewActivityAPI(s).ActivityGET(w, httptest.NewRequest(http.MethodGet, "/api/activity?since=lately", nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("bad since: %d", w.Code)
	}
}

func TestBotChatGET(t *testing.T) {
	s, news := exploreStore(t)
	conv, _ := s.StartConversation(news.ID, "")
	s.InsertMessage(mate.Message{ConversationID: conv.ID, Role: "assistant", MateID: news.ID, Status: "succeeded", Content: "today", TriggerEvent: "scheduled"})
	r := httptest.NewRequest(http.MethodGet, "/api/mates/"+news.ID+"/chat", nil)
	r.SetPathValue("id", news.ID)
	w := httptest.NewRecorder()
	NewConversationAPI(s).BotChatGET(w, r)
	var out struct {
		Bot      string
		Messages []mate.ChatLine
	}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if w.Code != http.StatusOK || out.Bot != "News" || len(out.Messages) != 1 || !out.Messages[0].Unread {
		t.Fatalf("%d %+v", w.Code, out)
	}
	r = httptest.NewRequest(http.MethodGet, "/api/mates/nope/chat", nil)
	r.SetPathValue("id", "nope")
	w = httptest.NewRecorder()
	NewConversationAPI(s).BotChatGET(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("unknown bot: %d", w.Code)
	}
}
