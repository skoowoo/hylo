package view

import (
	"encoding/json"
	"io/fs"
	"regexp"
	"strings"
	"testing"
)

// Templates and scripts only resolve icon names when a page renders, so a
// typo would surface at runtime; check every literal name against the set.
func TestIconNamesExist(t *testing.T) {
	use := regexp.MustCompile(`\{\{\s*(?:icon|sideIcon)\s+"([^"]+)"|hyloIcon\('([^']+)'`)
	html, err := fs.Glob(assetFS, "assets/*.html")
	if err != nil {
		t.Fatal(err)
	}
	js, err := fs.Glob(assetFS, "assets/*.js")
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, f := range append(html, js...) {
		b, err := fs.ReadFile(assetFS, f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range use.FindAllStringSubmatch(string(b), -1) {
			seen++
			name := m[1] + m[2]
			if _, ok := lineIcons[name]; !ok {
				t.Errorf("%s: no line icon %q", f, name)
			}
		}
	}
	if seen < 50 {
		t.Fatalf("found only %d icon uses; the pattern is out of date", seen)
	}
	for _, tab := range settingsTabs {
		if _, ok := lineIcons[tab.Icon]; !ok {
			t.Errorf("settings tab %s: no line icon %q", tab.ID, tab.Icon)
		}
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
	solid := string(lineIconDef{Paths: "<path/>", Zoom: 1}.svg(`fill="currentColor" stroke="none"`))
	if strings.Count(solid, "fill=") != 1 || strings.Count(solid, "stroke=") != 1 || !strings.Contains(solid, `fill="currentColor"`) {
		t.Errorf("fill/stroke in attrs should replace the line defaults:\n%s", solid)
	}
	if _, err := icon("no-such-icon"); err == nil {
		t.Error("unknown icon name should error")
	}
}

// Page scripts draw icons from assets/shared_icons_line.js, written by the
// generator; it has to match what templates render, icon for icon.
func TestLineIconsJSMatchesTemplates(t *testing.T) {
	b, err := fs.ReadFile(assetFS, "assets/shared_icons_line.js")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`var __hyloLineIcons = (\{.*\});`).FindSubmatch(b)
	if m == nil {
		t.Fatal("no __hyloLineIcons map in shared_icons_line.js")
	}
	var js map[string]string
	if err := json.Unmarshal(m[1], &js); err != nil {
		t.Fatal(err)
	}
	if len(js) != len(lineIcons) {
		t.Errorf("JS has %d icons, Go has %d; run make icons", len(js), len(lineIcons))
	}
	for n, d := range lineIcons {
		if got, want := js[n], string(d.svg("")); got != want {
			t.Errorf("%s differs; run make icons\n js: %s\n go: %s", n, got, want)
		}
	}
}
