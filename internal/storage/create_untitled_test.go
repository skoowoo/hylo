package storage

import (
	"strings"
	"sync"
	"testing"
)

// TestCreateUntitledNoteWritesContentAtRoot checks the happy path: a note
// is created in dir with the given content and an "Untitled ..." name.
func TestCreateUntitledNoteWritesContentAtRoot(t *testing.T) {
	v := newTestVault(t)

	p, err := v.CreateUntitledNote("/", []byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if p.Dir() != "/" {
		t.Fatalf("dir = %q, want /", p.Dir())
	}
	if !strings.HasPrefix(p.Base(), "Untitled ") || !strings.HasSuffix(p.Base(), ".md") {
		t.Fatalf("unexpected generated name: %q", p.Base())
	}

	content, err := v.ReadNote(p)
	if err != nil || string(content) != "hello" {
		t.Fatalf("content = %q, err = %v", content, err)
	}
}

// TestCreateUntitledNoteDedupesOnCollision creates several untitled notes
// back-to-back — fast enough in a test that the timestamp-based candidate
// name collides almost every time — and checks the dedup suffix kicks in
// instead of one call silently overwriting an earlier note's content.
func TestCreateUntitledNoteDedupesOnCollision(t *testing.T) {
	v := newTestVault(t)

	const n = 20
	paths := make([]Path, n)
	for i := 0; i < n; i++ {
		p, err := v.CreateUntitledNote("/", []byte{byte('a' + i)})
		if err != nil {
			t.Fatal(err)
		}
		paths[i] = p
	}

	seen := make(map[Path]bool, n)
	for i, p := range paths {
		if seen[p] {
			t.Fatalf("duplicate path %q at index %d", p, i)
		}
		seen[p] = true
		content, err := v.ReadNote(p)
		if err != nil {
			t.Fatalf("ReadNote(%q): %v", p, err)
		}
		if len(content) != 1 || content[0] != byte('a'+i) {
			t.Fatalf("note %q content = %q, want %q — an earlier note's content may have been overwritten", p, content, string(byte('a'+i)))
		}
	}
}

// TestCreateUntitledNoteConcurrent fires many CreateUntitledNote calls from
// real goroutines at once (unlike the sequential-loop dedup test above) —
// the concurrent windows/requests this app runs could plausibly hit
// "New Note" within the same second. g.mu.Lock() inside CreateUntitledNote
// must fully serialize them; run with -race to catch any data race too.
func TestCreateUntitledNoteConcurrent(t *testing.T) {
	v := newTestVault(t)

	const n = 20
	paths := make([]Path, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			paths[i], errs[i] = v.CreateUntitledNote("/", []byte{byte('a' + i)})
		}(i)
	}
	wg.Wait()

	seen := make(map[Path]bool, n)
	for i := range paths {
		if errs[i] != nil {
			t.Fatalf("goroutine %d: %v", i, errs[i])
		}
		if seen[paths[i]] {
			t.Fatalf("duplicate path %q from goroutine %d", paths[i], i)
		}
		seen[paths[i]] = true
		content, err := v.ReadNote(paths[i])
		if err != nil {
			t.Fatalf("ReadNote(%q): %v", paths[i], err)
		}
		if len(content) != 1 || content[0] != byte('a'+i) {
			t.Fatalf("note %q content = %q, want %q — a concurrent create may have overwritten it", paths[i], content, string(byte('a'+i)))
		}
	}
}

// TestCreateUntitledNoteRejectsSystemDir mirrors RenameNote's guard: an
// underscore-prefixed directory is system-managed and must not silently
// gain an ad-hoc user note.
func TestCreateUntitledNoteRejectsSystemDir(t *testing.T) {
	v := newTestVault(t)
	if _, err := v.CreateUntitledNote("/_knowledge", []byte("x")); err == nil {
		t.Fatal("expected an error creating under a system directory")
	}
}
