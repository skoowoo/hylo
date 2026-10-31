package channels

import (
	"regexp"
	"strings"
)

// Formats follow editor/src/cm-live/fence-card: `key: value` fields plus list items.
var (
	reCardField  = regexp.MustCompile(`^\s*([a-z][a-z-]*):\s*(.*?)\s*$`)
	reCardItem   = regexp.MustCompile(`^\s*[-*+]\s+(.*\S)\s*$`)
	reChoiceItem = regexp.MustCompile(`^\s*(?:[-*+]|\d+[.)])\s+(\[[ xX]\]\s+)?(.*\S)\s*$`)
)

// cardText writes a card fence (registry.js) as what the card shows, with its
// buttons as a list a user can answer by typing. What only the card uses
// (receipt ids, the sources swap) is dropped. Other fences are code: ok is false.
func cardText(lang, body string) (string, bool) {
	switch lang {
	case "embed":
		var urls []string
		for _, l := range strings.Split(body, "\n") {
			if l = strings.TrimSpace(l); l != "" {
				urls = append(urls, l)
			}
		}
		return strings.Join(urls, "\n"), true
	case "choices":
		return choicesText(body), true
	case "sources":
		return sourcesText(parseCard(body)), true
	case "proposal":
		c := parseCard(body)
		return joinLines(c.field("title"), c.field("summary"), c.field("details"), itemsText(c.items)), true
	case "receipt":
		c := parseCard(body)
		var undo string
		if u := c.field("undo"); u != "" {
			undo = "↩ " + u
		}
		var title string
		if t := c.field("title"); t != "" {
			title = "✓ " + t
		}
		return joinLines(title, itemsText(c.items), undo), true
	case "mindmap":
		return deskMindmap, true
	}
	return "", false
}

type card struct {
	fields map[string][]string
	items  []string
}

func parseCard(body string) card {
	c := card{fields: map[string][]string{}}
	for _, l := range strings.Split(body, "\n") {
		if m := reCardItem.FindStringSubmatch(l); m != nil {
			c.items = append(c.items, plainText(m[1]))
			continue
		}
		if m := reCardField.FindStringSubmatch(l); m != nil && m[2] != "" {
			c.fields[m[1]] = append(c.fields[m[1]], plainText(m[2]))
		}
	}
	return c
}

func (c card) field(k string) string {
	if v := c.fields[k]; len(v) > 0 {
		return v[0]
	}
	return ""
}

func itemsText(items []string) string {
	lines := make([]string, len(items))
	for i, it := range items {
		lines[i] = "- " + it
	}
	return strings.Join(lines, "\n")
}

func choicesText(body string) string {
	var lines []string
	for _, l := range strings.Split(body, "\n") {
		m := reChoiceItem.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		mark := "- "
		if m[1] != "" {
			mark = "☐ "
		}
		lines = append(lines, mark+plainText(m[2]))
	}
	return strings.Join(lines, "\n")
}

// sourcesText is the card's one line: the notes, then the skills.
func sourcesText(c card) string {
	var parts []string
	var notes []string
	for _, n := range c.fields["note"] {
		notes = append(notes, strings.TrimSuffix(n, ".md"))
	}
	if len(notes) > 0 {
		parts = append(parts, "📝 "+strings.Join(notes, " · "))
	}
	for _, s := range c.fields["skill"] {
		parts = append(parts, "✨ "+s)
	}
	return strings.Join(parts, "  ")
}

func joinLines(parts ...string) string {
	var out []string
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, "\n")
}
