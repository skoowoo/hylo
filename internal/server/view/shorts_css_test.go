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
