package view

import (
	"math"
	"strconv"
)

// accentPreset is one selectable brand color. All presets are luminance-matched
// to the original indigo (white text on Accent ≈ 4.6:1, Accent on the light
// canvas ≈ 4.4:1), so any of them can stand in for --accent without touching
// component CSS. Hover and text differ per theme, same as the indigo defaults in
// shared_tokens.go: hover darker on light / lighter on dark, text = Accent on
// light / TextDark on dark.
type accentPreset struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Accent   string `json:"a"`
	RGB      string `json:"r"`
	HovLight string `json:"hl"`
	HovDark  string `json:"hd"`
	// TextDark is Accent lifted to ≈4.6:1 on the dark canvas, for accent-coloured
	// text (links, active labels). Fills keep Accent so white-on-accent stays ≥4.5.
	TextDark string `json:"td"`
	// Hue is Accent's OKLCH hue, filled in at init.
	Hue string `json:"h"`
}

// accentDefaultID must stay in sync with --accent/--accent-rgb/--accent-hov/--accent-h in
// shared_tokens.go: it is the one preset applied by CSS alone, no inline vars.
const accentDefaultID = "indigo"

var accentPresets = []accentPreset{
	{"indigo", "Indigo", "#5e6ad2", "94,106,210", "#4c56c8", "#7b86e8", "#707ad7", ""},
	{"blue", "Blue", "#2974d6", "41,116,214", "#2569c3", "#538fde", "#4184db", ""},
	{"teal", "Teal", "#15827d", "21,130,125", "#137772", "#1a9f98", "#18928c", ""},
	{"green", "Green", "#268555", "38,133,85", "#23794e", "#2fa068", "#2a955f", ""},
	{"amber", "Amber", "#a4670d", "164,103,13", "#955e0c", "#c3801c", "#b8740f", ""},
	{"coral", "Coral", "#d04628", "208,70,40", "#bc4024", "#de6c53", "#da593e", ""},
	{"rose", "Rose", "#d13a73", "209,58,115", "#c32e67", "#db6794", "#d75486", ""},
	{"violet", "Violet", "#8e5bd0", "142,91,208", "#834dcb", "#a47bd9", "#9a6cd5", ""},
}

// oklchHue is the OKLCH hue in degrees of a #rrggbb colour.
func oklchHue(hex string) float64 {
	ch := func(i int) float64 {
		v, _ := strconv.ParseUint(hex[i:i+2], 16, 8)
		c := float64(v) / 255
		if c <= 0.04045 {
			return c / 12.92
		}
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	r, g, b := ch(1), ch(3), ch(5)
	l := math.Cbrt(0.4122214708*r + 0.5363325363*g + 0.0514459929*b)
	m := math.Cbrt(0.2119034982*r + 0.6806995451*g + 0.1073969566*b)
	s := math.Cbrt(0.0883024619*r + 0.2817188376*g + 0.6299787005*b)
	A := 1.9779984951*l - 2.4285922050*m + 0.4505937099*s
	B := 0.0259040371*l + 0.7827717662*m - 0.8086757660*s
	return math.Mod(math.Atan2(B, A)*180/math.Pi+360, 360)
}

func init() {
	for i := range accentPresets {
		accentPresets[i].Hue = strconv.FormatFloat(oklchHue(accentPresets[i].Accent), 'f', 1, 64)
	}
}
