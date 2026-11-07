package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/hardhacker/hylo/internal/mate"
)

func TestAgendaBelongsToTheMainBot(t *testing.T) {
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

	post := func(run, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/api/agenda", bytes.NewBufferString(body))
		r.Header.Set(RunHeader, run)
		w := httptest.NewRecorder()
		a.AgendaPOST(w, r)
		return w
	}
	if w := post("run-news", `{"text":"x"}`); w.Code != http.StatusForbidden {
		t.Fatalf("another bot added to an agenda: %d", w.Code)
	}
	if w := post("run-over", `{"text":"x"}`); w.Code != http.StatusForbidden {
		t.Fatalf("a run that ended stood in for the main bot: %d", w.Code)
	}
	if w := post("run-main", `{"text":"remind about the dentist","due":"2026-11-07 09:00"}`); w.Code != http.StatusOK {
		t.Fatalf("add: %d %s", w.Code, w.Body)
	}
	if w := post("", `{"text":"from outside a run"}`); w.Code != http.StatusOK {
		t.Fatalf("add without a run: %d %s", w.Code, w.Body)
	}
	if w := post("run-main", `{"text":"x","due":"tomorrow"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("bad due: %d", w.Code)
	}

	items, _ := s.OpenAgenda(main.ID)
	want := time.Date(2026, 11, 7, 9, 0, 0, 0, time.Local)
	if len(items) != 2 || !items[0].Due.Equal(want) {
		t.Fatalf("items = %+v", items)
	}

	r := httptest.NewRequest(http.MethodPost, "/api/agenda/"+items[0].ID+"/done", nil)
	r.SetPathValue("id", items[0].ID)
	w := httptest.NewRecorder()
	a.AgendaDonePOST(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("done: %d %s", w.Code, w.Body)
	}
	w = httptest.NewRecorder()
	a.AgendaGET(w, httptest.NewRequest(http.MethodGet, "/api/agenda", nil))
	var out struct{ Items []mate.AgendaItem }
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if len(out.Items) != 1 || out.Items[0].Text != "from outside a run" {
		t.Fatalf("open = %+v", out.Items)
	}
}
