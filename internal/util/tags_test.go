package util

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// testdata/tags.json is shared with editor/test so both sides agree on what a #tag is.
func TestInlineTagsFixture(t *testing.T) {
	raw, err := os.ReadFile("testdata/tags.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		MD   string   `json:"md"`
		Tags []string `json:"tags"`
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		got := ExtractTags([]byte(c.MD)).Inline
		if !reflect.DeepEqual(got, c.Tags) {
			t.Errorf("%q: got %q, want %q", c.MD, got, c.Tags)
		}
	}
}

func TestExtractTagsFrontmatter(t *testing.T) {
	cases := []struct {
		md   string
		want []string
	}{
		{"---\ntags: [AI, indie dev]\n---\n#ai #new", []string{"ai", "indie-dev", "new"}},
		{"---\ntags:\n  - \"#Reading\"\n---\n", []string{"reading"}},
		{"---\ntags: a, b c\n---\n", []string{"a", "b", "c"}},
		{"---\ntitle: x\n---\n#body", []string{"body"}},
		{"---\ntags: [x]\n---\n", []string{"x"}},
	}
	for _, c := range cases {
		if got := ExtractTags([]byte(c.md)).All(); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%q: got %q, want %q", c.md, got, c.want)
		}
	}
}

func TestValidTag(t *testing.T) {
	for tag, want := range map[string]bool{
		"books": true, "读书": true, "项目/hylo": true, "12-3": true,
		"": false, "c++": false, "2024": false, "a/": false, "/a": false, "a//b": false,
	} {
		if got := ValidTag(tag); got != want {
			t.Errorf("ValidTag(%q) = %v, want %v", tag, got, want)
		}
	}
}
