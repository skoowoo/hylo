package storage

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/hardhacker/hylo/internal/util"
)

// Watcher monitors the vault directory tree for external file-system changes
// and keeps the metadata database in sync.
//
// Only markdown files (.md, .markdown) are tracked.
// Hidden files and directories (any path component starting with ".") are
// ignored, which covers vault internals such as .hylo and the
// temporary files created by Vault's two-phase write protocol.
//
// On kqueue (macOS, BSD), each directory must be watched (fsnotify Add) before the
// kernel delivers events for paths inside it. After MkdirAll, writeNoteLocked and
// Mkdir call ensureDirChain synchronously so fw.Add runs before CreateTemp→Rename;
// otherwise the watcher goroutine may not have processed a directory Create yet
// and the final note path can miss fsnotify events (e.g. _clips/yyyy-mm-dd/... vs
// _clips/file.md).
//
// All events are dispatched immediately with no debouncing; write-event debouncing
// is handled by the plugin Manager before events reach plugins.
type Watcher struct {
	g      *Vault
	fw     *fsnotify.Watcher
	logger *slog.Logger
}

// StartWatcher starts a background goroutine that watches the vault root for
// external file-system changes and syncs the metadata database accordingly.
// The goroutine stops cleanly when ctx is cancelled.
//
// A watcher failure is non-fatal: the caller logs the error and the server
// continues running without live sync. Manual consistency repairs can be
// performed by restarting the daemon.
func (g *Vault) StartWatcher(ctx context.Context, logger *slog.Logger) error {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("vault watcher: create fsnotify watcher: %w", err)
	}

	w := &Watcher{g: g, fw: fw, logger: logger}

	w.logger.Debug("vault watcher: scanning vault tree for directory watches", "vault_root", g.root)
	// Register the root and every non-hidden subdirectory that already exists.
	if err := w.watchRecursive(g.root, "initial_scan"); err != nil {
		fw.Close()
		return fmt.Errorf("vault watcher: initial scan: %w", err)
	}
	w.logger.Debug("vault watcher: initial directory watch scan finished", "vault_root", g.root)

	g.mu.Lock()
	g.ensureWatch = w.ensureDirChain
	g.mu.Unlock()

	go w.run(ctx)
	return nil
}

// run is the event loop. It blocks until ctx is cancelled or the fsnotify
// channel is closed.
func (w *Watcher) run(ctx context.Context) {
	defer w.fw.Close()

	for {
		select {
		case <-ctx.Done():
			return

		case event, ok := <-w.fw.Events:
			if !ok {
				return
			}
			w.handle(event)

		case err, ok := <-w.fw.Errors:
			if !ok {
				return
			}
			w.logger.Warn("vault watcher: fs event error", "err", err)
		}
	}
}

// handle processes a single fsnotify event.
func (w *Watcher) handle(event fsnotify.Event) {
	// Ignore anything inside a hidden directory or hidden files.
	if w.isHidden(event.Name) {
		return
	}

	// Ensure the path is inside the vault root.
	rel, err := filepath.Rel(w.g.root, event.Name)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return
	}
	slashRel := "/" + toSlash(rel)

	switch {
	case event.Has(fsnotify.Create):
		info, statErr := os.Stat(event.Name)
		if statErr != nil {
			return
		}
		if info.IsDir() {
			// Walk the new directory so every level is watched. watchRecursive
			// registers the directory itself and all non-hidden subdirectories
			// beneath it (MkdirAll may have created several levels in one call).
			if walkErr := w.watchRecursive(event.Name, "fs_event"); walkErr != nil {
				w.logger.Warn("vault watcher: recursive watch after mkdir",
					"path", event.Name, "err", walkErr)
			}
			return
		}
		if !util.IsMarkdownPath(event.Name) {
			return
		}
		if w.syncUpsert(slashRel, info) {
			w.g.emit("fs_create", slashRel, false, info.ModTime())
		}

	case event.Has(fsnotify.Write):
		info, statErr := os.Stat(event.Name)
		if statErr != nil || info.IsDir() {
			return
		}
		if !util.IsMarkdownPath(event.Name) {
			return
		}
		if w.syncUpsert(slashRel, info) {
			w.g.emit("fs_write", slashRel, false, info.ModTime())
		}

	case event.Has(fsnotify.Remove), event.Has(fsnotify.Rename):
		// RENAME fires on the disappearing path; the new name emits a
		// separate CREATE event that is handled by the case above.
		if !util.IsMarkdownPath(event.Name) {
			return
		}
		if w.syncDelete(slashRel) {
			w.g.emit("fs_delete", slashRel, false, time.Now())
		}
	}
}

