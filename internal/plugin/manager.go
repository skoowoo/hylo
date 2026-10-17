package plugin

import (
	"context"
	"log/slog"
	"sync"
)

// Manager owns the lifecycle of all registered plugins and fans out
// vault events to each of them.
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

// Dispatch fans out an event to every registered plugin. Safe to call
// concurrently from any goroutine.
func (m *Manager) Dispatch(e Event) {
	for _, p := range m.plugins {
		p.Notify(e)
	}
}
