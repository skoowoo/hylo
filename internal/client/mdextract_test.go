package client

import (
	"strings"
	"testing"
)

// ── ParseHeadings ───────────────────────────────────────────────────────────

func TestParseHeadings(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []Heading
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
			want:  []Heading{{1, "Hello"}},
		},
		{
			name:  "multiple levels",
			input: "# Title\n\n## Section\n\n### Sub\n",
			want:  []Heading{{1, "Title"}, {2, "Section"}, {3, "Sub"}},
		},
		{
			name:  "heading inside fenced code block is ignored",
			input: "# Real\n\n```\n# Fake\n```\n",
			want:  []Heading{{1, "Real"}},
		},
		{
			name:  "inline formatting stripped",
			input: "## Hello **world** and `code`\n",
			want:  []Heading{{2, "Hello world and code"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseHeadings([]byte(tc.input))
			if len(got) != len(tc.want) {
				t.Fatalf("got %d headings, want %d\ngot:  %v\nwant: %v", len(got), len(tc.want), got, tc.want)
			}
			for i, h := range got {
				if h.Level != tc.want[i].Level || h.Text != tc.want[i].Text {
					t.Errorf("[%d] got {%d %q}, want {%d %q}", i, h.Level, h.Text, tc.want[i].Level, tc.want[i].Text)
				}
			}
		})
	}
}

// ── ParseSection ────────────────────────────────────────────────────────────

func TestParseSection(t *testing.T) {
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
			got := ParseSection([]byte(src), tc.query)
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

// ── ParseCodeBlocks ─────────────────────────────────────────────────────────

func TestParseCodeBlocks(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []CodeBlock
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
			want:  []CodeBlock{{"", "hello\n"}},
		},
		{
			name:  "single block with lang",
			input: "```go\nfmt.Println()\n```\n",
			want:  []CodeBlock{{"go", "fmt.Println()\n"}},
		},
		{
			name:  "lang with metadata after space is trimmed",
			input: "```go run\ncode\n```\n",
			want:  []CodeBlock{{"go", "code\n"}},
		},
		{
			name:  "multiple blocks",
			input: "```python\nprint()\n```\n\ntext\n\n```sh\necho hi\n```\n",
			want: []CodeBlock{
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
			got := ParseCodeBlocks([]byte(tc.input))
			if len(got) != len(tc.want) {
				t.Fatalf("got %d blocks, want %d\ngot:  %v\nwant: %v", len(got), len(tc.want), got, tc.want)
			}
			for i, b := range got {
				if b.Lang != tc.want[i].Lang {
					t.Errorf("[%d] lang: got %q, want %q", i, b.Lang, tc.want[i].Lang)
				}
				if b.Content != tc.want[i].Content {
					t.Errorf("[%d] content: got %q, want %q", i, b.Content, tc.want[i].Content)
				}
			}
		})
	}
}

// ── ParseLists ──────────────────────────────────────────────────────────────

func TestParseLists(t *testing.T) {
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
			got := ParseLists([]byte(tc.input))
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
