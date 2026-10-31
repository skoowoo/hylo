package handler

import (
	"errors"
	"net/http"

	"github.com/hardhacker/hylo/internal/mate"
)

// TriggerRunPOST handles POST /api/mates/{id}/triggers/{tid}/run: fires a
// scheduled trigger now, so a new one can show what it does right away. Like
// any wake, it waits until the user is done in the bot's chat.
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
			if errors.Is(err, mate.ErrTriggerDisabled) || errors.Is(err, mate.ErrBotBusy) {
				status = http.StatusConflict
			}
			http.Error(w, err.Error(), status)
			return
		}
		respondJSON(w, http.StatusAccepted, map[string]any{"ok": true})
		return
	}
	http.Error(w, "trigger not found", http.StatusNotFound)
}
