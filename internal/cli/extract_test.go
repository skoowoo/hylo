package cli

import (
	"strings"
	"testing"

	"github.com/hardhacker/hylo/internal/client"
)

// ── mdParseHeadings ───────────────────────────────────────────────────────────

func TestMdParseHeadings(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []heading
	}{
		{
			name:  "empty",
			input: "",
			want:  nil,
		},
		{
			name:  "no headings",
			input: "just a paragraph\n",
			want:  nil,
		},
		{
			name:  "single h1",
			input: "# Hello\n",
			want:  []heading{{1, "Hello"}},
		},
		{
			name:  "multiple levels",
			input: "# Title\n\n## Section\n\n### Sub\n",
			want:  []heading{{1, "Title"}, {2, "Section"}, {3, "Sub"}},
		},
		{
			name:  "heading inside fenced code block is ignored",
			input: "# Real\n\n```\n# Fake\n```\n",
			want:  []heading{{1, "Real"}},
		},
		{
			name:  "inline formatting stripped",
			input: "## Hello **world** and `code`\n",
			want:  []heading{{2, "Hello world and code"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := mdParseHeadings([]byte(tc.input))
			if len(got) != len(tc.want) {
				t.Fatalf("got %d headings, want %d\ngot:  %v\nwant: %v", len(got), len(tc.want), got, tc.want)
			}
			for i, h := range got {
				if h.level != tc.want[i].level || h.text != tc.want[i].text {
					t.Errorf("[%d] got {%d %q}, want {%d %q}", i, h.level, h.text, tc.want[i].level, tc.want[i].text)
				}
			}
		})
	}
}

// ── mdParseSection ────────────────────────────────────────────────────────────

func TestMdParseSection(t *testing.T) {
	doc := `# Intro

Some intro text.

## Goals

- goal one
- goal two

## Details

Detail paragraph.

### Sub-detail

Sub content.

## Conclusion

End.
`
	tests := []struct {
		name        string
		input       string // overrides doc when non-empty
		query       string
		wantNil     bool
		wantContain []string
		wantExclude []string
	}{
		{
			name:    "no match returns nil",
			query:   "nonexistent",
			wantNil: true,
		},
		{
			name:        "query with ## prefix is stripped before matching",
			query:       "## Goals",
			wantContain: []string{"## Goals", "goal one"},
			wantExclude: []string{"## Details"},
		},
		{
			name:        "match by substring (case-insensitive)",
			query:       "GOALS",
			wantContain: []string{"## Goals", "goal one", "goal two"},
			wantExclude: []string{"## Details", "## Conclusion"},
		},
		{
			name:        "section ends at next same-level heading",
			query:       "details",
			wantContain: []string{"## Details", "Detail paragraph.", "### Sub-detail", "Sub content."},
			wantExclude: []string{"## Conclusion"},
		},
		{
			name:        "last section runs to EOF",
			query:       "conclusion",
			wantContain: []string{"## Conclusion", "End."},
		},
		{
			// Only one H1 in the doc, so the section spans to EOF —
			// H2s are sub-sections and are included.
			name:        "h1 section with no peer spans to EOF",
			query:       "intro",
			wantContain: []string{"# Intro", "Some intro text.", "## Goals", "## Conclusion"},
		},
		{
			// Two H1s: first spans only until the second.
			name:        "h1 section ends at next h1",
			input:       "# First\n\nfirst body\n\n# Second\n\nsecond body\n",
			query:       "first",
			wantContain: []string{"# First", "first body"},
			wantExclude: []string{"# Second", "second body"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src := doc
			if tc.input != "" {
				src = tc.input
			}
			got := mdParseSection([]byte(src), tc.query)
			if tc.wantNil {
				if got != nil {
					t.Fatalf("expected nil, got %q", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil section, got nil")
			}
			s := string(got)
			for _, sub := range tc.wantContain {
				if !strings.Contains(s, sub) {
					t.Errorf("section missing %q\ngot: %s", sub, s)
				}
			}
			for _, sub := range tc.wantExclude {
				if strings.Contains(s, sub) {
					t.Errorf("section should not contain %q\ngot: %s", sub, s)
				}
			}
		})
	}
}

// ── mdParseCodeBlocks ─────────────────────────────────────────────────────────

func TestMdParseCodeBlocks(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []codeBlock
	}{
		{
			name:  "empty",
			input: "",
			want:  nil,
		},
		{
			name:  "no code blocks",
			input: "just text\n",
			want:  nil,
		},
		{
			name:  "single block no lang",
			input: "```\nhello\n```\n",
			want:  []codeBlock{{"", "hello\n"}},
		},
		{
			name:  "single block with lang",
			input: "```go\nfmt.Println()\n```\n",
			want:  []codeBlock{{"go", "fmt.Println()\n"}},
		},
		{
			name:  "lang with metadata after space is trimmed",
			input: "```go run\ncode\n```\n",
			want:  []codeBlock{{"go", "code\n"}},
		},
		{
			name:  "multiple blocks",
			input: "```python\nprint()\n```\n\ntext\n\n```sh\necho hi\n```\n",
			want: []codeBlock{
				{"python", "print()\n"},
				{"sh", "echo hi\n"},
			},
		},
		{
			name:  "indented code block (non-fenced) is not extracted",
			input: "    indented code\n",
			want:  nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := mdParseCodeBlocks([]byte(tc.input))
			if len(got) != len(tc.want) {
				t.Fatalf("got %d blocks, want %d\ngot:  %v\nwant: %v", len(got), len(tc.want), got, tc.want)
			}
			for i, b := range got {
				if b.lang != tc.want[i].lang {
					t.Errorf("[%d] lang: got %q, want %q", i, b.lang, tc.want[i].lang)
				}
				if b.content != tc.want[i].content {
					t.Errorf("[%d] content: got %q, want %q", i, b.content, tc.want[i].content)
				}
			}
		})
	}
}

