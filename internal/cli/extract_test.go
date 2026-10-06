package cli

import (
	"testing"

	"github.com/hardhacker/hylo/internal/client"
)

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
