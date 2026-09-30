package view

import "net/http"

// QuickCapture serves the screenshot-capture panel (desktop-app's global
// Alt+Shift+C shortcut) as a plain, framework-free page — no Tailwind/
// Alpine/HTMX, since quickCaptureJS is vanilla DOM code with no reactive
// state. themeBootstrapScript/accentBootstrapScript are included (both are
// pure localStorage/matchMedia, no window.vaultrDesktop dependency) so the
// panel picks up the user's in-app theme/accent choice instead of only the
// OS prefers-color-scheme; electronBootstrapScript/electronShellSafeReloadScript
// are skipped since both branch on window.vaultrDesktop, which this panel's
// preload (quick-capture/preload.js, exposing only window.quickCapture)
// doesn't provide.
//
// This is the panel's only content source — desktop-app/src/quick-capture.js
// won't even trigger the capture if the server isn't up yet, and closes the
// panel outright if this fails to load mid-flight, rather than keeping a
// second, separately-maintained offline copy of the UI around.
var quickCapturePageHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>Quick Capture</title>
` + themeBootstrapScript + `
` + accentBootstrapScript + `
` + noteFontsHTML + `
  <style>
` + appTokensCSS + `
` + quickCaptureCSS + `
  </style>
</head>
<body>
` + quickCaptureHTML + `
  <script>
` + quickCaptureJS + `
  </script>
</body>
</html>`

// QuickCapture handles GET /quick-capture.
func (vh *ViewHandler) QuickCapture(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(quickCapturePageHTML))
}
