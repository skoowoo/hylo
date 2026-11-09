package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hardhacker/hylo/internal/apps"
	"github.com/hardhacker/hylo/internal/mate"
	"github.com/hardhacker/hylo/internal/skills"
)

func TestAppToolsNeedTheBotsSwitch(t *testing.T) {
	s, err := mate.Open(filepath.Join(t.TempDir(), "vault"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	main := &mate.Mate{Name: "Hylo", Main: true, Enabled: true}
	other := &mate.Mate{Name: "News", Enabled: true}
	for _, m := range []*mate.Mate{main, other} {
		if err := s.CreateMate(m); err != nil {
			t.Fatal(err)
		}
	}
	a := &AgentAPI{store: s}
	a.runBots.Store("run-news", other.ID)
	a.runBots.Store("run-main", main.ID)
	bots := func() ([]apps.Bot, error) {
		return []apps.Bot{{ID: main.ID, Main: true, Dir: "/"}, {ID: other.ID, Dir: "/_bots/_news"}}, nil
	}
	t.Setenv("HOME", t.TempDir())
	mgr, err := apps.New(apps.Options{Dir: filepath.Join(t.TempDir(), "apps"), Bots: bots, Skills: skills.Open(t.TempDir())})
	if err != nil {
		t.Fatal(err)
	}
	h := NewAppsHTTP(mgr, a.RunBot)

	call := func(run string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/api/apps/podwise/call", bytes.NewBufferString(`{"tool":"get_me"}`))
		r.SetPathValue("id", "podwise")
		r.Header.Set(RunHeader, run)
		w := httptest.NewRecorder()
		h.Call(w, r)
		return w
	}
	if w := call("run-news"); w.Code != http.StatusForbidden {
		t.Fatalf("a bot without the app on called it: %d", w.Code)
	}
	if w := call("run-over"); w.Code != http.StatusForbidden {
		t.Fatalf("a run that ended called an app: %d", w.Code)
	}
	// The main bot has it on by default and gets through to the app, which isn't connected.
	if w := call("run-main"); w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "Settings → Apps") {
		t.Fatalf("main bot: %d %s", w.Code, w.Body)
	}
	if err := mgr.SetOn(other.ID, "podwise", true); err != nil {
		t.Fatal(err)
	}
	if w := call("run-news"); w.Code != http.StatusConflict {
		t.Fatalf("a bot with the app turned on: %d %s", w.Code, w.Body)
	}
}

func TestSignInOnlyOnThisMachine(t *testing.T) {
	for host, want := range map[string]string{
		"127.0.0.1:54321": "http://127.0.0.1:54321/apps/oauth/callback",
		"localhost:8080":  "http://localhost:8080/apps/oauth/callback",
		"[::1]:9":         "http://[::1]:9/apps/oauth/callback",
		"notes.example":   "",
		"192.168.1.5:80":  "",
	} {
		r := httptest.NewRequest(http.MethodPost, "/api/apps/podwise/sign-in", nil)
		r.Host = host
		if got := signInRedirect(r); got != want {
			t.Errorf("%s: %q, want %q", host, got, want)
		}
	}
}
