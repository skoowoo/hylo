package channels

import (
	"fmt"
	"html"
	"regexp"
	"strings"

	gm "github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

// The apps get plain text: Telegram and WeChat show markdown as typed, and
// Discord has no wiki links or cards. One conversion keeps them alike.
var mdParser = gm.New(gm.WithExtensions(extension.GFM)).Parser()

var (
	reWikilink   = regexp.MustCompile(`!?\[\[([^\]\[|]+?)(?:\|([^\]\[]+?))?\]\]`)
	reHighlight  = regexp.MustCompile(`==([^=\n]+)==`)
	reEscape     = regexp.MustCompile("\\\\([!-/:-@\\[-`{-~])")
	reBreakTag   = regexp.MustCompile(`(?i)^<br\s*/?>$`)
	reHTMLTag    = regexp.MustCompile(`<[^>]+>`)
	reBlankLines = regexp.MustCompile(`\n{3,}`)
)

// What reads badly on a phone is left on the desktop, with a line saying so.
const (
	deskTable   = "[Table: view on desktop]"
	deskCode    = "[Code: view on desktop]"
	deskDiagram = "[Diagram: view on desktop]"
	deskMindmap = "[Mind map: view on desktop]"
	deskContent = "[Content: view on desktop]"
)

// plainText turns a reply's markdown into text read top to bottom: lists stay
// lists, links become bare URLs, cards become the text they show, and tables,
// code and diagrams become a pointer to the desktop.
func plainText(md string) string {
	src := []byte(md)
	doc := mdParser.Parse(text.NewReader(src))
	r := plainRenderer{src: src}
	out := strings.Join(r.blocks(doc), "\n\n")
	lines := strings.Split(out, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t")
	}
	return strings.TrimSpace(reBlankLines.ReplaceAllString(strings.Join(lines, "\n"), "\n\n"))
}

type plainRenderer struct{ src []byte }

// blocks renders each child block of n to its text, dropping empty ones.
func (r plainRenderer) blocks(n ast.Node) []string {
	var out []string
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if s := r.block(c); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func (r plainRenderer) block(n ast.Node) string {
	switch n := n.(type) {
	case *ast.Paragraph, *ast.TextBlock, *ast.Heading:
		return r.inline(n)
	case *ast.List:
		return r.list(n)
	case *ast.Blockquote:
		lines := strings.Split(strings.Join(r.blocks(n), "\n\n"), "\n")
		for i, l := range lines {
			lines[i] = strings.TrimRight("> "+l, " ")
		}
		return strings.Join(lines, "\n")
	case *ast.FencedCodeBlock:
		lang := strings.ToLower(string(n.Language(r.src)))
		if card, ok := cardText(lang, r.lines(n)); ok {
			return card
		}
		if lang == "mermaid" {
			return deskDiagram
		}
		return deskCode
	case *ast.CodeBlock:
		return deskCode
	case *ast.HTMLBlock:
		if strings.TrimSpace(reHTMLTag.ReplaceAllString(r.lines(n), "")) == "" {
			return ""
		}
		return deskContent
	case *east.Table:
		return deskTable
	case *ast.ThematicBreak:
		return ""
	default:
		return strings.Join(r.blocks(n), "\n\n")
	}
}

func (r plainRenderer) lines(n ast.Node) string {
	var b strings.Builder
	segs := n.Lines()
	for i := 0; i < segs.Len(); i++ {
		seg := segs.At(i)
		b.Write(seg.Value(r.src))
	}
	return b.String()
}

// list keeps the list as text: its markers, numbering and nesting, with a box
// for a task. Lines after an item's first are indented under its text.
func (r plainRenderer) list(n *ast.List) string {
	var items []string
	num := n.Start
	for it := n.FirstChild(); it != nil; it = it.NextSibling() {
		marker := "- "
		if n.IsOrdered() {
			marker = fmt.Sprintf("%d. ", num)
			num++
		}
		if box := taskBox(it); box != nil {
			mark := "☐ "
			if box.IsChecked {
				mark = "☑ "
			}
			if n.IsOrdered() {
				marker += mark
			} else {
				marker = mark
			}
		}
		parts := r.blocks(it)
		sep := "\n"
		if !n.IsTight {
			sep = "\n\n"
		}
		lines := strings.Split(strings.Join(parts, sep), "\n")
		pad := strings.Repeat(" ", len([]rune(marker)))
		for i, l := range lines {
			if i == 0 {
				lines[i] = marker + l
			} else if l != "" {
				lines[i] = pad + l
			}
		}
		items = append(items, strings.Join(lines, "\n"))
	}
	if n.IsTight {
		return strings.Join(items, "\n")
	}
	return strings.Join(items, "\n\n")
}

func taskBox(item ast.Node) *east.TaskCheckBox {
	if first := item.FirstChild(); first != nil {
		if box, ok := first.FirstChild().(*east.TaskCheckBox); ok {
			return box
		}
	}
	return nil
}

// inline flattens n's inline children. Wiki links, highlights and escapes are
// read only in prose, never inside code or a URL.
func (r plainRenderer) inline(n ast.Node) string {
	var out, prose strings.Builder
	flush := func() {
		s := reWikilink.ReplaceAllStringFunc(prose.String(), wikiLabel)
		s = reHighlight.ReplaceAllString(s, "$1")
		s = reEscape.ReplaceAllString(s, "$1")
		out.WriteString(html.UnescapeString(s))
		prose.Reset()
	}
	literal := func(s string) {
		flush()
		out.WriteString(s)
	}
	var walk func(ast.Node)
	walk = func(n ast.Node) {
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			switch c := c.(type) {
			case *ast.Text:
				prose.Write(c.Segment.Value(r.src))
				if c.HardLineBreak() || c.SoftLineBreak() {
					prose.WriteByte('\n')
				}
			case *ast.String:
				prose.Write(c.Value)
			case *ast.CodeSpan:
				literal(r.plain(c))
			case *ast.AutoLink:
				literal(string(c.URL(r.src)))
			case *ast.Link:
				literal(withURL(r.inline(c), string(c.Destination)))
			case *ast.Image:
				literal(withURL(r.inline(c), string(c.Destination)))
			case *ast.RawHTML:
				var tag strings.Builder
				for i := 0; i < c.Segments.Len(); i++ {
					seg := c.Segments.At(i)
					tag.Write(seg.Value(r.src))
				}
				if reBreakTag.MatchString(tag.String()) {
					prose.WriteByte('\n')
				}
			case *east.TaskCheckBox:
			default:
				walk(c)
			}
		}
	}
	walk(n)
	flush()
	return strings.TrimSpace(out.String())
}

// plain is a code span's text as written.
func (r plainRenderer) plain(n ast.Node) string {
	var b strings.Builder
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if t, ok := c.(*ast.Text); ok {
			b.Write(t.Segment.Value(r.src))
		}
	}
	return b.String()
}

// withURL writes a link as its label then its URL; a label that is the URL goes once.
func withURL(label, url string) string {
	switch {
	case url == "" || strings.HasPrefix(url, "#"):
		return label
	case label == "" || label == url || strings.TrimPrefix(strings.TrimPrefix(url, "mailto:"), "tel:") == label:
		return url
	default:
		return label + " " + url
	}
}

// wikiLabel shows a note link as the name a reader sees: the alias, else the note.
func wikiLabel(m string) string {
	g := reWikilink.FindStringSubmatch(m)
	if g[2] != "" {
		return strings.TrimSpace(g[2])
	}
	return strings.TrimSuffix(strings.TrimSpace(g[1]), ".md")
}
