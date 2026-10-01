package storage

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestWriteNoteComputesPreview checks that every write path that funnels
// through writeNoteLocked (WriteNote here) fills in preview.text from the
// note's own content, with no separate call required.
func TestWriteNoteComputesPreview(t *testing.T) {
	v := newTestVault(t)

	p := Path("/note.md")
	if err := v.WriteNote(p, []byte("# Title\n\nThe actual excerpt."), ""); err != nil {
		t.Fatal(err)
	}

	n, err := v.StatNote(p)
	if err != nil {
		t.Fatal(err)
	}
	if n.Preview.Text != "The actual excerpt." {
		t.Fatalf("preview.text = %q, want %q", n.Preview.Text, "The actual excerpt.")
	}
}

// TestWriteNoteRefreshesPreviewOnOverwrite checks preview isn't a
// write-once value — it must track the latest content, matching autosave
// calling WriteNoteWithMeta repeatedly as the user types.
func TestWriteNoteRefreshesPreviewOnOverwrite(t *testing.T) {
	v := newTestVault(t)
	p := Path("/note.md")

	if err := v.WriteNote(p, []byte("first version"), ""); err != nil {
		t.Fatal(err)
	}
	if err := v.WriteNote(p, []byte("second, different version"), ""); err != nil {
		t.Fatal(err)
	}

	n, err := v.StatNote(p)
	if err != nil {
		t.Fatal(err)
	}
	if n.Preview.Text != "second, different version" {
		t.Fatalf("preview.text = %q, want the latest content", n.Preview.Text)
	}
}

// TestWriteNoteEmptyContentClearsPreview checks preview isn't "sticky" —
// once content is cleared out, the stale excerpt must not linger.
func TestWriteNoteEmptyContentClearsPreview(t *testing.T) {
	v := newTestVault(t)
	p := Path("/note.md")

	if err := v.WriteNote(p, []byte("some text"), ""); err != nil {
		t.Fatal(err)
	}
	if err := v.WriteNote(p, []byte(""), ""); err != nil {
		t.Fatal(err)
	}

	n, err := v.StatNote(p)
	if err != nil {
		t.Fatal(err)
	}
	if n.Preview.Text != "" {
		t.Fatalf("preview.text = %q, want empty after clearing content", n.Preview.Text)
	}
}

// TestOpenDBMigratesExistingV24Database simulates a vault created before the
// preview column existed: a meta.db at user_version 24 with a notes table
// that has no preview column and one pre-existing row. Opening it through
// the current code must add the column, preserve the row, and bump
// user_version — the exact scenario the migration framework exists for.
// There is no backfill step, so the pre-existing row is expected to keep
// the column's default ('{}', i.e. an empty NotePreview) until it's next
// written through the app.
func TestOpenDBMigratesExistingV24Database(t *testing.T) {
	root := t.TempDir()
	internal := filepath.Join(root, ".hylo")
	if err := os.MkdirAll(internal, 0o750); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(internal, "meta.db")

	seed, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	// The pre-migration (v24) shape: no preview column.
	if _, err := seed.Exec(`
		CREATE TABLE notes (
		    id            INTEGER PRIMARY KEY AUTOINCREMENT,
		    dir           TEXT    NOT NULL,
		    name          TEXT    NOT NULL,
		    size          INTEGER NOT NULL DEFAULT 0,
		    created_at    INTEGER NOT NULL,
		    updated_at    INTEGER NOT NULL,
		    indexed       INTEGER NOT NULL DEFAULT 0,
		    kind          TEXT    NOT NULL DEFAULT '',
		    title         TEXT    NOT NULL DEFAULT '',
		    pinned        INTEGER NOT NULL DEFAULT 0,
		    compile_count INTEGER NOT NULL DEFAULT 0,
		    tags          TEXT    NOT NULL DEFAULT '',
		    UNIQUE(dir, name)
		)`); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UnixNano()
	if _, err := seed.Exec(
		`INSERT INTO notes(dir, name, size, created_at, updated_at) VALUES ('/', 'old.md', 5, ?, ?)`,
		now, now,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := seed.Exec(`PRAGMA user_version = 24`); err != nil {
		t.Fatal(err)
	}
	if err := seed.Close(); err != nil {
		t.Fatal(err)
	}

	db, err := openDB(root)
	if err != nil {
		t.Fatalf("openDB on a pre-existing v24 database: %v", err)
	}
	defer db.Close()

	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != currentDBVersion {
		t.Fatalf("user_version = %d, want %d after migration", version, currentDBVersion)
	}

	var previewRaw, name string
	if err := db.QueryRow(`SELECT name, preview FROM notes WHERE dir = '/' AND name = 'old.md'`).
		Scan(&name, &previewRaw); err != nil {
		t.Fatalf("pre-existing row missing after migration: %v", err)
	}
	if name != "old.md" {
		t.Fatalf("pre-existing row's name changed to %q", name)
	}
	if got := unmarshalPreview(previewRaw); got.Text != "" {
		t.Fatalf("preview = %+v, want the migration's default (empty NotePreview), not backfilled", got)
	}
}

// TestOpenDBMigrationIsIdempotent checks that opening an up-to-date database
// twice in a row (as every normal app launch does) never re-runs
// ALTER TABLE against a column that's already there.
func TestOpenDBMigrationIsIdempotent(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".hylo"), 0o750); err != nil {
		t.Fatal(err)
	}

	db1, err := openDB(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := db1.Close(); err != nil {
		t.Fatal(err)
	}

	db2, err := openDB(root)
	if err != nil {
		t.Fatalf("second openDB on an already-current database: %v", err)
	}
	defer db2.Close()

	var version int
	if err := db2.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != currentDBVersion {
		t.Fatalf("user_version = %d, want %d", version, currentDBVersion)
	}
}

