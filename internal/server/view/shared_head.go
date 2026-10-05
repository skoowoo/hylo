package view

import "strings"

// headOpts controls which optional CDN resources are included in <head>.
type headOpts struct {
	title      string // page title; may contain Go template directives (e.g. "{{.Title}}")
	withFonts  bool   // include noteFontsHTML (Google Fonts)
	withTW     bool   // include Tailwind CDN
	withAlpine bool   // include Alpine.js CDN (deferred)
	withHTMX   bool   // include htmx CDN
}

// Fragment endpoints answer errors with plain text, so error statuses must
// never swap into a pane; htmx:response:error surfaces them instead.
const htmxConfig = `{"extensions":"alpine-compat,hylo","noSwap":[204,304,"4xx","5xx"]}`

// headHTML returns the opening <head> block containing meta tags, the theme
// and Electron bootstrap IIFEs, and any requested CDN scripts.
// Each caller then appends its own <style> block and closes with </head>.
func headHTML(opts headOpts) string {
	var b strings.Builder
	b.WriteString("<head>\n")
	b.WriteString("  <meta charset=\"utf-8\">\n")
	b.WriteString("  <meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	b.WriteString("  <meta name=\"referrer\" content=\"no-referrer\">\n")
	b.WriteString("  <title>")
	b.WriteString(opts.title)
	b.WriteString("</title>\n")
	b.WriteString(themeBootstrapScript)
	b.WriteByte('\n')
	b.WriteString(accentBootstrapScript)
	b.WriteByte('\n')
	b.WriteString(electronBootstrapScript)
	b.WriteByte('\n')
	if opts.withFonts {
		b.WriteString(noteFontsHTML)
		b.WriteByte('\n')
	}
	if opts.withTW {
		b.WriteString("  <script src=\"/static/vendor/tailwind.js\"></script>\n")
	}
	if opts.withAlpine {
		b.WriteString("  <script defer src=\"/static/vendor/alpine-focus.min.js\"></script>\n")
		b.WriteString("  <script defer src=\"/static/vendor/alpine.min.js\"></script>\n")
	}
	if opts.withHTMX {
		b.WriteString("  <meta name=\"htmx-config\" content='" + htmxConfig + "'>\n")
		b.WriteString("  <script src=\"/static/vendor/htmx.min.js\"></script>\n")
		b.WriteString("  <script src=\"/static/vendor/htmx-alpine-compat.min.js\"></script>\n")
	}
	return b.String()
}
