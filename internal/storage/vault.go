package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hardhacker/hylo/internal/plugin"
	"github.com/hardhacker/hylo/internal/util"
)

const (
	// vaultInternalDir holds metadata DB, search index, and other
	// vault-internal data. User notes live directly under the vault root.
	vaultInternalDir = ".hylo"

	vaultDBFileName = "meta.db"
)

func vaultInternalPath(root string) string {
	return filepath.Join(root, vaultInternalDir)
}

func vaultDBPath(root string) string {
	return filepath.Join(vaultInternalPath(root), vaultDBFileName)
}

// IsVaultInitialized reports whether dir contains a .hylo directory.
// It does not create any paths on disk.
func IsVaultInitialized(dir string) (bool, error) {
	expanded, err := expandHome(dir)
	if err != nil {
		return false, fmt.Errorf("vault: expand path: %w", err)
	}
	abs, err := filepath.Abs(expanded)
	if err != nil {
		return false, fmt.Errorf("vault: resolve path: %w", err)
	}
	st, err := os.Stat(vaultInternalPath(abs))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("vault: stat %s: %w", vaultInternalPath(abs), err)
	}
	if !st.IsDir() {
		return false, nil
	}
	return true, nil
}

// EventHook is an optional callback invoked after every successful vault mutation.
// It must not block.
type EventHook func(e plugin.Event)

// Vault is the root of the Hylo personal database.
//
// All note operations take explicit (dir, name string) coordinates:
//   - dir  — vault-absolute slash path of the containing directory, e.g. "/journal/2026".
//     The vault root is represented as "/".
//   - name — base filename, e.g. "april.md".
//
// All directory operations take a single dir string using the same convention.
//
// Vault is safe for concurrent use: read operations share a read lock and
// write operations hold an exclusive write lock.
//
// Every write operation follows a simple protocol:
//  1. Mutate the filesystem.
//  2. Update the metadata database.
type Vault struct {
	mu          sync.RWMutex
	root        string              // absolute, cleaned OS path
	db          *sql.DB             // SQLite metadata index; always non-nil after New()
	shortsDir   string              // configurable; defaults to ShortDir const
	onEvent     EventHook           // optional; called after each successful mutation
	ensureWatch func(absDir string) // set by StartWatcher; sync fsnotify watches after MkdirAll
}

// New creates a Vault rooted at dir.
// dir is expanded (~ → home dir) and created if it does not yet exist.
func New(dir string) (*Vault, error) {
	expanded, err := expandHome(dir)
	if err != nil {
		return nil, fmt.Errorf("vault: expand path: %w", err)
	}
	abs, err := filepath.Abs(expanded)
	if err != nil {
		return nil, fmt.Errorf("vault: resolve path: %w", err)
	}
	if err := os.MkdirAll(abs, 0o750); err != nil {
		return nil, fmt.Errorf("vault: create root %q: %w", abs, err)
	}
	if err := os.MkdirAll(vaultInternalPath(abs), 0o750); err != nil {
		return nil, fmt.Errorf("vault: create internal dir: %w", err)
	}
	db, err := openDB(abs)
	if err != nil {
		return nil, fmt.Errorf("vault: open metadata db: %w", err)
	}
	return &Vault{root: abs, db: db, shortsDir: ShortDir}, nil
}

// Close releases the vault's underlying resources (SQLite DB).
func (g *Vault) Close() error {
	return g.db.Close()
}

// Root returns the absolute path of the vault root on disk.
func (g *Vault) Root() string { return g.root }

// SetShortsDir overrides the default shorts directory (used when callers pass an empty dir).
func (g *Vault) SetShortsDir(dir string) {
	if dir != "" {
		g.shortsDir = strings.TrimRight(dir, "/")
	}
}

// ── compile plugin hooks ──────────────────────────────────────────────────────

// MarkNoteAsKnowledge sets kind = 'knowledge', compile_count, and tags on the note.
// title is optional; pass an empty string to leave the existing title unchanged.
func (g *Vault) MarkNoteAsKnowledge(p Path, title string, compileCount int, tags []string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return dbSetKindKnowledge(g.db, p, title, compileCount, tags)
}

// MarkNoteCompiled sets compile_count = 1 on the raw note at p, marking it as
// submitted for compilation. Idempotent; always writes 1, never increments.
func (g *Vault) MarkNoteCompiled(p Path) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return dbSetCompileCount(g.db, p, 1)
}

// MarkNoteAsIndex sets kind = 'index' on the note.
// title is optional; pass an empty string to leave the existing title unchanged.
func (g *Vault) MarkNoteAsIndex(p Path, title string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return dbSetKindIndex(g.db, p, title)
}

// UpsertNoteAsset records an extracted resource on a note. Does not rewrite markdown.
// Duplicate (note, kind, filename) updates source_url and ord.
func (g *Vault) UpsertNoteAsset(p Path, a NoteAsset) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	a.NoteDir = p.Dir()
	a.NoteName = p.Base()
	return dbUpsertNoteAsset(g.db, a)
}

// ReplaceNoteAssets atomically replaces all resources of a kind for a note.
// Passing an empty slice clears existing resources of that kind.
func (g *Vault) ReplaceNoteAssets(p Path, kind NoteAssetKind, assets []NoteAsset) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return dbReplaceNoteAssets(g.db, p, kind, assets)
}

// DeleteNoteAssetsByFilename removes extracted asset references to filename.
func (g *Vault) DeleteNoteAssetsByFilename(filename string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return dbDeleteNoteAssetsByFilename(g.db, filename)
}

// GetNoteAsset returns the first resource of the given kind (lowest ord).
func (g *Vault) GetNoteAsset(p Path, kind NoteAssetKind) (NoteAsset, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return dbGetNoteAsset(g.db, p, kind)
}

// ListNoteAssets returns all resources of the given kind, ordered by ord.
func (g *Vault) ListNoteAssets(p Path, kind NoteAssetKind) ([]NoteAsset, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return dbListNoteAssets(g.db, p, kind)
}

// NoteAssetFilenames returns path → first filename for the given kind across notes.
func (g *Vault) NoteAssetFilenames(paths []Path, kind NoteAssetKind) (map[string]string, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return dbGetNoteAssetsByKind(g.db, paths, kind)
}

// GetKnowledgeDeps returns all source note paths that the given knowledge note depends on.
func (g *Vault) GetKnowledgeDeps(knowledge Path) ([]Path, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return dbGetKnowledgeDeps(g.db, knowledge)
}

// GetSourceKnowledges returns all knowledge note paths that list the given note as a source.
func (g *Vault) GetSourceKnowledges(source Path) ([]Path, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return dbGetSourceKnowledges(g.db, source)
}

// GetNotesByPaths returns metadata for each path in the slice.
// Missing rows are silently skipped.
func (g *Vault) GetNotesByPaths(paths []Path) ([]Note, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return dbGetByPaths(g.db, paths)
}

// SetKnowledgeDeps replaces all knowledge_deps rows for the given knowledge note
// with the supplied source paths, inside a single transaction.
// Passing an empty sources slice clears all existing dependencies.
func (g *Vault) SetKnowledgeDeps(knowledge Path, sources []Path) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return dbReplaceKnowledgeDeps(g.db, knowledge, sources)
}

// SetIndexDeps replaces all index_deps rows for the given index note with the
// supplied knowledge paths, inside a single transaction.
// Passing an empty knowledges slice clears all existing entries.
func (g *Vault) SetIndexDeps(index Path, knowledges []Path) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return dbReplaceIndexDeps(g.db, index, knowledges)
}

// GetIndexDeps returns all knowledge note paths listed by the given index note.
func (g *Vault) GetIndexDeps(index Path) ([]Path, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return dbGetIndexDeps(g.db, index)
}

