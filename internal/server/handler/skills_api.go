package handler

import (
	"encoding/json"
	"net/http"
	"strings"

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

// Enable handles POST /api/skills/{name}/enable.
func (h *SkillsHTTP) Enable(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !validSkillName(name) {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid skill name"})
		return
	}
	if err := h.mgr.Enable(name); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"ok": true, "name": name, "enabled": true})
}

// Disable handles POST /api/skills/{name}/disable.
func (h *SkillsHTTP) Disable(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !validSkillName(name) {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid skill name"})
		return
	}
	if err := h.mgr.Disable(name); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"ok": true, "name": name, "enabled": false})
}

// Remove handles DELETE /api/skills/{name}.
func (h *SkillsHTTP) Remove(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !validSkillName(name) {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid skill name"})
		return
	}
	if err := h.mgr.Remove(name); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"ok": true, "name": name})
}

// Install handles POST /api/skills/install.
func (h *SkillsHTTP) Install(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RepoURL   string `json:"repoUrl"`
		SubPath   string `json:"subPath"`
		SkillName string `json:"skill"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}
	if body.RepoURL == "" {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": "repoUrl is required"})
		return
	}
	if body.SkillName == "" {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": "skill name is required (--skill)"})
		return
	}
	if !validSkillName(body.SkillName) {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid skill name"})
		return
	}
	if err := h.mgr.Install(body.RepoURL, body.SubPath, body.SkillName); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"ok": true, "name": body.SkillName})
}

func validSkillName(name string) bool {
	return name != "" &&
		!strings.Contains(name, "/") &&
		!strings.Contains(name, "\\") &&
		!strings.Contains(name, "..") &&
		!strings.HasPrefix(name, ".")
}
