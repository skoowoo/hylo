package channels

import (
	"reflect"
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
	u := "https://en.wikipedia.org/wiki/Rust_(programming_language)"
	cases := []struct{ in, want string }{
		{"**bold**, *it* and ~~gone~~", "bold, it and gone"},
		{"[link](https://x.com) and <https://y.com>", "link https://x.com and https://y.com"},
		{"[https://x.com](https://x.com)", "https://x.com"},
		{"![chart](https://x.com/a.png)", "chart https://x.com/a.png"},
		{"_see_ " + u + " and https://x.com/a_b_c", "see " + u + " and https://x.com/a_b_c"},
		{"[[RAG pitfalls]], [[a/Plan.md|the plan]] and ==this==", "RAG pitfalls, the plan and this"},
		{"`[[ -f x ]] && a_b` \\*not em\\* &amp;", "[[ -f x ]] && a_b *not em* &"},
		{"## Title\n\nline one\nline two", "Title\n\nline one\nline two"},
		{"> quoted\n> more", "> quoted\n> more"},
		{"Totals:\n\n| a | b |\n|---|---|\n| 1 | 2 |", "Totals:\n\n[Table: view on desktop]"},
		{"before\n\n---\n\nafter", "before\n\nafter"},
		{"Run:\n\n```sh\nmake build\n```\n\n    indented", "Run:\n\n[Code: view on desktop]\n\n[Code: view on desktop]"},
		{"```mermaid\ngraph TD; A-->B\n```", "[Diagram: view on desktop]"},
		{"<details><summary>More</summary>x</details>\n\n<!-- note -->\n\nafter", "[Content: view on desktop]\n\nafter"},
	}
	for _, c := range cases {
		if got := plainText(c.in); got != c.want {
			t.Errorf("plainText(%q)\n got %q\nwant %q", c.in, got, c.want)
		}
	}
}

func TestPlainTextKeepsLists(t *testing.T) {
	in := "1. First\n2. Second\n   - **sub** a\n   - [ ] todo\n   - [x] done\n3. Third\n\n* star\n+ plus"
	want := "1. First\n2. Second\n   - sub a\n   ☐ todo\n   ☑ done\n3. Third\n\n- star\n\n- plus"
	if got := plainText(in); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
	if got := plainText("3. three\n4. four"); got != "3. three\n4. four" {
		t.Fatalf("start number lost: %q", got)
	}
}

func TestPlainTextCards(t *testing.T) {
	in := "Today:\n\n```embed\nhttps://a.example/1\nhttps://a.example/2\n```\n\nWhy it matters.\n\n" +
		"```choices\n- Yes, **go** ahead\n- Not now\n```\n\n" +
		"```choices\n- [ ] Tea\n- [ ] Coffee\n```\n\n" +
		"```sources\nnote: RAG pitfalls\nnote: [[Vector store notes]]\nskill: hylo-writing\nswap: Redo it\n```\n\n" +
		"```proposal\ntitle: Daily AI briefing\nsummary: Every day at 08:00\ndetails: Main bot · daily 08:00\n- Go ahead\n- Not now\n```\n\n" +
		"```receipt\ntitle: Daily briefing is on\n- Runs at 08:00\nundo: Undo it\nids: trigger 9f/a1\n```\n\n" +
		"```mindmap\n# Pricing\n## Value metric\n- per seat\n- per usage\n  - API calls\n\nignored paragraph\n## Packaging\n- Free\n```\n\n" +
		"- item\n\n  ```choices\n  - nested\n  ```"
	want := "Today:\n\nhttps://a.example/1\nhttps://a.example/2\n\nWhy it matters.\n\n" +
		"- Yes, go ahead\n- Not now\n\n" +
		"☐ Tea\n☐ Coffee\n\n" +
		"📝 RAG pitfalls · Vector store notes  ✨ hylo-writing\n\n" +
		"Daily AI briefing\nEvery day at 08:00\nMain bot · daily 08:00\n- Go ahead\n- Not now\n\n" +
		"✓ Daily briefing is on\n- Runs at 08:00\n↩ Undo it\n\n" +
		"[Mind map: view on desktop]\n\n" +
		"- item\n\n  - nested"
	if got := plainText(in); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}
