package storage

import (
	"fmt"
	"testing"
)

// The Activity map asks after every note touched in a year at once.
func TestGetNotesByPathsTakesThousandsOfPaths(t *testing.T) {
	v, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"/a/x.md", "/b/y.md"} {
		if err := v.WriteNote(Path(p), []byte("x"), ""); err != nil {
			t.Fatal(err)
		}
	}
	paths := []Path{"/a/x.md", "/b/y.md", "/a/x.md"}
	for i := range 20000 {
		paths = append(paths, Path(fmt.Sprintf("/gone/n%d.md", i)))
	}
	notes, err := v.GetNotesByPaths(paths)
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 2 {
		t.Fatalf("notes = %+v", notes)
	}
	names, err := v.GetNotesByNames([]string{"x.md", "y.md", "x.md"})
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 {
		t.Fatalf("by names = %+v", names)
	}
}
