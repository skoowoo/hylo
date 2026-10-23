package channels

import (
	"regexp"
	"strconv"
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
	reURL        = regexp.MustCompile(`https?://[^\s)]+`)
	reURLSlot    = regexp.MustCompile("\x00[0-9]+\x00")
)

// cardLangs are the fences Hylo's chat draws as cards (editor/src/cm-live/fence-card/registry.js).
// No other app draws them, so they go out as the text they carry.
var cardLangs = map[string]bool{"embed": true, "choices": true, "sources": true, "proposal": true, "receipt": true}

var (
	reFenceOpen = regexp.MustCompile("^[ \t]*(`{3,}|~{3,})[ \t]*([\\w-]+)[ \t]*$")
	reCardItem  = regexp.MustCompile(`^-\s+(?:\[[ xX]\]\s+)?(.+)$`)
	reCardField = regexp.MustCompile(`^(\w+):(?:\s+(.*))?$`)
)

// unwrapCards turns card fences into their visible text: URLs on their own
// lines, options and items as a list, a title and summary as lines. What only
// the card uses (receipt ids, the undo and swap buttons, folded details) is dropped.
func unwrapCards(text string) string {
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	for i := 0; i < len(lines); i++ {
		m := reFenceOpen.FindStringSubmatch(lines[i])
		if m == nil || !cardLangs[m[2]] {
			out = append(out, lines[i])
			continue
		}
		end := closingFence(lines, i+1, m[1])
		if end < 0 {
			out = append(out, lines[i])
			continue
		}
		for _, l := range lines[i+1 : end] {
			if s, ok := cardLine(strings.TrimSpace(l)); ok {
				out = append(out, s)
			}
		}
		i = end
	}
	return strings.Join(out, "\n")
}

func closingFence(lines []string, from int, open string) int {
	for j := from; j < len(lines); j++ {
		l := strings.TrimSpace(lines[j])
		if len(l) >= len(open) && strings.Trim(l, open[:1]) == "" {
			return j
		}
	}
	return -1
}

func cardLine(l string) (string, bool) {
	if l == "" {
		return "", false
	}
	if m := reCardItem.FindStringSubmatch(l); m != nil {
		return "- " + m[1], true
	}
	if m := reCardField.FindStringSubmatch(l); m != nil {
		switch m[1] {
		case "title", "summary":
			return m[2], m[2] != ""
		case "note":
			return "- " + m[2], m[2] != ""
		default:
			return "", false
		}
	}
	return l, true
}

// plainText strips markdown for apps that show messages as plain text.
// URLs are set aside while emphasis is stripped, so `_` and `*` inside them survive.
func plainText(text string) string {
	out := reImageLink.ReplaceAllString(text, "[$1]")
	out = reMDLink.ReplaceAllString(out, "$1 ($2)")
	var urls []string
	out = reURL.ReplaceAllStringFunc(out, func(u string) string {
		urls = append(urls, u)
		return "\x00" + strconv.Itoa(len(urls)-1) + "\x00"
	})
	out = reBold3.ReplaceAllString(out, "$1")
	out = reBold2.ReplaceAllString(out, "$1")
	out = reItalic1.ReplaceAllString(out, "$1")
	out = reBoldU2.ReplaceAllString(out, "$1")
	out = reItalicU1.ReplaceAllString(out, "$1")
	out = reHeading.ReplaceAllString(out, "")
	out = reBlankLines.ReplaceAllString(out, "\n\n")
	out = reURLSlot.ReplaceAllStringFunc(out, func(s string) string {
		i, _ := strconv.Atoi(s[1 : len(s)-1])
		return urls[i]
	})
	return strings.TrimSpace(out)
}
