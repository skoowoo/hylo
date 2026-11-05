package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/hardhacker/hylo/internal/storage"
)

// Sample handles POST /api/vault/sample.
//
//	each        — how many to draw from each of raw notes, knowledge units, and short entries (default 2, max 8)
//	older_than  — days back the draw must reach; 0 means 90, negative keeps the whole history
func (gh *VaultHandler) Sample(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Each      int `json:"each"`
		OlderThan int `json:"older_than"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
		return
	}
	items, err := gh.vault.Sample(storage.SampleOptions{Each: req.Each, OlderThan: req.OlderThan})
	if err != nil {
		writeVaultError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"items": items})
}
