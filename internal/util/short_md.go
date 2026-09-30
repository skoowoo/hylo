package util

import (
	"bytes"
	"fmt"
	"html"
	"regexp"
	"strings"
)

// shortFileDelimiterRe splits a daily short file into raw entry blobs. Entries
// are stored separated by a markdown thematic-break line (--- or ***).
var shortFileDelimiterRe = regexp.MustCompile(`\r?\n(?:---|\*\*\*)\s*\r?\n`)

const shortNoteHeadingPrefix = "###### Short Note:"

// ShortNoteEntry is one logical short inside a daily markdown file after
// frontmatter has been stripped. Timestamp comes from the leading H2 line when
// present; BodyMD is the remainder rendered through MarkdownToHTMLFragmentChecked.
type ShortNoteEntry struct {
	Timestamp string
	BodyMD    []byte
}

// ParseShortNoteFile splits daily short file body text into entries separated
// by --- lines (as written by storage.AppendShort). User-supplied --- lines in
// a single entry are sanitized away at write time, so boundaries are unambiguous.
func ParseShortNoteFile(body []byte) []ShortNoteEntry {
	if len(body) == 0 {
		return nil
	}
	// Strip YAML frontmatter (kind: short daily files carry it).
	if fm, stripped := ParseFrontmatter(body); fm.HasMeta() {
		body = stripped
	}
	norm := bytes.ReplaceAll(body, []byte("\r\n"), []byte("\n"))
	parts := shortFileDelimiterRe.Split(string(norm), -1)
	out := make([]ShortNoteEntry, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		stamp, inner := parseShortEntryHeading(part)
		out = append(out, ShortNoteEntry{
			Timestamp: stamp,
			BodyMD:    []byte(inner),
		})
	}
	return out
}

func parseShortEntryHeading(md string) (timestamp string, body string) {
	md = strings.TrimSpace(md)
	lines := strings.SplitN(md, "\n", 2)
	first := strings.TrimSpace(lines[0])
	if strings.HasPrefix(first, shortNoteHeadingPrefix) {
		timestamp = strings.TrimSpace(strings.TrimPrefix(first, shortNoteHeadingPrefix))
		if len(lines) < 2 {
			return timestamp, ""
		}
		return timestamp, strings.TrimSpace(lines[1])
	}
	return "", md
}

// RenderShortNoteFileToHTML renders a full daily short file body into an HTML
// fragment: a list of “card” blocks, each entry passed through
// MarkdownToHTMLFragmentChecked so wikilinks / GFM match normal note rendering.
// Entries are shown newest-first (last segment in the file appears at the top).
func RenderShortNoteFileToHTML(body []byte, exists func(string) bool) ([]byte, error) {
	entries := ParseShortNoteFile(body)
	if len(entries) == 0 {
		return nil, nil
	}
	var buf bytes.Buffer
	buf.WriteString(`<div class="short-day-frag">`)
	buf.WriteString(`<div class="short-entry-stack" role="list">`)
	for i := len(entries) - 1; i >= 0; i-- {
		e := &entries[i]
		if len(bytes.TrimSpace(e.BodyMD)) == 0 {
			// Still show timestamp-only bubble
			buf.WriteString(`<div class="short-entry-card hairline" role="listitem">`)
			if e.Timestamp != "" {
				buf.WriteString(`<div class="short-entry-time"><span class="short-entry-time-inner">`)
				buf.WriteString(html.EscapeString(e.Timestamp))
				buf.WriteString(`</span></div>`)
			}
			buf.WriteString(`<div class="short-entry-content prose short-entry-prose short-entry-prose-empty"></div>`)
			buf.WriteString(`</div>`)
			continue
		}
		htmlChunk, err := MarkdownToHTMLFragmentChecked(e.BodyMD, exists)
		if err != nil {
			return nil, fmt.Errorf("short entry (file order %d): %w", i, err)
		}
		buf.WriteString(`<div class="short-entry-card hairline" role="listitem">`)
		if e.Timestamp != "" {
			buf.WriteString(`<div class="short-entry-time"><span class="short-entry-time-inner">`)
			buf.WriteString(html.EscapeString(e.Timestamp))
			buf.WriteString(`</span></div>`)
		}
		buf.WriteString(`<div class="short-entry-content prose short-entry-prose">`)
		buf.Write(htmlChunk)
		buf.WriteString(`</div>`)
		buf.WriteString(`</div>`)
	}
	buf.WriteString(`</div></div>`)
	return buf.Bytes(), nil
}
