package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/hardhacker/hylo/internal/storage"
)

type shortRequest struct {
	ID      string `json:"id"`
	Hash    string `json:"hash"`
	Content string `json:"content"`
}

func decodeShortRequest(w http.ResponseWriter, r *http.Request, needID bool) (shortRequest, bool) {
	var req shortRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
		return req, false
	}
	if needID && req.ID == "" {
		http.Error(w, `missing required field: "id"`, http.StatusBadRequest)
		return req, false
	}
	return req, true
}

// ShortList handles POST /api/vault/shorts/list: return individual short entries.
func (gh *VaultHandler) ShortList(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Start string `json:"start"`
		End   string `json:"end"`
		Limit int    `json:"limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
		return
	}
	opts := storage.ShortListOptions{Limit: req.Limit}
	if req.Start != "" {
		t, err := time.ParseInLocation(time.DateOnly, req.Start, time.Local)
		if err != nil {
			http.Error(w, "invalid start date (use YYYY-MM-DD): "+err.Error(), http.StatusBadRequest)
			return
		}
		opts.After = t
	}
	if req.End != "" {
		t, err := time.ParseInLocation(time.DateOnly, req.End, time.Local)
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

// Short handles POST /api/vault/shorts: append a new entry to today's daily file.
func (gh *VaultHandler) Short(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeShortRequest(w, r, false)
	if !ok {
		return
	}
	entry, err := gh.vault.AppendShort([]byte(req.Content))
	if err != nil {
		writeVaultError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, entry)
}

// ShortGet handles POST /api/vault/shorts/get.
func (gh *VaultHandler) ShortGet(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeShortRequest(w, r, true)
	if !ok {
		return
	}
	entry, err := gh.vault.GetShort(req.ID)
	if err != nil {
		writeVaultError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, entry)
}

// ShortUpdate handles POST /api/vault/shorts/update.
func (gh *VaultHandler) ShortUpdate(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeShortRequest(w, r, true)
	if !ok {
		return
	}
	entry, err := gh.vault.UpdateShort(req.ID, req.Hash, []byte(req.Content))
	if err != nil {
		writeVaultError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, entry)
}

// ShortDelete handles POST /api/vault/shorts/delete.
func (gh *VaultHandler) ShortDelete(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeShortRequest(w, r, true)
	if !ok {
		return
	}
	if err := gh.vault.DeleteShort(req.ID, req.Hash); err != nil {
		writeVaultError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
