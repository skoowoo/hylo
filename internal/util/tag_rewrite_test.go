package util

import "testing"

func TestRewriteTag(t *testing.T) {
	cases := []struct {
		name, src, from, to, want string
	}{
		{"inline delete keeps the word", "今天 #读书 了，#Reading too\n", "读书", "", "今天 读书 了，#Reading too\n"},
		{"inline case-insensitive", "a #Reading b\n", "reading", "", "a Reading b\n"},
		{"inline rename", "a #读书 b\n", "读书", "books", "a #books b\n"},
		{"inline exact tag only", "#读书/笔记 #读书x `#读书`\n", "读书", "", "#读书/笔记 #读书x `#读书`\n"},
		{"block item dropped", "---\ntitle: x\ntags:\n  - a\n  - b\n---\nbody\n", "a", "", "---\ntitle: x\ntags:\n  - b\n---\nbody\n"},
		{"block last item drops key", "---\ntags:\n  - a\ntitle: x\n---\n", "a", "", "---\ntitle: x\n---\n"},
		{"block rename", "---\ntags:\n  - a # note\n  - b\n---\n", "A", "c", "---\ntags:\n  - c\n  - b\n---\n"},
		{"block rename merges", "---\ntags:\n  - a\n  - b\n---\n#a\n", "a", "b", "---\ntags:\n  - b\n---\n#b\n"},
		{"flow", "---\ntags: [a, \"#Reading\", b]\n---\n", "reading", "", "---\ntags: [a, b]\n---\n"},
		{"flow rename quotes", "---\ntags: [a]\n---\n", "a", "c++", "---\ntags: [\"c++\"]\n---\n"},
		{"only field drops front matter", "---\ntags:\n  - a\n---\nbody\n", "a", "", "body\n"},
		{"flow last drops key", "---\nkind: x\ntags: [a]\n---\n", "a", "", "---\nkind: x\n---\n"},
		{"scalar", "---\ntags: a, b\n---\n", "b", "c", "---\ntags: a, c\n---\n"},
		{"quoted scalar stays quoted", "---\ntags: \"#a, #b\"\n---\n", "a", "", "---\ntags: \"#b\"\n---\n"},
		{"spaced tag", "---\ntags: [indie dev, b]\n---\n", "indie-dev", "", "---\ntags: [b]\n---\n"},
		{"front matter value elsewhere untouched", "---\ntitle: a\ntags: [b]\n---\n", "a", "", "---\ntitle: a\ntags: [b]\n---\n"},
	}
	for _, c := range cases {
		got, changed := RewriteTag([]byte(c.src), c.from, c.to)
		if string(got) != c.want {
			t.Errorf("%s:\ngot  %q\nwant %q", c.name, got, c.want)
		}
		if changed != (c.src != c.want) {
			t.Errorf("%s: changed = %v", c.name, changed)
		}
	}
}

func TestRewriteTagLeavesNoTrace(t *testing.T) {
	src := "---\ntags: [x, Y]\n---\n#y and #x\n\n## Head #y\n"
	out, _ := RewriteTag([]byte(src), "y", "")
	for _, tag := range ExtractTags(out).All() {
		if tag == "y" {
			t.Fatalf("y survived: %q", out)
		}
	}
}
