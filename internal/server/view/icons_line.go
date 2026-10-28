package view

import (
	"fmt"
	"html/template"
	"strconv"
)

// Line icons are Lucide-based (ISC, see THIRD_PARTY_NOTICES). Sources live in
// icons/*.svg and `make icons` compiles them into shared_icons_line.go;
// /dev/icons shows them all.

// lineIconStroke is the drawn stroke of every line icon, in 24px-box units.
const lineIconStroke = 2

type lineIconDef struct {
	Paths string
	// Zoom scales the glyph about the centre so each icon reads at the same
	// optical size; the stroke shrinks by the same factor to stay uniform.
	Zoom float64
	// Glyph marks a small UI mark (chevron, close) that is drawn smaller than
	// nav icons by design, so the specimen keeps it out of the size check.
	Glyph   bool
	Base    string
	Changes string
}

func (d lineIconDef) svg(class string) template.HTML {
	half := 12 / d.Zoom
	cls := ""
	if class != "" {
		cls = ` class="` + template.HTMLEscapeString(class) + `"`
	}
	return template.HTML(fmt.Sprintf(`<svg%s fill="none" stroke="currentColor" stroke-width="%s" stroke-linecap="round" stroke-linejoin="round" viewBox="%s %s %s %s" aria-hidden="true">`, //nolint:gosec // static markup
		cls, svgNum(lineIconStroke/d.Zoom), svgNum(12-half), svgNum(12-half), svgNum(2*half), svgNum(2*half)) + d.Paths + `</svg>`)
}

// svgNum keeps three decimals, enough that a zoomed viewBox stays centred.
func svgNum(v float64) string {
	return strconv.FormatFloat(v, 'f', 3, 64)
}

// icon renders a line icon; an optional class goes on the <svg>.
func icon(name string, class ...string) (template.HTML, error) {
	d, ok := lineIcons[name]
	if !ok {
		return "", fmt.Errorf("icon: no line icon %q", name)
	}
	c := ""
	if len(class) > 0 {
		c = class[0]
	}
	return d.svg(c), nil
}

func sideIcon(name string) (template.HTML, error) {
	return icon(name, "side-nav-icon home-side-icon")
}
