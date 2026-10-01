package storage

import (
	"io/fs"
	"path"
	"strings"
	"time"

	"github.com/hardhacker/hylo/internal/util"
)

// PathParts splits a vault-absolute path into (dir, name).
// All vault paths are slash-prefixed; the root directory is "/".
//
//	"/file.md"              → ("/", "file.md")
//	"/journal/2026/april.md" → ("/journal/2026", "april.md")
func PathParts(p string) (dir, name string) {
	idx := strings.LastIndex(p, "/")
	name = p[idx+1:]
	if idx == 0 {
		dir = "/"
	} else {
		dir = p[:idx]
	}
	return dir, name
}

// JoinPath joins a vault-absolute directory and a filename into a single path.
// It is the inverse of PathParts.
//
//	("/", "file.md")              → "/file.md"
//	("/journal/2026", "april.md") → "/journal/2026/april.md"
func JoinPath(dir, name string) string {
	if dir == "/" {
		return "/" + name
	}
	return dir + "/" + name
}

// Path represents a vault-absolute slash-prefixed path string.
// All vault paths must begin with "/". The vault root is represented as "/".
type Path string

// ParsePath cleans s and returns a Path if it begins with "/".
// The path is normalized with path.Clean before being returned, so
// traversals like ".." and redundant slashes are resolved:
//
//	"/journal/../secret.md" → "/secret.md"
//	"/journal//note.md"     → "/journal/note.md"
func ParsePath(s string) (Path, bool) {
	if !strings.HasPrefix(s, "/") {
		return "", false
	}
	return Path(path.Clean(s)), true
}

// String returns the full vault-absolute path string.
func (p Path) String() string {
	return string(p)
}

// Dir returns the vault-absolute path of the directory containing this path.
func (p Path) Dir() string {
	dir, _ := PathParts(string(p))
	return dir
}

// Base returns the final component of the path.
func (p Path) Base() string {
	_, name := PathParts(string(p))
	return name
}

// IsRoot reports whether the path represents the vault root.
func (p Path) IsRoot() bool {
	return p == "/"
}

// Kind classifies what a note is.
//
// Empty string ("") means an ordinary user note (created via API or filesystem scan).
// Named kinds are set by specific subsystems:
//   - KindShort     — short daily aggregation file
//   - KindKnowledge — knowledge note produced by the compile plugin
//   - KindIndex     — index note produced by the compile plugin
type Kind string

const (
	KindShort     Kind = "short"
	KindKnowledge Kind = "knowledge"
	KindIndex     Kind = "index"

	// ShortDir is the default vault directory for short notes.
	ShortDir = "_shorts"
)

// Note is the metadata for a single markdown file inside a Vault.
// It never carries content; use Vault.ReadNote for that.
type Note struct {
	Dir          string      `json:"dir"`  // vault-absolute directory path; "/" = vault root
	Name         string      `json:"name"` // base filename, e.g. "april.md"
	Size         int64       `json:"size,omitempty"`
	CreatedAt    time.Time   `json:"created_at,omitempty"`
	UpdatedAt    time.Time   `json:"updated_at"`
	Indexed      bool        `json:"indexed,omitempty"`
	Pinned       bool        `json:"pinned,omitempty"`
	CompileCount int         `json:"compile_count,omitempty"`
	Kind         Kind        `json:"kind,omitempty"`
	Title        string      `json:"title,omitempty"` // human-readable title; set by plugins (e.g. compile), empty for raw notes
	Tags         []string    `json:"tags,omitempty"`  // frontmatter tags
	Preview      NotePreview `json:"preview"`         // cached content summary, recomputed on every write
}

// NotePreview holds cached, cheap-to-render summary information about a
// note's content, refreshed on every write (see Vault.writeNoteLocked).
// It's stored as a single JSON object in the notes.preview column rather
// than one column per field, so more summary info (e.g. a detected cover
// color, estimated reading time) can be added later as a new field here,
// without another schema migration.
type NotePreview struct {
	Text      string `json:"text,omitempty"`       // short excerpt of the body; see util.GeneratePreview
	TodoTotal int    `json:"todo_total,omitempty"` // GFM task-list checkboxes anywhere in the note
	TodoDone  int    `json:"todo_done,omitempty"`  // how many of those are checked
	HasImage  bool   `json:"has_image,omitempty"`  // note contains at least one embedded image
	HasCode   bool   `json:"has_code,omitempty"`   // note contains at least one fenced/indented code block
}

