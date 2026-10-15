# Extract — Partial Note Reading

The `extract_*` tools read parts of a note without loading the full content. Use them whenever the full note is not needed.

## Tools

**Outline** — heading structure only, as `{level, text}` per heading:
```
extract_outline {"path": "<path-or-name>"}
```

**Section** — one named section and its content (ends at the next heading of equal or higher level):
```
extract_section {"path": "<path-or-name>", "heading": "<heading>"}   # case-insensitive
extract_section {"path": "today.md", "heading": "## meeting notes"}
```

**Segment** — line range; the result also carries `total_lines`:
```
extract_segment {"path": "<path-or-name>", "head": 20}             # first N lines
extract_segment {"path": "<path-or-name>", "tail": 10}             # last N lines
extract_segment {"path": "<path-or-name>", "start": 5, "end": 30}  # specific range (1-based, inclusive)
```

**Tags** — front-matter tags only:
```
extract_tag {"path": "<path-or-name>"}
```

**Code blocks** — all fenced code blocks:
```
extract_code {"path": "<path-or-name>"}
```

**Lists** — all bullet and numbered lists:
```
extract_list {"path": "<path-or-name>"}
```

**Links** — all links (wikilinks and markdown links); each result shows kind and target:
```
extract_link {"path": "<path-or-name>"}
```

## Path rules

Same as `read`: accepts vault-absolute paths (`/journal/today.md`) or bare filenames (resolves to most recently updated match).
