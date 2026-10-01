package compile

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/hardhacker/hylo/internal/plugin"
	"github.com/hardhacker/hylo/internal/storage"
)


// Handler serves HTTP endpoints for the distill plugin.
type Handler struct {
	vault  *storage.Vault
	plugin *Plugin
}

// NewHandler returns a handler backed by the given plugin and vault.
func NewHandler(vault *storage.Vault, p *Plugin) *Handler {
	return &Handler{vault: vault, plugin: p}
}

type triggerRequest struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

type triggerResponse struct {
	Status string `json:"status"`
	Path   string `json:"path"`
}

type errorResponse struct {
	Error string `json:"error"`
}

// Trigger handles POST /api/compile/trigger.
// Dispatches an EventCompileRequested to the mate agent and returns 202 Accepted immediately.
func (h *Handler) Trigger(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req triggerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
		return
	}

	vaultPath, err := resolveTriggerPath(h.vault, req.Path, req.Name)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeCompileJSON(w, http.StatusNotFound, errorResponse{Error: err.Error()})
			return
		}
		writeCompileJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	sp, ok := storage.ParsePath(vaultPath)
	if !ok {
		writeCompileJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid path"})
		return
	}
	knowledges, err := h.vault.GetSourceKnowledges(sp)
	if err != nil {
		writeCompileJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	if len(knowledges) > 0 {
		writeCompileJSON(w, http.StatusConflict, errorResponse{Error: ErrNoteAlreadyCompiled.Error()})
		return
	}

	dispatch := h.plugin.dispatchFn.Load()
	if dispatch == nil {
		writeCompileJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "compile: dispatch not wired"})
		return
	}
	dispatch(plugin.Event{
		Type: plugin.EventCompileRequested,
		Path: vaultPath,
		Time: time.Now(),
	})

	writeCompileJSON(w, http.StatusAccepted, triggerResponse{Status: "accepted", Path: vaultPath})
}

func writeCompileJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func resolveTriggerPath(vault *storage.Vault, path, name string) (string, error) {
	switch {
	case path != "" && name != "":
		return "", errors.New(`specify either "path" or "name", not both`)
	case path != "":
		p, ok := storage.ParsePath(path)
		if !ok {
			return "", errors.New(`path must be absolute (start with "/")`)
		}
		base := ensureMarkdownName(p.Base())
		return storage.JoinPath(p.Dir(), base), nil
	case name != "":
		if strings.ContainsAny(name, "/\\") {
			return "", errors.New(`"name" must be a filename only (no path separators)`)
		}
		n := ensureMarkdownName(name)
		notes, err := vault.GetNotesByName(n)
		if err != nil {
			return "", err
		}
		if len(notes) == 0 {
			return "", storage.ErrNotFound
		}
		return notes[0].Path().String(), nil
	default:
		return "", errors.New(`missing required field: "path" or "name"`)
	}
}

func ensureMarkdownName(name string) string {
	if name == "" {
		return name
	}
	if strings.HasSuffix(strings.ToLower(name), ".md") {
		return name
	}
	return name + ".md"
}
