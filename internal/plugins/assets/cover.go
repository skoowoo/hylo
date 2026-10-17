package assets

import (
	"github.com/hardhacker/hylo/internal/storage"
	"github.com/hardhacker/hylo/internal/util"
)

// maxCoverCandidates bounds how many refs are tried, since each remote one
// can cost a full fetch timeout.
const maxCoverCandidates = 3

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
	add := func(r storage.ImageRef) {
		s := coverSource{remote: r.Remote, local: r.Local}
		if s == (coverSource{}) || seen[s] || len(out) >= maxCoverCandidates {
			return
		}
		seen[s] = true
		out = append(out, s)
	}

	for _, e := range fm.All {
		if e.Key == "image_url" && e.Value != "" {
			add(storage.ClassifyImageRef(e.Value))
			break
		}
	}
	for _, r := range storage.ParseImageRefs(body) {
		add(r)
	}
	return out
}
