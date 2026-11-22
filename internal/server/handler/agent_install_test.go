package handler

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/hardhacker/hylo/internal/agent"
)

func TestAgentInstallGuardsExternalSetup(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("install/update is macOS-only")
	}
	dir := t.TempDir()
	fakeBin := filepath.Join(dir, "codex")
	if err := os.WriteFile(fakeBin, []byte("#!/bin/sh\necho 0.1.0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("OD_AGENT_HOME", t.TempDir())

	api := NewAgentAPI(slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil, agent.NewAgentCache(0, nil))
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/agents/{id}/install", api.AgentInstallPOST)
	mux.HandleFunc("POST /api/agents/{id}/update", api.AgentUpdatePOST)
	mux.HandleFunc("POST /api/agents/{id}/cancel", api.AgentInstallCancelPOST)

	// Already installed -> install must refuse (409) and never overwrite.
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/agents/codex/install", nil))
	if rec.Code != http.StatusConflict {
		t.Fatalf("install on existing bin: status = %d, body = %s", rec.Code, rec.Body.String())
	}

	// Installed externally (not in ~/.local/bin) -> update must refuse (409) with hint.
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/agents/codex/update", nil))
	if rec.Code != http.StatusConflict {
		t.Fatalf("update on external bin: status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if !strings.Contains(body["error"], "Installed externally") {
		t.Fatalf("unexpected error body = %+v", body)
	}

	// Unknown agent -> 404.
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/agents/unknown/install", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown agent: status = %d", rec.Code)
	}

	// Cancel idle -> 200 {"canceled": false}.
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/agents/codex/cancel", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("cancel idle: status = %d", rec.Code)
	}
}
