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
	var icons, glyphs []specimenIcon
	for _, n := range names {
		d := lineIcons[n]
		ic := specimenIcon{Name: n, Base: d.Base, Changes: d.Changes, Zoom: d.Zoom, SVG: d.svg("")}
		if d.Glyph {
			glyphs = append(glyphs, ic)
		} else {
			icons = append(icons, ic)
		}
	}
	data := map[string]any{
		"Count": len(names),
		"Tables": []specimenTable{
			{"Icons", "Optical size is the side of a square with the icon's filled silhouette area, in 24px-box units; nav icons sit at 17.5–20.5. Offset is how far the ink's centre sits from the box centre.", true, icons},
			{"Glyphs", "Small UI marks, drawn smaller than icons on purpose, so their size isn't checked.", false, glyphs},
		},
		"Stroke": lineIconStroke,
		"Nav": [][]specimenRow{
			{{"open-tabs", "Open"}},
			{{"inbox", "Inbox"}, {"bot", "Bots"}, {"knowledge", "Knowledge"}, {"tags", "Tags"}, {"memory", "Memory"}},
			{{"pinned", "Pinned"}, {"recent", "Recent"}, {"shorts", "Shorts"}, {"images", "Images"}, {"folders", "Folders"}},
		},
		"Children": []specimenRow{{"note", "Weekly review"}, {"draft", "Untitled"}, {"folder", "projects"}},
	}
	var buf bytes.Buffer
	if err := views.ExecuteTemplate(&buf, "page_icons.html", data); err != nil {
		http.Error(w, fmt.Sprintf("render: %s", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buf.Bytes())
}