// GetAllIndexDepCounts returns a map of index note PathString → knowledge dep count
// for every index note that has at least one dep.
func (g *Vault) GetAllIndexDepCounts() (map[string]int, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return dbGetAllIndexDepCounts(g.db)
}

// GetKnowledgeIndexes returns all index note paths that include the given knowledge note.
func (g *Vault) GetKnowledgeIndexes(knowledge Path) ([]Path, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return dbGetKnowledgeIndexes(g.db, knowledge)
}

// ReplaceKnowledgeLinksForNote incrementally updates knowledge_links for a
// single source note. wikilinkNames are the raw wikilink targets extracted from
// the note body; they are resolved against the set of compile-origin notes in
// the DB. Unresolvable names are silently skipped.
func (g *Vault) ReplaceKnowledgeLinksForNote(source Path, entityType string, wikilinkNames []string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	var targets []Path
	if len(wikilinkNames) > 0 {
		candidates, err := dbGetByNames(g.db, wikilinkNames)
		if err != nil {
			return fmt.Errorf("storage: replace knowledge links: lookup names: %w", err)
		}
		for _, n := range candidates {
			if n.Kind == KindKnowledge && n.Path() != source {
				targets = append(targets, n.Path())
			}
		}
	}
	return dbReplaceKnowledgeLinks(g.db, source, entityType, targets)
}

// GetAllKnowledgeLinks returns every directed wikilink edge between knowledge notes.
func (g *Vault) GetAllKnowledgeLinks() ([]KnowledgeEdge, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return dbGetAllKnowledgeLinks(g.db)
}

// GetKnowledgeLinksForIndex returns wikilink edges whose both endpoints belong
// to the given index note's knowledge set.
func (g *Vault) GetKnowledgeLinksForIndex(index Path) ([]KnowledgeEdge, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return dbGetKnowledgeLinksForIndex(g.db, index)
}

// BackfillKnowledgeLinks rebuilds the knowledge_links table from the current
// vault filesystem. It reads every knowledge note already registered in the
// notes table, extracts wikilinks from their bodies, resolves targets to other
// knowledge notes, and replaces the knowledge_links rows.
// This is safe to call at any time: it only touches knowledge_links.
func (g *Vault) BackfillKnowledgeLinks(knowledgeOutputDir string) error {
	if knowledgeOutputDir == "" {
		knowledgeOutputDir = "/_knowledge"
	}
	if !strings.HasPrefix(knowledgeOutputDir, "/") {
		knowledgeOutputDir = "/" + knowledgeOutputDir
	}
	distillPrefix := knowledgeOutputDir + "/"

	g.mu.Lock()
	defer g.mu.Unlock()

	// Clear existing edges.
	if _, err := g.db.Exec(`DELETE FROM knowledge_links`); err != nil {
		return fmt.Errorf("storage: backfill knowledge_links: clear: %w", err)
	}

	// Load all knowledge notes from DB.
	knowledgeNotes, err := dbListAll(g.db, ListOptions{
		OnlyKinds: []Kind{KindKnowledge},
	})
	if err != nil {
		return fmt.Errorf("storage: backfill knowledge_links: list: %w", err)
	}

	// Collect all wikilink targets and entity_type from each knowledge note's body.
	type kLink struct {
		path          Path
		entityType    string
		wikilinkNames []string
	}
	var linksToProcess []kLink

	for _, n := range knowledgeNotes {
		slashRel := n.PathString()
		// Only process notes under the knowledge output directory.
		if slashRel != knowledgeOutputDir && !strings.HasPrefix(slashRel, distillPrefix) {
			continue
		}
		absPath, err := g.osPath(n.Path())
		if err != nil {
			continue
		}
		data, err := os.ReadFile(absPath)
		if err != nil {
			continue
		}
		fm, body := util.ParseFrontmatter(data)
		// Extract entity_type from frontmatter.
		var entityType string
		for _, e := range fm.All {
			if e.Key == "entity_type" {
				entityType = strings.TrimSpace(e.Value)
				break
			}
		}
		if wlNames := util.ExtractWikilinkNames(body); len(wlNames) > 0 {
			linksToProcess = append(linksToProcess, kLink{
				path:          n.Path(),
				entityType:    entityType,
				wikilinkNames: wlNames,
			})
		}
	}

	if len(linksToProcess) == 0 {
		return nil
	}

	// Batch-resolve names to knowledge note paths.
	seenName := make(map[string]bool)
	var allNames []string
	for _, kl := range linksToProcess {
		for _, name := range kl.wikilinkNames {
			if !seenName[name] {
				seenName[name] = true
				allNames = append(allNames, name)
			}
		}
	}
	knowledgeByName := make(map[string]Path)
	if candidates, err := dbGetByNames(g.db, allNames); err == nil {
		for _, n := range candidates {
			if n.Kind == KindKnowledge {
				knowledgeByName[n.Name] = n.Path()
			}
		}
	}

	for _, kl := range linksToProcess {
		var targets []Path
		for _, name := range kl.wikilinkNames {
			if tp, ok := knowledgeByName[name]; ok && tp != kl.path {
				targets = append(targets, tp)
			}
		}
		if len(targets) > 0 {
			_ = dbReplaceKnowledgeLinks(g.db, kl.path, kl.entityType, targets)
		}
	}
	return nil
}

// BackfillPreviews recomputes the preview column (excerpt text + checklist/
// image/code flags — see util.GeneratePreview) for every note currently
// registered in the metadata DB, reading each one's content fresh from disk.
// Unlike ScanAndRegisterFull, it only touches the preview column — kind,
// tags, knowledge deps, and the search index are all left exactly as they
// are. Used by `hylo init --preview-only` to backfill a vault that predates
// the preview feature (or one last registered by a version of
// ScanAndRegisterFull that didn't compute it), without redoing the rest of a
// full rescan. A note whose file can't be read (permissions, removed
// mid-run) is skipped rather than aborting the rest of the batch — preview is
// a disposable cache, not something worth failing the whole command over.
func (g *Vault) BackfillPreviews() (int, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	notes, err := dbListAll(g.db, ListOptions{})
	if err != nil {
		return 0, fmt.Errorf("storage: backfill previews: list notes: %w", err)
	}

	count := 0
	for _, n := range notes {
		absPath, err := g.osPath(n.Path())
		if err != nil {
			continue
		}
		data, err := os.ReadFile(absPath)
		if err != nil {
			continue
		}
		preview := notePreviewFromSummary(util.GeneratePreview(data, 0))
		if err := dbSetPreview(g.db, n.Path(), preview); err != nil {
			continue
		}
		count++
	}
	return count, nil
}

// SetNoteTitle sets the title column for the note at p.
func (g *Vault) SetNoteTitle(p Path, title string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return dbSetTitle(g.db, p, title)
}

func (g *Vault) IsNoteTracked(p Path) (bool, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	var id int
	err := g.db.QueryRow(`SELECT id FROM notes WHERE dir = ? AND name = ?`, p.Dir(), p.Base()).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// ── event hook ────────────────────────────────────────────────────────────────

// SetEventHook registers a callback that is invoked (non-blocking) after every
// successful note or directory mutation. Pass nil to remove a registered hook.
func (g *Vault) SetEventHook(fn EventHook) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.onEvent = fn
}

// emit fires the event hook for a standard mutation event.
// Must be called after the write-lock has been released.
func (g *Vault) emit(typ, path string, isDir bool, t time.Time) {
	g.emitEvent(plugin.Event{Type: plugin.EventType(typ), Path: path, IsDir: isDir, Time: t})
}

// emitEvent fires the event hook with a fully-populated Event.
func (g *Vault) emitEvent(e plugin.Event) {
	g.mu.RLock()
	fn := g.onEvent
	g.mu.RUnlock()
	if fn != nil {
		fn(e)
	}
}

// ── note read operations ──────────────────────────────────────────────────────

