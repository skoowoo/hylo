package view

import (
	"testing"
	"unicode/utf8"
)

func TestPreviewCut(t *testing.T) {
	for _, c := range []struct{ name, in, want string }{
		{"last block", "one\n\ntwo\n\nthr", "one\n\ntwo\n"},
		{"blank inside fence", "intro\n\n```\na\n\nb", "intro\n"},
		{"after closed fence", "```\na\n\nb\n```\n\nnext\n\npart", "```\na\n\nb\n```\n\nnext\n"},
		{"no blank line", "line one\nline two\nhalf", "line one\nline two\n"},
		{"split rune", "单行没有换行的长文"[:7], "单行"},
	} {
		got := string(previewCut([]byte(c.in)))
		if got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
		if !utf8.ValidString(got) {
			t.Errorf("%s: invalid UTF-8 %q", c.name, got)
		}
	}
}
