package mcp

import (
	"context"
	"strings"

	"github.com/hardhacker/hylo/internal/client"
	"github.com/hardhacker/hylo/internal/notes"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type extractReq struct {
	Path string `json:"path" jsonschema:"vault-absolute path or bare filename"`
}

type outlineReq struct {
	Path   string `json:"path" jsonschema:"vault-absolute path or bare filename"`
	Indent string `json:"indent,omitempty" jsonschema:"string used per indentation level, default two spaces"`
}

type sectionReq struct {
	Path    string `json:"path" jsonschema:"vault-absolute path or bare filename"`
	Heading string `json:"heading" jsonschema:"heading text, matched case-insensitively; the section ends at the next heading of the same or higher level"`
}

type segmentReq struct {
	Path  string `json:"path" jsonschema:"vault-absolute path or bare filename"`
	Head  int    `json:"head,omitempty" jsonschema:"first N lines"`
	Tail  int    `json:"tail,omitempty" jsonschema:"last N lines"`
	Start int    `json:"start,omitempty" jsonschema:"first line (1-based, inclusive); use with end"`
	End   int    `json:"end,omitempty" jsonschema:"last line (1-based, inclusive); use with start"`
}

type headingOut struct {
	Level int    `json:"level"`
	Text  string `json:"text"`
}

type outlineOut struct {
	Headings []headingOut `json:"headings"`
}

type sectionOut struct {
	Content string `json:"content"`
}

type codeBlockOut struct {
	Lang    string `json:"lang"`
	Content string `json:"content"`
}

type codeOut struct {
	Blocks []codeBlockOut `json:"blocks"`
}

type linkOut struct {
	Kind  string `json:"kind" jsonschema:"link, image, autolink or wikilink"`
	Text  string `json:"text,omitempty"`
	URL   string `json:"url"`
	Title string `json:"title,omitempty"`
}

type linksOut struct {
	Links []linkOut `json:"links"`
}

type listsOut struct {
	Lists []string `json:"lists" jsonschema:"top-level lists as raw markdown"`
}

type segmentOut struct {
	Total   int    `json:"total_lines"`
	Content string `json:"content"`
}

type extractTagsOut struct {
	HasFrontMatter bool     `json:"has_front_matter"`
	Tags           []string `json:"tags"`
}

func registerExtractTools(s *sdk.Server, c *client.Client) {
	src := func(p string) ([]byte, error) { return notes.ReadSource(c, p) }

	addTool(s, "extract_outline", "Extract the heading outline of a note.", true,
		func(_ context.Context, in extractReq) (outlineOut, error) {
			b, err := src(in.Path)
			if err != nil {
				return outlineOut{}, err
			}
			hs := client.ParseHeadings(b)
			out := outlineOut{Headings: make([]headingOut, len(hs))}
			for i, h := range hs {
				out.Headings[i] = headingOut{Level: h.Level, Text: h.Text}
			}
			return out, nil
		})

	addTool(s, "extract_section", "Extract a named section from a note.", true,
		func(_ context.Context, in sectionReq) (sectionOut, error) {
			b, err := src(in.Path)
			if err != nil {
				return sectionOut{}, err
			}
			sec, err := notes.Section(b, in.Heading)
			return sectionOut{Content: sec}, err
		})

	addTool(s, "extract_code", "Extract all fenced code blocks from a markdown note.", true,
		func(_ context.Context, in extractReq) (codeOut, error) {
			b, err := src(in.Path)
			if err != nil {
				return codeOut{}, err
			}
			blocks := client.ParseCodeBlocks(b)
			out := codeOut{Blocks: make([]codeBlockOut, len(blocks))}
			for i, blk := range blocks {
				out.Blocks[i] = codeBlockOut{Lang: blk.Lang, Content: blk.Content}
			}
			return out, nil
		})

	addTool(s, "extract_link", "Extract all links from a note.", true,
		func(_ context.Context, in extractReq) (linksOut, error) {
			b, err := src(in.Path)
			if err != nil {
				return linksOut{}, err
			}
			links := client.ParseLinks(b)
			out := linksOut{Links: make([]linkOut, len(links))}
			for i, l := range links {
				out.Links[i] = linkOut{Kind: l.Kind, Text: l.Text, URL: l.URL, Title: l.Title}
			}
			return out, nil
		})

	addTool(s, "extract_list", "Extract all lists from a markdown note.", true,
		func(_ context.Context, in extractReq) (listsOut, error) {
			b, err := src(in.Path)
			if err != nil {
				return listsOut{}, err
			}
			lists := client.ParseLists(b)
			if lists == nil {
				lists = []string{}
			}
			return listsOut{Lists: lists}, nil
		})

	addTool(s, "extract_segment", "Extract a line range from a note: head N, tail N, or start/end.", true,
		func(_ context.Context, in segmentReq) (segmentOut, error) {
			spec := notes.SegmentSpec{Head: in.Head, Tail: in.Tail, Start: in.Start, End: in.End}
			if err := spec.Validate(); err != nil {
				return segmentOut{}, err
			}
			b, err := src(in.Path)
			if err != nil {
				return segmentOut{}, err
			}
			lines, total := notes.Segment(string(b), spec)
			return segmentOut{Total: total, Content: strings.Join(lines, "\n")}, nil
		})

	addTool(s, "extract_tag", "Extract tags from a note's front matter.", true,
		func(_ context.Context, in extractReq) (extractTagsOut, error) {
			b, err := src(in.Path)
			if err != nil {
				return extractTagsOut{}, err
			}
			hasFM, tags := notes.Tags(b)
			return extractTagsOut{HasFrontMatter: hasFM, Tags: tags}, nil
		})
}
