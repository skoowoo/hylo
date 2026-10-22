package channels

import (
	"regexp"
	"strings"
)

var (
	reImageLink  = regexp.MustCompile(`!\[([^\]]*)\]\([^)]+\)`)
	reMDLink     = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	reBold3      = regexp.MustCompile(`\*\*\*(.+?)\*\*\*`)
	reBold2      = regexp.MustCompile(`\*\*(.+?)\*\*`)
	reItalic1    = regexp.MustCompile(`\*(.+?)\*`)
	reBoldU2     = regexp.MustCompile(`__(.+?)__`)
	reItalicU1   = regexp.MustCompile(`_(.+?)_`)
	reHeading    = regexp.MustCompile(`(?m)^#{1,6}\s+`)
	reBlankLines = regexp.MustCompile(`\n{3,}`)
)

// plainText strips markdown for apps that show messages as plain text.
func plainText(text string) string {
	out := reImageLink.ReplaceAllString(text, "[$1]")
	out = reMDLink.ReplaceAllString(out, "$1 ($2)")
	out = reBold3.ReplaceAllString(out, "$1")
	out = reBold2.ReplaceAllString(out, "$1")
	out = reItalic1.ReplaceAllString(out, "$1")
	out = reBoldU2.ReplaceAllString(out, "$1")
	out = reItalicU1.ReplaceAllString(out, "$1")
	out = reHeading.ReplaceAllString(out, "")
	out = reBlankLines.ReplaceAllString(out, "\n\n")
	return strings.TrimSpace(out)
}
