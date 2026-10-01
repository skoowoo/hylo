package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/hardhacker/hylo/internal/mate"
)

// MateAPI handles /api/mates endpoints.
type MateAPI struct {
	logger *slog.Logger
	store  *mate.Store
}

// NewMateAPI constructs the handler.
func NewMateAPI(logger *slog.Logger, store *mate.Store) *MateAPI {
	return &MateAPI{logger: logger, store: store}
}

// MatesGET handles GET /api/mates.
func (h *MateAPI) MatesGET(w http.ResponseWriter, r *http.Request) {
	list, err := h.store.ListMates()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if list == nil {
		list = []mate.Mate{}
	}
	respondJSON(w, http.StatusOK, map[string]any{"mates": list})
}

// MatesPOST handles POST /api/mates.
func (h *MateAPI) MatesPOST(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name         string             `json:"name"`
		Description  string             `json:"description"`
		AgentID      string             `json:"agentId"`
		Model        string             `json:"model"`
		Color        string             `json:"color"`
		Cwd          string             `json:"cwd"`
		SystemPrompt string             `json:"systemPrompt"`
		Enabled      bool               `json:"enabled"`
		Triggers     []mate.MateTrigger `json:"triggers"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(body.Name) == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}
	if err := mate.ValidateTriggers(body.Triggers); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	m := &mate.Mate{
		Name:         strings.TrimSpace(body.Name),
		Description:  body.Description,
		AgentID:      body.AgentID,
		Model:        body.Model,
		Color:        body.Color,
		Cwd:          body.Cwd,
		SystemPrompt: body.SystemPrompt,
		Enabled:      body.Enabled,
	}

	if err := h.store.CreateMate(m); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if len(body.Triggers) > 0 {
		for i := range body.Triggers {
			body.Triggers[i].MateID = m.ID
		}
		if err := h.store.ReplaceTriggersForMate(m.ID, body.Triggers); err != nil {
			h.logger.Warn("mate: save triggers", slog.String("err", err.Error()))
		}
		m.Triggers = body.Triggers
	}

	respondJSON(w, http.StatusCreated, map[string]any{"mate": m})
}

// MateGET handles GET /api/mates/{id} — returns mate with triggers.
func (h *MateAPI) MateGET(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	m, err := h.store.GetMate(id)
	if err != nil || m == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	triggers, _ := h.store.ListTriggers(id)
	if triggers == nil {
		triggers = []mate.MateTrigger{}
	}
	m.Triggers = triggers
	respondJSON(w, http.StatusOK, map[string]any{"mate": m})
}

// MatePUT handles PUT /api/mates/{id} — updates mate config and replaces all triggers.
func (h *MateAPI) MatePUT(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	existing, err := h.store.GetMate(id)
	if err != nil || existing == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	var body struct {
		Name         string             `json:"name"`
		Description  string             `json:"description"`
		AgentID      string             `json:"agentId"`
		Model        string             `json:"model"`
		Color        string             `json:"color"`
		Cwd          string             `json:"cwd"`
		SystemPrompt string             `json:"systemPrompt"`
		Enabled      bool               `json:"enabled"`
		Triggers     []mate.MateTrigger `json:"triggers"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(body.Name) == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}

	triggers := body.Triggers
	if triggers == nil {
		triggers = []mate.MateTrigger{}
	}
	if err := mate.ValidateTriggers(triggers); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	existing.Name = strings.TrimSpace(body.Name)
	existing.Description = body.Description
	existing.AgentID = body.AgentID
	existing.Model = body.Model
	existing.Color = body.Color
	existing.Cwd = body.Cwd
	existing.SystemPrompt = body.SystemPrompt
	existing.Enabled = body.Enabled

	if err := h.store.UpdateMate(existing); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := h.store.ReplaceTriggersForMate(id, triggers); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	existing.Triggers = triggers

	respondJSON(w, http.StatusOK, map[string]any{"mate": existing})
}

// MateEventsGET handles GET /api/mate-events — returns the built-in event definitions.
func (h *MateAPI) MateEventsGET(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]any{"events": mate.BuiltinEvents})
}

// MatesReorderPOST handles POST /api/mates/reorder — sets the display order of mates.
func (h *MateAPI) MatesReorderPOST(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.IDs) == 0 {
		http.Error(w, "ids required", http.StatusBadRequest)
		return
	}
	if err := h.store.ReorderMates(body.IDs); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// MateDELETE handles DELETE /api/mates/{id}.
func (h *MateAPI) MateDELETE(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.store.DeleteMate(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"ok": true})
}
