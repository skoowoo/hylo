package util

import (
	"bytes"
	"strings"
)

// RewriteWikilinkTarget rewrites every [[Old]]/[[Old|display]] in src whose
// target stems to oldName (extension optional either side, see wikilinkStem)
// to point at newName instead, keeping display text as-is. ![[...]] image
// embeds are skipped even when the inner text matches — that syntax always
// resolves as an image (expandWikiImages), never a note — so this walks match
// indices rather than ReplaceAllFunc, to check the byte before each match for
// a leading "!". Used by renamesync's sweep, since a rename (unlike a move)
// changes the very string every by-name wikilink resolves against.
//
// Reports whether anything changed; src is returned unmodified otherwise.
func RewriteWikilinkTarget(src []byte, oldName, newName string) ([]byte, bool) {
	oldStem := wikilinkStem(oldName)
	newStem := wikilinkStem(newName)
	if oldStem == "" || oldStem == newStem {
		return src, false
	}

	locs := wikilinkRe.FindAllSubmatchIndex(src, -1)
	if locs == nil {
		return src, false
	}

	var buf bytes.Buffer
	last := 0
	changed := false
	for _, loc := range locs {
		start, end := loc[0], loc[1]
		if start > 0 && src[start-1] == '!' {
			continue // ![[...]] image embed — never a note reference here
		}
		target := strings.TrimSpace(string(src[loc[2]:loc[3]]))
		if wikilinkStem(target) != oldStem {
			continue
		}
		buf.Write(src[last:start])
		buf.WriteString("[[")
		buf.WriteString(newStem)
		if loc[4] >= 0 {
			if display := strings.TrimSpace(string(src[loc[4]:loc[5]])); display != "" {
				buf.WriteByte('|')
				buf.WriteString(display)
			}
		}
		buf.WriteString("]]")
		last = end
		changed = true
	}
	if !changed {
		return src, false
	}
	buf.Write(src[last:])
	return buf.Bytes(), true
}

// RewriteFrontmatterBareNameRef replaces bare (path-free) source_notes: list
// items naming oldName with newName, inside raw's frontmatter block only.
// Complements RewriteFrontmatterPathRef, which handles the full-path form of
// the same field — a path-shaped item is left to that one instead, so the two
// never touch the same line. Only simple block-style list items
// ("  - item"/"  - item.md") are recognized; flow-style lists aren't (not the
// documented skills/hylo-compile-note format).
//
// Reports whether anything changed; raw is returned unmodified otherwise.
func RewriteFrontmatterBareNameRef(raw []byte, oldName, newName string) ([]byte, bool) {
	fm, body := ParseFrontmatter(raw)
	if !fm.HasMeta() {
		return raw, false
	}
	oldStem := wikilinkStem(oldName)
	newStem := wikilinkStem(newName)
	if oldStem == "" || oldStem == newStem {
		return raw, false
	}

	// block is the frontmatter delimiters + YAML bytes that precede body;
	// body is a suffix subslice of raw (ParseFrontmatter never copies).
	block := raw[:len(raw)-len(body)]
	lines := bytes.Split(block, []byte("\n"))

	inList := false
	changed := false
	for i, line := range lines {
		trimmed := bytes.TrimSpace(line)
		if bytes.HasPrefix(trimmed, []byte("source_notes:")) {
			inList = strings.TrimSpace(strings.TrimPrefix(string(trimmed), "source_notes:")) == ""
			continue
		}
		if !inList {
			continue
		}
		if !bytes.HasPrefix(trimmed, []byte("-")) {
			inList = false
			continue
		}
		item := strings.TrimSpace(strings.TrimPrefix(string(trimmed), "-"))
		item = strings.Trim(item, `"'`)
		if item == "" || strings.Contains(item, "/") {
			continue
		}
		if wikilinkStem(item) != oldStem {
			continue
		}
		lines[i] = bytes.Replace(line, []byte(item), []byte(newStem), 1)
		changed = true
	}
	if !changed {
		return raw, false
	}
	out := append(bytes.Join(lines, []byte("\n")), body...)
	return out, true
}

// wikilinkStem strips a trailing ".md"/".markdown" (case-insensitive), so
// "Foo" and "Foo.md" both stem to "Foo" — matches expandWikilinks's own
// normalization before it resolves a target against the notes table.
func wikilinkStem(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".markdown"):
		return name[:len(name)-len(".markdown")]
	case strings.HasSuffix(lower, ".md"):
		return name[:len(name)-len(".md")]
	default:
		return name
	}
}
