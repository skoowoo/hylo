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
}
