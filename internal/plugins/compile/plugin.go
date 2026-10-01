// Package compile implements a Hylo plugin for knowledge compilation.
// EventCompileRequested is emitted by the HTTP handler when the user manually
// triggers compilation; actual LLM work is performed by a mate agent.
package compile

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"sync"

	"github.com/hardhacker/hylo/internal/plugin"
	"github.com/hardhacker/hylo/internal/storage"
	"github.com/hardhacker/hylo/internal/util"
)

// ErrNoteAlreadyCompiled is returned by the trigger handler when the note is already compiled.
var ErrNoteAlreadyCompiled = errors.New("compile: note already compiled")

// DispatchFunc fans out plugin events (typically plugin.Manager.Dispatch).
type DispatchFunc func(plugin.Event)

// Plugin implements plugin.Plugin for note compilation event dispatch.
type Plugin struct {
	knowledgeDir string // vault-absolute path, e.g. "/_knowledge"
	vault        *storage.Vault
	logger       *slog.Logger
	dispatchFn   atomicDispatch
}

type atomicDispatch struct {
	mu sync.RWMutex
	fn DispatchFunc
}

func (a *atomicDispatch) Store(fn DispatchFunc) {
	a.mu.Lock()
	a.fn = fn
	a.mu.Unlock()
}

func (a *atomicDispatch) Load() DispatchFunc {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.fn
}

// New creates a Compile plugin. knowledgeDir is the vault-relative (or absolute)
// path to the knowledge output directory (e.g. "_knowledge" or "/_knowledge").
func New(knowledgeDir string, vault *storage.Vault, logger *slog.Logger) (*Plugin, error) {
	if knowledgeDir == "" {
		knowledgeDir = "_knowledge"
	}
	if !strings.HasPrefix(knowledgeDir, "/") {
		knowledgeDir = "/" + knowledgeDir
	}

	return &Plugin{
		knowledgeDir: knowledgeDir,
		vault:        vault,
		logger:       logger,
	}, nil
}

// SetDispatch wires the plugin event bus (called from server setup).
func (p *Plugin) SetDispatch(fn DispatchFunc) {
	p.dispatchFn.Store(fn)
}

// Name implements plugin.Plugin.
func (p *Plugin) Name() string { return "compile" }

// Notify implements plugin.Plugin. Non-blocking.
// Fires for create/write events on .md files inside the knowledge output dir.
func (p *Plugin) Notify(e plugin.Event) {
	isCreate := e.Type == plugin.EventFSCreate || e.Type == plugin.EventVaultCreate
	isWrite := e.Type == plugin.EventFSWrite || e.Type == plugin.EventVaultWrite
	if !isCreate && !isWrite {
		return
	}
	if !strings.HasSuffix(e.Path, ".md") {
		return
	}
	vaultPath := normalizeVaultPath(e.Path)
	if p.isKnowledgePath(vaultPath) {
		go p.handleKnowledgeDirNote(vaultPath)
	}
}

// Start implements plugin.Plugin.
func (p *Plugin) Start(ctx context.Context) error {
	p.logger.Info("compile: plugin started", "knowledge_dir", p.knowledgeDir)
	<-ctx.Done()
	return nil
}

// Stop implements plugin.Plugin.
func (p *Plugin) Stop() error { return nil }

// ── path helpers ──────────────────────────────────────────────────────────────

func normalizeVaultPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return p
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}

// isKnowledgePath reports whether vaultPath is inside the knowledge output dir.
func (p *Plugin) isKnowledgePath(vaultPath string) bool {
	return vaultPath == p.knowledgeDir || strings.HasPrefix(vaultPath, p.knowledgeDir+"/")
}

// handleKnowledgeDirNote is the entry point for create/write events on notes
// inside the knowledge output dir. It parses the frontmatter once, extracts
// the kind, and routes to the appropriate handler.
func (p *Plugin) handleKnowledgeDirNote(vaultPath string) {
	sp, ok := storage.ParsePath(vaultPath)
	if !ok {
		return
	}
	raw, err := p.vault.ReadNote(sp)
	if err != nil {
		p.logger.Warn("compile: knowledge dir update: cannot read note", "path", vaultPath, "err", err)
		return
	}
	fm, body := util.ParseFrontmatter(raw)
	if !fm.HasMeta() {
		return
	}
	switch strings.ToLower(fm.Kind) {
	case "knowledge":
		p.handleKnowledgeNoteUpdate(sp, fm, body)
	case "index":
		p.handleIndexNoteUpdate(sp, fm, body)
	}
}

