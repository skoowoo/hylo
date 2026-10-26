package view

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
}

// accentDefaultID must stay in sync with --accent/--accent-rgb/--accent-hov in
// shared_tokens.go: it is the one preset applied by CSS alone, no inline vars.
const accentDefaultID = "indigo"

var accentPresets = []accentPreset{
	{"indigo", "Indigo", "#5e6ad2", "94,106,210", "#4c56c8", "#7b86e8", "#707ad7"},
	{"blue", "Blue", "#2974d6", "41,116,214", "#2569c3", "#538fde", "#4184db"},
	{"teal", "Teal", "#15827d", "21,130,125", "#137772", "#1a9f98", "#18928c"},
	{"green", "Green", "#268555", "38,133,85", "#23794e", "#2fa068", "#2a955f"},
	{"amber", "Amber", "#a4670d", "164,103,13", "#955e0c", "#c3801c", "#b8740f"},
	{"coral", "Coral", "#d04628", "208,70,40", "#bc4024", "#de6c53", "#da593e"},
	{"rose", "Rose", "#d13a73", "209,58,115", "#c32e67", "#db6794", "#d75486"},
	{"violet", "Violet", "#8e5bd0", "142,91,208", "#834dcb", "#a47bd9", "#9a6cd5"},
}
