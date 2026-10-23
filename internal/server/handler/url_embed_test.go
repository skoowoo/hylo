package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hardhacker/hylo/internal/storage"
)

func TestEmbedsCachedAndRejected(t *testing.T) {
	v, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = v.Close() })
	now := time.Now().UnixMilli()
	if err := v.PutURLEmbed(storage.URLEmbed{
		URL: "https://example.com/a", FinalURL: "https://cdn.example.com/a",
		Kind: "page", Title: "Hello", Status: "ok",
		FetchedAt: now, ExpiresAt: now + int64(time.Hour/time.Millisecond),
	}); err != nil {
		t.Fatal(err)
	}

	h, _ := NewEmbedHandlers(v, nil)
	body := []byte(`{"urls":["https://Example.com/a#frag"]}`)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/embeds", bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	type embed struct {
		URL      string `json:"url"`
		FinalURL string `json:"final_url"`
		Title    string `json:"title"`
		Kind     string `json:"kind"`
	}
	var embeds []embed
	dec := json.NewDecoder(rec.Body)
	for dec.More() {
		var e embed
		if err := dec.Decode(&e); err != nil {
			t.Fatal(err)
		}
		embeds = append(embeds, e)
	}
	if len(embeds) != 1 || embeds[0].Title != "Hello" || embeds[0].Kind != "page" || embeds[0].URL != "https://Example.com/a#frag" {
		t.Fatalf("embeds = %+v", embeds)
	}
	if embeds[0].FinalURL != "https://cdn.example.com/a" {
		t.Fatalf("final = %q", embeds[0].FinalURL)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/embeds", strings.NewReader("{")))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad json status = %d", rec.Code)
	}

	var many []string
	for i := 0; i < 21; i++ {
		many = append(many, "https://example.com/"+string(rune('a'+i%26)))
	}
	payload, _ := json.Marshal(map[string]any{"urls": many})
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/embeds", bytes.NewReader(payload)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("too many status = %d", rec.Code)
	}
}

func TestEmbedProgressSurvivesRefresh(t *testing.T) {
	v, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = v.Close() })
	now := time.Now().UnixMilli()
	row := storage.URLEmbed{
		URL: "https://cdn.example.com/ep.mp3", FinalURL: "https://cdn.example.com/ep.mp3",
		Kind: "audio", Title: "Episode", Status: "ok",
		FetchedAt: now, ExpiresAt: now + int64(time.Hour/time.Millisecond),
	}
	if err := v.PutURLEmbed(row); err != nil {
		t.Fatal(err)
	}

	list, progress := NewEmbedHandlers(v, nil)
	rec := httptest.NewRecorder()
	body := []byte(`{"url":"https://CDN.example.com/ep.mp3","position_ms":12500}`)
	progress.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/embeds/progress", bytes.NewReader(body)))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("progress status = %d, body = %s", rec.Code, rec.Body.String())
	}

	row.Title = "Renamed"
	if err := v.PutURLEmbed(row); err != nil {
		t.Fatal(err)
	}
	got, err := v.FreshURLEmbeds([]string{row.URL}, now)
	if err != nil {
		t.Fatal(err)
	}
	if got[row.URL].PositionMS != 12500 || got[row.URL].Title != "Renamed" {
		t.Fatalf("row = %+v", got[row.URL])
	}
	rec = httptest.NewRecorder()
	list.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/embeds", strings.NewReader(`{"urls":["https://cdn.example.com/ep.mp3"]}`)))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"position_ms":12500`) {
		t.Fatalf("resolve = %d %s", rec.Code, rec.Body.String())
	}

	page := row
	page.URL = "https://example.com/story"
	page.Kind = "page"
	page.Title = "Story"
	if err := v.PutURLEmbed(page); err != nil {
		t.Fatal(err)
	}
	rec = httptest.NewRecorder()
	progress.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/embeds/progress", strings.NewReader(`{"url":"https://example.com/story","position_ms":4000}`)))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("page progress status = %d", rec.Code)
	}
	got, err = v.FreshURLEmbeds([]string{page.URL}, now)
	if err != nil {
		t.Fatal(err)
	}
	if got[page.URL].PositionMS != 0 {
		t.Fatalf("page position = %d", got[page.URL].PositionMS)
	}

	rec = httptest.NewRecorder()
	progress.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/embeds/progress", strings.NewReader(`{"url":"https://cdn.example.com/ep.mp3","position_ms":-1}`)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("negative status = %d", rec.Code)
	}
}
