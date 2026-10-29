package util

import (
	"bytes"
	"strings"
)

// ExtractShortImages splits short-note markdown into its text body and the
// names of any ![[...]] image embeds it contains, in encounter order. Shorts
// attach images out-of-band of typed text (see shorts-compose-images in
// home.js), so rendering always shows text first and images below — wherever
// the raw ![[...]] lines happen to sit in the stored markdown — rather than
// wherever the user happened to paste them inline.
func ExtractShortImages(md []byte) (text []byte, imageNames []string) {
	matches := wikiImageRe.FindAllSubmatch(md, -1)
	for _, m := range matches {
		imageNames = append(imageNames, strings.TrimSpace(string(m[1])))
	}
	return bytes.TrimSpace(wikiImageRe.ReplaceAll(md, nil)), imageNames
}
