package view

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"
)

// Templates only resolve icon names when a page renders, so a typo would
// surface as a runtime error; check every literal name against the set.
func TestTemplateIconNamesExist(t *testing.T) {
	use := regexp.MustCompile(`\{\{\s*(?:icon|sideIcon)\s+"([^"]+)"`)
	files, err := fs.Glob(assetFS, "assets/*.html")
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, f := range files {
		b, err := fs.ReadFile(assetFS, f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range use.FindAllStringSubmatch(string(b), -1) {
			seen++
			if _, ok := lineIcons[m[1]]; !ok {
				t.Errorf("%s: no line icon %q", f, m[1])
			}
		}
	}
	if seen == 0 {
		t.Fatal("found no icon uses; the pattern is out of date")
	}
}

func TestLineIconZoomKeepsCentreAndStroke(t *testing.T) {
	got := string(lineIconDef{Paths: "<path/>", Zoom: 1.08}.svg("x"))
	for _, want := range []string{
		`class="x"`,
		`stroke-width="1.852"`,
		`viewBox="0.889 0.889 22.222 22.222"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("svg missing %s:\n%s", want, got)
		}
	}
	if _, err := icon("no-such-icon"); err == nil {
		t.Error("unknown icon name should error")
	}
}
