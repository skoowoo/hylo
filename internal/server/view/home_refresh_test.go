package view

import (
	"bytes"
	"regexp"
	"testing"

	"github.com/hardhacker/hylo/internal/storage"
)

// htmx morph copies attributes with setAttribute, which WKWebView rejects for
// names starting with '@' or ':'.
func TestHomeRefreshUsesMorphSafeAttributes(t *testing.T) {
	var buf bytes.Buffer
	data := homePageData{
		Folders:        []storage.DirSummary{{Dir: "/a"}, {Dir: "/b c"}},
		IndexNotes:     []noteItem{{}},
		KnowledgeCount: 1,
	}
	if err := homeRefreshTemplate.Execute(&buf, data); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if bad := regexp.MustCompile(`\s[@:][\w.-]+=`).FindString(out); bad != "" {
		t.Fatalf("shorthand attribute %q in morphed template; use x-on:/x-bind:", bad)
	}
	if n := regexp.MustCompile(`data-drop-dir=`).FindAllString(out, -1); len(n) != 2 {
		t.Fatalf("expected 2 folder buttons, got %d", len(n))
	}
}
