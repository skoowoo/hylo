package util

import (
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// RewriteTag renames tag from to `to` in src, or removes it when to is "".
// A front matter item is replaced or dropped (the key too once its list is
// empty); an inline #tag is renamed, or loses its '#' so the sentence around
// it stays whole. Only the touched lines change. Reports whether src changed.
func RewriteTag(src []byte, from, to string) ([]byte, bool) {
	fromKey, toKey := NormalizeTag(from), NormalizeTag(to)
	if fromKey == "" || fromKey == toKey {
		return src, false
	}
	_, body := ParseFrontmatter(src)
	head := src[:len(src)-len(body)]
	newHead, c1 := rewriteFrontmatterTags(head, fromKey, toKey)
	newBody, c2 := rewriteInlineTags(body, fromKey, toKey)
	if !c1 && !c2 {
		return src, false
	}
	return append(append([]byte{}, newHead...), newBody...), true
}

func rewriteInlineTags(body []byte, fromKey, toKey string) ([]byte, bool) {
	var out []byte
	last := 0
	for _, s := range inlineTagSpans(body) {
		if NormalizeTag(string(body[s.from:s.to])) != fromKey {
			continue
		}
		out = append(out, body[last:s.from]...)
		if toKey == "" {
			out = append(out, body[s.from+1:s.to]...)
		} else {
			out = append(out, '#')
			out = append(out, toKey...)
		}
		last = s.to
	}
	if out == nil {
		return body, false
	}
	return append(out, body[last:]...), true
}

// rewriteFrontmatterTags edits the tags field of head, the "---" block that
// ParseFrontmatter found (empty when there is none).
func rewriteFrontmatterTags(head []byte, fromKey, toKey string) ([]byte, bool) {
	if len(head) == 0 {
		return head, false
	}
	lines := strings.Split(string(head), "\n")
	closing := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimRight(lines[i], "\r") == "---" {
			closing = i
			break
		}
	}
	if closing < 0 {
		return head, false
	}
	var doc yaml.Node
	if yaml.Unmarshal([]byte(strings.Join(lines[1:closing], "\n")), &doc) != nil || len(doc.Content) == 0 {
		return head, false
	}
	mapping := doc.Content[0]
	var key, val *yaml.Node
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if strings.EqualFold(strings.TrimSpace(mapping.Content[i].Value), "tags") {
			key, val = mapping.Content[i], mapping.Content[i+1]
		}
	}
	if key == nil {
		return head, false
	}

	// yaml lines are 1-based from the line after the opening "---", which
	// makes them indexes into lines.
	drop := map[int]bool{}
	switch {
	case val.Kind == yaml.SequenceNode && val.Style&yaml.FlowStyle == 0:
		kept, present := 0, false
		for _, item := range val.Content {
			if k := NormalizeTag(item.Value); k != fromKey && k != "" {
				kept++
				present = present || k == toKey
			}
		}
		matched := false
		for _, item := range val.Content {
			if NormalizeTag(item.Value) != fromKey {
				continue
			}
			matched = true
			if toKey == "" || present {
				drop[item.Line] = true
				continue
			}
			line := lines[item.Line]
			dash := strings.Index(line, "-")
			if dash < 0 {
				return head, false
			}
			lines[item.Line] = line[:dash] + "- " + yamlTagScalar(toKey)
			present, kept = true, kept+1
		}
		if !matched {
			return head, false
		}
		if kept == 0 {
			drop[key.Line] = true
		}
	case val.Kind == yaml.SequenceNode || val.Kind == yaml.ScalarNode:
		// One line only: a flow list wrapped over several is left alone.
		if val.Line != key.Line || (val.Kind == yaml.SequenceNode && !strings.Contains(lines[key.Line], "]")) {
			return head, false
		}
		var items []string
		if val.Kind == yaml.SequenceNode {
			for _, item := range val.Content {
				items = append(items, item.Value)
			}
		} else {
			items = strings.FieldsFunc(val.Value, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' })
		}
		next, matched := renameInList(items, fromKey, toKey)
		if !matched {
			return head, false
		}
		line := lines[key.Line]
		prefix := line[:strings.Index(line, ":")+1]
		switch {
		case len(next) == 0:
			drop[key.Line] = true
		case val.Kind == yaml.SequenceNode:
			quoted := make([]string, len(next))
			for i, t := range next {
				quoted[i] = yamlTagScalar(t)
			}
			lines[key.Line] = prefix + " [" + strings.Join(quoted, ", ") + "]"
		default:
			v := strings.Join(next, ", ")
			// Unquoted, " #b" would start a YAML comment.
			if val.Style&(yaml.DoubleQuotedStyle|yaml.SingleQuotedStyle) != 0 || strings.Contains(v, "#") {
				v = strconv.Quote(v)
			}
			lines[key.Line] = prefix + " " + v
		}
	default:
		return head, false
	}

	out := lines[:0:0]
	empty := true
	for i, l := range lines {
		if drop[i] {
			continue
		}
		out = append(out, l)
		if i > 0 && i < closing && strings.TrimSpace(l) != "" {
			empty = false
		}
	}
	// A bare "---\n---" would read as a rule, not front matter.
	if empty {
		return nil, true
	}
	return []byte(strings.Join(out, "\n")), true
}

// renameInList maps fromKey to toKey in items (dropping it when toKey is ""
// or already there), keeping every other item as written.
func renameInList(items []string, fromKey, toKey string) ([]string, bool) {
	present := false
	for _, it := range items {
		present = present || (toKey != "" && NormalizeTag(it) == toKey)
	}
	var out []string
	matched := false
	for _, it := range items {
		if NormalizeTag(it) != fromKey {
			out = append(out, it)
			continue
		}
		matched = true
		if toKey != "" && !present {
			out = append(out, toKey)
			present = true
		}
	}
	return out, matched
}

var plainTagScalar = regexp.MustCompile(`^[\p{L}\p{N}\p{M}_\-/.]+$`)

func yamlTagScalar(s string) string {
	if plainTagScalar.MatchString(s) {
		return s
	}
	return strconv.Quote(s)
}
