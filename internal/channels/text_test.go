package channels

import (
	"reflect"
	"strings"
	"testing"
)

func TestSplitText(t *testing.T) {
	cases := []struct {
		text  string
		limit int
		want  []string
	}{
		{"aaaa\nbbbb", 5, []string{"aaaa", "bbbb"}},
		{"short", 10, []string{"short"}},
		{"abcdef", 4, []string{"abcd", "ef"}},
		// CJK must not be cut mid-rune.
		{"你好世界再见", 4, []string{"你好世界", "再见"}},
		// An emoji takes two UTF-16 units.
		{"ab😀cd", 3, []string{"ab", "😀c", "d"}},
	}
	for _, c := range cases {
		if got := splitText(c.text, c.limit); !reflect.DeepEqual(got, c.want) {
			t.Errorf("splitText(%q, %d) = %q, want %q", c.text, c.limit, got, c.want)
		}
	}
}

func TestPlainText(t *testing.T) {
	got := plainText("**bold** and [link](https://x.com)")
	if strings.Contains(got, "**") || !strings.Contains(got, "link (https://x.com)") {
		t.Fatalf("got %q", got)
	}
	u := "https://en.wikipedia.org/wiki/Rust_(programming_language)"
	if got := plainText("_see_ " + u + " and https://x.com/a_b_c*d*"); got != "see "+u+" and https://x.com/a_b_c*d*" {
		t.Fatalf("url mangled: %q", got)
	}
}

func TestUnwrapCards(t *testing.T) {
	in := "Today:\n\n```embed\nhttps://a.example/1\nhttps://a.example/2\n```\n\nWhy it matters.\n\n" +
		"```choices\n- Yes, go ahead\n- [ ] Not now\n```\n\n" +
		"```sources\nnote: RAG pitfalls\nskill: hylo-writing\nswap: Redo it\n```\n\n" +
		"```receipt\ntitle: Daily briefing is on\n- Runs at 08:00\nundo: Undo it\nids: trigger 9f/a1\n```\n\n" +
		"```go\nfmt.Println(1)\n```"
	want := "Today:\n\nhttps://a.example/1\nhttps://a.example/2\n\nWhy it matters.\n\n" +
		"- Yes, go ahead\n- Not now\n\n" +
		"- RAG pitfalls\n\n" +
		"Daily briefing is on\n- Runs at 08:00\n\n" +
		"```go\nfmt.Println(1)\n```"
	if got := unwrapCards(in); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
	open := "```embed\nhttps://a.example/1"
	if got := unwrapCards(open); got != open {
		t.Fatalf("unclosed fence changed: %q", got)
	}
}