// handleKnowledgeNoteUpdate syncs title, origin, and compile_count to the DB,
// updates knowledge_deps from the source_notes list in the frontmatter, and
// incrementally updates knowledge_links from wikilinks in the note body.
func (p *Plugin) handleKnowledgeNoteUpdate(sp storage.Path, fm util.Frontmatter, body []byte) {
	var kfm knowledgeFrontmatter
	for _, e := range fm.All {
		switch e.Key {
		case "title":
			kfm.title = e.Value
		case "compile_count":
			if n, err := strconv.Atoi(strings.TrimSpace(e.Value)); err == nil {
				kfm.compileCount = n
			}
		case "source_notes":
			kfm.sourceNotes = e.List
		case "tags":
			kfm.tags = e.List
		case "entity_type":
			kfm.entityType = strings.TrimSpace(e.Value)
		}
	}
	tags := kfm.tags
	if kfm.entityType != "" {
		tags = append([]string{kfm.entityType}, kfm.tags...)
	}
	if err := p.vault.MarkNoteAsKnowledge(sp, kfm.title, kfm.compileCount, tags); err != nil {
		p.logger.Warn("compile: knowledge metadata update: db update failed", "path", sp, "err", err)
		return
	}
	p.logger.Info("compile: knowledge metadata updated",
		"path", sp,
		"title", kfm.title,
		"compile_count", kfm.compileCount,
		"source_notes", kfm.sourceNotes,
	)
	p.syncKnowledgeDeps(sp, kfm.sourceNotes)
	if err := p.vault.ReplaceKnowledgeLinksForNote(sp, kfm.entityType, util.ExtractWikilinkNames(body)); err != nil {
		p.logger.Warn("compile: failed to update knowledge_links", "path", sp, "err", err)
	}
}

// handleIndexNoteUpdate syncs domain/title to the DB and updates index_deps
// from the knowledge note paths listed in the index table body.
func (p *Plugin) handleIndexNoteUpdate(sp storage.Path, fm util.Frontmatter, body []byte) {
	var ifm indexFrontmatter
	for _, e := range fm.All {
		if e.Key == "domain" {
			ifm.domain = e.Value
		}
	}
	ifm.knowledgePaths = parseIndexTablePaths(body)

	if err := p.vault.MarkNoteAsIndex(sp, ifm.domain); err != nil {
		p.logger.Warn("compile: index metadata update: db update failed", "path", sp, "err", err)
		return
	}
	p.logger.Info("compile: index metadata updated",
		"path", sp,
		"domain", ifm.domain,
		"knowledge_paths", ifm.knowledgePaths,
	)
	p.syncIndexDeps(sp, ifm.knowledgePaths)
}

// parseIndexTablePaths extracts vault-absolute knowledge note paths from the
// body of an index note by scanning each markdown table row and treating the
// last non-empty cell as a path when it starts with "/" and ends with ".md".
func parseIndexTablePaths(body []byte) []string {
	var paths []string
	for _, line := range bytes.Split(body, []byte("\n")) {
		s := strings.TrimSpace(string(line))
		if !strings.HasPrefix(s, "|") {
			continue
		}
		parts := strings.Split(s, "|")
		for i := len(parts) - 1; i >= 0; i-- {
			v := strings.TrimSpace(parts[i])
			if v == "" {
				continue
			}
			if strings.HasPrefix(v, "/") && strings.HasSuffix(v, ".md") {
				paths = append(paths, util.StripMarkdownEscapes(v))
			}
			break
		}
	}
	return paths
}

// knowledgeFrontmatter holds the fields extracted from a knowledge note's front matter.
type knowledgeFrontmatter struct {
	title        string
	compileCount int
	sourceNotes  []string
	tags         []string
	entityType   string
}

