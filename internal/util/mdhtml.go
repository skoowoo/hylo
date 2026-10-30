package util

import (
	"bytes"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strings"

	gm "github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	gmhtml "github.com/yuin/goldmark/renderer/html"
)

// wikiImageRe matches ![[filename.ext]] (Obsidian image embeds).
var wikiImageRe = regexp.MustCompile(`!\[\[([^\]\[]+?)\]\]`)

// wikilinkRe matches [[target]] and [[target|display text]].
// The leading (?:^|[^!]) ensures we don't re-match image embeds.
var wikilinkRe = regexp.MustCompile(`\[\[([^\]\[|]+?)(?:\|([^\]\[]+?))?\]\]`)

// expandWikiImages replaces Obsidian-style image embeds ![[filename]] with
// standard Markdown images pointing to /api/images/serve?name=<filename>.
// This must be called BEFORE expandWikilinks so that the ![[...]] patterns
// are consumed first.
func expandWikiImages(src []byte) []byte {
	return wikiImageRe.ReplaceAllFunc(src, func(match []byte) []byte {
		m := wikiImageRe.FindSubmatch(match)
		filename := strings.TrimSpace(string(m[1]))
		return []byte(fmt.Sprintf("![%s](/api/images/serve?name=%s)", filename, url.QueryEscape(filename)))
	})
}

// expandWikilinks replaces Obsidian-style wikilinks with standard Markdown links
// that point to /notes?name=<target>.md.
func expandWikilinks(src []byte) []byte {
	return wikilinkRe.ReplaceAllFunc(src, func(match []byte) []byte {
		m := wikilinkRe.FindSubmatch(match)
		target := strings.TrimSpace(string(m[1]))
		display := strings.TrimSpace(string(m[2]))
		if display == "" {
			display = target
		}
		name := target
		if !strings.HasSuffix(name, ".md") {
			name += ".md"
		}
		// A '#' in a wikilink is never a tag (InlineTags skips wikilinks).
		display = strings.ReplaceAll(display, "#", `\#`)
		return []byte(fmt.Sprintf("[%s](/notes?name=%s)", display, url.QueryEscape(name)))
	})
}

var mdHTML = gm.New(
	gm.WithExtensions(extension.GFM),
	gm.WithParserOptions(
		parser.WithAutoHeadingID(),
	),
	gm.WithRendererOptions(
		gmhtml.WithUnsafe(),
	),
)

// MarkdownToHTMLFragment converts UTF-8 Markdown to an HTML fragment (body inner HTML only).
func MarkdownToHTMLFragment(src []byte) ([]byte, error) {
	src = expandWikiImages(src)
	src = expandWikilinks(src)
	var buf bytes.Buffer
	if err := mdHTML.Convert(src, &buf); err != nil {
		return nil, fmt.Errorf("render markdown: %w", err)
	}
	return buf.Bytes(), nil
}

// MarkdownToHTMLFragmentChecked is like MarkdownToHTMLFragment but marks wiki links
// that point to non-existent notes with the CSS class "wikilink-broken".
// exists(name) should return true when a note filename is present in the vault;
// a nil exists function behaves identically to MarkdownToHTMLFragment.
func MarkdownToHTMLFragmentChecked(src []byte, exists func(string) bool) ([]byte, error) {
	src = expandWikiImages(src)
	src = expandWikilinksChecked(src, exists)
	var buf bytes.Buffer
	if err := mdHTML.Convert(src, &buf); err != nil {
		return nil, fmt.Errorf("render markdown: %w", err)
	}
	return buf.Bytes(), nil
}

// expandWikilinksChecked is like expandWikilinks but emits a non-navigable
// <span class="wikilink-broken"> for targets that fail the exists check,
// instead of a link — the target note is gone, so there's nowhere to go.
func expandWikilinksChecked(src []byte, exists func(string) bool) []byte {
	return wikilinkRe.ReplaceAllFunc(src, func(match []byte) []byte {
		m := wikilinkRe.FindSubmatch(match)
		target := strings.TrimSpace(string(m[1]))
		display := strings.TrimSpace(string(m[2]))
		if display == "" {
			display = target
		}
		name := target
		if !strings.HasSuffix(name, ".md") {
			name += ".md"
		}
		if exists != nil && !exists(name) {
			return []byte(fmt.Sprintf(`<span class="wikilink-broken" title="Note not found">%s</span>`,
				html.EscapeString(display)))
		}
		href := "/notes?name=" + url.QueryEscape(name)
		return []byte(fmt.Sprintf("[%s](%s)", display, href))
	})
}

// ExtractWikilinkNames returns the deduplicated list of note filenames (e.g. "stem.md")
// referenced by all Obsidian-style wikilinks in src.
func ExtractWikilinkNames(src []byte) []string {
	matches := wikilinkRe.FindAllSubmatch(src, -1)
	seen := make(map[string]struct{}, len(matches))
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		target := strings.TrimSpace(string(m[1]))
		name := target
		if !strings.HasSuffix(name, ".md") {
			name += ".md"
		}
		if _, ok := seen[name]; !ok {
			seen[name] = struct{}{}
			out = append(out, name)
		}
	}
	return out
}

// h1Re matches the first ATX heading (# Title) in markdown source.
var h1Re = regexp.MustCompile(`(?m)^#[ \t]+(.+)`)

// ExtractMarkdownH1 returns the text of the first H1 heading in src, or "".
func ExtractMarkdownH1(src []byte) string {
	m := h1Re.FindSubmatch(src)
	if m == nil {
		return ""
	}
	return strings.TrimSpace(string(m[1]))
}

// StripFirstH1 removes the first ATX H1 heading line from src.
func StripFirstH1(src []byte) []byte {
	loc := h1Re.FindIndex(src)
	if loc == nil {
		return src
	}
	end := loc[1]
	if end < len(src) && src[end] == '\n' {
		end++
	}
	out := make([]byte, 0, len(src)-(end-loc[0]))
	out = append(out, src[:loc[0]]...)
	out = append(out, src[end:]...)
	return out
}
