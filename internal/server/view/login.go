package view

import (
	"bytes"
	"html/template"
	"net/http"
)

const loginCSS = `
    html, body { height: 100%; margin: 0; }
    body {
      display: flex; align-items: center; justify-content: center;
      background: var(--bg); color: var(--fg);
      font-family: var(--font-ui); font-size: var(--text-base);
      -webkit-font-smoothing: antialiased;
    }
    .login-drag { position: fixed; top: 0; left: 0; right: 0; height: 36px; }
    .login-card {
      width: min(360px, calc(100vw - 32px));
      background: var(--surface-soft); border: 1px solid var(--line-hair);
      border-radius: var(--r-lg, 12px); padding: 28px 24px 24px;
    }
    .login-logo {
      margin: 0 0 20px; font-size: var(--text-title-lg); font-weight: 600;
      letter-spacing: 0.2em;
    }
    .login-card .field-input { height: var(--btn-h); padding: 0 0.75rem; font-family: var(--font-ui); font-size: var(--text-base); box-sizing: border-box; }
    .login-btn { width: 100%; margin-top: 12px; }
    .login-btn:disabled { opacity: 0.5; cursor: default; }
    .login-msg { min-height: 1.25rem; margin-top: 10px; font-size: var(--text-sm); }
    .login-err { color: var(--s-err); }
    .login-hint { margin: 14px 0 0; color: var(--muted); font-size: var(--text-xs); line-height: 1.5; }
    .login-hint code { font-family: var(--font-mono); color: var(--fg); background: var(--btn-bg); padding: 1px 5px; border-radius: var(--r-xs, 4px); }
`

var loginPageHTML = `<!DOCTYPE html>
<html lang="en">
` + headHTML(headOpts{title: "Sign in — Hylo", withFonts: true}) + `
  <style>
` + appTokensCSS + baseCSS + loginCSS + `
  </style>
</head>
<body>
  <div class="login-drag" data-tauri-drag-region></div>
  <main class="login-card">
    <p class="login-logo">HYLO</p>
    <form method="post" action="/login">
      <input type="hidden" name="next" value="{{.Next}}">
      <input class="field-input" type="password" name="key" placeholder="API key"
             autocomplete="current-password" spellcheck="false" autofocus required>
      <button id="login-btn" class="btn-solid login-btn" type="submit">Sign in</button>
      <div id="login-msg" class="login-msg" role="alert">{{if .Error}}<span class="login-err">{{.Error}}</span>{{end}}</div>
    </form>
    <p class="login-hint">Run <code>hylo auth show</code> on the server to see the key.</p>
  </main>
</body>
</html>`

var loginPageTemplate = template.Must(template.New("login").Parse(loginPageHTML))

type loginData struct {
	Next  string
	Error string
}

// RenderLogin serves the sign-in page. It is a plain form post so sign-in
// works without JS and cannot be broken by webview fetch/redirect quirks.
func RenderLogin(w http.ResponseWriter, r *http.Request, status int, next, errMsg string) {
	w.Header().Set("Cache-Control", "no-store")
	var buf bytes.Buffer
	if err := loginPageTemplate.Execute(&buf, loginData{Next: next, Error: errMsg}); err != nil {
		http.Error(w, "render: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}
