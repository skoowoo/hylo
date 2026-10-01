package assets

import (
	"net/url"
	"path"
	"regexp"
	"strings"

	"github.com/hardhacker/hylo/internal/storage"
	"github.com/hardhacker/hylo/internal/util"
)

// maxCoverCandidates bounds how many refs are tried, since each remote one
// can cost a full fetch timeout.
const maxCoverCandidates = 3

var (
	// Group 1: <angle-bracket url>; 2: plain url (one level of balanced
	// parens allowed); 3: wiki embed. One regex keeps document order.
	imageRefRe = regexp.MustCompile(`!\[[^\]]*\]\(\s*(?:<([^>]+)>|((?:[^()\s]|\([^()\s]*\))+))(?:\s+(?:"[^"]*"|'[^']*'))?\s*\)` +
		`|!\[\[([^\]|]+)(?:\|[^\]]*)?\]\]`)
	fencedCodeRe = regexp.MustCompile("(?ms)^[ \t]*(?:```|~~~).*?^[ \t]*(?:```|~~~)[^\n]*$")
)

// coverSource is one candidate cover ref found in a note.
type coverSource struct {
	remote string
	local  string
}

// parseCoverSources returns candidate covers in priority order: frontmatter
// image_url first, then body images by position.
func parseCoverSources(raw []byte) []coverSource {
	fm, body := util.ParseFrontmatter(raw)

	var out []coverSource
	seen := map[coverSource]bool{}
	add := func(s coverSource) {
		if s == (coverSource{}) || seen[s] || len(out) >= maxCoverCandidates {
			return
		}
		seen[s] = true
		out = append(out, s)
	}

	for _, e := range fm.All {
		if e.Key == "image_url" && e.Value != "" {
			add(classifyRef(e.Value))
			break
		}
	}

	body = fencedCodeRe.ReplaceAll(body, []byte("\n"))
	for _, m := range imageRefRe.FindAllSubmatch(body, -1) {
		switch {
		case len(m[1]) > 0:
			add(classifyRef(string(m[1])))
		case len(m[2]) > 0:
			add(classifyRef(string(m[2])))
		case len(m[3]) > 0:
			add(classifyLocalName(string(m[3])))
		}
	}
	return out
}

func classifyRef(ref string) coverSource {
	ref = strings.TrimSpace(ref)
	lower := strings.ToLower(ref)
	switch {
	case ref == "":
		return coverSource{}
	case strings.HasPrefix(lower, "http://"), strings.HasPrefix(lower, "https://"):
		return coverSource{remote: ref}
	case strings.HasPrefix(ref, "//"):
		return coverSource{remote: "https:" + ref}
	}

	u, err := url.Parse(ref)
	if err != nil {
		return classifyLocalName(ref)
	}
	if u.Scheme != "" {
		return coverSource{}
	}
	if name := u.Query().Get("name"); name != "" {
		return classifyLocalName(name)
	}
	return classifyLocalName(u.Path)
}

// classifyLocalName maps a file ref to its vault-wide unique basename.
func classifyLocalName(ref string) coverSource {
	name := strings.TrimSpace(path.Base(strings.ReplaceAll(strings.TrimSpace(ref), "\\", "/")))
	if name == "" || name == "." || name == "/" || !storage.IsImagePath(name) {
		return coverSource{}
	}
	return coverSource{local: name}
}
