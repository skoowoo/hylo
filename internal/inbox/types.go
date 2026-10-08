// Package inbox stores the notifications surfaced to the user when a
// background agent bot run finishes.
package inbox

import "time"

// Message is one entry in the inbox.
type Message struct {
	ID        string    `json:"id"`
	Source    string    `json:"source"` // name of the agent bot that produced it
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	IsRead    bool      `json:"isRead"`
	CreatedAt time.Time `json:"createdAt"`
}

// ListFilter narrows List results.
type ListFilter struct {
	UnreadOnly bool
	Limit      int // 0 = default page size
	Offset     int
}
