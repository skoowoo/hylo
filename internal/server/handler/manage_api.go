package handler

import (
	"errors"
	"net/http"

	"github.com/hardhacker/hylo/internal/mate"
)

var errManageFromChat = errors.New("bots, triggers and skills can only be changed from a chat with the user")

// ManageAllowed keeps trigger runs from changing bots, triggers and skills, or a
// WeChat message could rewire a bot. An empty runID is the user's own agent.
func (a *AgentAPI) ManageAllowed(runID string) error {
	if runID == "" {
		return nil
	}
	if run := a.hub.Get(runID); run != nil && run.Interactive {
		return nil
	}
	return errManageFromChat
}

// TriggerRunPOST handles POST /api/mates/{id}/triggers/{tid}/run: fires a
// scheduled trigger now, so a new one can show what it does right away.
func (a *AgentAPI) TriggerRunPOST(w http.ResponseWriter, r *http.Request) {
	if a.runner == nil || a.store == nil {
		http.Error(w, "triggers unavailable", http.StatusServiceUnavailable)
		return
	}
	triggers, err := a.store.ListTriggers(r.PathValue("id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tid := r.PathValue("tid")
	for _, t := range triggers {
		if t.ID != tid {
			continue
		}
		// An event trigger's prompt expects the note or message that fired it.
		if !mate.TriggerHasScheduled(t) {
			http.Error(w, "only scheduled triggers can run now", http.StatusBadRequest)
			return
		}
		if err := a.runner.FireNow(t); err != nil {
			status := http.StatusInternalServerError
			switch {
			case errors.Is(err, mate.ErrTriggerDisabled):
				status = http.StatusConflict
			case errors.Is(err, mate.ErrRunnerBusy):
				status = http.StatusServiceUnavailable
			}
			http.Error(w, err.Error(), status)
			return
		}
		respondJSON(w, http.StatusAccepted, map[string]any{"ok": true})
		return
	}
	http.Error(w, "trigger not found", http.StatusNotFound)
}
