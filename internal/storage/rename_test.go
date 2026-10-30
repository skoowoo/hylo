package storage

import (
	"errors"
	"strings"
	"testing"
)

// TestRenameNoteUpdatesNameOnly renames a plain note and checks the dir,
// content, and DB row all land at the new name untouched otherwise.
func TestRenameNoteUpdatesNameOnly(t *testing.T) {
	v := newTestVault(t)

	p := Path("/journal/old.md")
	if err := v.WriteNote(p, []byte("hello"), ""); err != nil {
		t.Fatal(err)
	}

	newPath, jobID, err := v.RenameNote(p, "new.md")
	if err != nil {
		t.Fatal(err)
	}
	if newPath != Path("/journal/new.md") {
		t.Fatalf("newPath = %q, want /journal/new.md", newPath)
	}
	if jobID == 0 {
		t.Fatalf("expected a non-zero rename job id")
	}

	if _, err := v.StatNote(p); !errors.Is(err, ErrNotFound) {
		t.Fatalf("old path should be gone, got err = %v", err)
	}
	n, err := v.StatNote(newPath)
	if err != nil {
		t.Fatalf("new path should exist: %v", err)
	}
	if n.Dir != "/journal" || n.Name != "new.md" {
		t.Fatalf("unexpected note meta: %+v", n)
	}
	content, err := v.ReadNote(newPath)
	if err != nil || string(content) != "hello" {
		t.Fatalf("content = %q, err = %v", content, err)
	}

	job, err := v.GetRenameJob(jobID)
	if err != nil {
		t.Fatalf("GetRenameJob: %v", err)
	}
	if job.Status != RenameJobPending || job.Dir != "/journal" || job.OldName != "old.md" || job.NewName != "new.md" {
		t.Fatalf("unexpected job: %+v", job)
	}
}

func TestRenameNoteNoopSameName(t *testing.T) {
	v := newTestVault(t)
	p := Path("/note.md")
	if err := v.WriteNote(p, []byte("x"), ""); err != nil {
		t.Fatal(err)
	}
	newPath, jobID, err := v.RenameNote(p, "note.md")
	if err != nil {
		t.Fatal(err)
	}
	if newPath != p {
		t.Fatalf("newPath = %q, want unchanged %q", newPath, p)
	}
	if jobID != 0 {
		t.Fatalf("no-op rename should not enqueue a job, got id %d", jobID)
	}
}

// TestRenameNoteDestinationCollisionIsVaultWide differs from the move case:
// the destination check must reject a same-named note anywhere in the vault,
// not just one in the same directory (see dbRename's comment).
func TestRenameNoteDestinationCollisionIsVaultWide(t *testing.T) {
	v := newTestVault(t)
	if err := v.WriteNote("/a/old.md", []byte("a"), ""); err != nil {
		t.Fatal(err)
	}
	if err := v.WriteNote("/b/taken.md", []byte("b"), ""); err != nil {
		t.Fatal(err)
	}

	_, _, err := v.RenameNote("/a/old.md", "taken.md")
	if !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("err = %v, want ErrAlreadyExists", err)
	}
	if _, err := v.StatNote("/a/old.md"); err != nil {
		t.Fatalf("source should be untouched: %v", err)
	}
	content, err := v.ReadNote("/b/taken.md")
	if err != nil || string(content) != "b" {
		t.Fatalf("destination should be untouched: content=%q err=%v", content, err)
	}
}

// TestRenameNoteCaseOnlyChange covers a case-insensitive filesystem (macOS's
// default APFS, Windows): os.Stat(newPath) resolves to the same file as the
// old path there, and must not be mistaken for a real destination collision.
func TestRenameNoteCaseOnlyChange(t *testing.T) {
	v := newTestVault(t)
	p := Path("/journal/note.md")
	if err := v.WriteNote(p, []byte("hello"), ""); err != nil {
		t.Fatal(err)
	}

	newPath, _, err := v.RenameNote(p, "Note.md")
	if err != nil {
		t.Fatal(err)
	}
	if newPath != Path("/journal/Note.md") {
		t.Fatalf("newPath = %q, want /journal/Note.md", newPath)
	}
	n, err := v.StatNote(newPath)
	if err != nil || n.Name != "Note.md" {
		t.Fatalf("n = %+v, err = %v", n, err)
	}
	content, err := v.ReadNote(newPath)
	if err != nil || string(content) != "hello" {
		t.Fatalf("content = %q, err = %v", content, err)
	}
}

func TestRenameNoteRejectsSlashInNewName(t *testing.T) {
	v := newTestVault(t)
	if err := v.WriteNote("/note.md", []byte("x"), ""); err != nil {
		t.Fatal(err)
	}
	if _, _, err := v.RenameNote("/note.md", "sub/new.md"); !errors.Is(err, ErrInvalidPath) {
		t.Fatalf("err = %v, want ErrInvalidPath", err)
	}
}

