package storage

import "testing"

// TestDeleteNoteCascadesRelations covers the full raw → knowledge → index chain:
// deleting any one of the three must clean up every relation row it appears in
// (on either side) without deleting the other notes.
func TestDeleteNoteCascadesRelations(t *testing.T) {
	v := newTestVault(t)

	raw := Path("/raw.md")
	knowledge := Path("/_knowledge/k.md")
	index := Path("/idx.md")

	for _, p := range []Path{raw, knowledge, index} {
		if err := v.WriteNote(p, []byte("body"), ""); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}
	if err := v.MarkNoteAsKnowledge(knowledge, "", 0); err != nil {
		t.Fatal(err)
	}
	if err := v.MarkNoteAsIndex(index, ""); err != nil {
		t.Fatal(err)
	}
	if err := v.MarkNoteCompiled(raw); err != nil {
		t.Fatal(err)
	}
	if err := v.SetKnowledgeDeps(knowledge, []Path{raw}); err != nil {
		t.Fatal(err)
	}
	if err := v.SetIndexDeps(index, []Path{knowledge}); err != nil {
		t.Fatal(err)
	}

	// Deleting the raw source note must drop the knowledge_deps row that points
	// at it, but must not touch the knowledge note itself.
	if err := v.DeleteNote(raw); err != nil {
		t.Fatalf("delete raw: %v", err)
	}
	if deps, err := v.GetKnowledgeDeps(knowledge); err != nil || len(deps) != 0 {
		t.Fatalf("knowledge_deps after raw delete = %v, %v; want empty", deps, err)
	}
	if _, err := v.StatNote(knowledge); err != nil {
		t.Fatalf("knowledge note should survive raw delete: %v", err)
	}

	// Re-create the raw source and re-link it so we can verify the reverse
	// direction: deleting the knowledge note must drop its own dependency row
	// and reset the source's compile_count back to 0 (re-compilable), while
	// leaving the index note (and its now-empty listing) alone.
	if err := v.WriteNote(raw, []byte("body"), ""); err != nil {
		t.Fatal(err)
	}
	if err := v.MarkNoteCompiled(raw); err != nil {
		t.Fatal(err)
	}
	if err := v.SetKnowledgeDeps(knowledge, []Path{raw}); err != nil {
		t.Fatal(err)
	}

	if err := v.DeleteNote(knowledge); err != nil {
		t.Fatalf("delete knowledge: %v", err)
	}
	if deps, err := v.GetSourceKnowledges(raw); err != nil || len(deps) != 0 {
		t.Fatalf("source knowledges after knowledge delete = %v, %v; want empty", deps, err)
	}
	n, err := v.StatNote(raw)
	if err != nil {
		t.Fatalf("raw note should survive knowledge delete: %v", err)
	}
	if n.CompileCount != 0 {
		t.Fatalf("raw compile_count = %d, want 0 (re-compilable)", n.CompileCount)
	}
	if idxDeps, err := v.GetIndexDeps(index); err != nil || len(idxDeps) != 0 {
		t.Fatalf("index_deps after knowledge delete = %v, %v; want empty", idxDeps, err)
	}
	if _, err := v.StatNote(index); err != nil {
		t.Fatalf("index note should survive knowledge delete: %v", err)
	}

	// Deleting the index note only clears its own listing; nothing else to check
	// here since its knowledge dependency was already gone, but the delete itself
	// must still succeed and remove the note.
	if err := v.DeleteNote(index); err != nil {
		t.Fatalf("delete index: %v", err)
	}
	if _, err := v.StatNote(index); err == nil {
		t.Fatal("index note should be gone")
	}
}

// TestDeleteNoteKeepsCompileCountWhileOtherDependentsRemain ensures a source
// note shared by two knowledge notes only becomes re-compilable once the last
// dependent knowledge note is deleted, not the first.
func TestDeleteNoteKeepsCompileCountWhileOtherDependentsRemain(t *testing.T) {
	v := newTestVault(t)

	raw := Path("/raw.md")
	k1 := Path("/_knowledge/k1.md")
	k2 := Path("/_knowledge/k2.md")

	for _, p := range []Path{raw, k1, k2} {
		if err := v.WriteNote(p, []byte("body"), ""); err != nil {
			t.Fatal(err)
		}
	}
	if err := v.MarkNoteAsKnowledge(k1, "", 0); err != nil {
		t.Fatal(err)
	}
	if err := v.MarkNoteAsKnowledge(k2, "", 0); err != nil {
		t.Fatal(err)
	}
	if err := v.MarkNoteCompiled(raw); err != nil {
		t.Fatal(err)
	}
	if err := v.SetKnowledgeDeps(k1, []Path{raw}); err != nil {
		t.Fatal(err)
	}
	if err := v.SetKnowledgeDeps(k2, []Path{raw}); err != nil {
		t.Fatal(err)
	}

	if err := v.DeleteNote(k1); err != nil {
		t.Fatal(err)
	}
	n, err := v.StatNote(raw)
	if err != nil {
		t.Fatal(err)
	}
	if n.CompileCount != 1 {
		t.Fatalf("compile_count = %d after first dependent deleted, want 1 (k2 still depends on raw)", n.CompileCount)
	}

	if err := v.DeleteNote(k2); err != nil {
		t.Fatal(err)
	}
	n, err = v.StatNote(raw)
	if err != nil {
		t.Fatal(err)
	}
	if n.CompileCount != 0 {
		t.Fatalf("compile_count = %d after last dependent deleted, want 0", n.CompileCount)
	}
}

// TestDeleteNoteCascadesKnowledgeLinks ensures a knowledge note's wikilink
// edges are removed on both sides (as source and as target) on delete.
func TestDeleteNoteCascadesKnowledgeLinks(t *testing.T) {
	v := newTestVault(t)

	a := Path("/_knowledge/a.md")
	b := Path("/_knowledge/b.md")
	c := Path("/_knowledge/c.md")

	for _, p := range []Path{a, b, c} {
		if err := v.WriteNote(p, []byte("body"), ""); err != nil {
			t.Fatal(err)
		}
		if err := v.MarkNoteAsKnowledge(p, "", 0); err != nil {
			t.Fatal(err)
		}
	}
	// a -> b, b -> c
	if err := v.ReplaceKnowledgeLinksForNote(a, "", []string{"b.md"}); err != nil {
		t.Fatal(err)
	}
	if err := v.ReplaceKnowledgeLinksForNote(b, "", []string{"c.md"}); err != nil {
		t.Fatal(err)
	}

	if err := v.DeleteNote(b); err != nil {
		t.Fatal(err)
	}

	edges, err := v.GetAllKnowledgeLinks()
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range edges {
		if e.Source == b || e.Target == b {
			t.Fatalf("knowledge_links still references deleted note b: %+v", e)
		}
	}
}
