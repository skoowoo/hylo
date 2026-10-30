package view

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net/url"

	"github.com/hardhacker/hylo/internal/server/static"
)

// views holds every page and fragment template; each assets/*.html file is
// named by its file name, plus whatever it defines.
var views = template.Must(template.New("views").Funcs(viewFuncs).ParseFS(assetFS, "assets/*.html"))

// renderHTML renders a fragment that another template embeds.
func renderHTML(name string, data any) (template.HTML, error) {
	var buf bytes.Buffer
	if err := views.ExecuteTemplate(&buf, name, data); err != nil {
		return "", err
	}
	return template.HTML(buf.String()), nil //nolint:gosec // rendered by html/template
}

var viewFuncs = template.FuncMap{
	"asset": static.URL,
	// dict passes named arguments to a component template.
	"dict": func(kv ...any) map[string]any {
		m := make(map[string]any, len(kv)/2)
		for i := 0; i+1 < len(kv); i += 2 {
			m[kv[i].(string)] = kv[i+1]
		}
		return m
	},
	"settingsTabs":  func() []settingsTab { return settingsTabs },
	"accentPresets": func() []accentPreset { return accentPresets },
	"accentDefault": func() string { return accentDefaultID },
	// tagsJSON serializes the wall's tag list for __hyloRenderTagCloud
	// (home.js) to lay out client-side via d3-cloud. Returned as a plain
	// string, not template.HTML, so html/template still HTML-escapes it into
	// the data-tags attribute (the JSON's own double quotes become &#34;) —
	// the browser unescapes that back to valid JSON before JS reads it.
	"tagsJSON": func(tags []tagItem) string {
		b, err := json.Marshal(tags)
		if err != nil {
			return "[]"
		}
		return string(b)
	},
	"label": func(item noteItem) string {
		if item.Title != "" {
			return item.Title
		}
		return item.Name
	},
	"folderLabel": func(dir string) string {
		if dir == "/" {
			return "/"
		}
		if len(dir) > 1 && dir[0] == '/' {
			return dir[1:]
		}
		return dir
	},
	// xshow gives an x-show element its server-known visibility before Alpine runs.
	"xshow": func(show bool) template.HTMLAttr {
		if show {
			return ""
		}
		return `style="display:none"`
	},
	"encdir":   url.QueryEscape,
	"sideIcon": sideIcon,
	"icon":     icon,
	"coverURL": func(name string) string {
		if name == "" {
			return ""
		}
		return "/api/images/serve?name=" + url.QueryEscape(name)
	},
	// todoRingDash returns the stroke-dasharray for the todo progress ring
	// (see .note-badge--todo-fill): "<filled length> <circumference>",
	// where filled is the arc length representing done/total around a
	// circle of radius todoRingRadius. The gap value only needs to be at
	// least the circle's full circumference — however much filled leaves
	// unused just stays empty, which is exactly the un-done remainder.
	"todoRingDash": func(done, total int) string {
		if total <= 0 {
			return fmt.Sprintf("0 %.2f", todoRingCircumference)
		}
		ratio := float64(done) / float64(total)
		if ratio < 0 {
			ratio = 0
		} else if ratio > 1 {
			ratio = 1
		}
		return fmt.Sprintf("%.2f %.2f", todoRingCircumference*ratio, todoRingCircumference)
	},
}
