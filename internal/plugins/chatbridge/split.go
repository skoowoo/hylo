package chatbridge

import (
	"unicode/utf16"
	"unicode/utf8"
)

// SplitText splits text into segments of at most limit UTF-16 units, breaking
// at the last newline that fits. UTF-16 is the strictest count the apps use
// (Telegram's), so a segment fits every app, and no rune is ever cut.
func SplitText(text string, limit int) []string {
	if limit <= 0 {
		return []string{text}
	}
	var out []string
	for {
		end, lastNL, n := 0, -1, 0
		for end < len(text) {
			r, size := utf8.DecodeRuneInString(text[end:])
			w := 1
			if utf16.RuneLen(r) == 2 {
				w = 2
			}
			if n+w > limit {
				break
			}
			if r == '\n' {
				lastNL = end
			}
			n += w
			end += size
		}
		if end == len(text) {
			return append(out, text)
		}
		if lastNL > 0 {
			out = append(out, text[:lastNL])
			text = text[lastNL+1:]
		} else {
			out = append(out, text[:end])
			text = text[end:]
		}
	}
}