func (g *Vault) StatNote(p Path) (Note, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	abs, absErr := g.osPath(p)
	if absErr != nil {
		return Note{}, absErr
	}

	n, dbErr := dbGet(g.db, p)
	if dbErr != nil {
		// DB is authoritative: not found there means not found.
		// Check FS only to surface ErrIsDir when the path is a directory.
		if errors.Is(dbErr, ErrNotFound) {
			if info, statErr := os.Stat(abs); statErr == nil && info.IsDir() {
				return Note{}, ErrIsDir
			}
		}
		return Note{}, dbErr
	}

	// Verify the file still exists on disk (guard against stale DB rows).
	if _, statErr := os.Stat(abs); statErr != nil {
		return Note{}, mapOSError(statErr)
	}
	return n, nil
}

func (g *Vault) ReadNote(p Path) ([]byte, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.readNoteLocked(p)
}

func (g *Vault) ReadNoteStream(p Path) (io.ReadCloser, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if err := validateNoteName(p.Base()); err != nil {
		return nil, err
	}
	abs, err := g.osPath(p)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(abs)
	if err != nil {
		return nil, mapOSError(err)
	}
	return f, nil
}

// ── note write operations ─────────────────────────────────────────────────────

func (g *Vault) WriteNote(p Path, data []byte, kind Kind) error {
	return g.WriteNoteWithMeta(p, data, Note{Kind: kind})
}

// WriteNoteWithMeta writes data to p and persists meta fields (Origin, Title)
// atomically in the same DB upsert. Use this instead of WriteNote
// followed by separate Set* calls when the metadata is known up front.
func (g *Vault) WriteNoteWithMeta(p Path, data []byte, meta Note) error {
	g.mu.Lock()
	isNew, modTime, err := g.writeNoteLocked(p, data, meta)
	g.mu.Unlock()
	if err != nil {
		return err
	}
	if isNew {
		g.emit("vault_create", p.String(), false, modTime)
	} else {
		g.emit("vault_write", p.String(), false, modTime)
	}
	return nil
}

