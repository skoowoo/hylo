package handler

import "golang.org/x/text/language"

// langSignals is what the desktop app reads from the OS about the user's language.
type langSignals struct {
	Languages []string `json:"languages"`
	Locale    string   `json:"locale"`
	Inputs    []string `json:"inputs"`
}

// userLang picks the language to speak before the user has written anything.
// A non-English system language is taken at its word. An English one often
// isn't the user's own: an input method for another language gives that away,
// and their other preferred languages or region decide between several.
func userLang(s langSignals) string {
	langs := parseTags(s.Languages)
	if len(langs) == 0 {
		return ""
	}
	if !isEnglish(langs[0]) {
		return langs[0].String()
	}
	var typed []language.Tag
	for _, t := range parseTags(s.Inputs) {
		if !isEnglish(t) {
			typed = append(typed, t)
		}
	}
	if len(typed) == 0 {
		return langs[0].String()
	}
	for _, t := range typed {
		for _, l := range langs[1:] {
			if langKey(l) == langKey(t) {
				return l.String()
			}
		}
	}
	if loc, err := language.Parse(s.Locale); err == nil {
		if r, conf := loc.Region(); conf == language.Exact {
			home := langKey(language.Make("und-" + r.String()))
			for _, t := range typed {
				if langKey(t) == home {
					return t.String()
				}
			}
		}
	}
	return typed[0].String()
}

func parseTags(ss []string) []language.Tag {
	var tags []language.Tag
	for _, s := range ss {
		if t, err := language.Parse(s); err == nil && t != language.Und {
			tags = append(tags, t)
		}
	}
	return tags
}

func isEnglish(t language.Tag) bool {
	b, _ := t.Base()
	return b.String() == "en"
}

// langKey tells written languages apart: zh-CN and zh-Hans match, zh-TW doesn't.
func langKey(t language.Tag) string {
	b, _ := t.Base()
	s, _ := t.Script()
	return b.String() + "-" + s.String()
}
