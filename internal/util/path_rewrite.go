package util

import "bytes"

// RewriteFrontmatterPathRef replaces every literal occurrence of oldPath with
// newPath inside raw's YAML front matter block only — never the body — using
// a plain byte substring replace, not a YAML re-serialization, so nothing
// else in the file (formatting, comments, field order) changes.
//
// Used by Vault.MoveNote to fix up a moved raw note's full-path reference
// inside a dependent knowledge note's source_notes: list (see
// skills/hylo-compile-note): that list is what the compile plugin re-syncs
// knowledge_deps from on every save, so leaving a stale path there would
// silently undo the move's DB update the next time the note is resaved.
//
// Reports whether anything changed; raw is returned unmodified otherwise.
func RewriteFrontmatterPathRef(raw []byte, oldPath, newPath string) ([]byte, bool) {
	fm, body := ParseFrontmatter(raw)
	if !fm.HasMeta() {
		return raw, false
	}
	// body is a suffix subslice of raw (ParseFrontmatter never copies), so this
	// recovers the exact frontmatter-block-plus-delimiters bytes that precede it.
	block := raw[:len(raw)-len(body)]
	if !bytes.Contains(block, []byte(oldPath)) {
		return raw, false
	}
	out := make([]byte, 0, len(raw)+len(newPath)-len(oldPath))
	out = append(out, bytes.ReplaceAll(block, []byte(oldPath), []byte(newPath))...)
	out = append(out, body...)
	return out, true
}

// RewriteTableRowPathRef replaces every literal occurrence of oldPath with
// newPath, but only on lines that look like markdown table rows (trimmed
// line starts with "|") — mirrors the line filter the compile plugin's
// parseIndexTablePaths uses to read an index note's knowledge-note path
// column, so a coincidental mention of the same path in the note's prose is
// left alone.
//
// Used by Vault.MoveNote to fix up a moved knowledge note's full-path
// reference inside a dependent index note's table (see
// skills/hylo-index-knowledge), for the same reason as
// RewriteFrontmatterPathRef above.
//
// Reports whether anything changed; raw is returned unmodified otherwise.
func RewriteTableRowPathRef(raw []byte, oldPath, newPath string) ([]byte, bool) {
	if !bytes.Contains(raw, []byte(oldPath)) {
		return raw, false
	}
	lines := bytes.Split(raw, []byte("\n"))
	changed := false
	for i, line := range lines {
		if !bytes.HasPrefix(bytes.TrimSpace(line), []byte("|")) {
			continue
		}
		if bytes.Contains(line, []byte(oldPath)) {
			lines[i] = bytes.ReplaceAll(line, []byte(oldPath), []byte(newPath))
			changed = true
		}
	}
	if !changed {
		return raw, false
	}
	return bytes.Join(lines, []byte("\n")), true
}