// CreateUntitledNote creates a brand-new note in dir, auto-named
// "Untitled <timestamp>" (deduplicated with a "(2)", "(3)", … suffix if
// that exact name is somehow already taken), with the given initial
// content. Used by the editor's "new note" flow: nothing is created
// server-side until the user types the first character (see
// POST /api/vault/create-untitled) — there's no separate draft state to
// publish or discard, and no filename to pick up front. The caller can
// rename it any time afterward via RenameNote.
func (g *Vault) CreateUntitledNote(dir string, content []byte) (Path, error) {
	if dirHasUnderscoreSegment(dir) {
		return "", ErrInvalidPath
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	base := "Untitled " + time.Now().Format("2006-01-02 15-04-05")
	name := base + ".md"
	for i := 2; ; i++ {
		abs, err := g.osPath(Path(JoinPath(dir, name)))
		if err != nil {
			return "", err
		}
		taken, err := dbNameTaken(g.db, name)
		if err != nil {
			return "", err
		}
		if !taken {
			if _, statErr := os.Stat(abs); errors.Is(statErr, fs.ErrNotExist) {
				break
			} else if statErr != nil {
				return "", mapOSError(statErr)
			}
		}
		name = fmt.Sprintf("%s (%d).md", base, i)
	}

	p := Path(JoinPath(dir, name))
	if _, _, err := g.writeNoteLocked(p, content, Note{}); err != nil {
		return "", err
	}
	return p, nil
}

// PrependNote inserts incoming into the note at p immediately after the first
// H1 heading. When heading is non-empty the content is inserted at the start
// of the first section whose heading matches (case-insensitive), falling back
// to MdPrepend if not found. An empty heading always uses MdPrepend.
func (g *Vault) PrependNote(p Path, incoming []byte, heading string) error {
	if err := validateNoteName(p.Base()); err != nil {
		return err
	}
	if !util.IsValidText(incoming) {
		return ErrBinaryContent
	}
	g.mu.Lock()
	existing, err := g.readNoteLocked(p)
	if err != nil && !errors.Is(err, ErrNotFound) {
		g.mu.Unlock()
		return err
	}
	var merged []byte
	if heading != "" {
		merged = util.MdPrependSection(existing, incoming, heading)
	} else {
		merged = util.MdPrepend(existing, incoming)
	}
	isNew, modTime, err := g.writeNoteLocked(p, merged, Note{})
	g.mu.Unlock()
	if err != nil {
		return err
	}
	if isNew {
		g.emit("vault_create", p.String(), false, modTime)
	} else {
		g.emit("vault_write", p.String(), false, modTime)
	}
	return nil
}

// AppendNote appends incoming to the note at p. When heading is non-empty the
// content is inserted after the last section whose heading matches
// (case-insensitive), falling back to end-of-file if not found. An empty
// heading always appends to the end of the file.
func (g *Vault) AppendNote(p Path, incoming []byte, heading string) error {
	if err := validateNoteName(p.Base()); err != nil {
		return err
	}
	if !util.IsValidText(incoming) {
		return ErrBinaryContent
	}
	g.mu.Lock()
	existing, err := g.readNoteLocked(p)
	if err != nil && !errors.Is(err, ErrNotFound) {
		g.mu.Unlock()
		return err
	}
	var merged []byte
	if heading != "" {
		merged = util.MdAppendSection(existing, incoming, heading)
	} else {
		merged = util.MdAppend(existing, incoming)
	}
	isNew, modTime, err := g.writeNoteLocked(p, merged, Note{})
	g.mu.Unlock()
	if err != nil {
		return err
	}
	if isNew {
		g.emit("vault_create", p.String(), false, modTime)
	} else {
		g.emit("vault_write", p.String(), false, modTime)
	}
	return nil
}

// DeleteNote permanently removes the note at p from the vault.
// dbDelete cascades the metadata cleanup to every relation table p might
// participate in (knowledge_deps, index_deps, knowledge_links, note_assets)
// and resets compile_count on any source left with no knowledge dependents —
// it never deletes another note, only the relation rows that reference p.
func (g *Vault) DeleteNote(p Path) error {
	g.mu.Lock()
	absSrc, err := g.osPath(p)
	if err != nil {
		g.mu.Unlock()
		return err
	}

	if rmErr := os.Remove(absSrc); rmErr != nil && !errors.Is(rmErr, fs.ErrNotExist) {
		g.mu.Unlock()
		return mapOSError(rmErr)
	}
	if dbErr := dbDelete(g.db, p); dbErr != nil {
		g.mu.Unlock()
		return fmt.Errorf("vault: metadata delete %q: %w", p.String(), dbErr)
	}

	g.mu.Unlock()
	g.emit("vault_delete", p.String(), false, time.Now())
	return nil
}

// MoveNote moves the note at p to the same-named file under newDir, leaving
// the filename untouched. newDir is created if it doesn't exist yet (mirrors
// WriteNoteWithMeta's directory auto-creation). Returns the note's new path.
//
// dbMove cascades every relation table p might participate in to the new
// directory synchronously, regardless of kind — moving a raw source note,
// a knowledge note, or an index note are all handled the same way at the DB
// layer. On top of that, a dependent knowledge note's source_notes:
// frontmatter or a dependent index note's table may hold p's full path as a
// literal string (see skills/hylo-compile-note and
// skills/hylo-index-knowledge); those are rewritten too, best-effort, once
// the move itself has committed — see dbMove's comment for why leaving them
// stale would eventually undo this.
//
// Returns ErrAlreadyExists if newDir already has a note named p.Base().
func (g *Vault) MoveNote(p Path, newDir string) (Path, error) {
	if err := validateNoteName(p.Base()); err != nil {
		return "", err
	}
	nd, ok := ParsePath(newDir)
	if !ok {
		return "", ErrInvalidPath
	}
	newPath := Path(JoinPath(nd.String(), p.Base()))
	if newPath == p {
		return p, nil
	}

	g.mu.Lock()

	absOld, err := g.osPath(p)
	if err != nil {
		g.mu.Unlock()
		return "", err
	}
	absNew, err := g.osPath(newPath)
	if err != nil {
		g.mu.Unlock()
		return "", err
	}
	internalRoot := filepath.Clean(vaultInternalPath(g.root))
	if strings.HasPrefix(absNew, internalRoot+string(os.PathSeparator)) || absNew == internalRoot {
		g.mu.Unlock()
		return "", ErrInvalidPath
	}
	if _, statErr := os.Stat(absNew); statErr == nil {
		g.mu.Unlock()
		return "", ErrAlreadyExists
	} else if !errors.Is(statErr, fs.ErrNotExist) {
		g.mu.Unlock()
		return "", mapOSError(statErr)
	}

	parent := filepath.Dir(absNew)
	if err := os.MkdirAll(parent, 0o750); err != nil {
		g.mu.Unlock()
		return "", fmt.Errorf("vault: mkdir for %q: %w", newPath.String(), err)
	}
	if g.ensureWatch != nil {
		g.ensureWatch(parent)
	}

	if err := os.Rename(absOld, absNew); err != nil {
		g.mu.Unlock()
		return "", mapOSError(err)
	}

	// Snapshot dependents before dbMove renames these very rows out from
	// under p's old (dir, name) identity below.
	knowledgeDependents, _ := dbGetSourceKnowledges(g.db, p)
	indexDependents, _ := dbGetKnowledgeIndexes(g.db, p)

	if dbErr := dbMove(g.db, p, nd.String()); dbErr != nil {
		// Best-effort rollback: the file move must match the DB, or the note
		// becomes unreadable at both its old and new path.
		_ = os.Rename(absNew, absOld)
		g.mu.Unlock()
		return "", fmt.Errorf("vault: metadata move %q: %w", p.String(), dbErr)
	}

	g.mu.Unlock()

	g.rewriteDependentPathRefs(knowledgeDependents, indexDependents, p, newPath)

	g.emit("vault_delete", p.String(), false, time.Now())
	g.emit("vault_create", newPath.String(), false, time.Now())
	return newPath, nil
}

// rewriteDependentPathRefs fixes up literal old-path references left behind
// in notes that depend on a just-moved note — see MoveNote's comment.
// Best-effort: a read/write failure on one dependent just leaves its
// reference stale (same as if MoveNote had never tried), it doesn't fail the
// move, which has already committed by the time this runs.
func (g *Vault) rewriteDependentPathRefs(knowledgeDependents, indexDependents []Path, oldPath, newPath Path) {
	old, next := oldPath.String(), newPath.String()
	for _, kp := range knowledgeDependents {
		raw, err := g.ReadNote(kp)
		if err != nil {
			continue
		}
		if out, changed := util.RewriteFrontmatterPathRef(raw, old, next); changed {
			_ = g.WriteNote(kp, out, "")
		}
	}
	for _, ip := range indexDependents {
		raw, err := g.ReadNote(ip)
		if err != nil {
			continue
		}
		if out, changed := util.RewriteTableRowPathRef(raw, old, next); changed {
			_ = g.WriteNote(ip, out, "")
		}
	}
}

// NameTaken reports whether newName is already used by some note in the
// vault. It backs a purely advisory frontend precheck (an inline "name
// already exists" hint while the user is still typing) — it is NOT the
// authoritative guard. RenameNote re-checks this itself, atomically with the
// actual rename and right before touching the filesystem, so a stale,
// skipped, or raced call to NameTaken can never let a colliding rename
// through.
func (g *Vault) NameTaken(newName string) (bool, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return dbNameTaken(g.db, newName)
}

// RenameNote renames the note at p to newName in place (use MoveNote for a
// directory change). Returns the new path and the id of the RenameJob
// enqueued for the vault-wide reference sweep — valid once err is nil, even
// though that sweep hasn't run yet (see internal/plugins/renamesync).
//
// The destination check is vault-wide, not scoped to p's dir: wikilinks
// resolve by name only (util.ExtractWikilinkNames), so filenames must stay
// unique across the whole vault or links become ambiguous.
//
// ErrRenameNotAllowed for a system-managed note (non-empty kind, or an
// underscore-prefixed directory — see dirHasUnderscoreSegment): those assume
// a stable filename elsewhere (AppendShort's daily naming, compile's own
// regeneration), so a bad rename is destructive in a way a bad move isn't.
//
// dbRename cascades relation tables the same way dbMove does, and reuses
// rewriteDependentPathRefs for direct dependents' literal full-path refs.
// What neither covers is every OTHER note that refers to p by name alone —
// a rename, unlike a move, changes the string those resolve against — so
// that vault-wide sweep is deferred to the async RenameJob this enqueues.
func (g *Vault) RenameNote(p Path, newName string) (Path, int64, error) {
	if err := validateNoteName(p.Base()); err != nil {
		return "", 0, err
	}
	if err := validateNoteName(newName); err != nil {
		return "", 0, err
	}
	if strings.ContainsRune(newName, '/') {
		return "", 0, ErrInvalidPath
	}
	newPath := Path(JoinPath(p.Dir(), newName))
	if newPath == p {
		return p, 0, nil
	}

	g.mu.Lock()

	existing, err := dbGet(g.db, p)
	if err != nil {
		g.mu.Unlock()
		return "", 0, err
	}
	if existing.Kind != "" || dirHasUnderscoreSegment(p.Dir()) {
		g.mu.Unlock()
		return "", 0, ErrRenameNotAllowed
	}

	absOld, err := g.osPath(p)
	if err != nil {
		g.mu.Unlock()
		return "", 0, err
	}
	absNew, err := g.osPath(newPath)
	if err != nil {
		g.mu.Unlock()
		return "", 0, err
	}
	if newInfo, statErr := os.Stat(absNew); statErr == nil {
		// A case-insensitive filesystem (macOS's default APFS, Windows) also
		// resolves absNew for a pure case change (note.md -> Note.md) to p
		// itself — os.SameFile tells that apart from an actual collision.
		if oldInfo, oldErr := os.Stat(absOld); oldErr != nil || !os.SameFile(oldInfo, newInfo) {
			g.mu.Unlock()
			return "", 0, ErrAlreadyExists
		}
	} else if !errors.Is(statErr, fs.ErrNotExist) {
		g.mu.Unlock()
		return "", 0, mapOSError(statErr)
	}

	// Check vault-wide name uniqueness before touching the filesystem: a
	// rejected rename must never move the file. dbRename repeats this check
	// inside its own transaction as the authoritative guard; this is just to
	// avoid doing (and then having to undo) an os.Rename for the common case.
	if taken, err := dbNameTaken(g.db, newName); err != nil {
		g.mu.Unlock()
		return "", 0, err
	} else if taken {
		g.mu.Unlock()
		return "", 0, ErrAlreadyExists
	}

	if err := os.Rename(absOld, absNew); err != nil {
		g.mu.Unlock()
		return "", 0, mapOSError(err)
	}

	// Snapshot dependents before dbRename renames these very rows out from
	// under p's old (dir, name) identity below.
	knowledgeDependents, _ := dbGetSourceKnowledges(g.db, p)
	indexDependents, _ := dbGetKnowledgeIndexes(g.db, p)

	if dbErr := dbRename(g.db, p, newName); dbErr != nil {
		// The precheck above means this should only fire on a genuine DB
		// error (not ErrAlreadyExists, unless another goroutine somehow
		// raced past the precheck). Roll the file back to match; if that
		// itself fails, say so explicitly instead of swallowing it — the
		// vault is left inconsistent (file at absNew, metadata still at p)
		// and needs a human, not a silent best-effort.
		if rbErr := os.Rename(absNew, absOld); rbErr != nil {
			g.mu.Unlock()
			return "", 0, fmt.Errorf("vault: metadata rename %q: %w (and rollback of the file rename also failed: %v — file is now at %q but metadata still refers to %q)", p.String(), dbErr, rbErr, absNew, p.String())
		}
		g.mu.Unlock()
		return "", 0, fmt.Errorf("vault: metadata rename %q: %w", p.String(), dbErr)
	}

	// The rename itself has already committed above; a failure to enqueue the
	// sweep job only means the vault-wide reference cleanup won't run for this
	// rename (best-effort, same spirit as the wikilink/table rewrites below) —
	// it must not undo or fail the rename. jobID is 0 in that case.
	jobID, _ := dbEnqueueRenameJob(g.db, p.Dir(), p.Base(), newName)

	g.mu.Unlock()

	g.rewriteDependentPathRefs(knowledgeDependents, indexDependents, p, newPath)

	g.emit("vault_delete", p.String(), false, time.Now())
	g.emit("vault_create", newPath.String(), false, time.Now())
	g.emitEvent(plugin.Event{
		Type: plugin.EventVaultRename, Path: newPath.String(), OldPath: p.String(), Time: time.Now(),
	})
	return newPath, jobID, nil
}

// ── rename job hooks (used by internal/plugins/renamesync) ────────────────────
// RenameNote enqueues the job; these let the sweep plugin claim/progress/
// finish it without touching SQL directly (same convention as every plugin).

// PendingRenameJobs returns every RenameJob still 'pending' or 'running',
// oldest first — safe to call repeatedly since re-running a finished sweep is a no-op.
func (g *Vault) PendingRenameJobs() ([]RenameJob, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return dbPendingRenameJobs(g.db)
}

// GetRenameJob returns a single RenameJob by id, for the HTTP status-polling endpoint.
func (g *Vault) GetRenameJob(id int64) (RenameJob, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return dbGetRenameJob(g.db, id)
}

// StartRenameJob marks a job 'running' and records the total note count the sweep will walk.
func (g *Vault) StartRenameJob(id int64, total int) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return dbStartRenameJob(g.db, id, total)
}

