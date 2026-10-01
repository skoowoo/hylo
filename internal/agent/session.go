package agent

import "github.com/google/uuid"

// Agent event type emitted when an adapter reports its native session id.
const AgentEventSession = "session"

// HostAssignsSessionID reports agents where the host should pre-assign a session
// id on the first turn (--session-id) rather than waiting for capture.
func HostAssignsSessionID(agentID string) bool {
	switch agentID {
	case "claude", "pi":
		return true
	default:
		return false
	}
}

// NewSessionID returns a new agent session identifier (UUID).
func NewSessionID() string {
	return uuid.NewString()
}

func sessionAgentEvent(sessionID string) map[string]any {
	return map[string]any{"type": AgentEventSession, "sessionId": sessionID}
}
