// Package static serves the embedded front-end files, plus the bundles the
// view assembles at init, under /static/ with content-hash cache validators.
package static

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"io/fs"
	"net/http"
	"time"
)

//go:embed *.js agents vendor
var embedded embed.FS

type file struct {
	body []byte
	hash string
}

// files is written only during package init (here and by view's Bundle calls).
var files = map[string]*file{}

func init() {
	err := fs.WalkDir(embedded, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		body, err := embedded.ReadFile(name)
		if err != nil {
			return err
		}
		add(name, body)
		return nil
	})
	if err != nil {
		panic(err)
	}
}

func add(name string, body []byte) {
	sum := sha256.Sum256(body)
	files[name] = &file{body: body, hash: hex.EncodeToString(sum[:6])}
}

// Bundle registers generated content at /static/<name> and returns its URL.
func Bundle(name, body string) string {
	if _, dup := files[name]; dup {
		panic("static: duplicate file " + name)
	}
	add(name, []byte(body))
	return URL(name)
}

// URL returns name's versioned URL; unknown names panic so a typo fails at startup.
func URL(name string) string {
	f, ok := files[name]
	if !ok {
		panic("static: unknown file " + name)
	}
	return "/static/" + name + "?v=" + f.hash
}

// Handler serves files by path relative to /static/. Requests carrying the
// current ?v= are immutable; bare paths (chunks, fonts, logos referenced from
// JS/CSS) revalidate by ETag.
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f, ok := files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		h := w.Header()
		h.Set("ETag", `"`+f.hash+`"`)
		if r.URL.Query().Get("v") == f.hash {
			h.Set("Cache-Control", "max-age=31536000, immutable")
		} else {
			h.Set("Cache-Control", "no-cache")
		}
		http.ServeContent(w, r, r.URL.Path, time.Time{}, bytes.NewReader(f.body))
	})
}
