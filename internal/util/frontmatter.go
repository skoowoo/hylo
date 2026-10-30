package util

import (
	"bytes"
	"fmt"
	"strings"
	"time"
	"unicode"

	"gopkg.in/yaml.v3"
)

func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

// FrontmatterEntry is one key-value pair from the YAML front matter,
// pre-processed for template rendering.
type FrontmatterEntry struct {
	Key    string
	Value  string   // non-empty for scalar values
	List   []string // non-empty for sequence values
	IsList bool
	IsURL  bool // true when Value is an http/https URL
}

// Frontmatter holds parsed YAML front matter.
// All contains every field in document order; the typed fields (Source, Tags,
// Compiled, Kind) are convenience accessors populated from All.
type Frontmatter struct {
	Source   string
	Tags     []string
	Compiled string
	Kind     string             // value of the "kind" field, lowercased (e.g. "knowledge")
	All      []FrontmatterEntry // all fields in document order, for rendering
}

// HasMeta reports whether the front matter contained any fields.
func (f Frontmatter) HasMeta() bool { return len(f.All) > 0 }

// ParseFrontmatter splits a markdown document into its YAML front matter and
// the remaining body. Front matter must begin on the very first line with "---"
// and be closed by a second "---" line. If no valid front matter is found, an
// empty Frontmatter and the original src are returned.
func ParseFrontmatter(src []byte) (Frontmatter, []byte) {
	const delim = "---"

	// Must start with "---\n" or "---\r\n".
	if !bytes.HasPrefix(src, []byte(delim)) {
		return Frontmatter{}, src
	}
	rest := src[len(delim):]
	if len(rest) == 0 || (rest[0] != '\n' && rest[0] != '\r') {
		return Frontmatter{}, src
	}
	if rest[0] == '\r' {
		rest = rest[1:]
	}
	rest = rest[1:]

	// Find the closing "---".
	end := bytes.Index(rest, []byte("\n"+delim))
	if end < 0 {
		return Frontmatter{}, src
	}
	yamlBlock := rest[:end]
	body := rest[end+1+len(delim):]
	if len(body) > 0 && body[0] == '\r' {
		body = body[1:]
	}
	if len(body) > 0 && body[0] == '\n' {
		body = body[1:]
	}

	// Parse into a yaml.Node to preserve field order.
	var doc yaml.Node
	if err := yaml.Unmarshal(yamlBlock, &doc); err != nil || doc.Kind == 0 {
		return Frontmatter{}, src
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return Frontmatter{}, src
	}
	mapping := doc.Content[0]
	if mapping.Kind != yaml.MappingNode {
		return Frontmatter{}, src
	}

	var fm Frontmatter
	// Mapping nodes store alternating key, value pairs.
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		keyNode := mapping.Content[i]
		valNode := mapping.Content[i+1]
		key := strings.ToLower(strings.TrimSpace(keyNode.Value))

		entry := FrontmatterEntry{Key: key}

		switch valNode.Kind {
		case yaml.SequenceNode:
			entry.IsList = true
			for _, item := range valNode.Content {
				entry.List = append(entry.List, item.Value)
			}
		case yaml.MappingNode:
			// Nested map: render as indented key: value pairs.
			entry.Value = renderMapping(valNode, 0)
		default:
			entry.Value = valNode.Value
			entry.IsURL = isURL(valNode.Value)
		}

		fm.All = append(fm.All, entry)

		// Populate typed convenience fields.
		switch key {
		case "source":
			fm.Source = entry.Value
		case "tags":
			fm.Tags = entry.List
			if !entry.IsList {
				// Obsidian also accepts `tags: a, b` and `tags: a b`.
				fm.Tags = strings.FieldsFunc(entry.Value, func(r rune) bool { return r == ',' || unicode.IsSpace(r) })
			}
		case "compiled", "distilled": // "distilled" for backward compat with older notes
			fm.Compiled = entry.Value
		case "kind":
			fm.Kind = entry.Value
		}
	}

	return fm, body
}

// IsKnowledgeNote reports whether content is a knowledge note by checking for
// kind: knowledge in its YAML front matter. This is the canonical way to detect
// knowledge notes from raw content, independent of database metadata.
func IsKnowledgeNote(content []byte) bool {
	fm, _ := ParseFrontmatter(content)
	return strings.EqualFold(fm.Kind, "knowledge")
}

// IsIndexNote reports whether content is an index note by checking for
// kind: index in its YAML front matter.
func IsIndexNote(content []byte) bool {
	fm, _ := ParseFrontmatter(content)
	return strings.EqualFold(fm.Kind, "index")
}

// IsShortNote reports whether content is a short note daily file by checking
// for kind: short in its YAML front matter.
func IsShortNote(content []byte) bool {
	fm, _ := ParseFrontmatter(content)
	return strings.EqualFold(fm.Kind, "short")
}

// FormatShortFrontmatter returns the YAML front matter block for a short note
// daily file: opening/closing "---" delimiters and a trailing blank line.
func FormatShortFrontmatter(date time.Time) string {
	return "---\nkind: short\ndate: " + date.Format("2006-01-02") + "\n---\n\n"
}

// renderMapping formats a YAML mapping node as "key: value" lines.
func renderMapping(node *yaml.Node, depth int) string {
	var sb strings.Builder
	indent := strings.Repeat("  ", depth)
	for i := 0; i+1 < len(node.Content); i += 2 {
		k := node.Content[i].Value
		v := node.Content[i+1]
		switch v.Kind {
		case yaml.MappingNode:
			fmt.Fprintf(&sb, "%s%s:\n%s", indent, k, renderMapping(v, depth+1))
		case yaml.SequenceNode:
			var items []string
			for _, item := range v.Content {
				items = append(items, item.Value)
			}
			fmt.Fprintf(&sb, "%s%s: [%s]\n", indent, k, strings.Join(items, ", "))
		default:
			fmt.Fprintf(&sb, "%s%s: %s\n", indent, k, v.Value)
		}
	}
	return sb.String()
}
