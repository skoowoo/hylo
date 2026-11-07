package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/hardhacker/hylo/internal/desktop"
	"github.com/hardhacker/hylo/internal/mate"
)

func TestDesktopPresenceToScreenTime(t *testing.T) {
	s, err := mate.Open(filepath.Join(t.TempDir(), "vault"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	d := NewDesktopAPI(desktop.New(s, true))
	post := func(body string) int {
		w := httptest.NewRecorder()
		d.PresencePOST(w, httptest.NewRequest(http.MethodPost, "/api/desktop/presence", bytes.NewBufferString(body)))
		return w.Code
	}
	if c := post(`{"app":"Xcode","idle":3}`); c != http.StatusNoContent {
		t.Fatalf("presence: %d", c)
	}
	w := httptest.NewRecorder()
	d.ScreenTimeGET(w, httptest.NewRequest(http.MethodGet, "/api/desktop/screen-time", nil))
	var st desktop.ScreenTime
	_ = json.Unmarshal(w.Body.Bytes(), &st)
	if !st.Enabled || !st.Now.Present || st.Now.App != "Xcode" || len(st.Timeline) != 1 || st.Apps[0].App != "Xcode" {
		t.Fatalf("screen time = %s", w.Body)
	}

	w = httptest.NewRecorder()
	d.DesktopDELETE(w, httptest.NewRequest(http.MethodDelete, "/api/desktop", nil))
	if ss, _ := s.DesktopSessions(st.Timeline[0].Start.Add(-1), st.Timeline[0].End.Add(1)); w.Code != http.StatusNoContent || len(ss) != 0 {
		t.Fatalf("clear: %d %v", w.Code, ss)
	}
	w = httptest.NewRecorder()
	d.ScreenTimeGET(w, httptest.NewRequest(http.MethodGet, "/api/desktop/screen-time", nil))
	st = desktop.ScreenTime{}
	_ = json.Unmarshal(w.Body.Bytes(), &st)
	if st.Now.Known || !st.Now.AwaySince.IsZero() {
		t.Fatalf("still knew the user after a clear: %s", w.Body)
	}

	off := NewDesktopAPI(desktop.New(s, false))
	w = httptest.NewRecorder()
	off.PresencePOST(w, httptest.NewRequest(http.MethodPost, "/api/desktop/presence", bytes.NewBufferString(`{"app":"Xcode"}`)))
	if n, _ := s.LastDesktop(); w.Code != http.StatusGone || !n.IsZero() {
		t.Fatalf("kept a report while off: %d %v", w.Code, n)
	}
}
