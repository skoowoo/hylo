package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hardhacker/hylo/internal/mate"
)

func TestSetupBotOnlyForAnEmptyVault(t *testing.T) {
	store, err := mate.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	api := &AgentAPI{logger: slog.Default(), store: store}
	post := func() (int, string) {
		w := httptest.NewRecorder()
		body := `{"agentId":"pi","model":"sonnet","greeting":"Your notes keep circling one question."}`
		api.SetupBotPOST(w, httptest.NewRequest(http.MethodPost, "/api/setup/bot", strings.NewReader(body)))
		var out struct{ MateID string }
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		return w.Code, out.MateID
	}

	code, id := post()
	if code != http.StatusCreated || id == "" {
		t.Fatalf("first setup: %d %q", code, id)
	}
	if bot, _ := store.GetMate(id); bot.Model != "sonnet" || !bot.Main || bot.Cwd != "/" {
		t.Fatalf("bot %+v", bot)
	}
	triggers, _ := store.ListTriggers(id)
	var names []string
	for _, tr := range triggers {
		names = append(names, tr.Name)
	}
	if got := strings.Join(names, ","); got != "WeChat,Discord,Compiler,Compiler,Memory,Recap,Recap" {
		t.Fatalf("trigger names in order: %s", got)
	}
	convs, _ := store.ListConversationsByType(id, mate.ConvTypeChat)
	msgs, _ := store.ListMessages(convs[0].ID)
	if len(msgs) != 1 || msgs[0].Role != "assistant" || msgs[0].AgentID != "pi" {
		t.Fatalf("greeting: %+v", msgs)
	}

	if code, id := post(); code != http.StatusOK || id != "" {
		t.Fatalf("second setup: %d %q", code, id)
	}
}

func TestHyloBotTriggersAreValid(t *testing.T) {
	_, triggers := hyloBot("pi", "")
	if err := mate.ValidateTriggers(triggers); err != nil {
		t.Fatal(err)
	}
}

