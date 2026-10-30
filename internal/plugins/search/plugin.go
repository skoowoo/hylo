// Package search implements a Hylo plugin that keeps the full-text search
// index in sync with the vault by reacting to vault mutation events.
//
// On startup the plugin backtracks any notes that were written while the daemon
// was not running (nodes where indexed=false in the metadata DB).
//
// During normal operation it handles three event types:
//   - EventFSCreate / EventFSWrite  → IndexUpsert + MarkIndexed
//   - EventFSDelete / EventVaultDelete → IndexDelete (one note path per event)
package search

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/hardhacker/hylo/internal/config"
	"github.com/hardhacker/hylo/internal/plugin"
	"github.com/hardhacker/hylo/internal/storage"
	"github.com/hardhacker/hylo/internal/util"
)

// Plugin is the search-index plugin. It is always registered; no config gate.
type Plugin struct {
	vault   *storage.Vault
	cfg     config.SearchConfig
	logger  *slog.Logger
	eventCh chan plugin.Event
	settle  *plugin.Coalescer
	indexer *BleveIndexer
}

// New creates a search Plugin backed by the given vault.
func New(cfg config.SearchConfig, vault *storage.Vault, logger *slog.Logger) *Plugin {
	// Normalize exclude prefixes to vault-absolute paths.
	for i, p := range cfg.ExcludePrefixes {
		if !strings.HasPrefix(p, "/") {
			cfg.ExcludePrefixes[i] = "/" + p
		}
	}
	p := &Plugin{
		vault:   vault,
		cfg:     cfg,
		logger:  logger,
		eventCh: make(chan plugin.Event, 512),
	}
	p.settle = plugin.NewCoalescer(plugin.IndexWindow, p.enqueue)
	return p
}

// Name implements plugin.Plugin.
func (p *Plugin) Name() string { return "search" }

// Notify implements plugin.Plugin. A note being edited is reindexed once its
// edits settle, not on every autosave; a Final write is indexed right here,
// so a batch is searchable when it returns and can't overflow eventCh.
func (p *Plugin) Notify(e plugin.Event) {
	if e.Final {
		p.handleEvent(e)
		return
	}
	switch e.Type {
	case plugin.EventFSCreate, plugin.EventFSWrite,
		plugin.EventVaultCreate, plugin.EventVaultWrite,
		plugin.EventFSDelete, plugin.EventVaultDelete:
		p.settle.Add(e)
	}
}

// enqueue is non-blocking; it drops events when the channel is full rather
// than blocking the vault's event-dispatch path.
func (p *Plugin) enqueue(e plugin.Event) {
	select {
	case p.eventCh <- e:
	default:
		p.logger.Warn("search: event channel full, dropping event",
			"type", e.Type, "path", e.Path)
	}
}

// Start implements plugin.Plugin. Runs the backfill then processes events until
// ctx is cancelled.
func (p *Plugin) Start(ctx context.Context) error {
	indexer, err := NewBleveIndexer(p.vault.Root(), p.cfg.UseJieba)
	if err != nil {
		return fmt.Errorf("search: %w", err)
	}
	p.indexer = indexer

	p.backfill(ctx)

	for {
		select {
		case <-ctx.Done():
			return nil
		case e := <-p.eventCh:
			p.handleEvent(e)
		}
	}
}

// Stop implements plugin.Plugin.
func (p *Plugin) Stop() error {
	p.settle.Stop()
	if p.indexer != nil {
		return p.indexer.Close()
	}
	return nil
}

// DocCount returns the number of documents currently in the search index.
// Returns 0 if the indexer is not yet open.
func (p *Plugin) DocCount() (uint64, error) {
	if p.indexer == nil {
		return 0, nil
	}
	return p.indexer.DocCount()
}

// TagDocCount returns how many indexed notes carry the given tag.
// Returns an error if the search index is not ready.
func (p *Plugin) TagDocCount(tag string) (uint64, error) {
	if p.indexer == nil {
		return 0, fmt.Errorf("search index not ready")
	}
	return p.indexer.TagDocCount(tag)
}

// TagDistribution returns tag → document counts via a Bleve facet (see BleveIndexer.TagDistribution).
// Returns an error if the search index is not ready.
func (p *Plugin) TagDistribution(topN int) ([]TagCount, error) {
	if p.indexer == nil {
		return nil, fmt.Errorf("search index not ready")
	}
	return p.indexer.TagDistribution(topN)
}

