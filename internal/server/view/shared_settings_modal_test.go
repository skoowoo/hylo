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
	html := settingsModalHTML()
	inner := strings.Index(html, `class="settings-modal-inner"`)
	closeBtn := strings.Index(html, `settings-modal-close"`)
	if inner < 0 || closeBtn < 0 {
		t.Fatalf("markers not found: inner=%d close=%d", inner, closeBtn)
	}
	if closeBtn < inner {
		t.Fatal("settings-modal-close must come after settings-modal-inner in the DOM, or macOS drag regions swallow its clicks")
	}
}

// TestHomePageTemplateRenders guards against the exact bug class that hit
// this file twice: the whole page (home.html, including everything
// settingsModalHTML()/settingsCtrlJS contribute) is parsed as ONE
// html/template named "home" (home.go's homePageTemplate). A stray literal
// `{{` anywhere in that output — e.g. a prompt default like `{{.Path}}`
// meant as plain text — gets parsed as a template action and either fails
// to parse (if malformed) or fails at Execute (if it references a field
// homePageData doesn't have, which is what actually happened). Running the
// real Execute path with a zero-value homePageData catches both failure
// modes without needing a live server or vault.
func TestHomePageTemplateRenders(t *testing.T) {
	var buf bytes.Buffer
	if err := homePageTemplate.Execute(&buf, homePageData{}); err != nil {
		t.Fatalf("home page template failed to execute — likely a stray {{ in settings modal HTML/JS content: %v", err)
	}
}
