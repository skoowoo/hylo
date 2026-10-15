# Regular Notes — CRUD

All note writes (create, modify, delete) must go through the note tools — never touch note files directly.

## List notes

```
list {}                                             # all notes, sorted by most recently updated
list {"path": "/journal"}                           # scope to a vault directory
list {"latest": 7}                                  # updated in the last 7 days
list {"start": "2026-01-01", "end": "2026-01-31"}   # date range
list {"limit": 20}                                  # cap results
```

## Search notes

```
search {"query": "meeting notes"}                # search filename + content + tags (default)
search {"query": "april", "field": "name"}       # filename only
search {"query": "TODO", "field": "content"}     # content only
search {"query": "project", "limit": 10}
```

Query syntax: bare words are OR-joined; `"quoted phrase"` matches exactly.

## Read a note

```
read {"path": "/journal/today.md"}   # vault-absolute path
read {"path": "today.md"}            # bare filename — resolves to most recently updated match
```

If multiple notes share the same name, use the full vault path to be unambiguous.

## Preview / partial read (save tokens)

Before reading a note in full, use the `extract_*` tools to inspect only what you need.

**Decide whether to read the whole note:**
```
extract_outline {"path": "today.md"}               # heading structure — see if the note is relevant
extract_segment {"path": "today.md", "head": 20}   # first 20 lines — quick content preview
extract_tag {"path": "today.md"}                   # front-matter tags only
```

**Read only the part you need:**
```
extract_section {"path": "today.md", "heading": "Goals"}             # a named section (case-insensitive)
extract_section {"path": "today.md", "heading": "## meeting notes"}
extract_segment {"path": "today.md", "start": 10, "end": 30}         # specific line range
extract_segment {"path": "today.md", "tail": 15}                     # last 15 lines
```

**Extract specific content types:**
```
extract_code {"path": "today.md"}   # all fenced code blocks
extract_list {"path": "today.md"}   # all bullet/numbered lists
extract_link {"path": "today.md"}   # all links
```

**Recommended workflow for unknown or long notes:**
1. `extract_outline` — scan the structure
2. `extract_section` — read only the relevant section(s)
3. `read` — fall back to full read only when the above isn't enough

## Create a note

```
create {"path": "/path/note.md", "content": "# Title\n\nContent here."}
```

Path rules:
- Must be vault-absolute and start with `/`
- Use a meaningful folder: `/journal/`, `/projects/`, `/research/`, etc.
- Use `.md` extension

## Modify a note

**Overwrite** (replace entire content):
```
create {"path": "/path/note.md", "content": "updated content", "force": true}
```

**Append** to the end, or to the end of a section with `heading`:
```
append {"path": "/path/note.md", "content": "new paragraph"}
append {"path": "/path/note.md", "content": "- new item", "heading": "Todo"}
```

**Prepend** to the top, or to the start of a section with `heading`:
```
prepend {"path": "/path/note.md", "content": "header line"}
```

Read the note first (`read`) if you need its current content before modifying.

## Move or rename a note

```
move {"path": "/path/note.md", "new_dir": "/archive"}
rename {"path": "/path/note.md", "new_name": "better-name.md"}
```

`rename` updates links to the note across the vault in the background; poll `rename_status {"job_id": <id>}` if you need to know it finished.

## Delete a note

```
delete {"path": "/path/note.md"}
```

Path must be vault-absolute. Use `resolve {"name": "<filename>"}` first if you only know the filename.
