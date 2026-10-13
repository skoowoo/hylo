package handler

import (
	"testing"

	"github.com/hardhacker/hylo/internal/agent"
)

func TestManageAllowedOnlyFromTheUser(t *testing.T) {
	a := &AgentAPI{hub: agent.NewHub()}
	chat := a.hub.CreateRun(nil)
	chat.Interactive = true
	trigger := a.hub.CreateRun(nil)
	if err := a.ManageAllowed(chat.ID); err != nil {
		t.Errorf("chat run refused: %v", err)
	}
	if err := a.ManageAllowed(""); err != nil {
		t.Errorf("outside a run refused: %v", err)
	}
	if a.ManageAllowed(trigger.ID) == nil || a.ManageAllowed("gone") == nil {
		t.Error("trigger or unknown run allowed")
	}
}
