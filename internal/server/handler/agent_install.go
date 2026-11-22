package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"

	"github.com/hardhacker/hylo/internal/agent"
	"github.com/hardhacker/hylo/internal/agent/installer"
)

// AgentInstallPOST handles POST /api/agents/{id}/install.
func (a *AgentAPI) AgentInstallPOST(w http.ResponseWriter, r *http.Request) {
	a.runAgentInstallAction(w, r, "install")
}

// AgentUpdatePOST handles POST /api/agents/{id}/update.
func (a *AgentAPI) AgentUpdatePOST(w http.ResponseWriter, r *http.Request) {
	a.runAgentInstallAction(w, r, "update")
}

// AgentInstallCancelPOST handles POST /api/agents/{id}/cancel.
func (a *AgentAPI) AgentInstallCancelPOST(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing agent id", http.StatusBadRequest)
		return
	}
	canceled := a.installer.Cancel(id)
	respondJSON(w, http.StatusOK, map[string]any{"canceled": canceled})
}

func (a *AgentAPI) runAgentInstallAction(w http.ResponseWriter, r *http.Request, action string) {
	id := r.PathValue("id")
	def := agent.GetAgentDef(id)
	inst := installer.Get(id)
	if def == nil || inst == nil {
		http.Error(w, "unknown agent", http.StatusNotFound)
		return
	}
	if !inst.Supported() {
		respondJSON(w, http.StatusBadRequest, map[string]any{
			"error": "install/update is only supported on macOS",
		})
		return
	}

	resolved := agent.ResolveAgentExecutable(def, nil)
	if action == "install" && resolved != "" {
		respondJSON(w, http.StatusConflict, map[string]any{
			"error": def.Name + " is already installed at " + resolved,
		})
		return
	}
	if action == "update" {
		if resolved == "" {
			respondJSON(w, http.StatusConflict, map[string]any{
				"error": def.Name + " is not installed",
			})
			return
		}
		home, _ := os.UserHomeDir()
		if ov := os.Getenv("OD_AGENT_HOME"); ov != "" {
			home = ov
		}
		src := installer.DetectInstallSource(inst, resolved, home)
		if src != installer.SourceNative {
			respondJSON(w, http.StatusConflict, map[string]any{
				"error": inst.ExternalUpdateHint(src, resolved),
			})
			return
		}
	}

	parent := a.life
	if parent == nil {
		parent = context.Background()
	}
	a.logger.Info("agent cli "+action, slog.String("agentId", id))

	out, err := a.installer.Run(parent, inst, action, resolved, agent.ShellEnv())
	if err != nil {
		if errors.Is(err, installer.ErrBusy) {
			respondJSON(w, http.StatusConflict, map[string]any{"error": "already running"})
			return
		}
		if errors.Is(err, context.Canceled) {
			respondJSON(w, http.StatusOK, map[string]any{"ok": false, "canceled": true})
			return
		}
		msg := err.Error()
		if errors.Is(err, context.DeadlineExceeded) {
			msg = "Timed out"
		}
		respondJSON(w, http.StatusInternalServerError, map[string]any{
			"ok":     false,
			"error":  msg,
			"output": out,
		})
		return
	}

	payload := map[string]any{"ok": true}
	if a.agentCache != nil {
		if refreshed, ok := a.agentCache.RefreshOne(context.Background(), id); ok {
			payload["agent"] = refreshed
		}
	}
	respondJSON(w, http.StatusOK, payload)
}
