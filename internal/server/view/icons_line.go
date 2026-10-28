package view

import (
	"fmt"
	"html/template"
	"net/url"
	"strconv"
	"strings"
)

// Line icons are Lucide-based (ISC, see THIRD_PARTY_NOTICES). Sources live in
// icons/*.svg; `make icons` compiles them into shared_icons_line.go (with the
// stroke width), assets/shared_icons_line.js and editor/src/line-icons.js;
// /dev/icons shows them all.

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

// svg renders the icon. attrs is a class name, or raw attributes when it
// holds an "=" (sizes, Alpine bindings); a fill, stroke or stroke-width there
// replaces the line default, for a solid badge or a mark drawn inside a
// small box (the task checkbox tick).
func (d lineIconDef) svg(attrs string) template.HTML {
	half := 12 / d.Zoom
	extra := ""
	if strings.Contains(attrs, "=") {
		extra = " " + attrs
	} else if attrs != "" {
		extra = ` class="` + template.HTMLEscapeString(attrs) + `"`
	}
	paint := ""
	if !strings.Contains(attrs, "fill=") {
		paint += ` fill="none"`
	}
	if !strings.Contains(attrs, "stroke=") {
		paint += ` stroke="currentColor"`
	}
	if !strings.Contains(attrs, "stroke-width=") {
		paint += ` stroke-width="` + svgNum(lineIconStroke/d.Zoom) + `"`
	}
	return template.HTML(fmt.Sprintf(`<svg%s%s stroke-linecap="round" stroke-linejoin="round" viewBox="%s %s %s %s" aria-hidden="true">`, //nolint:gosec // static markup
		extra, paint, svgNum(12-half), svgNum(12-half), svgNum(2*half), svgNum(2*half)) + d.Paths + `</svg>`)
}

// svgNum keeps three decimals, enough that a zoomed viewBox stays centred.
func svgNum(v float64) string {
	return strconv.FormatFloat(v, 'f', 3, 64)
}

// icon renders a line icon: {{icon "name"}}, {{icon "name" "class"}} or
// {{icon "name" `class="x" width="13" height="13"`}}.
func icon(name string, attrs ...string) (template.HTML, error) {
	d, ok := lineIcons[name]
	if !ok {
		return "", fmt.Errorf("icon: no line icon %q", name)
	}
	a := ""
	if len(attrs) > 0 {
		a = attrs[0]
	}
	return d.svg(a), nil
}

// lineIconVar declares --icon-<name> as a mask-ready data URL of a line icon
// (attrs as in icon()), for icons drawn by CSS pseudo-elements.
func lineIconVar(name, iconName, attrs string) string {
	d, ok := lineIcons[iconName]
	if !ok {
		panic("lineIconVar: no line icon " + iconName)
	}
	svg := strings.Replace(string(d.svg(strings.TrimSpace(`xmlns="http://www.w3.org/2000/svg" `+attrs))), `stroke="currentColor"`, `stroke="black"`, 1)
	return fmt.Sprintf("--icon-%s:url(\"data:image/svg+xml,%s\");", name, url.PathEscape(svg))
}

func sideIcon(name string) (template.HTML, error) {
	return icon(name, "side-nav-icon home-side-icon")
}
