package plugin

import "context"

// ReplyResult is delivered to an event producer when a matched mate trigger run finishes.
type ReplyResult struct {
	Text   string
	Status string // succeeded | failed | canceled | unhandled (no trigger took the event)
}

// ReplyFunc is an optional one-shot callback registered on an inbound plugin event.
// Mate passes it through to the agent run and invokes it after the run reaches a
// terminal state (e.g. a chat bridge sends the assistant text back to the user),
// or at once when no trigger runs for the event.
type ReplyFunc func(ctx context.Context, result ReplyResult) error
