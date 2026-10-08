package plugin

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// opClass collapses paired FS / vault event types to a single deduplication key.
type opClass uint8

const (
	opCreate opClass = iota
	opWrite
	opDelete
)

// dedupTTLs is the window within which a second event for the same (path, op)
// is considered a duplicate of the first and silently dropped.
//
// opWrite's TTL must be long enough to catch the fs_write the watcher emits
// shortly after vault_write so that plugins see exactly one event per API write.
var dedupTTLs = [3]time.Duration{
	opCreate: 3 * time.Second,
	opWrite:  5 * time.Second,
	opDelete: 3 * time.Second,
}

// eventOpClass maps an EventType to its deduplication class. Returns false for
// event types that are not subject to deduplication (e.g. future extension types).
func eventOpClass(t EventType) (opClass, bool) {
	switch t {
	case EventFSCreate, EventVaultCreate:
		return opCreate, true
	case EventFSWrite, EventVaultWrite:
		return opWrite, true
	case EventFSDelete, EventVaultDelete:
		return opDelete, true
	}
	return 0, false
}

type dedupKey struct {
	path string
	op   opClass
}

// Manager owns the lifecycle of all registered plugins and fans out
// vault events to each of them.
//
// # Event deduplication
//
// Every Vault-API mutation emits both a vault_* event (synchronous, from the
// Vault write path) and, shortly after, a corresponding fs_* event (async,
// from the fsnotify watcher). The Manager coalesces each (path, op) pair
// within a short time window so plugins receive exactly one event per
// mutation. External writes that bypass the Vault API produce only fs_*
// events and are always dispatched.
//
// Usage:
//
//	mgr := plugin.NewManager(logger)
//	mgr.Register(myPlugin)
//	mgr.Start(ctx)
//	// ... server running ...
//	mgr.Stop()
type Manager struct {
	plugins []Plugin
	logger  *slog.Logger
	cancel  context.CancelFunc
	wg      sync.WaitGroup

	dedupMu sync.Mutex
	dedup   map[dedupKey]time.Time // last-dispatched timestamp per (path, op)
}

// NewManager returns a Manager that logs through logger.
func NewManager(logger *slog.Logger) *Manager {
	return &Manager{logger: logger}
}

// Register adds a plugin. Must be called before Start.
func (m *Manager) Register(p Plugin) {
	m.plugins = append(m.plugins, p)
}

// Get returns the first registered plugin with the given name, or nil.
func (m *Manager) Get(name string) Plugin {
	for _, p := range m.plugins {
		if p.Name() == name {
			return p
		}
	}
	return nil
}

// Start launches every plugin in its own goroutine.
// The provided ctx is used as the parent for a cancellable child context
// so Stop() can shut down all plugins independently of the parent.
func (m *Manager) Start(ctx context.Context) {
	ctx, m.cancel = context.WithCancel(ctx)
	for _, p := range m.plugins {
		p := p
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			m.logger.Info("plugin starting", "plugin", p.Name())
			if err := p.Start(ctx); err != nil && err != context.Canceled {
				m.logger.Error("plugin exited with error", "plugin", p.Name(), "err", err)
			}
			m.logger.Info("plugin stopped", "plugin", p.Name())
		}()
	}
}

// Stop cancels the context shared by all plugins and waits for them to exit,
// bounded by ctx. Plugin.Stop runs only after Start has returned: cleanup such
// as closing the search index deadlocks if it races a Start still inside it.
func (m *Manager) Stop(ctx context.Context) {
	if m.cancel != nil {
		m.cancel()
	}
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		m.logger.Warn("plugins did not stop within shutdown timeout; skipping cleanup")
		return
	}
	for _, p := range m.plugins {
		if err := p.Stop(); err != nil {
			m.logger.Warn("plugin stop error", "plugin", p.Name(), "err", err)
		}
	}
}

// Dispatch fans out a vault event to every registered plugin, first applying
// deduplication for (path, op) pairs that have already been dispatched within
// their class TTL. Safe to call concurrently from any goroutine.
func (m *Manager) Dispatch(e Event) {
	if op, ok := eventOpClass(e.Type); ok {
		key := dedupKey{path: e.Path, op: op}
		ttl := dedupTTLs[op]
		now := e.Time
		if now.IsZero() {
			now = time.Now()
		}

		m.dedupMu.Lock()
		if m.dedup == nil {
			m.dedup = make(map[dedupKey]time.Time)
		}
		last, seen := m.dedup[key]
		if seen && now.Sub(last) < ttl {
			m.dedupMu.Unlock()
			m.logger.Debug("plugin manager: duplicate event suppressed",
				"type", e.Type, "path", e.Path,
				"age_ms", now.Sub(last).Milliseconds())
			return
		}
		m.dedup[key] = now
		m.dedupMu.Unlock()
	}

	for _, p := range m.plugins {
		p.Notify(e)
	}
}
