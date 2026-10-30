package handler

import (
	"encoding/json"
	"net/http"

	"github.com/hardhacker/hylo/internal/util"
)

type tagRewriteRequest struct {
	Tag string `json:"tag"`
	To  string `json:"to"`
}

type tagRewriteResponse struct {
	Tag     string   `json:"tag"`
	To      string   `json:"to,omitempty"`
	Updated int      `json:"updated"`
	Paths   []string `json:"paths"`
}

// TagDelete handles POST /api/tag/delete {"tag"} — removes the tag from every
// note's front matter and turns its inline #tags into plain words.
func (h *VaultHandler) TagDelete(w http.ResponseWriter, r *http.Request) {
	h.rewriteTag(w, r, false)
}

// TagRename handles POST /api/tag/rename {"tag","to"} — renames the tag in
// every note, merging it into `to` where a note already has both.
func (h *VaultHandler) TagRename(w http.ResponseWriter, r *http.Request) {
	h.rewriteTag(w, r, true)
}

func (h *VaultHandler) rewriteTag(w http.ResponseWriter, r *http.Request, rename bool) {
	var req tagRewriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	from := util.NormalizeTag(req.Tag)
	if from == "" {
		http.Error(w, `missing required field: "tag"`, http.StatusBadRequest)
		return
	}
	to := ""
	if rename {
		to = util.NormalizeTag(req.To)
		if !util.ValidTag(to) {
			http.Error(w, `"to" must be a tag: letters, digits, _ - /, not digits alone`, http.StatusBadRequest)
			return
		}
	}
	updated, err := h.vault.RewriteTag(from, to)
	paths := make([]string, len(updated))
	for i, p := range updated {
		paths[i] = p.String()
	}
	if err != nil {
		writeVaultError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, tagRewriteResponse{Tag: from, To: to, Updated: len(paths), Paths: paths})
}
