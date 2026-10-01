# Extract — Partial Note Reading

`hylo extract` reads parts of a note without loading the full content. Use it whenever the full note is not needed.

## Subcommands

**Outline** — heading structure only:
```bash
hylo extract outline <path-or-name>
```

**Section** — one named section and its content (ends at the next heading of equal or higher level):
```bash
hylo extract section <path-or-name> "<heading>"   # case-insensitive
hylo extract section today.md "## meeting notes"
```

**Segment** — line range:
```bash
hylo extract segment <path-or-name> --head 20        # first N lines
hylo extract segment <path-or-name> --tail 10        # last N lines
hylo extract segment <path-or-name> --start 5 --end 30  # specific range (1-based, inclusive)
```

**Tags** — front-matter tags only:
```bash
hylo extract tag <path-or-name>
```

**Code blocks** — all fenced code blocks:
```bash
hylo extract code <path-or-name>
```

**Lists** — all bullet and numbered lists:
```bash
hylo extract list <path-or-name>
```

**Links** — all links (wikilinks and markdown links); each result shows type and target:
```bash
hylo extract link <path-or-name>
```

## Path rules

Same as `hylo read`: accepts vault-absolute paths (`/journal/today.md`) or bare filenames (resolves to most recently updated match).
