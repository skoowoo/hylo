package util

import (
	"bytes"
	"strings"
	"unicode"
	"unicode/utf8"

	gm "github.com/yuin/goldmark"
	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"golang.org/x/text/unicode/norm"
)

// NoteTags holds a note's tags by where they were written, each list
// normalized (NormalizeTag) and deduplicated.
type NoteTags struct {
	Frontmatter []string
	Inline      []string
}

// All merges both lists, front matter first.
func (t NoteTags) All() []string {
	seen := make(map[string]bool, len(t.Frontmatter)+len(t.Inline))
	out := make([]string, 0, len(t.Frontmatter)+len(t.Inline))
	for _, list := range [][]string{t.Frontmatter, t.Inline} {
		for _, tag := range list {
			if !seen[tag] {
				seen[tag] = true
				out = append(out, tag)
			}
		}
	}
	return out
}

// ExtractTags is the one place a note's tags come from: the front matter
// `tags` field plus Obsidian-style #tags in the body (see testdata/tags.json,
// which the editor's HashTag syntax is tested against too).
func ExtractTags(src []byte) NoteTags {
	fm, body := ParseFrontmatter(src)
	return NoteTags{
		Frontmatter: normalizeTags(fm.Tags),
		Inline:      normalizeTags(InlineTags(body)),
	}
}

// NormalizeTag returns the canonical form tags are compared and stored in:
// no leading '#', NFC, spaces as hyphens, lowercase. "" when nothing is left.
func NormalizeTag(tag string) string {
	tag = strings.TrimLeft(strings.TrimSpace(tag), "#")
	tag = strings.Join(strings.Fields(tag), "-")
	return strings.ToLower(norm.NFC.String(tag))
}

// ValidTag reports whether a normalized tag reads back as itself when written
// inline as #tag, so a rename never produces a broken or different tag.
func ValidTag(tag string) bool {
	for _, r := range tag {
		if !isTagRune(r) {
			return false
		}
	}
	return tag != "" && trimTagRun(tag) == tag
}

func normalizeTags(tags []string) []string {
	seen := make(map[string]bool, len(tags))
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		if t = NormalizeTag(t); t != "" && !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}

func isTagRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsMark(r) || r == '_' || r == '-' || r == '/'
}

var tagParser = gm.DefaultParser()

// InlineTags returns the raw #tags in a markdown body (front matter already
// stripped), in document order, skipping code, HTML and wikilinks.
func InlineTags(body []byte) []string {
	spans := inlineTagSpans(body)
	out := make([]string, len(spans))
	for i, s := range spans {
		out[i] = string(body[s.from+1 : s.to])
	}
	return out
}

// inlineTagSpans returns each #tag's byte range in body, '#' included.
func inlineTagSpans(body []byte) []byteRange {
	if bytes.IndexByte(body, '#') < 0 {
		return nil
	}
	skip := tagSkipRanges(body)
	var out []byteRange
	for i := 0; i < len(body); i++ {
		if body[i] != '#' || inRanges(skip, i) {
			continue
		}
		if i > 0 {
			if prev, _ := utf8.DecodeLastRune(body[:i]); !unicode.IsSpace(prev) {
				continue
			}
		}
		end := i + 1
		for end < len(body) {
			r, size := utf8.DecodeRune(body[end:])
			if !isTagRune(r) {
				break
			}
			end += size
		}
		if tag := trimTagRun(string(body[i+1 : end])); tag != "" {
			out = append(out, byteRange{i, i + 1 + len(tag)})
		}
		i = end - 1
	}
	return out
}

// trimTagRun applies Obsidian's limits to a run of tag characters: no leading
// '/', cut at "//", no trailing '/', and not digits alone ("#2024").
func trimTagRun(run string) string {
	if strings.HasPrefix(run, "/") {
		return ""
	}
	if k := strings.Index(run, "//"); k >= 0 {
		run = run[:k]
	}
	run = strings.TrimRight(run, "/")
	for _, r := range run {
		if !unicode.IsDigit(r) {
			return run
		}
	}
	return ""
}

type byteRange struct{ from, to int }

func inRanges(rs []byteRange, i int) bool {
	for _, r := range rs {
		if i >= r.from && i < r.to {
			return true
		}
	}
	return false
}

// tagSkipRanges collects the byte ranges where '#' never starts a tag.
func tagSkipRanges(body []byte) []byteRange {
	var rs []byteRange
	addLines := func(lines *text.Segments) {
		for k := 0; k < lines.Len(); k++ {
			s := lines.At(k)
			rs = append(rs, byteRange{s.Start, s.Stop})
		}
	}
	doc := tagParser.Parse(text.NewReader(body))
	_ = gast.Walk(doc, func(n gast.Node, entering bool) (gast.WalkStatus, error) {
		if !entering {
			return gast.WalkContinue, nil
		}
		switch n := n.(type) {
		case *gast.FencedCodeBlock:
			if n.Info != nil {
				rs = append(rs, byteRange{n.Info.Segment.Start, n.Info.Segment.Stop})
			}
			addLines(n.Lines())
		case *gast.CodeBlock:
			addLines(n.Lines())
		case *gast.HTMLBlock:
			addLines(n.Lines())
			if n.HasClosure() {
				rs = append(rs, byteRange{n.ClosureLine.Start, n.ClosureLine.Stop})
			}
		case *gast.RawHTML:
			addLines(n.Segments)
		case *gast.CodeSpan:
			if first, ok := n.FirstChild().(*gast.Text); ok {
				last := n.LastChild().(*gast.Text)
				rs = append(rs, byteRange{first.Segment.Start, last.Segment.Stop})
			}
			return gast.WalkSkipChildren, nil
		}
		return gast.WalkContinue, nil
	})
	for _, m := range wikilinkRe.FindAllIndex(body, -1) {
		rs = append(rs, byteRange{m[0], m[1]})
	}
	return rs
}