// ── mdParseLists ──────────────────────────────────────────────────────────────

func TestMdParseLists(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantCount int
		wantTexts []string // substrings expected in each list (by index)
	}{
		{
			name:      "empty",
			input:     "",
			wantCount: 0,
		},
		{
			name:      "no lists",
			input:     "just a paragraph\n",
			wantCount: 0,
		},
		{
			name:      "unordered list",
			input:     "- apple\n- banana\n- cherry\n",
			wantCount: 1,
			wantTexts: []string{"apple"},
		},
		{
			name:      "ordered list",
			input:     "1. first\n2. second\n",
			wantCount: 1,
			wantTexts: []string{"first"},
		},
		{
			name:      "two separate lists",
			input:     "- a\n- b\n\ntext\n\n- c\n- d\n",
			wantCount: 2,
		},
		{
			name:      "nested list counts as one",
			input:     "- item\n  - nested\n  - nested2\n",
			wantCount: 1,
			wantTexts: []string{"nested"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := mdParseLists([]byte(tc.input))
			if len(got) != tc.wantCount {
				t.Fatalf("got %d lists, want %d\ngot: %v", len(got), tc.wantCount, got)
			}
			for i, sub := range tc.wantTexts {
				if i >= len(got) {
					break
				}
				if !strings.Contains(got[i], sub) {
					t.Errorf("[%d] expected %q in list text\ngot: %s", i, sub, got[i])
				}
			}
		})
	}
}

// ── client.ParseLinks ─────────────────────────────────────────────────────────

