package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/hardhacker/hylo/internal/skills"
)

// SkillsHTTP handles /api/skills endpoints.
type SkillsHTTP struct {
	mgr *skills.Manager
}

// NewSkillsHTTP constructs the handler.
func NewSkillsHTTP(mgr *skills.Manager) *SkillsHTTP {
	return &SkillsHTTP{mgr: mgr}
}

// List handles GET /api/skills.
func (h *SkillsHTTP) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.mgr.List()
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"skills": list})
}

// Catalog handles GET /api/skills/catalog.
func (h *SkillsHTTP) Catalog(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]any{"skills": h.mgr.Catalog()})
}

// skillDir reads the optional {"dir": "/_news"} body of enable and disable; none means the vault root.
func skillDir(r *http.Request) (string, error) {
	var body struct {
		Dir string `json:"dir"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return body.Dir, nil
}

// Enable handles POST /api/skills/{name}/enable.
func (h *SkillsHTTP) Enable(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !skills.ValidName(name) {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid skill name"})
		return
	}
	dir, err := skillDir(r)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}
	if err := h.mgr.Enable(name, dir); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"ok": true, "name": name, "enabled": true})
}

// Disable handles POST /api/skills/{name}/disable.
func (h *SkillsHTTP) Disable(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !skills.ValidName(name) {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid skill name"})
		return
	}
	dir, err := skillDir(r)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}
	if err := h.mgr.Disable(name, dir); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"ok": true, "name": name, "enabled": false})
}

// Remove handles DELETE /api/skills/{name}.
func (h *SkillsHTTP) Remove(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !skills.ValidName(name) {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid skill name"})
		return
	}
	if err := h.mgr.Remove(name); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"ok": true, "name": name})
}

// Add handles POST /api/skills/add.
func (h *SkillsHTTP) Add(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Repo  string `json:"repo"`
		Skill string `json:"skill"`
		Force bool   `json:"force"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}
	if body.Repo == "" {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": "repo is required"})
		return
	}
	name, err := h.mgr.Add(body.Repo, body.Skill, body.Force)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"ok": true, "name": name})
}