// UpdateRenameJobProgress records how many notes the sweep has walked (done)
// and how many it actually rewrote (updatedCount) so far. Call periodically,
// not per-note, to avoid hammering the metadata DB.
func (g *Vault) UpdateRenameJobProgress(id int64, done, updatedCount int) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return dbUpdateRenameJobProgress(g.db, id, done, updatedCount)
}

// FinishRenameJob marks a job 'done' with its final counts.
func (g *Vault) FinishRenameJob(id int64, total, updatedCount int) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return dbFinishRenameJob(g.db, id, total, updatedCount)
}

// FailRenameJob marks a job 'failed' with an error message. A failed job is
// not returned by PendingRenameJobs again, so a single poison job (e.g. one
// whose ListAllNotes call keeps erroring) can't loop the process forever.
func (g *Vault) FailRenameJob(id int64, errMsg string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return dbFailRenameJob(g.db, id, errMsg)
}

// AppendShort appends content as a new short note entry to today's daily file
// inside shortsDir (defaults to ShortDir when empty). The file is named
// YYYY-MM-DD.md and lives at /<shortsDir>/YYYY-MM-DD.md.
//
// Each entry is prefixed with a sequence heading ("## Short Note N") and a
// creation timestamp. Entries are separated by "---". Any "---" lines present
// in the incoming content are replaced with blank lines so they cannot break
// the entry structure.
//
// The note is stored with origin "short" so it can be filtered distinctly.
// Returns the vault path of the daily file and the note's metadata.
func (g *Vault) AppendShort(content []byte, shortsDir string) (Path, Note, error) {
	if !util.IsValidText(content) {
		return "", Note{}, ErrBinaryContent
	}
	if shortsDir == "" {
		shortsDir = g.shortsDir
	}

	sanitized := shortSanitize(content)

	now := time.Now()
	p := Path("/" + shortsDir + "/" + now.Format("2006-01-02") + ".md")
	timestamp := now.Format("2006-01-02 15:04:05")

	g.mu.Lock()

	existing, err := g.readNoteLocked(p)
	if err != nil && !errors.Is(err, ErrNotFound) {
		g.mu.Unlock()
		return "", Note{}, err
	}

	entry := fmt.Sprintf("###### Short Note: %s\n\n%s", timestamp, strings.TrimSpace(string(sanitized)))

	// Strip any frontmatter (or absence thereof) to get the raw entry body,
	// then always reconstruct with a fresh frontmatter header.
	_, existingBody := util.ParseFrontmatter(existing)
	rawBody := strings.TrimSpace(string(existingBody))

	fm := util.FormatShortFrontmatter(now)
	var merged string
	if rawBody != "" {
		merged = fm + rawBody + "\n\n---\n\n" + entry
	} else {
		merged = fm + entry
	}

	isNew, modTime, writeErr := g.writeNoteLocked(p, []byte(merged), Note{Kind: KindShort})
	g.mu.Unlock()
	if writeErr != nil {
		return "", Note{}, writeErr
	}

	if isNew {
		g.emit("vault_create", p.String(), false, modTime)
	} else {
		g.emit("vault_write", p.String(), false, modTime)
	}
	g.emitEvent(plugin.Event{
		Type:    plugin.EventVaultShortAppend,
		Path:    p.String(),
		Time:    modTime,
		Content: strings.TrimSpace(string(sanitized)),
	})

	g.mu.RLock()
	note, dbErr := dbGet(g.db, p)
	g.mu.RUnlock()
	if dbErr != nil {
		return p, Note{}, dbErr
	}
	return p, note, nil
}

// shortSanitize replaces any line that is exactly "---" with a blank line so
// that user content cannot accidentally inject an entry separator.
func shortSanitize(content []byte) []byte {
	lines := strings.Split(string(content), "\n")
	for i, l := range lines {
		if strings.TrimSpace(l) == "---" {
			lines[i] = ""
		}
	}
	return []byte(strings.Join(lines, "\n"))
}

// shortEntryDelimRe matches any markdown thematic-break line (--- or ***)
// used as an entry separator, with optional surrounding whitespace / CRLF.
var shortEntryDelimRe = regexp.MustCompile(`\r?\n(?:---|\*\*\*)\s*\r?\n`)

// ParseShortEntries parses the raw content of a short daily file and returns
// individual entries, newest first. DailyPath is set on every returned entry.
func ParseShortEntries(content []byte, dailyPath string) []ShortEntry {
	raw := strings.TrimSpace(string(content))
	if raw == "" {
		return nil
	}
	// Strip YAML frontmatter (kind: short daily files carry it).
	if fm, body := util.ParseFrontmatter([]byte(raw)); fm.HasMeta() {
		raw = strings.TrimSpace(string(body))
		if raw == "" {
			return nil
		}
	}
	// Split on any markdown thematic-break separator (--- or ***) with optional
	// surrounding whitespace, to tolerate the current "\n\n---\n\n" format, older
	// single-newline variants, and files that used "***" as the delimiter.
	chunks := shortEntryDelimRe.Split(raw, -1)
	entries := make([]ShortEntry, 0, len(chunks))
	for _, chunk := range chunks {
		chunk = strings.TrimSpace(chunk)
		if chunk == "" {
			continue
		}
		lines := strings.SplitN(chunk, "\n", 3)
		const prefix = "###### Short Note: "
		if !strings.HasPrefix(lines[0], prefix) {
			continue
		}
		t, err := time.ParseInLocation("2006-01-02 15:04:05", strings.TrimPrefix(lines[0], prefix), time.Local)
		if err != nil {
			continue
		}
		body := ""
		if len(lines) >= 3 {
			body = strings.TrimSpace(lines[2])
		}
		entries = append(entries, ShortEntry{Content: body, CreatedAt: t, DailyPath: dailyPath})
	}
	// Reverse to return newest-first (file stores oldest-first).
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}
	return entries
}

