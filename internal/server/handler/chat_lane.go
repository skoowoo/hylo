package handler

import (
	"context"
	"sync"
	"time"

	"github.com/hardhacker/hylo/internal/mate"
)

// wakeQuiet is how long the user must have been quiet in a bot's chat before a
// wake may start there, so a wake never lands between their turns.
var wakeQuiet = 7 * time.Minute

// wakeGap spaces wakes out, so a backlog never runs back to back.
var wakeGap = time.Minute

// maxWakeWaits caps the wakes one bot has waiting: a bulk import fires one per note.
const maxWakeWaits = 5

// chatLane is one bot's chat: one turn at a time, the user's before any wake.
type chatLane struct {
	mu       sync.Mutex
	busy     bool
	users    []*ticket
	wakes    []*ticket
	keys     map[string]bool // keys of the waiting wakes
	lastUser time.Time       // when the user's last turn ended
	lastWake time.Time       // when the last wake ended
	timer    *time.Timer     // set while waiting wakes sit out the quiet or the gap
}

// ticket is a turn's place in a lane; ch closes when the chat is its.
type ticket struct {
	ch   chan struct{}
	wake bool
	key  string
}

// join queues a turn. A wake whose key is already waiting gets no ticket and no
// error; one past maxWakeWaits gets mate.ErrBotBusy.
func (l *chatLane) join(wake bool, key string) (*ticket, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	t := &ticket{ch: make(chan struct{}), wake: wake, key: key}
	if !wake {
		l.users = append(l.users, t)
	} else {
		if key != "" && l.keys[key] {
			return nil, nil
		}
		if len(l.wakes) >= maxWakeWaits {
			return nil, mate.ErrBotBusy
		}
		if key != "" {
			if l.keys == nil {
				l.keys = map[string]bool{}
			}
			l.keys[key] = true
		}
		l.wakes = append(l.wakes, t)
	}
	l.next()
	return t, nil
}

// wait blocks until t has the chat, or ctx ends and t gives up its place.
func (l *chatLane) wait(ctx context.Context, t *ticket) bool {
	select {
	case <-t.ch:
		return true
	case <-ctx.Done():
		l.abandon(t)
		return false
	}
}

// abandon takes t out of the lane, passing the chat on if it was already t's.
func (l *chatLane) abandon(t *ticket) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.drop(t) {
		l.leaveLocked(t)
	}
}

// leave ends t's turn.
func (l *chatLane) leave(t *ticket) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.leaveLocked(t)
}

func (l *chatLane) leaveLocked(t *ticket) {
	l.busy = false
	if t.wake {
		l.lastWake = time.Now()
	} else {
		l.lastUser = time.Now()
	}
	l.next()
}

func (l *chatLane) drop(t *ticket) bool {
	q := &l.users
	if t.wake {
		q = &l.wakes
	}
	for i, w := range *q {
		if w == t {
			*q = append((*q)[:i], (*q)[i+1:]...)
			delete(l.keys, t.key)
			return true
		}
	}
	return false
}

// next hands a free chat on: to the user, else to a wake once the user has
// been quiet for wakeQuiet and the last wake ended wakeGap ago. Caller holds mu.
func (l *chatLane) next() {
	if l.busy {
		return
	}
	if len(l.users) > 0 {
		l.hand(&l.users)
		return
	}
	if len(l.wakes) == 0 {
		return
	}
	if wait := max(wakeQuiet-time.Since(l.lastUser), wakeGap-time.Since(l.lastWake)); wait > 0 {
		// A turn ending since this was set only pushes the time out; next re-checks.
		if l.timer == nil {
			l.timer = time.AfterFunc(wait, func() {
				l.mu.Lock()
				defer l.mu.Unlock()
				l.timer = nil
				l.next()
			})
		}
		return
	}
	l.hand(&l.wakes)
}

func (l *chatLane) hand(q *[]*ticket) {
	t := (*q)[0]
	*q = (*q)[1:]
	delete(l.keys, t.key)
	l.busy = true
	close(t.ch)
}