// TestRenameNoteRejectsSystemKind covers the product rule: short/knowledge/
// index notes are self-managed elsewhere and must not be renamed.
func TestRenameNoteRejectsSystemKind(t *testing.T) {
	v := newTestVault(t)
	knowledge := Path("/_knowledge/k.md")
	if err := v.WriteNote(knowledge, []byte("---\nkind: knowledge\n---\n\nbody\n"), ""); err != nil {
		t.Fatal(err)
	}
	if err := v.MarkNoteAsKnowledge(knowledge, "", 0); err != nil {
		t.Fatal(err)
	}
	if _, _, err := v.RenameNote(knowledge, "renamed.md"); !errors.Is(err, ErrRenameNotAllowed) {
		t.Fatalf("err = %v, want ErrRenameNotAllowed", err)
	}
}

// TestRenameNoteRejectsUnderscoreDir covers notes living under a system
// directory (e.g. "/_shorts") even if their own kind happens to be empty.
func TestRenameNoteRejectsUnderscoreDir(t *testing.T) {
	v := newTestVault(t)
	p := Path("/_shorts/2026-01-01.md")
	if err := v.WriteNote(p, []byte("x"), ""); err != nil {
		t.Fatal(err)
	}
	if _, _, err := v.RenameNote(p, "renamed.md"); !errors.Is(err, ErrRenameNotAllowed) {
		t.Fatalf("err = %v, want ErrRenameNotAllowed", err)
	}
}

// TestRenameNoteRewritesKnowledgeSourceRef mirrors
// TestMoveNoteRewritesKnowledgeSourceRef: the dependent knowledge note's
// source_notes: frontmatter (a full path here) must follow the rename.
func TestRenameNoteRewritesKnowledgeSourceRef(t *testing.T) {
	v := newTestVault(t)

	raw := Path("/journal/old.md")
	knowledge := Path("/_knowledge/k.md")

	if err := v.WriteNote(raw, []byte("raw body"), ""); err != nil {
		t.Fatal(err)
	}
	kBody := "---\nkind: knowledge\nsource_notes:\n  - /journal/old.md\n---\n\nbody\n"
	if err := v.WriteNote(knowledge, []byte(kBody), ""); err != nil {
		t.Fatal(err)
	}
	if err := v.MarkNoteAsKnowledge(knowledge, "", 0); err != nil {
		t.Fatal(err)
	}
	if err := v.SetKnowledgeDeps(knowledge, []Path{raw}); err != nil {
		t.Fatal(err)
	}

	newPath, _, err := v.RenameNote(raw, "new.md")
	if err != nil {
		t.Fatal(err)
	}

	deps, err := v.GetKnowledgeDeps(knowledge)
	if err != nil || len(deps) != 1 || deps[0] != newPath {
		t.Fatalf("knowledge_deps = %v, err = %v; want [%s]", deps, err, newPath)
	}

	out, err := v.ReadNote(knowledge)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "/journal/old.md") {
		t.Fatalf("stale path should have been rewritten: %s", out)
	}
	if !strings.Contains(string(out), "/journal/new.md") {
		t.Fatalf("new path should appear in frontmatter: %s", out)
	}
}

// TestRenameNoteEnqueuesJobForWikilinkSweep confirms RenameNote itself only
// does the bounded, synchronous work — it enqueues a job rather than
// rewriting vault-wide wikilinks inline. The sweep itself is
// internal/plugins/renamesync's job and is covered by util.RewriteWikilinkTarget's
// own tests plus that package's integration test.
func TestRenameNoteEnqueuesJobForWikilinkSweep(t *testing.T) {
	v := newTestVault(t)

	target := Path("/notes/old.md")
	referrer := Path("/notes/referrer.md")
	if err := v.WriteNote(target, []byte("target body"), ""); err != nil {
		t.Fatal(err)
	}
	if err := v.WriteNote(referrer, []byte("see [[old]] for more"), ""); err != nil {
		t.Fatal(err)
	}

	if _, _, err := v.RenameNote(target, "new.md"); err != nil {
		t.Fatal(err)
	}

	// RenameNote must not itself have rewritten the referrer's body — that's
	// the async sweep's job, tracked by the pending RenameJob.
	out, err := v.ReadNote(referrer)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "[[old]]") {
		t.Fatalf("RenameNote should not synchronously touch unrelated notes' wikilinks, got: %s", out)
	}

	jobs, err := v.PendingRenameJobs()
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 || jobs[0].OldName != "old.md" || jobs[0].NewName != "new.md" {
		t.Fatalf("unexpected pending jobs: %+v", jobs)
	}
}
