package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/hardhacker/hylo/internal/client"
	"github.com/hardhacker/hylo/internal/mate"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type fakeBots struct {
	mu      sync.Mutex
	bot     mate.Mate
	created []mate.Mate
	added   []string
	enabled []string
	ran     []string
	deleted []string
}

func botSession(t *testing.T, f *fakeBots) *sdk.ClientSession {
	t.Helper()
	mux := http.NewServeMux()
	reply := func(w http.ResponseWriter, v any) { json.NewEncoder(w).Encode(v) }
	mux.HandleFunc("GET /api/mates", func(w http.ResponseWriter, _ *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		reply(w, map[string]any{"mates": []mate.Mate{f.bot}})
	})
	mux.HandleFunc("GET /api/mates/{id}", func(w http.ResponseWriter, _ *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		reply(w, map[string]any{"mate": f.bot})
	})
	mux.HandleFunc("POST /api/mates", func(w http.ResponseWriter, r *http.Request) {
		var m mate.Mate
		json.NewDecoder(r.Body).Decode(&m)
		m.ID = "new"
		f.mu.Lock()
		f.created = append(f.created, m)
		f.mu.Unlock()
		w.WriteHeader(http.StatusCreated)
		reply(w, map[string]any{"mate": m})
	})
	mux.HandleFunc("PUT /api/mates/{id}", func(w http.ResponseWriter, r *http.Request) {
		var m mate.Mate
		json.NewDecoder(r.Body).Decode(&m)
		for i := range m.Triggers {
			if m.Triggers[i].ID == "" {
				m.Triggers[i].ID = "t-new"
			}
		}
		f.mu.Lock()
		f.bot = m
		f.mu.Unlock()
		reply(w, map[string]any{"mate": m})
	})
	mux.HandleFunc("DELETE /api/mates/{id}", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.deleted = append(f.deleted, r.PathValue("id"))
		f.mu.Unlock()
		reply(w, map[string]any{"ok": true})
	})
	mux.HandleFunc("POST /api/mates/{id}/triggers/{tid}/run", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.ran = append(f.ran, r.PathValue("tid"))
		f.mu.Unlock()
		w.WriteHeader(http.StatusAccepted)
	})
	mux.HandleFunc("GET /api/skills", func(w http.ResponseWriter, _ *http.Request) {
		reply(w, map[string]any{"skills": []client.SkillInfo{
			{Name: "hylo-notes", Default: true, Bots: []client.SkillBot{{ID: "news", Name: "News"}}},
			{Name: "the-news", Bots: []client.SkillBot{{ID: "news", Name: "News"}}},
			{Name: "humanizer"},
		}})
	})
	mux.HandleFunc("GET /api/skills/catalog", func(w http.ResponseWriter, _ *http.Request) {
		reply(w, map[string]any{"skills": []client.CatalogEntry{
			{Name: "hackernews", Repo: "a/b", Category: "news"},
			{Name: "the-news", Repo: "c/the-news", Category: "news", Added: true},
			{Name: "humanizer", Repo: "c/d", Added: true},
		}})
	})
	mux.HandleFunc("POST /api/skills/add", func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Repo, Skill string }
		json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		f.added = append(f.added, body.Repo+":"+body.Skill)
		f.mu.Unlock()
		reply(w, map[string]any{"ok": true, "name": body.Skill})
	})
	mux.HandleFunc("POST /api/skills/{name}/enable", func(w http.ResponseWriter, r *http.Request) {
		var body struct{ BotID string }
		json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		f.enabled = append(f.enabled, r.PathValue("name")+"@"+body.BotID)
		f.mu.Unlock()
		reply(w, map[string]any{"ok": true})
	})
	ts := httptest.NewServer(NewHandler(client.NewInProcess(mux)))
	t.Cleanup(ts.Close)
	cs, err := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "0"}, nil).
		Connect(context.Background(), &sdk.StreamableClientTransport{Endpoint: ts.URL}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

func TestBotCreateBorrowsTheAgent(t *testing.T) {
	f := &fakeBots{bot: mate.Mate{ID: "hylo", Name: "Hylo", AgentID: "claude", Model: "opus"}}
	cs := botSession(t, f)
	if m, isErr := call(t, cs, "bot_create", map[string]any{"name": "News"}); isErr || m["id"] != "new" {
		t.Fatalf("bot_create: %v %v", m, isErr)
	}
	if c := f.created[0]; c.AgentID != "claude" || c.Model != "opus" || !c.Enabled {
		t.Fatalf("created %+v", c)
	}
}

func TestTriggerAddDefaultsToScheduled(t *testing.T) {
	f := &fakeBots{bot: mate.Mate{ID: "news", Name: "News", Triggers: []mate.MateTrigger{{ID: "t1", EventTypes: []string{"scheduled"}, Schedule: "daily 08:00", Prompt: "a"}}}}
	cs := botSession(t, f)
	m, isErr := call(t, cs, "trigger_add", map[string]any{"bot_id": "news", "prompt": "b", "schedule": "daily 08:05"})
	if isErr || m["trigger_id"] != "t-new" {
		t.Fatalf("trigger_add: %v %v", m, isErr)
	}
	if ts := f.bot.Triggers; len(ts) != 2 || ts[0].ID != "t1" || ts[1].EventTypes[0] != "scheduled" || !ts[1].Enabled {
		t.Fatalf("triggers %+v", ts)
	}
	if _, isErr := call(t, cs, "trigger_add", map[string]any{"bot_id": "news", "prompt": "c"}); !isErr {
		t.Fatal("a trigger needs a schedule or events")
	}
	if _, isErr := call(t, cs, "trigger_run", map[string]any{"bot_id": "news", "trigger_id": "t1"}); isErr || f.ran[0] != "t1" {
		t.Fatalf("trigger_run: %v %v", f.ran, isErr)
	}
	if _, isErr := call(t, cs, "trigger_delete", map[string]any{"bot_id": "news", "trigger_id": "t1"}); isErr || len(f.bot.Triggers) != 1 || f.bot.Triggers[0].ID != "t-new" {
		t.Fatalf("trigger_delete: %+v", f.bot.Triggers)
	}
}

// The catalog gives exactly what skills_add takes, and adding leaves turning on to skills_enable.
func TestSkillsCatalogAddEnable(t *testing.T) {
	f := &fakeBots{}
	cs := botSession(t, f)
	m, _ := call(t, cs, "skills_catalog", map[string]any{"category": "news"})
	if n := len(m["skills"].([]any)); n != 2 {
		t.Fatalf("news skills: %v", m)
	}
	if m, isErr := call(t, cs, "skills_add", map[string]any{"repo": "a/b", "skill": "hackernews"}); isErr || m["name"] != "hackernews" {
		t.Fatalf("skills_add: %v %v", m, isErr)
	}
	if len(f.enabled) != 0 {
		t.Fatal("adding turned the skill on")
	}
	call(t, cs, "skills_enable", map[string]any{"name": "hackernews", "bot_id": "news"})
	if strings.Join(f.added, ",") != "a/b:hackernews" || strings.Join(f.enabled, ",") != "hackernews@news" {
		t.Fatalf("added %v enabled %v", f.added, f.enabled)
	}
}

func TestBotUpdateAndDelete(t *testing.T) {
	f := &fakeBots{bot: mate.Mate{ID: "news", Name: "News", Description: "d", SystemPrompt: "old", Enabled: true,
		Triggers: []mate.MateTrigger{{ID: "t1", EventTypes: []string{"scheduled"}, Schedule: "daily 08:00", Prompt: "a"}}}}
	cs := botSession(t, f)
	if _, isErr := call(t, cs, "bot_update", map[string]any{"bot_id": "news", "system_prompt": "new", "enabled": false}); isErr {
		t.Fatal("bot_update failed")
	}
	if b := f.bot; b.Name != "News" || b.Description != "d" || b.SystemPrompt != "new" || b.Enabled || len(b.Triggers) != 1 {
		t.Fatalf("bot %+v", b)
	}
	if _, isErr := call(t, cs, "bot_delete", map[string]any{"bot_id": "news"}); isErr || f.deleted[0] != "news" {
		t.Fatalf("bot_delete: %v", f.deleted)
	}
	f.bot.Main = true
	if _, isErr := call(t, cs, "bot_update", map[string]any{"bot_id": "news", "name": "x"}); !isErr {
		t.Fatal("main bot must not be updated")
	}
	if _, isErr := call(t, cs, "bot_delete", map[string]any{"bot_id": "news"}); !isErr || len(f.deleted) != 1 {
		t.Fatal("main bot must not be deleted")
	}
}

func TestCapabilitiesInOneCall(t *testing.T) {
	f := &fakeBots{bot: mate.Mate{ID: "news", Name: "News", Cwd: "/_bots/_news", Enabled: true,
		Triggers: []mate.MateTrigger{{ID: "t1", EventTypes: []string{"scheduled"}, Schedule: "daily 08:00", Prompt: "long prompt"}}}}
	cs := botSession(t, f)
	m, isErr := call(t, cs, "capabilities", map[string]any{})
	if isErr {
		t.Fatalf("capabilities: %v", m)
	}
	bots := m["bots"].([]any)
	b := bots[0].(map[string]any)
	tr := b["triggers"].([]any)[0].(map[string]any)
	if sk := b["skills"].([]any); len(bots) != 1 || len(sk) != 1 || sk[0] != "the-news" || tr["schedule"] != "daily 08:00" || tr["prompt"] != nil {
		t.Fatalf("bots %v", bots)
	}
	var catalog []string
	for _, e := range m["catalog"].([]any) {
		catalog = append(catalog, e.(map[string]any)["name"].(string))
	}
	if strings.Join(catalog, ",") != "hackernews" {
		t.Fatalf("catalog must leave out added skills: %v", catalog)
	}
	if u := m["unused_skills"].([]any); len(u) != 1 || u[0] != "humanizer" {
		t.Fatalf("unused %v", u)
	}
	if len(m["events"].([]any)) != len(mate.BuiltinEvents) {
		t.Fatal("events")
	}
}