// ListShortEntries returns individual short note entries, newest first, by
// reading and parsing the daily aggregation files in the shorts directory.
func (g *Vault) ListShortEntries(opts ShortListOptions) ([]ShortEntry, error) {
	d := strings.Trim(opts.Dir, "/")
	if d == "" {
		d = g.shortsDir
	}
	listOpts := ListOptions{
		OnlyKinds:  []Kind{KindShort},
		SortByTime: true,
		After:      opts.After,
		Before:     opts.Before,
	}
	g.mu.RLock()
	dailyNotes, err := dbListByDir(g.db, "/"+d, listOpts)
	g.mu.RUnlock()
	if err != nil {
		return nil, err
	}

	var all []ShortEntry
	for _, daily := range dailyNotes {
		g.mu.RLock()
		content, readErr := g.readNoteLocked(daily.Path())
		g.mu.RUnlock()
		if readErr != nil {
			continue
		}
		for _, e := range ParseShortEntries(content, daily.PathString()) {
			if !opts.After.IsZero() && e.CreatedAt.Before(opts.After) {
				continue
			}
			if !opts.Before.IsZero() && !e.CreatedAt.Before(opts.Before) {
				continue
			}
			all = append(all, e)
			if opts.Limit > 0 && len(all) >= opts.Limit {
				return all, nil
			}
		}
	}
	return all, nil
}

func (g *Vault) MarkNoteIndexed(p Path) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return dbMarkIndexed(g.db, p)
}

// ResetAllIndexed sets indexed = 0 for every note in the metadata database.
// Call this before deleting and rebuilding the search index so that the next
// Backfill re-processes every note with the new index schema.
func (g *Vault) ResetAllIndexed() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return dbResetAllIndexed(g.db)
}

// ── dir operations ────────────────────────────────────────────────────────────

func (g *Vault) StatDir(p Path) (Dir, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	abs, err := g.osPath(p)
	if err != nil {
		return Dir{}, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return Dir{}, mapOSError(err)
	}
	if !info.IsDir() {
		return Dir{}, ErrIsFile
	}
	return dirFromFileInfo(p.String(), info), nil
}

func (g *Vault) ListDir(p Path, opts ListOptions) ([]Note, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if _, err := g.osPath(p); err != nil {
		return nil, err
	}
	if err := checkListOpts(opts); err != nil {
		return nil, err
	}
	return dbListByDir(g.db, p.String(), opts)
}

// ListChildDirs returns the immediate subdirectory names under p as they appear
// on disk. Hidden directories (names starting with ".") are omitted. Ordering
// is lexicographic.
func (g *Vault) ListChildDirs(p Path) ([]string, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	abs, err := g.osPath(p)
	if err != nil {
		return nil, err
	}
	fi, statErr := os.Stat(abs)
	if statErr != nil {
		return nil, mapOSError(statErr)
	}
	if !fi.IsDir() {
		return nil, ErrIsFile
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return nil, mapOSError(err)
	}

	var dirs []string
	for _, ent := range entries {
		if !ent.IsDir() {
			continue
		}
		name := ent.Name()
		if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
			continue
		}
		dirs = append(dirs, name)
	}
	sort.Strings(dirs)
	return dirs, nil
}

// CountNotes returns the total number of notes tracked in the metadata database.
func (g *Vault) CountNotes() (int, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return dbCount(g.db)
}

// CountKnowledgeNotes returns the total number of compiled knowledge notes.
func (g *Vault) CountKnowledgeNotes() (int, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return dbCountByKind(g.db, string(KindKnowledge), "")
}

// CountRawNotes returns the number of non-knowledge notes in the vault.
func (g *Vault) CountRawNotes() (int, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return dbCountByKind(g.db, "", string(KindKnowledge))
}

// CountShortDays returns the number of short daily aggregation files (one per calendar day).
func (g *Vault) CountShortDays() (int, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return dbCountByKind(g.db, string(KindShort), "")
}

// ListAllNotes returns every note in the vault regardless of directory.
// Intended for vault-wide operations such as the search index backfill.
func (g *Vault) ListAllNotes(opts ListOptions) ([]Note, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if err := checkListOpts(opts); err != nil {
		return nil, err
	}
	return dbListAll(g.db, opts)
}

func checkListOpts(opts ListOptions) error {
	if len(opts.OnlyKinds) > 0 && len(opts.ExcludeKinds) > 0 {
		return fmt.Errorf("storage: OnlyKinds and ExcludeKinds cannot both be set")
	}
	return nil
}

// GetNotesByNames returns metadata for every note whose filename matches any
// entry in names. Results are ordered by updated_at descending.
func (g *Vault) GetNotesByNames(names []string) ([]Note, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return dbGetByNames(g.db, names)
}

// GetNotesByName returns metadata for every note whose filename (name column) equals
// the given base name. Name must satisfy validateNoteName (e.g. "today.md").
// Results are ordered by updated_at descending, then dir ascending.
func (g *Vault) GetNotesByName(name string) ([]Note, error) {
	if err := validateNoteName(name); err != nil {
		return nil, err
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	return dbGetByName(g.db, name)
}

// SetNotePinned sets or clears the pinned flag for the note at p.
func (g *Vault) SetNotePinned(p Path, pinned bool) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, err := dbGet(g.db, p); err != nil {
		return err
	}
	return dbSetPinned(g.db, p, pinned)
}

// ListPinnedNotes returns all pinned notes ordered by updated_at descending.
func (g *Vault) ListPinnedNotes() ([]Note, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return dbListPinned(g.db)
}

// ListDirNotes returns notes whose dir column exactly matches dir, queried from
// the metadata DB only (no filesystem check). Use for dirs that are known to
// exist in the DB (e.g. returned by ListDirs). dir should be "/" for the vault
// root or begin with "/" for sub-directories.
func (g *Vault) ListDirNotes(dir string, opts ListOptions) ([]Note, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if err := checkListOpts(opts); err != nil {
		return nil, err
	}
	return dbListByDir(g.db, dir, opts)
}

// ListDirs returns distinct directories that contain notes, sorted alphabetically.
// Directories are read from the DB (not the filesystem). The vault root is "/".
// Any directory whose path contains an underscore-prefixed segment (e.g. "/_shorts",
// "/_clips/2026") is excluded, along with all its children.
func (g *Vault) ListDirs() ([]DirSummary, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	all, err := dbListDirs(g.db)
	if err != nil {
		return nil, err
	}
	out := all[:0]
	for _, d := range all {
		if !dirHasUnderscoreSegment(d.Dir) {
			out = append(out, d)
		}
	}
	return out, nil
}

// ListAllDirs is like ListDirs but includes underscore-prefixed system directories.
func (g *Vault) ListAllDirs() ([]DirSummary, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return dbListDirs(g.db)
}

// dirHasUnderscoreSegment reports whether any path segment in dir starts with "_".
func dirHasUnderscoreSegment(dir string) bool {
	for _, seg := range strings.Split(dir, "/") {
		if strings.HasPrefix(seg, "_") {
			return true
		}
	}
	return false
}

// ListRecentShortNotes returns metadata for the most recently created short
// notes (kind "short"), ordered by created_at descending.
// When limit <= 0, a default of 7 is used.
func (g *Vault) ListRecentShortNotes(limit int) ([]Note, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return dbListRecentByKindByCreatedDesc(g.db, KindShort, limit)
}

// ErrShortDailyWrongKind means a note exists at the expected short path but
// metadata kind is not "short".
var ErrShortDailyWrongKind = errors.New("storage: note at short path does not have short kind")

