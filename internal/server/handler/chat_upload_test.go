package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/hardhacker/hylo/internal/agent"
	"github.com/hardhacker/hylo/internal/config"
	"github.com/hardhacker/hylo/internal/mate"
	"github.com/hardhacker/hylo/internal/storage"
)

func testAgentAPI(t *testing.T) *AgentAPI {
	t.Helper()
	v, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = v.Close() })
	return &AgentAPI{logger: slog.Default(), cfg: &config.Config{}, vault: v}
}

func tinyPNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestChatUploadAndPrepare(t *testing.T) {
	a := testAgentAPI(t)
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	part, err := w.CreateFormFile("file", "shot.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(tinyPNG(t)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/chat/uploads", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	a.ChatUploadPOST(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("upload status %d: %s", rec.Code, rec.Body.String())
	}
	var att mate.Attachment
	if err := json.Unmarshal(rec.Body.Bytes(), &att); err != nil {
		t.Fatal(err)
	}
	if att.MIME != "image/png" || att.Name != "shot.png" || !chatUploadName.MatchString(att.ID) {
		t.Fatalf("attachment %+v", att)
	}

	got, paths, err := a.prepareChatImages([]mate.Attachment{att}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || len(paths) != 1 || filepath.Base(paths[0]) != att.ID {
		t.Fatalf("prepared %+v %v", got, paths)
	}

	get := httptest.NewRequest(http.MethodGet, "/api/chat/uploads/"+att.ID, nil)
	get.SetPathValue("id", att.ID)
	grec := httptest.NewRecorder()
	a.ChatUploadGET(grec, get)
	if grec.Code != http.StatusOK || grec.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("get %d %s", grec.Code, grec.Header().Get("Content-Type"))
	}

	bad := httptest.NewRequest(http.MethodGet, "/api/chat/uploads/../x", nil)
	bad.SetPathValue("id", "../x")
	brec := httptest.NewRecorder()
	a.ChatUploadGET(brec, bad)
	if brec.Code != http.StatusNotFound {
		t.Fatalf("traversal status %d", brec.Code)
	}
}

func TestChatUploadRejectsNonImage(t *testing.T) {
	a := testAgentAPI(t)
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	part, err := w.CreateFormFile("file", "notes.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/chat/uploads", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	a.ChatUploadPOST(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(a.uploadRoot()); err == nil {
		entries, _ := os.ReadDir(a.uploadRoot())
		if len(entries) != 0 {
			t.Fatalf("rejected upload left files: %v", entries)
		}
	}
}

func TestPrepareChatImagesCapsAndDropsOutsidePaths(t *testing.T) {
	a := testAgentAPI(t)
	outside := filepath.Join(t.TempDir(), "x.png")
	if err := os.WriteFile(outside, tinyPNG(t), 0o644); err != nil {
		t.Fatal(err)
	}
	got, paths, err := a.prepareChatImages(nil, []string{outside})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 || len(paths) != 0 {
		t.Fatalf("outside path leaked: %+v %v", got, paths)
	}

	var atts []mate.Attachment
	for i := 0; i < maxChatImages+1; i++ {
		body := &bytes.Buffer{}
		w := multipart.NewWriter(body)
		part, err := w.CreateFormFile("file", "a.png")
		if err != nil {
			t.Fatal(err)
		}
		_, _ = part.Write(tinyPNG(t))
		_ = w.Close()
		req := httptest.NewRequest(http.MethodPost, "/api/chat/uploads", body)
		req.Header.Set("Content-Type", w.FormDataContentType())
		rec := httptest.NewRecorder()
		a.ChatUploadPOST(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("upload %d: %s", rec.Code, rec.Body.String())
		}
		var att mate.Attachment
		if err := json.Unmarshal(rec.Body.Bytes(), &att); err != nil {
			t.Fatal(err)
		}
		atts = append(atts, att)
	}
	if _, _, err := a.prepareChatImages(atts, nil); err == nil {
		t.Fatal("expected a cap error")
	}
}

func TestRetryWithMissingImageKeepsTheFailedTurn(t *testing.T) {
	a := testAgentAPI(t)
	store, err := mate.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	a.store = store
	a.hub = agent.NewHub()
	conv, err := store.CreateConversation("bot", "Test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.InsertMessage(mate.Message{ConversationID: conv.ID, Role: "user", Content: "look"}); err != nil {
		t.Fatal(err)
	}
	failed, err := store.InsertMessage(mate.Message{ConversationID: conv.ID, Role: "assistant", Status: "failed"})
	if err != nil {
		t.Fatal(err)
	}
	gone := []mate.Attachment{{ID: "11111111-1111-1111-1111-111111111111.png", Name: "shot.png"}}
	run := a.hub.CreateRun(map[string]string{"conversationId": conv.ID})
	a.executeChat(context.Background(), run, chatBody{
		ConversationID: conv.ID, Message: "look", RetryOf: failed.ID, Attachments: gone,
	})
	msgs, err := store.ListMessages(conv.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 {
		t.Fatalf("retry with a missing image dropped the turn: %+v", msgs)
	}
}
