package view

import (
	"io/fs"
	"regexp"
	"strings"
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

// Opacity fades elsewhere hit the same WebKit redraw wherever the faded box
// sits off whole pixels (text flow, hover controls), so a dim is a colour and
// a reveal is instant. Only these fade, each on whole pixels or mid-motion.
func TestOpacityFadesStayOnWholePixels(t *testing.T) {
	allowed := []string{".lb-enter", ".lb-leave", ".graph-tooltip", ".home-side-children-inner", ".note-card.is-dragging", ".note-card.is-move-success"}
	rule := regexp.MustCompile(`([^{}]*)\{([^{}]*)\}`)
	fade := regexp.MustCompile(`transition[a-z-]*:[^;}]*\b(opacity|all)\b`)
	names, _ := fs.Glob(assetFS, "assets/*.css")
	for _, n := range names {
		for _, m := range rule.FindAllStringSubmatch(readAsset(strings.TrimPrefix(n, "assets/")), -1) {
			if !fade.MatchString(m[2]) {
				continue
			}
			ok := false
			for _, a := range allowed {
				ok = ok || strings.Contains(m[1], a)
			}
			if !ok {
				t.Errorf("%s: %s fades opacity", n, strings.TrimSpace(m[1]))
			}
		}
	}
}
