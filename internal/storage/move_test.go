package storage

import (
	"errors"
	"strings"
	"testing"
)

// TestMoveNoteUpdatesDirOnly moves a plain note and checks the filename,
// content, and DB row all land at the new directory untouched otherwise.
func TestMoveNoteUpdatesDirOnly(t *testing.T) {
	v := newTestVault(t)

	p := Path("/inbox/note.md")
	if err := v.WriteNote(p, []byte("hello"), ""); err != nil {
		t.Fatal(err)
	}

	newPath, err := v.MoveNote(p, "/archive")
	if err != nil {
		t.Fatal(err)
	}
	if newPath != Path("/archive/note.md") {
		t.Fatalf("newPath = %q, want /archive/note.md", newPath)
	}

	if _, err := v.StatNote(p); !errors.Is(err, ErrNotFound) {
		t.Fatalf("old path should be gone, got err = %v", err)
	}
	n, err := v.StatNote(newPath)
	if err != nil {
		t.Fatalf("new path should exist: %v", err)
	}
	if n.Dir != "/archive" || n.Name != "note.md" {
		t.Fatalf("unexpected note meta: %+v", n)
	}
	content, err := v.ReadNote(newPath)
	if err != nil || string(content) != "hello" {
		t.Fatalf("content = %q, err = %v", content, err)
	}
}

func TestMoveNoteNoopSameDir(t *testing.T) {
	v := newTestVault(t)
	p := Path("/note.md")
	if err := v.WriteNote(p, []byte("x"), ""); err != nil {
		t.Fatal(err)
	}
	newPath, err := v.MoveNote(p, "/")
	if err != nil {
		t.Fatal(err)
	}
	if newPath != p {
		t.Fatalf("newPath = %q, want unchanged %q", newPath, p)
	}
}

func TestMoveNoteDestinationCollision(t *testing.T) {
	v := newTestVault(t)
	if err := v.WriteNote("/a/note.md", []byte("a"), ""); err != nil {
		t.Fatal(err)
	}
	if err := v.WriteNote("/b/note.md", []byte("b"), ""); err != nil {
		t.Fatal(err)
	}

	_, err := v.MoveNote("/a/note.md", "/b")
	if !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("err = %v, want ErrAlreadyExists", err)
	}
	// Neither note should have moved or been touched.
	if _, err := v.StatNote("/a/note.md"); err != nil {
		t.Fatalf("source should be untouched: %v", err)
	}
	content, err := v.ReadNote("/b/note.md")
	if err != nil || string(content) != "b" {
		t.Fatalf("destination should be untouched: content=%q err=%v", content, err)
	}
}

func TestMoveNoteCreatesDestinationDir(t *testing.T) {
	v := newTestVault(t)
	if err := v.WriteNote("/note.md", []byte("x"), ""); err != nil {
		t.Fatal(err)
	}
	newPath, err := v.MoveNote("/note.md", "/a/b/c")
	if err != nil {
		t.Fatal(err)
	}
	if newPath != Path("/a/b/c/note.md") {
		t.Fatalf("newPath = %q", newPath)
	}
}

// TestMoveNoteRewritesKnowledgeSourceRef covers moving a raw source note:
// knowledge_deps.source_dir must follow it, and the dependent knowledge
// note's source_notes: frontmatter list must be rewritten to the new path so
// a future compile resync doesn't resurrect the stale one (see dbMove's and
// Vault.MoveNote's comments).
func TestMoveNoteRewritesKnowledgeSourceRef(t *testing.T) {
	v := newTestVault(t)

	raw := Path("/journal/note.md")
	knowledge := Path("/_knowledge/k.md")

	if err := v.WriteNote(raw, []byte("raw body"), ""); err != nil {
		t.Fatal(err)
	}
	kBody := "---\nkind: knowledge\nsource_notes:\n  - /journal/note.md\n---\n\nbody\n"
	if err := v.WriteNote(knowledge, []byte(kBody), ""); err != nil {
		t.Fatal(err)
	}
	if err := v.MarkNoteAsKnowledge(knowledge, "", 0); err != nil {
		t.Fatal(err)
	}
	if err := v.SetKnowledgeDeps(knowledge, []Path{raw}); err != nil {
		t.Fatal(err)
	}

	newPath, err := v.MoveNote(raw, "/archive")
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
	if strings.Contains(string(out), "/journal/note.md") {
		t.Fatalf("stale path should have been rewritten: %s", out)
	}
	if !strings.Contains(string(out), "/archive/note.md") {
		t.Fatalf("new path should appear in frontmatter: %s", out)
	}
}

// TestMoveNoteRewritesIndexKnowledgeRef covers moving a knowledge note:
// index_deps.knowledge_dir must follow it, and the dependent index note's
// table must be rewritten to the new path.
func TestMoveNoteRewritesIndexKnowledgeRef(t *testing.T) {
	v := newTestVault(t)

	knowledge := Path("/_knowledge/k.md")
	index := Path("/_knowledge/_indexes/AI.md")

	if err := v.WriteNote(knowledge, []byte("---\nkind: knowledge\n---\n\nbody\n"), ""); err != nil {
		t.Fatal(err)
	}
	if err := v.MarkNoteAsKnowledge(knowledge, "", 0); err != nil {
		t.Fatal(err)
	}
	idxBody := "---\nkind: index\n---\n\n" +
		"| Title | Path |\n| --- | --- |\n| K | /_knowledge/k.md |\n"
	if err := v.WriteNote(index, []byte(idxBody), ""); err != nil {
		t.Fatal(err)
	}
	if err := v.MarkNoteAsIndex(index, ""); err != nil {
		t.Fatal(err)
	}
	if err := v.SetIndexDeps(index, []Path{knowledge}); err != nil {
		t.Fatal(err)
	}

	newPath, err := v.MoveNote(knowledge, "/_knowledge/Agents")
	if err != nil {
		t.Fatal(err)
	}

	deps, err := v.GetIndexDeps(index)
	if err != nil || len(deps) != 1 || deps[0] != newPath {
		t.Fatalf("index_deps = %v, err = %v; want [%s]", deps, err, newPath)
	}

	out, err := v.ReadNote(index)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "| /_knowledge/k.md |") {
		t.Fatalf("stale table path should have been rewritten: %s", out)
	}
	if !strings.Contains(string(out), "/_knowledge/Agents/k.md") {
		t.Fatalf("new path should appear in the table: %s", out)
	}
}

// TestMoveNoteLeavesWikilinksAlone confirms move never has to touch wikilinks:
// resolution is name-based, so a note's own dir change doesn't affect how
// other notes reference it via [[name]].
func TestMoveNoteLeavesWikilinksAlone(t *testing.T) {
	v := newTestVault(t)

	a := Path("/_knowledge/a.md")
	b := Path("/_knowledge/b.md")
	for _, p := range []Path{a, b} {
		if err := v.WriteNote(p, []byte("body"), ""); err != nil {
			t.Fatal(err)
		}
		if err := v.MarkNoteAsKnowledge(p, "", 0); err != nil {
			t.Fatal(err)
		}
	}
	if err := v.ReplaceKnowledgeLinksForNote(a, "", []string{"b.md"}); err != nil {
		t.Fatal(err)
	}

	newB, err := v.MoveNote(b, "/_knowledge/Sub")
	if err != nil {
		t.Fatal(err)
	}

	edges, err := v.GetAllKnowledgeLinks()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range edges {
		if e.Source == a && e.Target == newB {
			found = true
		}
	}
	if !found {
		t.Fatalf("edge a->b should follow the moved target's new path, got %+v", edges)
	}
}
