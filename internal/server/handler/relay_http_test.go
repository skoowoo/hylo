package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hardhacker/hylo/internal/config"
)

func TestRelayStatusReportsDisabled(t *testing.T) {
	h := NewRelayHTTP(&config.Config{}, nil)
	rec := httptest.NewRecorder()
	h.Status(rec, httptest.NewRequest(http.MethodGet, "/api/relay/status", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"state":"off"`) {
		t.Fatalf("status: %d %s", rec.Code, rec.Body.String())
	}
}
