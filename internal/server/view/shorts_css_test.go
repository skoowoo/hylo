package view

import (
	"regexp"
	"testing"
)

// An opacity/filter/transform animation composites its box; at a masonry
// card's fractional y WebKit then draws it, and whatever overlaps it, a device
// pixel off until the animation ends, so hovering a short visibly shakes.
func TestShortsTransitionsDontComposite(t *testing.T) {
	re := regexp.MustCompile(`transition[a-z-]*:[^;}]*\b(opacity|filter|transform|all)\b`)
	if m := re.FindString(readAsset("shorts.css")); m != "" {
		t.Fatalf("shorts.css animates a compositing property: %q", m)
	}
}

// The same holds for note card covers and the card controls drawn over them.
func TestCardControlsDontComposite(t *testing.T) {
	rule := regexp.MustCompile(`[^{}]*(note-card-cover|card-reveal|card-more|card-menu)[^{}]*\{[^}]*\}`)
	anim := regexp.MustCompile(`transition[a-z-]*:[^;}]*\b(opacity|filter|transform|all)\b|\b(transform|filter):`)
	for _, f := range []string{"home.css", "base.css", "cselect.css"} {
		for _, r := range rule.FindAllString(readAsset(f), -1) {
			if m := anim.FindString(r); m != "" {
				t.Errorf("%s: %q in %s", f, m, r)
			}
		}
	}
}