// ShortDailyVaultPath returns the vault-absolute path for the aggregated short file
// for local calendar date day under shortsDir (vault-relative without leading slash;
// empty uses ShortDir).
func ShortDailyVaultPath(shortsDir string, day time.Time) (Path, error) {
	d := strings.Trim(shortsDir, "/")
	if d == "" {
		d = ShortDir
	}
	base := day.Format("2006-01-02") + ".md"
	sp, ok := ParsePath(JoinPath("/"+d, base))
	if !ok {
		return "", fmt.Errorf("storage: invalid shorts path")
	}
	return sp, nil
}

// GetShortDailyNote returns metadata for /{shortsDir}/YYYY-MM-DD.md when the DB row
// exists and origin is "short".
func (g *Vault) GetShortDailyNote(shortsDir string, day time.Time) (Note, error) {
	sp, err := ShortDailyVaultPath(shortsDir, day)
	if err != nil {
		return Note{}, err
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	n, err := dbGet(g.db, sp)
	if err != nil {
		return Note{}, err
	}
	if n.Kind != KindShort {
		return Note{}, ErrShortDailyWrongKind
	}
	return n, nil
}

// ── vault initialisation ──────────────────────────────────────────────────────

// ScanAndRegister walks the vault root recursively and upserts a metadata row
// for every markdown file found on disk.
// Hidden directories (any component beginning with ".") are skipped, which
// excludes .hylo and editor temp files.
// Returns the number of files registered (newly inserted or updated).
func (g *Vault) ScanAndRegister() (int, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	count := 0
	err := filepath.WalkDir(g.root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil // skip inaccessible entries
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !util.IsMarkdownPath(d.Name()) {
			return nil
		}
		rel, relErr := filepath.Rel(g.root, path)
		if relErr != nil {
			return nil
		}
		slashRel := "/" + filepath.ToSlash(rel)
		dir, name := PathParts(slashRel)
		info, infoErr := d.Info()
		if infoErr != nil {
			return nil
		}
		if dbErr := dbUpsert(g.db, Note{
			Dir:       dir,
			Name:      name,
			Size:      info.Size(),
			UpdatedAt: info.ModTime(),
			Kind:      "",
		}); dbErr != nil {
			return fmt.Errorf("storage: register %q: %w", slashRel, dbErr)
		}
		count++
		return nil
	})
	return count, err
}

// ScanAndRegisterFull is the init-time variant of ScanAndRegister.
// It always starts from a clean slate (clears the notes table) and correctly
// distinguishes raw notes from knowledge notes:
//
//   - Notes whose vault path is under knowledgeOutputDir and whose YAML frontmatter
//     contains a "compiled:" field are inserted with origin "plugin:compile".
//
//   - All other notes are inserted with origin "fs".
//
// knowledgeOutputDir is normalised to a vault-absolute slash path (leading "/"
// is added when absent). An empty string defaults to "/_knowledge".
func (g *Vault) ScanAndRegisterFull(knowledgeOutputDir string) (int, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Normalise the knowledge output directory.
	if knowledgeOutputDir == "" {
		knowledgeOutputDir = "/_knowledge"
	}
	if !strings.HasPrefix(knowledgeOutputDir, "/") {
		knowledgeOutputDir = "/" + knowledgeOutputDir
	}
	distillPrefix := knowledgeOutputDir + "/"

	// Clear all existing rows so init is always a clean rebuild.
	if err := dbClearAll(g.db); err != nil {
		return 0, fmt.Errorf("storage: clear notes table: %w", err)
	}
	if _, err := g.db.Exec(`DELETE FROM knowledge_deps`); err != nil {
		return 0, fmt.Errorf("storage: clear knowledge_deps: %w", err)
	}
	if _, err := g.db.Exec(`DELETE FROM index_deps`); err != nil {
		return 0, fmt.Errorf("storage: clear index_deps: %w", err)
	}
	if _, err := g.db.Exec(`DELETE FROM knowledge_links`); err != nil {
		return 0, fmt.Errorf("storage: clear knowledge_links: %w", err)
	}

	// kDep records a knowledge note's source_notes list for post-walk dep resolution.
	type kDep struct {
		path        Path
		sourceNotes []string
	}
	var depsToRecord []kDep

	// iDep records an index note's parsed body for post-walk index_deps population.
	type iDep struct {
		path Path
		body []byte
	}
	var indexDepsToRecord []iDep

	// kLink records a knowledge note's wikilink targets for post-walk K→K edge resolution.
	type kLink struct {
		path          Path
		entityType    string
		wikilinkNames []string
	}
	var linksToRecord []kLink

	count := 0
	err := filepath.WalkDir(g.root, func(absPath string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !util.IsMarkdownPath(d.Name()) {
			return nil
		}

		rel, relErr := filepath.Rel(g.root, absPath)
		if relErr != nil {
			return nil
		}
		slashRel := "/" + filepath.ToSlash(rel)
		dir, name := PathParts(slashRel)

		info, infoErr := d.Info()
		if infoErr != nil {
			return nil
		}

		n := Note{
			Dir:       dir,
			Name:      name,
			Size:      info.Size(),
			UpdatedAt: info.ModTime(),
			CreatedAt: info.ModTime(),
			Kind:      "",
		}

		// Read once and reuse for kind detection below and for preview — every
		// other registration path (WriteNote, the fs watcher) already computes
		// preview from the content it just touched, and a full rescan is the
		// one path that previously never read plain notes' content at all, so
		// they came out of it with an empty preview until individually re-saved.
		data, readErr := os.ReadFile(absPath)

		// Detect knowledge notes by path prefix and `kind: knowledge` frontmatter,
		// matching the runtime check in the compile plugin.
		isUnderKnowledgeDir := slashRel == knowledgeOutputDir ||
			strings.HasPrefix(slashRel, distillPrefix)
		if isUnderKnowledgeDir && readErr == nil {
			fm, body := util.ParseFrontmatter(data)
			if fm.HasMeta() {
				var kind, title, domain, entityType string
				var compileCount int
				var srcNotes []string
				for _, e := range fm.All {
					switch e.Key {
					case "kind":
						kind = e.Value
					case "title":
						title = e.Value
					case "domain":
						domain = e.Value
					case "compile_count":
						if cv, err := strconv.Atoi(strings.TrimSpace(e.Value)); err == nil {
							compileCount = cv
						}
					case "source_notes":
						srcNotes = e.List
					case "entity_type":
						entityType = strings.TrimSpace(e.Value)
					}
				}
				switch {
				case strings.EqualFold(kind, "knowledge"):
					n.Kind = KindKnowledge
					n.Title = title
					n.CompileCount = compileCount
					n.Tags = fm.Tags
					if len(srcNotes) > 0 {
						depsToRecord = append(depsToRecord, kDep{
							path:        Path(slashRel),
							sourceNotes: srcNotes,
						})
					}
					// Collect wikilink targets for K→K edge resolution after walk.
					if wlNames := util.ExtractWikilinkNames(body); len(wlNames) > 0 {
						linksToRecord = append(linksToRecord, kLink{
							path:          Path(slashRel),
							entityType:    entityType,
							wikilinkNames: wlNames,
						})
					}
				case strings.EqualFold(kind, "index"):
					n.Kind = KindIndex
					n.Title = domain
					n.Tags = fm.Tags
					indexDepsToRecord = append(indexDepsToRecord, iDep{
						path: Path(slashRel),
						body: body,
					})
				}
			}
		}

		// Detect short note daily files by `kind: short` frontmatter under the
		// configured shorts directory.
		shortsVaultPrefix := "/" + strings.Trim(g.shortsDir, "/") + "/"
		if n.Kind == "" && strings.HasPrefix(slashRel, shortsVaultPrefix) {
			if readErr == nil && util.IsShortNote(data) {
				n.Kind = KindShort
			}
		}

		if readErr == nil {
			n.Preview = notePreviewFromSummary(util.GeneratePreview(data, 0))
		}

		if dbErr := dbInsertFull(g.db, n); dbErr != nil {
			return fmt.Errorf("storage: register %q: %w", slashRel, dbErr)
		}
		count++
		return nil
	})
	if err != nil {
		return count, err
	}

	// Populate knowledge_deps from source_notes frontmatter of knowledge notes.
	// Mirrors syncSourceNotes in the compile plugin. Entries with "/" are treated
	// as vault paths; plain names are batch-looked up (ambiguous matches skipped).
	if len(depsToRecord) > 0 {
		var allNames []string
		for _, dep := range depsToRecord {
			for _, sn := range dep.sourceNotes {
				if !strings.Contains(sn, "/") {
					name := sn
					if !strings.HasSuffix(strings.ToLower(name), ".md") {
						name += ".md"
					}
					allNames = append(allNames, name)
				}
			}
		}
		byName := make(map[string][]Note)
		if len(allNames) > 0 {
			if notes, err := dbGetByNames(g.db, allNames); err == nil {
				for _, n := range notes {
					byName[n.Name] = append(byName[n.Name], n)
				}
			}
		}
		for _, dep := range depsToRecord {
			var resolved []Path
			for _, sn := range dep.sourceNotes {
				if strings.Contains(sn, "/") {
					if sp, ok := ParsePath(sn); ok {
						resolved = append(resolved, sp)
					}
				} else {
					name := sn
					if !strings.HasSuffix(strings.ToLower(name), ".md") {
						name += ".md"
					}
					if matches := byName[name]; len(matches) == 1 {
						resolved = append(resolved, matches[0].Path())
					}
				}
			}
			if len(resolved) > 0 {
				_ = dbReplaceKnowledgeDeps(g.db, dep.path, resolved)
			}
		}
	}

	// Populate index_deps by parsing the markdown table in each index note body.
	for _, dep := range indexDepsToRecord {
		var resolved []Path
		for _, kp := range scanIndexTablePaths(dep.body) {
			if sp, ok := ParsePath(kp); ok {
				resolved = append(resolved, sp)
			}
		}
		if len(resolved) > 0 {
			_ = dbReplaceIndexDeps(g.db, dep.path, resolved)
		}
	}

	// Populate knowledge_links from wikilinks in knowledge note bodies.
	// Only edges pointing at other knowledge notes are recorded.
	if len(linksToRecord) > 0 {
		// Batch-collect all referenced names (deduped).
		seenName := make(map[string]bool)
		var allLinkNames []string
		for _, kl := range linksToRecord {
			for _, name := range kl.wikilinkNames {
				if !seenName[name] {
					seenName[name] = true
					allLinkNames = append(allLinkNames, name)
				}
			}
		}
		// Resolve names → knowledge note paths.
		knowledgeByName := make(map[string]Path)
		if len(allLinkNames) > 0 {
			if candidates, err := dbGetByNames(g.db, allLinkNames); err == nil {
				for _, n := range candidates {
					if n.Kind == KindKnowledge {
						knowledgeByName[n.Name] = n.Path()
					}
				}
			}
		}
		for _, kl := range linksToRecord {
			var targets []Path
			for _, name := range kl.wikilinkNames {
				if tp, ok := knowledgeByName[name]; ok && tp != kl.path {
					targets = append(targets, tp)
				}
			}
			if len(targets) > 0 {
				_ = dbReplaceKnowledgeLinks(g.db, kl.path, kl.entityType, targets)
			}
		}
	}

	return count, nil
}

