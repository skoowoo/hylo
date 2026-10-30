package util

import (
	"bytes"
	"fmt"
	"html"
	"unicode"
	"unicode/utf8"

	gm "github.com/yuin/goldmark"
	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	gmhtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// hashTagNode is an inline #tag; goldmark never calls inline parsers inside
// code or HTML, which is what tagSkipRanges reproduces for InlineTags.
type hashTagNode struct {
	gast.BaseInline
	tag []byte // as written, without '#'
}

var kindHashTag = gast.NewNodeKind("HashTag")

func (n *hashTagNode) Kind() gast.NodeKind { return kindHashTag }

func (n *hashTagNode) Dump(source []byte, level int) {
	gast.DumpHelper(n, source, level, map[string]string{"Tag": string(n.tag)}, nil)
}

type hashTagParser struct{}

func (hashTagParser) Trigger() []byte { return []byte{'#'} }

func (hashTagParser) Parse(_ gast.Node, block text.Reader, _ parser.Context) gast.Node {
	if prev := block.PrecendingCharacter(); !unicode.IsSpace(prev) {
		return nil
	}
	line, _ := block.PeekLine()
	end := 1
	for end < len(line) {
		r, size := utf8.DecodeRune(line[end:])
		if !isTagRune(r) {
			break
		}
		end += size
	}
	tag := trimTagRun(string(line[1:end]))
	if tag == "" {
		return nil
	}
	block.Advance(1 + len(tag))
	return &hashTagNode{tag: []byte(tag)}
}

type hashTagRenderer struct{}

func (hashTagRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(kindHashTag, func(w util.BufWriter, _ []byte, n gast.Node, entering bool) (gast.WalkStatus, error) {
		if entering {
			tag := string(n.(*hashTagNode).tag)
			fmt.Fprintf(w, `<span class="hashtag" data-tag="%s">#%s</span>`, html.EscapeString(NormalizeTag(tag)), html.EscapeString(tag))
		}
		return gast.WalkSkipChildren, nil
	})
}

type hashTagExtension struct{}

func (hashTagExtension) Extend(m gm.Markdown) {
	m.Parser().AddOptions(parser.WithInlineParsers(util.Prioritized(hashTagParser{}, 500)))
	m.Renderer().AddOptions(renderer.WithNodeRenderers(util.Prioritized(hashTagRenderer{}, 500)))
}

var mdHTMLTags = gm.New(
	gm.WithExtensions(extension.GFM, hashTagExtension{}),
	gm.WithParserOptions(parser.WithAutoHeadingID()),
	gm.WithRendererOptions(gmhtml.WithUnsafe()),
)

// MarkdownToHTMLFragmentTags is MarkdownToHTMLFragment with each #tag as
// <span class="hashtag" data-tag="<normalized>">, for pages that filter by it.
func MarkdownToHTMLFragmentTags(src []byte) ([]byte, error) {
	src = expandWikiImages(src)
	src = expandWikilinks(src)
	var buf bytes.Buffer
	if err := mdHTMLTags.Convert(src, &buf); err != nil {
		return nil, fmt.Errorf("render markdown: %w", err)
	}
	return buf.Bytes(), nil
}