// Search implements Searcher interface for the HTTP handler.
func (p *Plugin) Search(query string, opts SearchOptions) ([]SearchResult, error) {
	if p.indexer == nil {
		return nil, fmt.Errorf("search index not ready")
	}
	return p.indexer.Search(query, opts, func(path string) ([]byte, error) {
		sp, ok := storage.ParsePath(path)
		if !ok {
			return nil, fmt.Errorf("invalid path: %q", path)
		}
		return p.vault.ReadNote(sp)
	})
}

// ── event handling ────────────────────────────────────────────────────────────

func (p *Plugin) handleEvent(e plugin.Event) {
	if p.indexer == nil {
		return
	}

	switch e.Type {
	case plugin.EventFSCreate, plugin.EventFSWrite,
		plugin.EventVaultCreate, plugin.EventVaultWrite:
		p.indexFile(e.Path)
		p.logger.Debug("search: received event, and indexed", "path", e.Path)

	case plugin.EventFSDelete, plugin.EventVaultDelete:
		p.indexer.Delete(e.Path)
		p.logger.Debug("search: removed from index", "path", e.Path)
	}
}

// indexFile reads the file and upserts it into the search index.
func (p *Plugin) indexFile(pathStr string) {
	for _, prefix := range p.cfg.ExcludePrefixes {
		if strings.HasPrefix(pathStr, prefix) {
			return
		}
	}

	sp, ok := storage.ParsePath(pathStr)
	if !ok {
		return
	}

	// Check if the note is tracked in the metadata DB first.
	// If not, it means the write hasn't fully "succeeded" yet or it's an untracked file.
	tracked, err := p.vault.IsNoteTracked(sp)
	if err != nil {
		p.logger.Warn("search: could not check note tracking", "path", pathStr, "err", err)
		return
	}
	if !tracked {
		// Not in meta DB; ignore.
		return
	}

	content, err := p.vault.ReadNote(sp)
	if err != nil {
		p.logger.Warn("search: could not read note for indexing", "path", pathStr, "err", err)
		return
	}
	note, err := p.vault.StatNote(sp)
	if err != nil {
		p.logger.Warn("search: could not stat note for indexing", "path", pathStr, "err", err)
		return
	}
	tags := util.ExtractTags(content).All()
	p.indexer.Upsert(pathStr, note.Name, string(content), tags, note.UpdatedAt, string(note.Kind))
	if err := p.vault.MarkNoteIndexed(sp); err != nil {
		p.logger.Warn("search: could not mark note indexed", "path", pathStr, "err", err)
	}
	p.logger.Debug("search: indexed",
		"name", note.Name,
		"tags", len(tags),
		"content_len", len(content),
		"path", pathStr,
	)
}

// ── startup backfill ──────────────────────────────────────────────────────────

// Backfill indexes any notes that have indexed=false in the metadata DB.
// It opens the BleveIndexer if it is not already open.
// Returns the number of notes indexed.
// This is the public entry point used by both the daemon start path and the
// "hylo init" one-shot initialisation command.
func (p *Plugin) Backfill(ctx context.Context) (int, error) {
	if p.indexer == nil {
		indexer, err := NewBleveIndexer(p.vault.Root(), p.cfg.UseJieba)
		if err != nil {
			return 0, fmt.Errorf("search: open indexer: %w", err)
		}
		p.indexer = indexer
	}

	notes, err := p.vault.ListAllNotes(storage.ListOptions{OnlyUnindexed: true})
	if err != nil {
		return 0, fmt.Errorf("search: backfill: list notes: %w", err)
	}
	count := 0
	for _, note := range notes {
		if ctx.Err() != nil {
			break
		}
		p.indexFile(note.PathString())
		count++
	}
	return count, nil
}

// backfill is the internal daemon start-up wrapper around Backfill.
func (p *Plugin) backfill(ctx context.Context) {
	count, err := p.Backfill(ctx)
	if err != nil {
		p.logger.Warn("search: backfill failed", "err", err)
		return
	}
	if count > 0 {
		p.logger.Info("search: backfill complete", "indexed", count)
	}
}