// notePreviewFromSummary converts a util.GeneratePreview result into the
// persisted NotePreview shape. A small named conversion, rather than passing
// util.PreviewSummary around directly, keeps the notes.preview JSON shape
// (and its field names/tags) owned by storage rather than by util's parsing
// internals.
func notePreviewFromSummary(sum util.PreviewSummary) NotePreview {
	return NotePreview{
		Text:      sum.Text,
		TodoTotal: sum.TodoTotal,
		TodoDone:  sum.TodoDone,
		HasImage:  sum.HasImage,
		HasCode:   sum.HasCode,
	}
}

// Path returns the Note's full vault-absolute path as a Path object.
func (n Note) Path() Path {
	return Path(JoinPath(n.Dir, n.Name))
}

// PathString returns the Note's full vault-absolute path as a string.
func (n Note) PathString() string {
	return JoinPath(n.Dir, n.Name)
}

// Dir is the metadata for a directory inside a Vault.
// Each Dir is an independent storage unit; no parent/child relationships are
// modelled between directories — that structure exists only on the filesystem.
type Dir struct {
	Name string `json:"name"`
	Path string `json:"path"` // vault-relative slash path
}

// DirSummary holds a vault directory path and its note count, as returned by
// Vault.ListDirs. Dir is vault-absolute (e.g. "/", "/journal/2026").
type DirSummary struct {
	Dir   string `json:"dir"`
	Count int    `json:"count"`
}

// ListOptions controls ordering and pagination for Vault.ListDir.
type ListOptions struct {
	// SortByTime sorts results by modification time descending (newest first).
	// Default (false) sorts by name ascending.
	SortByTime bool

	// Limit caps the number of returned notes. 0 means no limit.
	Limit int

	// OnlyUnindexed filters the list to include only notes where indexed = 0.
	OnlyUnindexed bool

	// OnlyKinds, when non-empty, filters results to notes whose kind is in the set.
	// It cannot be set together with ExcludeKinds.
	OnlyKinds []Kind

	// ExcludeKinds, when non-empty, excludes notes whose kind is in the set.
	// It cannot be set together with OnlyKinds.
	ExcludeKinds []Kind

	// After and Before filter notes by updated_at. Zero values are ignored.
	// After is inclusive (updated_at >= After), Before is exclusive (updated_at < Before).
	After  time.Time
	Before time.Time
}

// RenameJobStatus is the lifecycle state of a RenameJob.
type RenameJobStatus string

const (
	RenameJobPending RenameJobStatus = "pending"
	RenameJobRunning RenameJobStatus = "running"
	RenameJobDone    RenameJobStatus = "done"
	RenameJobFailed  RenameJobStatus = "failed"
)

// RenameJob tracks the vault-wide wikilink/back-reference sweep that follows
// a Vault.RenameNote call. The rename itself (filesystem + metadata DB) is
// synchronous and already complete by the time a RenameJob exists; this
// record is purely for the slow part — rewriting every other note in the
// vault that refers to the renamed note by its old name.
//
// See internal/plugins/renamesync, which owns running the sweep and moving a
// job through these states.
type RenameJob struct {
	ID           int64
	Dir          string
	OldName      string
	NewName      string
	Status       RenameJobStatus
	Total        int
	Done         int
	UpdatedCount int
	Error        string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// ShortEntry is a single parsed entry from a short daily aggregation file.
type ShortEntry struct {
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	DailyPath string    `json:"daily_path"`
}

// ShortListOptions controls filtering for Vault.ListShortEntries.
type ShortListOptions struct {
	Dir    string    // defaults to ShortDir when empty
	After  time.Time // inclusive lower bound on entry created_at; zero = no bound
	Before time.Time // exclusive upper bound on entry created_at; zero = no bound
	Limit  int       // 0 = no limit
}

// dirFromFileInfo builds a Dir from an fs.FileInfo and its vault-relative path.
func dirFromFileInfo(relPath string, info fs.FileInfo) Dir {
	return Dir{
		Name: info.Name(),
		Path: relPath,
	}
}
