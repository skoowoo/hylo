package view

import (
	"bytes"
	"net/http"
)

type loginData struct {
	Next  string
	Error string
}

// RenderLogin serves the sign-in page. It is a plain form post so sign-in
// works without JS and cannot be broken by webview fetch/redirect quirks.
func RenderLogin(w http.ResponseWriter, r *http.Request, status int, next, errMsg string) {
	w.Header().Set("Cache-Control", "no-store")
	var buf bytes.Buffer
	if err := views.ExecuteTemplate(&buf, "page_login.html", loginData{Next: next, Error: errMsg}); err != nil {
		http.Error(w, "render: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}
