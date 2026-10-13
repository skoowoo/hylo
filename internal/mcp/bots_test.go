package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/hardhacker/hylo/internal/client"
	"github.com/hardhacker/hylo/internal/mate"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type fakeBots struct {
	mu        sync.Mutex
	bot       mate.Mate
	created   []mate.Mate
	installed []string
	enabled   []string
	ran       []string
}

func botSession(t *testing.T, f *fakeBots, query string) *sdk.ClientSession {
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
	mux.HandleFunc("POST /api/mates/{id}/triggers/{tid}/run", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.ran = append(f.ran, r.PathValue("tid"))
		f.mu.Unlock()
		w.WriteHeader(http.StatusAccepted)
	})
	mux.HandleFunc("GET /api/skills", func(w http.ResponseWriter, _ *http.Request) {
		reply(w, map[string]any{"skills": []client.SkillInfo{
			{Name: "hackernews", Category: "news", RepoURL: "https://github.com/a/b", SubPath: "hackernews"},
			{Name: "the-news", Category: "news", Installed: true},
			{Name: "mine", Installed: true},
			{Name: "humanizer", RepoURL: "https://github.com/c/d"},
		}})
	})
	mux.HandleFunc("POST /api/skills/install", func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Skill string }
		json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		f.installed = append(f.installed, body.Skill)
		f.mu.Unlock()
		reply(w, map[string]any{"ok": true})
	})
	mux.HandleFunc("POST /api/skills/{name}/enable", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.enabled = append(f.enabled, r.PathValue("name"))
		f.mu.Unlock()
		reply(w, map[string]any{"ok": true})
	})
	gate := func(run string) error {
		if run == "trigger-run" {
			return errors.New("not from a trigger")
		}
		return nil
	}
	ts := httptest.NewServer(NewHandler(client.NewInProcess(mux), WithManageGate(gate)))
	t.Cleanup(ts.Close)
	cs, err := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "0"}, nil).
		Connect(context.Background(), &sdk.StreamableClientTransport{Endpoint: ts.URL + query}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

func TestBotCreateBorrowsTheAgent(t *testing.T) {
	f := &fakeBots{bot: mate.Mate{ID: "hylo", Name: "Hylo", AgentID: "claude", Model: "opus"}}
	cs := botSession(t, f, "")
	if m, isErr := call(t, cs, "bot_create", map[string]any{"name": "News"}); isErr || m["id"] != "new" {
		t.Fatalf("bot_create: %v %v", m, isErr)
	}
	if c := f.created[0]; c.AgentID != "claude" || c.Model != "opus" || !c.Enabled {
		t.Fatalf("created %+v", c)
	}
}

func TestTriggerAddDefaultsToScheduled(t *testing.T) {
	f := &fakeBots{bot: mate.Mate{ID: "news", Name: "News", Triggers: []mate.MateTrigger{{ID: "t1", EventTypes: []string{"scheduled"}, Schedule: "daily 08:00", Prompt: "a"}}}}
	cs := botSession(t, f, "")
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

func TestTriggerRunsCannotManage(t *testing.T) {
	f := &fakeBots{bot: mate.Mate{ID: "hylo", Name: "Hylo", AgentID: "claude"}}
	cs := botSession(t, f, "?run=trigger-run")
	for _, tool := range []string{"bot_create", "trigger_add", "trigger_run", "skills_install"} {
		if _, isErr := call(t, cs, tool, map[string]any{"name": "x", "bot_id": "hylo", "trigger_id": "t1", "prompt": "p", "schedule": "daily 08:00"}); !isErr {
			t.Errorf("%s ran from a trigger run", tool)
		}
	}
	if _, isErr := call(t, cs, "bot_list", nil); isErr {
		t.Error("reading is fine from a trigger run")
	}
	if len(f.created)+len(f.installed)+len(f.ran) != 0 {
		t.Fatalf("changes went through: %+v", f)
	}
}

func TestSkillInstallOnlyFromTheCatalog(t *testing.T) {
	f := &fakeBots{}
	cs := botSession(t, f, "")
	m, _ := call(t, cs, "skills_list", map[string]any{"category": "news"})
	if n := len(m["skills"].([]any)); n != 2 {
		t.Fatalf("news skills: %v", m)
	}
	call(t, cs, "skills_install", map[string]any{"name": "hackernews"})
	call(t, cs, "skills_install", map[string]any{"name": "the-news"})
	if _, isErr := call(t, cs, "skills_install", map[string]any{"name": "some/repo"}); !isErr {
		t.Error("installed a skill outside the catalog")
	}
	if len(f.installed) != 1 || f.installed[0] != "hackernews" || len(f.enabled) != 1 || f.enabled[0] != "the-news" {
		t.Fatalf("installed %v enabled %v", f.installed, f.enabled)
	}
}