// indexFrontmatter holds the fields extracted from an index note's front matter and body.
type indexFrontmatter struct {
	domain         string
	knowledgePaths []string // extracted from the markdown table body
}

// syncKnowledgeDeps updates the knowledge_deps table so that the knowledge note's
// dependencies stay in sync with its frontmatter source_notes list, and marks
// each resolved source note as compiled.
//
// Entries containing "/" are treated as vault paths and looked up directly;
// plain filenames are batch-looked up by name. Filename lookups that return
// multiple matches are skipped as ambiguous.
func (p *Plugin) syncKnowledgeDeps(knowledgePath storage.Path, sourceNotes []string) {
	var resolved []storage.Path
	var names []string

	for _, sn := range sourceNotes {
		if strings.Contains(sn, "/") {
			sp, ok := storage.ParsePath(sn)
			if !ok {
				p.logger.Warn("compile: invalid source_note path, skipping", "source_note", sn)
				continue
			}
			resolved = append(resolved, sp)
		} else {
			name := sn
			if !strings.HasSuffix(name, ".md") {
				name += ".md"
			}
			names = append(names, name)
		}
	}

	if len(names) > 0 {
		notes, err := p.vault.GetNotesByNames(names)
		if err != nil {
			p.logger.Warn("compile: failed to look up source notes by name", "knowledge_path", knowledgePath, "err", err)
		} else {
			byName := make(map[string][]storage.Note, len(notes))
			for _, n := range notes {
				byName[n.Name] = append(byName[n.Name], n)
			}
			for _, name := range names {
				matches := byName[name]
				switch len(matches) {
				case 0:
					p.logger.Warn("compile: source note not found, skipping", "name", name)
				case 1:
					resolved = append(resolved, matches[0].Path())
				default:
					p.logger.Warn("compile: source note name ambiguous, skipping", "name", name, "count", len(matches))
				}
			}
		}
	}

	if err := p.vault.SetKnowledgeDeps(knowledgePath, resolved); err != nil {
		p.logger.Warn("compile: failed to update knowledge_deps", "knowledge_path", knowledgePath, "err", err)
	}
	// Mark each source note as compiled so CanCompile = false on future renders.
	for _, src := range resolved {
		if err := p.vault.MarkNoteCompiled(src); err != nil {
			p.logger.Warn("compile: failed to mark source note compiled", "path", src, "err", err)
		}
	}
}

// syncIndexDeps updates the index_deps table so that the index note's listed
// knowledge notes stay in sync with its table body.
//
// Entries containing "/" are treated as vault paths and looked up directly;
// plain filenames are batch-looked up by name. Filename lookups that return
// multiple matches are skipped as ambiguous.
func (p *Plugin) syncIndexDeps(indexPath storage.Path, knowledgePaths []string) {
	var resolved []storage.Path
	var names []string

	for _, kp := range knowledgePaths {
		if strings.Contains(kp, "/") {
			sp, ok := storage.ParsePath(kp)
			if !ok {
				p.logger.Warn("compile: invalid knowledge path in index, skipping", "path", kp)
				continue
			}
			resolved = append(resolved, sp)
		} else {
			name := kp
			if !strings.HasSuffix(name, ".md") {
				name += ".md"
			}
			names = append(names, name)
		}
	}

	if len(names) > 0 {
		notes, err := p.vault.GetNotesByNames(names)
		if err != nil {
			p.logger.Warn("compile: failed to look up knowledge notes by name", "index_path", indexPath, "err", err)
		} else {
			byName := make(map[string][]storage.Note, len(notes))
			for _, n := range notes {
				byName[n.Name] = append(byName[n.Name], n)
			}
			for _, name := range names {
				matches := byName[name]
				switch len(matches) {
				case 0:
					p.logger.Warn("compile: knowledge note not found, skipping", "name", name)
				case 1:
					resolved = append(resolved, matches[0].Path())
				default:
					p.logger.Warn("compile: knowledge note name ambiguous, skipping", "name", name, "count", len(matches))
				}
			}
		}
	}

	if err := p.vault.SetIndexDeps(indexPath, resolved); err != nil {
		p.logger.Warn("compile: failed to update index_deps", "index_path", indexPath, "err", err)
	}
}
