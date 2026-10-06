package client

import (
	"strings"

	"github.com/yuin/goldmark/ast"
)

type Heading struct {
	Level int
	Text  string
}

type CodeBlock struct {
	Lang    string
	Content string
}

// ParseHeadings returns all ATX headings in source using the goldmark AST.
// Headings inside fenced code blocks are correctly excluded.
func ParseHeadings(source []byte) []Heading {
	doc := MDParse(source)
	var out []Heading
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering || n.Kind() != ast.KindHeading {
			return ast.WalkContinue, nil
		}
		h := n.(*ast.Heading)
		out = append(out, Heading{Level: h.Level, Text: MDInlineText(h, source)})
		return ast.WalkSkipChildren, nil
	})
	return out
}

// ParseSection extracts the raw source bytes for the first heading whose
// text contains query (case-insensitive). The section spans from the heading
// line through all content until the next heading of equal or higher level,
// or end of file.
func ParseSection(source []byte, query string) []byte {
	doc := MDParse(source)
	query = strings.ToLower(strings.TrimLeft(strings.TrimSpace(query), "# \t"))

	type hpos struct {
		level int
		start int // byte offset of the heading line start
	}

	var positions []hpos
	matchIdx := -1

	for n := doc.FirstChild(); n != nil; n = n.NextSibling() {
		h, ok := n.(*ast.Heading)
		if !ok {
			continue
		}
		inner := mdNodeInnerStart(h, source)
		if inner < 0 {
			continue
		}
		lineStart := mdLineStart(source, inner)
		if matchIdx < 0 && strings.Contains(strings.ToLower(MDInlineText(h, source)), query) {
			matchIdx = len(positions)
		}
		positions = append(positions, hpos{level: h.Level, start: lineStart})
	}

	if matchIdx < 0 {
		return nil
	}

	secStart := positions[matchIdx].start
	secLevel := positions[matchIdx].level
	secEnd := len(source)
	for i := matchIdx + 1; i < len(positions); i++ {
		if positions[i].level <= secLevel {
			secEnd = positions[i].start
			break
		}
	}

	return source[secStart:secEnd]
}

// ParseCodeBlocks extracts all fenced code blocks from source.
func ParseCodeBlocks(source []byte) []CodeBlock {
	doc := MDParse(source)
	var out []CodeBlock
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		fcb, ok := n.(*ast.FencedCodeBlock)
		if !ok {
			return ast.WalkContinue, nil
		}
		lang := ""
		if fcb.Info != nil {
			raw := strings.TrimSpace(string(fcb.Info.Segment.Value(source)))
			if i := strings.IndexByte(raw, ' '); i >= 0 {
				lang = raw[:i]
			} else {
				lang = raw
			}
		}
		var sb strings.Builder
		for i := 0; i < fcb.Lines().Len(); i++ {
			seg := fcb.Lines().At(i)
			sb.Write(seg.Value(source))
		}
		out = append(out, CodeBlock{Lang: lang, Content: sb.String()})
		return ast.WalkContinue, nil
	})
	return out
}

// ParseLists extracts top-level lists from source as raw markdown strings.
// Nested lists are not reported separately.
func ParseLists(source []byte) []string {
	doc := MDParse(source)
	var out []string
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering || n.Kind() != ast.KindList {
			return ast.WalkContinue, nil
		}
		if s := mdRawSource(n, source); s != "" {
			out = append(out, s)
		}
		return ast.WalkSkipChildren, nil
	})
	return out
}

// ── low-level AST / source helpers ───────────────────────────────────────────

// mdNodeInnerStart returns the minimum byte offset of any content within n,
// searching both Lines() segments and ast.Text inline segments. Returns -1
// when n contains no content.
func mdNodeInnerStart(n ast.Node, source []byte) int {
	min := -1
	_ = ast.Walk(n, func(child ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if child.Type() == ast.TypeBlock {
			if lines := child.Lines(); lines != nil {
				for i := 0; i < lines.Len(); i++ {
					if s := lines.At(i).Start; min < 0 || s < min {
						min = s
					}
				}
			}
		}
		if t, ok := child.(*ast.Text); ok {
			if s := t.Segment.Start; min < 0 || s < min {
				min = s
			}
		}
		return ast.WalkContinue, nil
	})
	return min
}

// mdNodeInnerStop returns the maximum byte offset (exclusive) of any content
// within n. Returns 0 when n contains no content.
func mdNodeInnerStop(n ast.Node, source []byte) int {
	max := 0
	_ = ast.Walk(n, func(child ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if child.Type() == ast.TypeBlock {
			if lines := child.Lines(); lines != nil {
				for i := 0; i < lines.Len(); i++ {
					if s := lines.At(i).Stop; s > max {
						max = s
					}
				}
			}
		}
		if t, ok := child.(*ast.Text); ok {
			if s := t.Segment.Stop; s > max {
				max = s
			}
		}
		return ast.WalkContinue, nil
	})
	return max
}

// mdLineStart returns the byte offset of the start of the source line
// containing pos (scanning backward for a preceding '\n').
func mdLineStart(source []byte, pos int) int {
	for pos > 0 && source[pos-1] != '\n' {
		pos--
	}
	return pos
}

// mdLineEnd returns the byte offset one past the end of the source line
// containing pos (past '\n', or EOF).
func mdLineEnd(source []byte, pos int) int {
	for pos < len(source) && source[pos] != '\n' {
		pos++
	}
	if pos < len(source) {
		pos++ // past '\n'
	}
	return pos
}

// mdRawSource extracts the verbatim markdown source for block node n by
// finding the full line span from its first to its last content byte.
// For container blocks (blockquote, list) this preserves markers like "> " or "- ".
func mdRawSource(n ast.Node, source []byte) string {
	start := mdNodeInnerStart(n, source)
	stop := mdNodeInnerStop(n, source)
	if start < 0 || stop == 0 || start >= stop {
		return ""
	}
	lineStart := mdLineStart(source, start)
	lineEnd := mdLineEnd(source, stop-1)
	if lineEnd > len(source) {
		lineEnd = len(source)
	}
	return string(source[lineStart:lineEnd])
}
