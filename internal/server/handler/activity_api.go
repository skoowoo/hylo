package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/hardhacker/hylo/internal/mate"
)

// ActivityAPI records what the user does with notes; bots' activity is logged
// by the runs themselves.
type ActivityAPI struct {
	store *mate.Store
}

func NewActivityAPI(store *mate.Store) *ActivityAPI {
	return &ActivityAPI{store: store}
}

// VisitPOST handles POST /api/activity/visit: the note in front of the user,
// reported when the visit counts and again when it ends.
func (a *ActivityAPI) VisitPOST(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path   string `json:"path"`
		Action string `json:"action"`
		At     int64  `json:"at"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if !strings.HasPrefix(body.Path, "/") || !strings.HasSuffix(strings.ToLower(body.Path), ".md") || body.At <= 0 {
		http.Error(w, "invalid visit", http.StatusBadRequest)
		return
	}
	// The client's clock may run ahead of the server's.
	at := min(body.At, time.Now().UnixMilli())
	if err := a.store.RecordUserVisit(body.Path, body.Action, time.UnixMilli(at)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
