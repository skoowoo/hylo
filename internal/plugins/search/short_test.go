package search

import (
	"testing"
	"time"

	"github.com/hardhacker/hylo/internal/storage"
)

func TestFileDocsPreservesFrontmatterTagsOnShortDay(t *testing.T) {
	p, _ := storage.ParsePath("/_shorts/2026-11-18.md")
	raw := []byte("---\nkind: short\ntags:\n  - journal\n---\n\n###### Short Note: 2026-11-18 09:00:00\n\nentry body #inline\n")
	docs := fileDocs(p, raw, storage.KindShort, time.Now())
	if len(docs) != 2 {
		t.Fatalf("got %d docs, want 2 (entry + frontmatter doc)", len(docs))
	}
	if docs[0].ShortID != "2026-11-18-090000" || len(docs[0].Tags) != 1 || docs[0].Tags[0] != "inline" {
		t.Fatalf("entry doc = %+v", docs[0])
	}
	if docs[1].ShortID != "" || len(docs[1].Tags) != 1 || docs[1].Tags[0] != "journal" {
		t.Fatalf("frontmatter doc = %+v", docs[1])
	}
}

func TestUpsertFileShortEntriesAndDelete(t *testing.T) {
	idx, err := NewBleveIndexer(t.TempDir(), false)
	if err != nil {
		t.Fatalf("NewBleveIndexer: %v", err)
	}
	defer idx.Close()

	dayPath := "/_shorts/2026-11-18.md"
	t1 := time.Date(2026, 11, 18, 9, 0, 0, 0, time.Local)
	t2 := time.Date(2026, 11, 18, 14, 30, 0, 0, time.Local)

	docs := []FileDoc{
		{ShortID: "2026-11-18-090000", Content: "first line\nredis cache ttl tuned #backend", Tags: []string{"backend"}, UpdatedAt: t1},
		{ShortID: "2026-11-18-143000", Content: "bought coffee beans #life", Tags: []string{"life"}, UpdatedAt: t2},
	}
	idx.UpsertFile(dayPath, "2026-11-18.md", "short", docs)

	readFn := func(path, kind, shortID string) ([]byte, error) {
		for _, d := range docs {
			if d.ShortID == shortID {
				return []byte(d.Content), nil
			}
		}
		return nil, nil
	}

	hits, err := idx.Search("redis", SearchOptions{Kind: "short"}, readFn)
	if err != nil {
		t.Fatalf("Search redis: %v", err)
	}
	if len(hits) != 1 || hits[0].ShortID != "2026-11-18-090000" || hits[0].Name != "2026-11-18.md" {
		t.Fatalf("got %+v, want single hit for 2026-11-18-090000", hits)
	}
	if len(hits[0].Lines) != 1 || hits[0].Lines[0] != 2 {
		t.Fatalf("hit lines = %v, want [2]", hits[0].Lines)
	}

	// Short entry docs must not match filename queries for the daily file.
	nameHits, err := idx.Search("2026-11-18", SearchOptions{Type: "name"}, readFn)
	if err != nil {
		t.Fatalf("Search name: %v", err)
	}
	if len(nameHits) != 0 {
		t.Fatalf("short entries matched filename search: %+v", nameHits)
	}

	// Updating the day without the second entry must drop it from the index.
	docs = docs[:1]
	idx.UpsertFile(dayPath, "2026-11-18.md", "short", docs)

	hits, err = idx.Search("coffee", SearchOptions{}, readFn)
	if err != nil {
		t.Fatalf("Search coffee after removal: %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf("stale short entry still indexed: %+v", hits)
	}

	// Deleting the daily file must remove every remaining entry doc.
	idx.Delete(dayPath)
	n, err := idx.DocCount()
	if err != nil {
		t.Fatalf("DocCount: %v", err)
	}
	if n != 0 {
		t.Fatalf("DocCount after Delete = %d, want 0", n)
	}
}
