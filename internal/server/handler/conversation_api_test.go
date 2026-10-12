package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hardhacker/hylo/internal/mate"
)

func TestConversationOpensWithGreeting(t *testing.T) {
	store, err := mate.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	bot := &mate.Mate{Name: "Hylo", AgentID: "pi"}
	if err := store.CreateMate(bot); err != nil {
		t.Fatal(err)
	}
	api := NewConversationAPI(store)
	w := httptest.NewRecorder()
	body := `{"mateId":"` + bot.ID + `","greeting":"Your notes keep circling one question."}`
	api.ConversationsPOST(w, httptest.NewRequest(http.MethodPost, "/api/conversations", strings.NewReader(body)))
	if w.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", w.Code, w.Body)
	}
	convs, _ := store.ListConversationsByType(bot.ID, mate.ConvTypeChat)
	msgs, _ := store.ListMessages(convs[0].ID)
	if len(msgs) != 1 || msgs[0].Role != "assistant" || msgs[0].AgentID != "pi" || msgs[0].Content != "Your notes keep circling one question." {
		t.Fatalf("got %+v", msgs)
	}
}
