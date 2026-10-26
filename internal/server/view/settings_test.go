package view

import (
	"bytes"
	"strings"
	"testing"
)

// The desktop webview applies -webkit-app-region rects in DOM order, so the close
// button's no-drag only wins over the drag-enabled .home-list-head inside
// .settings-modal-inner if the button comes after it in the markup.
func TestSettingsCloseButtonAfterInner(t *testing.T) {
	var buf bytes.Buffer
	if err := views.ExecuteTemplate(&buf, "settings_modal.html", nil); err != nil {
		t.Fatal(err)
	}
	html := buf.String()
	inner := strings.Index(html, `class="settings-modal-inner"`)
	closeBtn := strings.Index(html, `settings-modal-close"`)
	if inner < 0 || closeBtn < 0 {
		t.Fatalf("markers not found: inner=%d close=%d", inner, closeBtn)
	}
	if closeBtn < inner {
		t.Fatal("settings-modal-close must come after settings-modal-inner in the DOM, or macOS drag regions swallow its clicks")
	}
}

// Templates resolve names, funcs and fields only when executed, so render the
// whole page once with zero data to catch a broken reference in any partial.
func TestHomePageTemplateRenders(t *testing.T) {
	var buf bytes.Buffer
	if err := views.ExecuteTemplate(&buf, "page_home.html", homePageData{}); err != nil {
		t.Fatal(err)
	}
}
