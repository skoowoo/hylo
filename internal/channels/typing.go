package channels

import (
	"context"
	"time"
)

// keepTyping calls show immediately and again every interval until ctx ends.
// Callers use it to refresh an app's typing indicator for the whole agent run.
func keepTyping(ctx context.Context, every time.Duration, show func()) {
	show()
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			show()
		}
	}
}
