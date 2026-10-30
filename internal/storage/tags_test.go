package storage

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// A full scan must derive tags and entity types exactly as live writes do.
func TestScanAndRegisterFullTags(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"plain.md":           "---\ntags: [Reading]\n---\n今天 #读书 和 #reading\n",
		"_knowledge/Leaf.md": "---\nkind: knowledge\nentity_type: concept\ntags: [ai]\n---\nno links here\n",
	}
	for name, body := range files {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	v, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = v.Close() })
	if _, err := v.ScanAndRegisterFull(); err != nil {
		t.Fatal(err)
	}

	for path, want := range map[Path][]string{
		"/plain.md":           {"reading", "读书"},
		"/_knowledge/Leaf.md": {"ai"},
	} {
		n, err := v.StatNote(path)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(n.Tags, want) {
			t.Errorf("%s tags = %q, want %q", path, n.Tags, want)
		}
	}
	types, err := v.GetKnowledgeEntityTypes()
	if err != nil {
		t.Fatal(err)
	}
	if got := types["/_knowledge/Leaf.md"]; got != "concept" {
		t.Errorf("leaf entity_type = %q, want concept", got)
	}
}

func TestRewriteTag(t *testing.T) {
	v := newTestVault(t)
	for p, body := range map[Path]string{
		"/a.md": "---\ntags: [Old, keep]\n---\nbody #old\n",
		"/b.md": "plain #old note\n",
		"/c.md": "no tag here\n",
		"/d.md": "---\ntags: [indie dev]\n---\n#old\n",
	} {
		if err := v.WriteNote(p, []byte(body), ""); err != nil {
			t.Fatal(err)
		}
	}
	updated, err := v.RewriteTag("old", "")
	if err != nil || len(updated) != 3 {
		t.Fatalf("updated = %v, err = %v", updated, err)
	}
	for p, want := range map[Path][]string{"/a.md": {"keep"}, "/b.md": {}, "/c.md": {}, "/d.md": {"indie-dev"}} {
		n, err := v.StatNote(p)
		if err != nil {
			t.Fatal(err)
		}
		if len(n.Tags) != len(want) || (len(want) > 0 && n.Tags[0] != want[0]) {
			t.Errorf("%s tags = %q, want %q", p, n.Tags, want)
		}
	}
	if again, _ := v.RewriteTag("old", ""); len(again) != 0 {
		t.Errorf("second run rewrote %v", again)
	}
}

// Front matter written with a space is stored as indie-dev and found by it.
func TestRewriteTagSpacedFrontmatter(t *testing.T) {
	v := newTestVault(t)
	if err := v.WriteNote("/a.md", []byte("---\ntags: [indie dev, b]\n---\nbody\n"), ""); err != nil {
		t.Fatal(err)
	}
	updated, err := v.RewriteTag("indie-dev", "")
	if err != nil || len(updated) != 1 {
		t.Fatalf("updated = %v, err = %v", updated, err)
	}
	n, err := v.StatNote("/a.md")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(n.Tags, []string{"b"}) {
		t.Errorf("tags = %q, want [b]", n.Tags)
	}
}