// TestScanAndRegisterFullComputesPreview mirrors `hylo init` (and the
// server's own auto-init on first launch) pointed at a directory that
// already has markdown files on disk before any Vault ever touched them:
// the files are written directly, bypassing WriteNote entirely, then
// registered via ScanAndRegisterFull. That's the one registration path that
// used to leave plain notes' preview empty (it never read their content at
// all) — this checks it now does.
func TestScanAndRegisterFullComputesPreview(t *testing.T) {
	root := t.TempDir()
	const body = "正文第一段，用来验证全量扫描（hylo init / 首次自动导入）时也会算出 preview。"
	if err := os.WriteFile(filepath.Join(root, "plain.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	v, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = v.Close() })

	if _, err := v.ScanAndRegisterFull(""); err != nil {
		t.Fatal(err)
	}

	n, err := v.StatNote(Path("/plain.md"))
	if err != nil {
		t.Fatal(err)
	}
	if n.Preview.Text == "" {
		t.Fatal("preview.text is empty after ScanAndRegisterFull; a full rescan must compute it for pre-existing files, not just for notes written through the app")
	}
}

// TestScanAndRegisterFullSkipsUnreadableFile checks a single file that
// fails to read (e.g. a permissions error, or removed mid-walk) doesn't
// abort the whole scan — it should register with an empty preview rather
// than failing every other note behind it in the walk.
func TestScanAndRegisterFullSkipsUnreadableFile(t *testing.T) {
	root := t.TempDir()
	unreadable := filepath.Join(root, "locked.md")
	if err := os.WriteFile(unreadable, []byte("secret"), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(unreadable, 0o644) }) // TempDir cleanup needs it readable again

	v, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = v.Close() })

	if _, err := v.ScanAndRegisterFull(""); err != nil {
		t.Fatal(err)
	}

	if _, err := v.StatNote(Path("/locked.md")); err != nil {
		t.Fatalf("unreadable file should still be registered (with an empty preview): %v", err)
	}
}

// TestBackfillPreviewsRecomputesFromDisk covers `hylo init --preview-only`:
// a note whose preview column is stale/empty (as any note registered before
// the preview feature existed would be) gets it recomputed from the file
// already sitting on disk.
func TestBackfillPreviewsRecomputesFromDisk(t *testing.T) {
	v := newTestVault(t)
	p := Path("/note.md")
	if err := v.WriteNote(p, []byte("正文内容，用来验证 backfill 能重新算出 preview。"), ""); err != nil {
		t.Fatal(err)
	}
	// Simulate a vault registered before the preview feature existed.
	if err := dbSetPreview(v.db, p, NotePreview{}); err != nil {
		t.Fatal(err)
	}

	count, err := v.BackfillPreviews()
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}

	n, err := v.StatNote(p)
	if err != nil {
		t.Fatal(err)
	}
	if n.Preview.Text == "" {
		t.Fatal("preview.text is still empty after BackfillPreviews")
	}
}

// TestBackfillPreviewsDoesNotTouchOtherColumns guards the reason
// BackfillPreviews goes through dbSetPreview (a plain UPDATE ... SET preview)
// instead of dbUpsert: dbUpsert always overwrites tags from whatever's
// passed in, so reusing it here without re-supplying every other column
// would silently wipe title/tags on every note in the vault.
func TestBackfillPreviewsDoesNotTouchOtherColumns(t *testing.T) {
	v := newTestVault(t)
	p := Path("/tagged.md")
	if err := v.WriteNote(p, []byte("---\ntags:\n  - a\n  - b\n---\n\n正文"), ""); err != nil {
		t.Fatal(err)
	}
	if err := v.SetNoteTitle(p, "自定义标题"); err != nil {
		t.Fatal(err)
	}

	if _, err := v.BackfillPreviews(); err != nil {
		t.Fatal(err)
	}

	n, err := v.StatNote(p)
	if err != nil {
		t.Fatal(err)
	}
	if n.Title != "自定义标题" {
		t.Fatalf("title = %q, want unchanged", n.Title)
	}
	if len(n.Tags) != 2 || n.Tags[0] != "a" || n.Tags[1] != "b" {
		t.Fatalf("tags = %v, want unchanged [a b]", n.Tags)
	}
}

// TestBackfillPreviewsSkipsUnreadableFile checks one unreadable note doesn't
// abort the batch — it's just left out of the count, same as
// ScanAndRegisterFull's handling of an unreadable file.
func TestBackfillPreviewsSkipsUnreadableFile(t *testing.T) {
	v := newTestVault(t)

	good := Path("/good.md")
	if err := v.WriteNote(good, []byte("可以正常读取的内容。"), ""); err != nil {
		t.Fatal(err)
	}

	bad := Path("/bad.md")
	if err := v.WriteNote(bad, []byte("will be locked"), ""); err != nil {
		t.Fatal(err)
	}
	absBad, err := v.osPath(bad)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(absBad, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(absBad, 0o644) })

	count, err := v.BackfillPreviews()
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1 (only the readable note)", count)
	}
}