// scanIndexTablePaths extracts vault-absolute knowledge note paths from the
// body of an index note. It looks at every markdown table row and treats the
// last non-empty cell as a path when it starts with "/" and ends with ".md".
func scanIndexTablePaths(body []byte) []string {
	var paths []string
	for _, line := range strings.Split(string(body), "\n") {
		s := strings.TrimSpace(line)
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

// ── internal helpers ──────────────────────────────────────────────────────────

func (g *Vault) readNoteLocked(p Path) ([]byte, error) {
	if err := validateNoteName(p.Base()); err != nil {
		return nil, err
	}
	abs, err := g.osPath(p)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, mapOSError(err)
	}
	return data, nil
}

// writeNoteLocked writes data to the note at p and syncs the metadata DB.
// meta carries the origin and any extra fields (Title) to persist
// atomically with the write. Dir, Name, Size, UpdatedAt are always derived from
// p and the file stat and do not need to be set by the caller.
// Returns isNew=true when the file did not exist on disk before this call, and
// modTime set to the file's actual modification time after the write.
// Must be called with g.mu held for writing.
func (g *Vault) writeNoteLocked(p Path, data []byte, meta Note) (isNew bool, modTime time.Time, err error) {
	if err = validateNoteName(p.Base()); err != nil {
		return false, time.Time{}, err
	}
	if !util.IsValidText(data) {
		return false, time.Time{}, ErrBinaryContent
	}
	abs, absErr := g.osPath(p)
	if absErr != nil {
		return false, time.Time{}, absErr
	}

	// Probe before writing so callers can emit the correct vault event type.
	_, statErr := os.Stat(abs)
	isNew = errors.Is(statErr, fs.ErrNotExist)

	parent := filepath.Dir(abs)
	if err = os.MkdirAll(parent, 0o750); err != nil {
		return false, time.Time{}, fmt.Errorf("vault: mkdir for %q: %w", p.String(), err)
	}
	if g.ensureWatch != nil {
		g.ensureWatch(parent)
	}

	// ── Phase 1: write filesystem ─────────────────────────────────────────────
	if err = os.WriteFile(abs, data, 0o644); err != nil {
		return false, time.Time{}, fmt.Errorf("vault: write %q: %w", p.String(), err)
	}

	// ── Phase 2: sync metadata ────────────────────────────────────────────────
	info, statErr2 := os.Stat(abs)
	if statErr2 != nil {
		return false, time.Time{}, fmt.Errorf("vault: stat after write %q: %w", p.String(), statErr2)
	}
	meta.Dir = p.Dir()
	meta.Name = p.Base()
	meta.Size = info.Size()
	meta.UpdatedAt = info.ModTime()
	if fm, _ := util.ParseFrontmatter(data); len(fm.Tags) > 0 {
		meta.Tags = fm.Tags
	}
	meta.Preview = notePreviewFromSummary(util.GeneratePreview(data, 0))
	if dbErr := dbUpsert(g.db, meta); dbErr != nil {
		return false, time.Time{}, fmt.Errorf("vault: metadata upsert %q: %w", p.String(), dbErr)
	}

	return isNew, info.ModTime(), nil
}

func validateNoteName(name string) error {
	if !util.IsMarkdownPath(name) {
		return fmt.Errorf("%w: %q", ErrUnsupportedType, name)
	}
	return nil
}

func (g *Vault) osPath(p Path) (string, error) {
	vaultPath := p.String()
	if !strings.HasPrefix(vaultPath, "/") {
		return "", fmt.Errorf("%w: %q is not a vault-absolute path (must start with '/')", ErrInvalidPath, vaultPath)
	}
	stripped := vaultPath[1:] // remove leading "/"
	joined := filepath.Join(g.root, filepath.FromSlash(stripped))
	if !strings.HasPrefix(joined, g.root+string(os.PathSeparator)) && joined != g.root {
		return "", ErrInvalidPath
	}
	return joined, nil
}

func mapOSError(err error) error {
	if errors.Is(err, fs.ErrNotExist) {
		return ErrFileNotFound
	}
	return err
}

func expandHome(p string) (string, error) {
	if !strings.HasPrefix(p, "~") {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, p[1:]), nil
}

func toSlash(p string) string { return filepath.ToSlash(p) }