// syncUpsert updates the metadata row for a markdown file and reports whether
// it succeeded. Callers are responsible for emitting the appropriate event.
//
// Note: the Vault API uses atomic temp→rename writes, so fsnotify reports both
// new files and overwrites as Create. For the compile plugin this is harmless
// because IsCompiled() acts as a safety guard.
func (w *Watcher) syncUpsert(relPath string, info os.FileInfo) bool {
	p, ok := ParsePath(relPath)
	if !ok {
		return false
	}

	// Read the file so an out-of-band change (external editor, sync client)
	// keeps preview current too, not just size/mtime.
	var preview NotePreview
	if abs, absErr := w.g.osPath(p); absErr == nil {
		if data, readErr := os.ReadFile(abs); readErr == nil {
			preview = notePreviewFromSummary(util.GeneratePreview(data, 0))
		}
	}

	w.g.mu.Lock()
	err := dbUpsert(w.g.db, Note{
		Dir:       p.Dir(),
		Name:      p.Base(),
		Size:      info.Size(),
		UpdatedAt: info.ModTime(),
		Preview:   preview,
	})
	w.g.mu.Unlock()

	if err != nil {
		w.logger.Error("vault watcher: note metadata upsert failed", "path", relPath, "err", err)
		return false
	}
	w.logger.Debug("vault watcher: note metadata upserted", "path", relPath)
	return true
}

// syncDelete removes the metadata row for a markdown file and reports whether
// it succeeded. Callers are responsible for emitting the appropriate event.
func (w *Watcher) syncDelete(relPath string) bool {
	p, ok := ParsePath(relPath)
	if !ok {
		return false
	}

	w.g.mu.Lock()
	err := dbDelete(w.g.db, p)
	w.g.mu.Unlock()

	if err != nil {
		w.logger.Error("vault watcher: note metadata delete failed", "path", relPath, "err", err)
		return false
	}
	w.logger.Debug("vault watcher: note metadata deleted", "path", relPath)
	return true
}

// ensureDirChain registers fsnotify watches for every directory on the path
// from vault root to absDir (inclusive). Called synchronously from the vault
// write path after MkdirAll and before the actual file write, so that the
// kernel can deliver the subsequent Create event on kqueue/macOS.
//
// absDir is the immediate parent of the note being written; it was just
// created by MkdirAll and has no subdirectories yet, so no recursive walk
// is needed here. Pre-existing subdirectories are covered by StartWatcher's
// initial scan and by the watcher goroutine handling fsnotify Create events.
func (w *Watcher) ensureDirChain(absDir string) {
	rel, err := filepath.Rel(w.g.root, absDir)
	if err != nil || strings.HasPrefix(rel, "..") {
		return
	}
	rel = filepath.ToSlash(filepath.Clean(rel))
	if rel == "." {
		return
	}
	cur := w.g.root
	for _, seg := range strings.Split(rel, "/") {
		if seg == "" {
			continue
		}
		cur = filepath.Join(cur, seg)
		if isHiddenName(seg) {
			return
		}
		w.addWatch(cur, "ensure_write")
	}
}

// addWatch registers absPath with fsnotify and logs at Info on success.
// Returns true if Add succeeded (including first-time registration).
func (w *Watcher) addWatch(absPath string, reason string) bool {
	err := w.fw.Add(absPath)
	rel, relErr := filepath.Rel(w.g.root, absPath)
	if relErr != nil {
		rel = absPath
	} else {
		rel = toSlash(rel)
		if rel == "." {
			rel = "."
		}
	}
	if err != nil {
		w.logger.Debug("vault watcher: watch add skipped", "path", rel, "reason", reason, "err", err)
		return false
	}
	w.logger.Debug("vault watcher: watching directory", "path", rel, "reason", reason)
	return true
}

// watchRecursive adds dir and every non-hidden subdirectory beneath it to the
// fsnotify watcher. Inaccessible paths are silently skipped.
func (w *Watcher) watchRecursive(dir string, reason string) error {
	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip inaccessible entries
		}
		if !d.IsDir() {
			return nil
		}
		if isHiddenName(d.Name()) {
			return filepath.SkipDir
		}
		w.addWatch(path, reason)
		return nil
	})
}

// isHidden reports whether absPath has any path component (relative to the
// vault root) that begins with ".", marking it as hidden.
func (w *Watcher) isHidden(absPath string) bool {
	rel, err := filepath.Rel(w.g.root, absPath)
	if err != nil {
		return true
	}
	for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
		if isHiddenName(part) {
			return true
		}
	}
	return false
}

// isHiddenName reports whether a single file/directory name is hidden.
func isHiddenName(name string) bool {
	return strings.HasPrefix(name, ".")
}