func TestParseLinks(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []client.Link
	}{
		{
			name:  "empty",
			input: "",
			want:  nil,
		},
		{
			name:  "no links",
			input: "just text\n",
			want:  nil,
		},
		{
			name:  "inline link",
			input: "[Click here](https://example.com)\n",
			want:  []client.Link{{Kind: "link", Text: "Click here", URL: "https://example.com"}},
		},
		{
			name:  "inline link with title",
			input: `[Docs](https://docs.example.com "Documentation")` + "\n",
			want:  []client.Link{{Kind: "link", Text: "Docs", URL: "https://docs.example.com", Title: "Documentation"}},
		},
		{
			name:  "image",
			input: "![Alt text](/img/logo.png)\n",
			want:  []client.Link{{Kind: "image", Text: "Alt text", URL: "/img/logo.png"}},
		},
		{
			name:  "image with title",
			input: `![Logo](/img/logo.png "Site Logo")` + "\n",
			want:  []client.Link{{Kind: "image", Text: "Logo", URL: "/img/logo.png", Title: "Site Logo"}},
		},
		{
			name:  "autolink",
			input: "<https://auto.example.com>\n",
			want:  []client.Link{{Kind: "autolink", URL: "https://auto.example.com"}},
		},
		{
			name:  "mixed",
			input: "[a](https://a.com)\n\n![img](/b.png)\n\n<https://c.com>\n",
			want: []client.Link{
				{Kind: "link", Text: "a", URL: "https://a.com"},
				{Kind: "image", Text: "img", URL: "/b.png"},
				{Kind: "autolink", URL: "https://c.com"},
			},
		},
		{
			name:  "link inside heading",
			input: "## See [docs](https://docs.com)\n",
			want:  []client.Link{{Kind: "link", Text: "docs", URL: "https://docs.com"}},
		},
		// wiki links
		{
			name:  "wiki link basic",
			input: "See [[SomePage]].\n",
			want:  []client.Link{{Kind: "wikilink", URL: "SomePage"}},
		},
		{
			name:  "wiki link with alias",
			input: "See [[SomePage|Display Name]].\n",
			want:  []client.Link{{Kind: "wikilink", URL: "SomePage", Text: "Display Name"}},
		},
		{
			name:  "wiki link with section anchor",
			input: "[[Guide#Installation]]\n",
			want:  []client.Link{{Kind: "wikilink", URL: "Guide#Installation"}},
		},
		{
			name:  "wiki link with alias and anchor",
			input: "[[Guide#Installation|Install Guide]]\n",
			want:  []client.Link{{Kind: "wikilink", URL: "Guide#Installation", Text: "Install Guide"}},
		},
		{
			name:  "wiki link does not shadow regular link",
			input: "[normal](https://x.com) and [[WikiPage]]\n",
			want: []client.Link{
				{Kind: "link", Text: "normal", URL: "https://x.com"},
				{Kind: "wikilink", URL: "WikiPage"},
			},
		},
		{
			name:  "multiple wiki links",
			input: "[[PageA]] and [[PageB|B alias]]\n",
			want: []client.Link{
				{Kind: "wikilink", URL: "PageA"},
				{Kind: "wikilink", URL: "PageB", Text: "B alias"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := client.ParseLinks([]byte(tc.input))
			if len(got) != len(tc.want) {
				t.Fatalf("got %d links, want %d\ngot:  %v\nwant: %v", len(got), len(tc.want), got, tc.want)
			}
			for i, l := range got {
				w := tc.want[i]
				if l.Kind != w.Kind {
					t.Errorf("[%d] kind: got %q, want %q", i, l.Kind, w.Kind)
				}
				if l.URL != w.URL {
					t.Errorf("[%d] url: got %q, want %q", i, l.URL, w.URL)
				}
				if l.Text != w.Text {
					t.Errorf("[%d] text: got %q, want %q", i, l.Text, w.Text)
				}
				if l.Title != w.Title {
					t.Errorf("[%d] title: got %q, want %q", i, l.Title, w.Title)
				}
			}
		})
	}
}

func TestParseRemoteImageURLs(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{name: "empty", input: "", want: nil},
		{
			name:  "markdown image https",
			input: "![](https://example.com/a.png)\n",
			want:  []string{"https://example.com/a.png"},
		},
		{
			name:  "skip relative image",
			input: "![](assets/local.png)\n",
			want:  nil,
		},
		{
			name:  "dedupe",
			input: "![](https://x.com/i.jpg)\n\n![](https://x.com/i.jpg)\n",
			want:  []string{"https://x.com/i.jpg"},
		},
		{
			name:  "autolink with image ext",
			input: "<https://cdn.example/photo.webp>\n",
			want:  []string{"https://cdn.example/photo.webp"},
		},
		{
			name:  "autolink html page skipped",
			input: "<https://example.com/page>\n",
			want:  nil,
		},
		{
			name:  "inline link to png",
			input: "[shot](https://ex.com/cap.PNG?q=1)\n",
			want:  []string{"https://ex.com/cap.PNG?q=1"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := client.ParseRemoteImageURLs([]byte(tc.input))
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("[%d] got %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}
