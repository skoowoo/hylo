package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/hardhacker/hylo/internal/storage"
)

type shortRequest struct {
	Content string `json:"content"`
	// Dir overrides the default shorts directory ("_shorts"). Optional.
	Dir string `json:"dir,omitempty"`
}

// ShortList handles POST /api/vault/shorts/list: return individual short entries.
func (gh *VaultHandler) ShortList(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Dir   string `json:"dir"`
		Start string `json:"start"`
		End   string `json:"end"`
		Limit int    `json:"limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
		return
	}
	opts := storage.ShortListOptions{Dir: req.Dir, Limit: req.Limit}
	if req.Start != "" {
		t, err := time.Parse(time.DateOnly, req.Start)
		if err != nil {
			http.Error(w, "invalid start date (use YYYY-MM-DD): "+err.Error(), http.StatusBadRequest)
			return
		}
		opts.After = t
	}
	if req.End != "" {
		t, err := time.Parse(time.DateOnly, req.End)
		if err != nil {
			http.Error(w, "invalid end date (use YYYY-MM-DD): "+err.Error(), http.StatusBadRequest)
			return
		}
		opts.Before = t
	}
	entries, err := gh.vault.ListShortEntries(opts)
	if err != nil {
		writeVaultError(w, err)
		return
	}
	if entries == nil {
		entries = []storage.ShortEntry{}
	}
	respondJSON(w, http.StatusOK, map[string]any{"entries": entries})
}

// Short handles POST /api/vault/shorts: append a new short note entry to
// today's daily file inside the shorts directory.
func (gh *VaultHandler) Short(w http.ResponseWriter, r *http.Request) {
	var req shortRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Content == "" {
		http.Error(w, `missing required field: "content"`, http.StatusBadRequest)
		return
	}

	_, note, err := gh.vault.AppendShort([]byte(req.Content), req.Dir)
	if err != nil {
		writeVaultError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, note)
}
