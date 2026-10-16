package handler

import (
	"encoding/json"
	"net/http"

	"github.com/hardhacker/hylo/internal/mate"
	"github.com/hardhacker/hylo/internal/skills"
	"github.com/hardhacker/hylo/internal/util"
)

// SkillsHTTP handles /api/skills endpoints. Skills are turned on and off per bot,
// in the bot's working directory.
type SkillsHTTP struct {
	mgr   *skills.Manager
	store *mate.Store
}

// NewSkillsHTTP constructs the handler.
func NewSkillsHTTP(mgr *skills.Manager, store *mate.Store) *SkillsHTTP {
	return &SkillsHTTP{mgr: mgr, store: store}
}

type skillBot struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type skillOut struct {
	Name        string     `json:"name"`
	Default     bool       `json:"default"`
	Repo        string     `json:"repo,omitempty"`
	Description string     `json:"description,omitempty"`
	Bots        []skillBot `json:"bots"`
}

// List handles GET /api/skills: every added skill, with the bots it is on for.
func (h *SkillsHTTP) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.mgr.List()
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	byDir := map[string][]skillBot{}
	if h.store != nil {
		bots, err := h.store.ListMates()
		if err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		for _, b := range bots {
			if dir, err := util.CleanVaultDir(b.Cwd); err == nil {
				byDir[dir] = append(byDir[dir], skillBot{ID: b.ID, Name: b.Name})
			}
		}
	}
	out := make([]skillOut, 0, len(list))
	for _, s := range list {
		o := skillOut{Name: s.Name, Default: s.Default, Repo: s.Repo, Description: s.Description, Bots: []skillBot{}}
		for _, d := range s.Dirs {
			o.Bots = append(o.Bots, byDir[d]...)
		}
		out = append(out, o)
	}
	respondJSON(w, http.StatusOK, map[string]any{"skills": out})
}

// Catalog handles GET /api/skills/catalog.
func (h *SkillsHTTP) Catalog(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]any{"skills": h.mgr.Catalog()})
}

// Enable handles POST /api/skills/{name}/enable with {"botId": "..."}.
func (h *SkillsHTTP) Enable(w http.ResponseWriter, r *http.Request) {
	h.toggle(w, r, true)
}

// Disable handles POST /api/skills/{name}/disable with {"botId": "..."}.
func (h *SkillsHTTP) Disable(w http.ResponseWriter, r *http.Request) {
	h.toggle(w, r, false)
}

func (h *SkillsHTTP) toggle(w http.ResponseWriter, r *http.Request, on bool) {
	name := r.PathValue("name")
	if !skills.ValidName(name) {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid skill name"})
		return
	}
	var body struct {
		BotID string `json:"botId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.BotID == "" {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": "botId is required"})
		return
	}
	if h.store == nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "bots are unavailable"})
		return
	}
	bot, err := h.store.GetMate(body.BotID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if bot == nil {
		respondJSON(w, http.StatusNotFound, map[string]any{"error": "no bot " + body.BotID})
		return
	}
	op := h.mgr.Disable
	if on {
		op = h.mgr.Enable
	}
	if err := op(name, bot.Cwd); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"ok": true, "name": name, "enabled": on})
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
