package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/hardhacker/hylo/internal/storage"
)

type listRequest struct {
	// Path is the vault-relative directory path (must start with "/" if non-empty).
	// Omit or set to "/" for the vault root.
	Path string `json:"path"`
	All  bool   `json:"all"`
	Sort string `json:"sort"`
	Limit  int    `json:"limit"`
	// Start and End are RFC 3339 date strings for updated_at filtering (inclusive start, exclusive end).
	Start string `json:"start"`
	End   string `json:"end"`
	// Latest limits results to notes updated within the last N days.
	Latest int `json:"latest"`
	// Kinds, when non-empty, filters results to notes whose kind is in the set (e.g. ["knowledge"], ["index"]).
	Kinds []string `json:"kinds"`
	// ExcludeKinds excludes notes whose kind is in the set. Mutually exclusive with Kinds.
	ExcludeKinds []string `json:"exclude_kinds"`
}

// List handles POST /api/vault/list.
//
// Body fields:
//
//	path            — vault-relative directory path; omit or "/" for root (must start with "/" if set)
//	all             — true: return every note in the vault (path ignored)
//	sort            — "time" to sort by updated_at DESC
//	limit           — maximum number of notes to return
//	kinds         — if set, only notes whose kind is in the list (e.g. ["knowledge","index"])
//	exclude_kinds — if set, exclude notes whose kind is in the list; mutually exclusive with kinds
func (gh *VaultHandler) List(w http.ResponseWriter, r *http.Request) {
	var req listRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if len(req.Kinds) > 0 && len(req.ExcludeKinds) > 0 {
		http.Error(w, `cannot set both "kinds" and "exclude_kinds"`, http.StatusBadRequest)
		return
	}
	onlyKinds := make([]storage.Kind, len(req.Kinds))
	for i, k := range req.Kinds {
		onlyKinds[i] = storage.Kind(k)
	}
	excludeKinds := make([]storage.Kind, len(req.ExcludeKinds))
	for i, k := range req.ExcludeKinds {
		excludeKinds[i] = storage.Kind(k)
	}
	opts := storage.ListOptions{
		SortByTime:   req.Sort == "time",
		Limit:        req.Limit,
		OnlyKinds:    onlyKinds,
		ExcludeKinds: excludeKinds,
	}
	if req.Latest > 0 {
		opts.After = time.Now().AddDate(0, 0, -req.Latest)
	}
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

	if req.All {
		gh.listAll(w, opts)
		return
	}

	// Resolve and validate path.
	dirStr := req.Path
	if dirStr == "" {
		dirStr = "/"
	}
	p, ok := storage.ParsePath(dirStr)
	if !ok {
		http.Error(w, `path must be absolute (start with "/")`, http.StatusBadRequest)
		return
	}

	gh.listDir(w, p, opts)
}

func (gh *VaultHandler) listAll(w http.ResponseWriter, opts storage.ListOptions) {
	notes, err := gh.vault.ListAllNotes(opts)
	if err != nil {
		writeVaultError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"path":  "/",
		"notes": notes,
		"all":   true,
	})
}

func (gh *VaultHandler) listDir(w http.ResponseWriter, p storage.Path, opts storage.ListOptions) {
	dirMeta, err := gh.vault.StatDir(p)
	if err != nil {
		writeVaultError(w, err)
		return
	}
	notes, err := gh.vault.ListDir(p, opts)
	if err != nil {
		writeVaultError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"path":  dirMeta.Path,
		"notes": notes,
	})
}
