package plugin

import (
	"sync"
	"time"
)

// Window is how long a coalescer holds a note's writes: until edits pause for
// Quiet, and never longer than MaxWait after the first held write.
// HoldCreates holds a create the same way, folding the writes that follow it
// in, so a new note is seen once with its first settled content.
type Window struct {
	Quiet       time.Duration
	MaxWait     time.Duration
	HoldCreates bool
}

// The editor autosaves after every pause in typing, so each consumer picks how
// much of an edit session it needs to see. Indexers catch up shortly after a
// pause; agent triggers wait for the session to end.
var (
	IndexWindow = Window{Quiet: 10 * time.Second, MaxWait: time.Minute}
	AgentWindow = Window{Quiet: time.Minute, MaxWait: 10 * time.Minute, HoldCreates: true}
)

// Coalescer collapses a burst of write events on one note into its last one,
// delivered once the window closes, so the final content always arrives.
// Other events pass through at once: a create first flushes a held write on
// its path, a delete discards it — and with a held create, is dropped too,
// since the note never surfaced. Deliver runs on the caller's goroutine for
// pass-through events and on a timer goroutine for coalesced ones.
type Coalescer struct {
	win     Window
	deliver func(Event)

	mu      sync.Mutex
	pending map[string]*held
}

type held struct {
	e     Event
	first time.Time
	timer *time.Timer
}

func NewCoalescer(win Window, deliver func(Event)) *Coalescer {
	return &Coalescer{win: win, deliver: deliver, pending: map[string]*held{}}
}

func (c *Coalescer) Add(e Event) {
	switch e.Type {
	case EventVaultWrite, EventFSWrite:
		c.hold(e)
		return
	case EventVaultDelete, EventFSDelete:
		c.mu.Lock()
		h := c.pending[e.Path]
		if h != nil {
			h.timer.Stop()
			delete(c.pending, e.Path)
		}
		c.mu.Unlock()
		if h != nil && isCreate(h.e) {
			return
		}
	case EventVaultCreate, EventFSCreate:
		c.flush(e.Path)
		if c.win.HoldCreates {
			c.hold(e)
			return
		}
	}
	c.deliver(e)
}

func isCreate(e Event) bool {
	return e.Type == EventVaultCreate || e.Type == EventFSCreate
}

func (c *Coalescer) hold(e Event) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if h := c.pending[e.Path]; h != nil {
		if isCreate(h.e) {
			e.Type, e.Moved = h.e.Type, h.e.Moved
		}
		h.e = e
		h.timer.Reset(min(c.win.Quiet, max(0, c.win.MaxWait-time.Since(h.first))))
		return
	}
	h := &held{e: e, first: time.Now()}
	h.timer = time.AfterFunc(c.win.Quiet, func() { c.fire(e.Path, h) })
	c.pending[e.Path] = h
}

func (c *Coalescer) fire(path string, h *held) {
	c.mu.Lock()
	if c.pending[path] != h {
		c.mu.Unlock()
		return
	}
	delete(c.pending, path)
	e := h.e
	c.mu.Unlock()
	c.deliver(e)
}

func (c *Coalescer) flush(path string) {
	c.mu.Lock()
	h := c.pending[path]
	if h != nil {
		h.timer.Stop()
		delete(c.pending, path)
	}
	c.mu.Unlock()
	if h != nil {
		c.deliver(h.e)
	}
}

// Stop discards held writes; startup backfills cover what they would have done.
func (c *Coalescer) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for path, h := range c.pending {
		h.timer.Stop()
		delete(c.pending, path)
	}
}
