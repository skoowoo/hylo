package view

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"sort"
)

type specimenIcon struct {
	Name, Base, Changes string
	Zoom                float64
	SVG                 template.HTML
}

type specimenTable struct {
	Title, Note string
	Check       bool
	Icons       []specimenIcon
}

type specimenRow struct {
	Icon, Label string
}

// IconSpecimen renders every line icon with its measurements (/dev/icons).
func (vh *ViewHandler) IconSpecimen(w http.ResponseWriter, r *http.Request) {
	names := make([]string, 0, len(lineIcons))
	for n := range lineIcons {
		names = append(names, n)
	}
	sort.Strings(names)
	navRows := [][]specimenRow{
		{{"open-tabs", "Open"}},
		{{"inbox", "Inbox"}, {"bot", "Bots"}, {"knowledge", "Knowledge"}, {"tags", "Tags"}, {"memory", "Memory"}},
		{{"pinned", "Pinned"}, {"recent", "Recent"}, {"shorts", "Shorts"}, {"images", "Images"}, {"folders", "Folders"}},
	}
	children := []specimenRow{{"note", "Weekly review"}, {"draft", "Untitled"}, {"folder", "projects"}}
	// Icons that stand in a nav list share one optical size; the rest sit
	// alone in toolbars and menus, where line-only shapes read fine smaller.
	nav := map[string]bool{"search": true, "new-note": true, "reading-pane": true, "settings": true}
	for _, g := range navRows {
		for _, r := range g {
			nav[r.Icon] = true
		}
	}
	for _, r := range children {
		nav[r.Icon] = true
	}
	for _, t := range settingsTabs {
		nav[t.Icon] = true
	}
	var navIcons, icons, glyphs []specimenIcon
	for _, n := range names {
		d := lineIcons[n]
		ic := specimenIcon{Name: n, Base: d.Base, Changes: d.Changes, Zoom: d.Zoom, SVG: d.svg("")}
		switch {
		case d.Glyph:
			glyphs = append(glyphs, ic)
		case nav[n]:
			navIcons = append(navIcons, ic)
		default:
			icons = append(icons, ic)
		}
	}
	data := map[string]any{
		"Count": len(names),
		"Tables": []specimenTable{
			{"Navigation icons", "Sidebar and settings rows. Optical size is the side of a square with the icon's filled silhouette area, in 24px-box units; these sit at 17.5–20.5. Offset is how far the ink's centre sits from the box centre.", true, navIcons},
			{"Other icons", "Toolbars, menus and buttons. Measured, not checked: the metric undercounts line-only shapes (lists, code, sliders), so judge these by eye.", false, icons},
			{"Glyphs", "Small UI marks, drawn smaller than icons on purpose, so their size isn't checked.", false, glyphs},
		},
		"Stroke":   lineIconStroke,
		"Nav":      navRows,
		"Children": children,
	}
	var buf bytes.Buffer
	if err := views.ExecuteTemplate(&buf, "page_icons.html", data); err != nil {
		http.Error(w, fmt.Sprintf("render: %s", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buf.Bytes())
}
