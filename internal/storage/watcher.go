package storage

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/hardhacker/hylo/internal/plugin"
	"github.com/hardhacker/hylo/internal/util"
)

// Watcher monitors the vault directory tree for external file-system changes
// and keeps the metadata database in sync.
//
// Markdown notes and image files are tracked.
// Hidden files and directories (any path component starting with ".") are
// ignored, which covers vault internals such as .hylo and the
// temporary files created by Vault's two-phase write protocol.
//
// On kqueue (macOS, BSD), each directory must be watched (fsnotify Add) before the
// kernel delivers events for paths inside it. After MkdirAll, writeNoteLocked and
// Mkdir call ensureDirChain synchronously so fw.Add runs before the file write;
// otherwise the watcher goroutine may not have processed a directory Create yet
// and the final note path can miss fsnotify events (e.g. _clips/yyyy-mm-dd/... vs
// _clips/file.md).
//
// Vault API mutations update the file and the metadata row under one lock and
// emit their own vault_* event, so the watcher compares each change with the
// row under that lock: a change the row already reflects is the API's echo and
// is dropped, everything else is synced and emitted as fs_*.
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
			w.indexNewFiles(event.Name)
			return
		}
		w.changed(slashRel, false)

	case event.Has(fsnotify.Write):
		w.changed(slashRel, false)

	case event.Has(fsnotify.Remove), event.Has(fsnotify.Rename):
		// RENAME fires on the disappearing path; the new name emits a
		// separate CREATE event that is handled by the case above.
		w.removed(slashRel)
	}
}

// changed syncs the row for a note or image whose file appeared or changed,
// and emits an event when the file holds a change the row does not reflect yet.
func (w *Watcher) changed(relPath string, moved bool) {
	switch {
	case util.IsMarkdownPath(relPath):
		typ, modTime, ok := w.syncUpsert(relPath)
		if ok {
			w.g.emitEvent(plugin.Event{Type: typ, Path: relPath, Time: modTime, Moved: moved && typ == plugin.EventFSCreate})
		}
	case IsImagePath(relPath):
		if modTime, ok := w.syncImageUpsert(relPath); ok {
			w.g.emit(string(plugin.EventFSImage), relPath, false, modTime)
		}
	}
}

// removed drops the row for a note or image whose file disappeared.
func (w *Watcher) removed(relPath string) {
	switch {
	case util.IsMarkdownPath(relPath):
		if w.syncDelete(relPath) {
			w.g.emit(string(plugin.EventFSDelete), relPath, false, time.Now())
		}
	case IsImagePath(relPath):
		if w.syncImageDelete(relPath) {
			w.g.emit(string(plugin.EventFSImage), relPath, false, time.Now())
		}
	}
}

// syncUpsert brings the metadata row for a markdown file in line with the file.
// It reports false when there is nothing to announce: the row already matches
// (the echo of a Vault API write), the file is gone, or the sync failed.
func (w *Watcher) syncUpsert(relPath string) (plugin.EventType, time.Time, bool) {
	p, ok := ParsePath(relPath)
	if !ok {
		return "", time.Time{}, false
	}
	abs, err := w.g.osPath(p)
	if err != nil {
		return "", time.Time{}, false
	}

	// Under the lock the file and its row are never mid-write by the Vault API.
	w.g.mu.Lock()
	defer w.g.mu.Unlock()

	info, err := os.Stat(abs)
	if err != nil || info.IsDir() {
		return "", time.Time{}, false
	}
	prev, err := dbGet(w.g.db, p)
	known := err == nil
	if known && prev.Size == info.Size() && prev.UpdatedAt.Equal(info.ModTime()) {
		return "", time.Time{}, false
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return "", time.Time{}, false
	}
	if err := dbUpsert(w.g.db, fileNoteMeta(Note{}, p, data, info)); err != nil {
		w.logger.Error("vault watcher: note metadata upsert failed", "path", relPath, "err", err)
		return "", time.Time{}, false
	}
	w.logger.Debug("vault watcher: note metadata upserted", "path", relPath)
	if known {
		return plugin.EventFSWrite, info.ModTime(), true
	}
	return plugin.EventFSCreate, info.ModTime(), true
}

// syncDelete removes the metadata row for a markdown file and reports whether
// a known note actually disappeared.
func (w *Watcher) syncDelete(relPath string) bool {
	p, ok := ParsePath(relPath)
	if !ok {
		return false
	}
	abs, err := w.g.osPath(p)
	if err != nil {
		return false
	}

	w.g.mu.Lock()
	defer w.g.mu.Unlock()

	// Recreated since the event (atomic save, delete+recreate): its Create syncs it.
	if _, err := os.Lstat(abs); err == nil {
		return false
	}
	if _, err := dbGet(w.g.db, p); errors.Is(err, ErrMetadataNotFound) {
		return false
	}
	if err := dbDelete(w.g.db, p); err != nil {
		w.logger.Error("vault watcher: note metadata delete failed", "path", relPath, "err", err)
		return false
	}
	w.logger.Debug("vault watcher: note metadata deleted", "path", relPath)
	return true
}

// syncImageUpsert is syncUpsert for an image row.
func (w *Watcher) syncImageUpsert(relPath string) (time.Time, bool) {
	dir, name := PathParts(relPath)
	abs := w.g.OsImagePath(Image{Dir: dir, Name: name})

	w.g.mu.Lock()
	defer w.g.mu.Unlock()

	info, err := os.Stat(abs)
	if err != nil || info.IsDir() {
		return time.Time{}, false
	}
	if prev, err := dbImageGet(w.g.db, dir, name); err == nil &&
		prev.Size == info.Size() && prev.UpdatedAt.Equal(info.ModTime()) {
		return time.Time{}, false
	}
	if err := dbImageUpsert(w.g.db, imageFromFile(dir, name, info)); err != nil {
		w.logger.Error("vault watcher: image metadata upsert failed", "path", relPath, "err", err)
		return time.Time{}, false
	}
	return info.ModTime(), true
}

// syncImageDelete is syncDelete for an image row.
func (w *Watcher) syncImageDelete(relPath string) bool {
	dir, name := PathParts(relPath)
	abs := w.g.OsImagePath(Image{Dir: dir, Name: name})

	w.g.mu.Lock()
	defer w.g.mu.Unlock()

	if _, err := os.Lstat(abs); err == nil {
		return false
	}
	if _, err := dbImageGet(w.g.db, dir, name); errors.Is(err, ErrMetadataNotFound) {
		return false
	}
	if err := w.g.forgetImageLocked(dir, name); err != nil {
		w.logger.Error("vault watcher: image metadata delete failed", "path", relPath, "err", err)
		return false
	}
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

const movedNoteAge = 10 * time.Second

// indexNewFiles picks up notes and images written into a new directory before its watch
// was in place (mkdir -p a/b && write a/b/n.md): on kqueue their events never arrive.
// A note older than the directory event came with a moved folder, not a write.
func (w *Watcher) indexNewFiles(dir string) {
	cutoff := time.Now().Add(-movedNoteAge)
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if isHiddenName(d.Name()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() || !(util.IsMarkdownPath(path) || IsImagePath(path)) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(w.g.root, path)
		if err != nil {
			return nil
		}
		w.changed("/"+toSlash(rel), info.ModTime().Before(cutoff))
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
