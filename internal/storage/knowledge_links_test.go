package storage

import "testing"

func TestGetKnowledgeEntityTypes(t *testing.T) {
	v := newTestVault(t)
	leaf, _ := ParsePath("/_knowledge/Leaf.md")
	untyped, _ := ParsePath("/_knowledge/Untyped.md")
	if err := v.ReplaceKnowledgeLinksForNote(leaf, "company", nil); err != nil {
		t.Fatal(err)
	}
	if err := v.ReplaceKnowledgeLinksForNote(untyped, "", nil); err != nil {
		t.Fatal(err)
	}

	types, err := v.GetKnowledgeEntityTypes()
	if err != nil {
		t.Fatal(err)
	}
	if len(types) != 1 || types[leaf.String()] != "company" {
		t.Fatalf("got %v", types)
	}
}

func TestBackfillKnowledgeLinksKeepsLeafEntityType(t *testing.T) {
	v := newTestVault(t)
	leaf := Path("/_knowledge/leaf.md")
	if err := v.WriteNote(leaf, []byte("---\nentity_type: concept\n---\n\nno links here\n"), ""); err != nil {
		t.Fatal(err)
	}
	if err := v.MarkNoteAsKnowledge(leaf, "", 0, nil); err != nil {
		t.Fatal(err)
	}

	if err := v.BackfillKnowledgeLinks(""); err != nil {
		t.Fatal(err)
	}
	types, err := v.GetKnowledgeEntityTypes()
	if err != nil {
		t.Fatal(err)
	}
	if types[leaf.String()] != "concept" {
		t.Fatalf("got %v", types)
	}
}
