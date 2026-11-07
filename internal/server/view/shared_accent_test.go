package view

import (
	"strings"
	"testing"
)

// The default accent is applied by CSS alone, so its hue token must match the preset.
func TestAccentHueToken(t *testing.T) {
	for _, p := range accentPresets {
		if p.ID == accentDefaultID && !strings.Contains(appTokensShared, "--accent-h:"+p.Hue+";") {
			t.Fatalf("shared_tokens.go --accent-h must be %s", p.Hue)
		}
	}
}
