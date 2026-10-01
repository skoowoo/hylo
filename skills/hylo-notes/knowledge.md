# Knowledge

The knowledge base is three layers deep:

```
Domain Index  →  Knowledge Units  →  Raw Notes (sources)
```

- **Domain Index** — one index per domain; lists all knowledge units in that domain; the entry point.
- **Knowledge Units** — compiled, synthesized notes distilled from raw notes; usually sufficient to answer a question. Each unit contains links to its source raw notes.
- **Raw Notes** — original source notes; follow the source links inside a knowledge unit to read them directly when more detail is needed.

Retrieval path: `list-indexes` → read domain index → read matching knowledge units → follow source links to raw notes → `hylo search` as last-resort fallback only.

Knowledge notes are auto-generated and are read-only (except for explicit deletes).

## Retrieval Workflow

1. `hylo knowledge list-indexes --table` — identify relevant domains
2. `hylo knowledge read <index-path>` — read the domain index to find matching knowledge units
3. `hylo extract outline <unit-path>` — preview the knowledge unit's structure before reading in full
4. `hylo extract section <unit-path> "<heading>"` — read only the relevant section if the outline is sufficient to pinpoint it; otherwise `hylo knowledge read <unit-path>`
5. If more detail is needed: follow the source links inside the knowledge unit; use `hylo extract outline <source-path>` first, then `hylo read <source-path>` for full content
6. Last resort only: `hylo search "X"` — use when source links are absent or the above steps don't cover the question

If the domain is unclear, skip steps 1–2 and go straight to `hylo knowledge search "X"`.

## Command Reference

### List domain indexes

```bash
hylo knowledge list-indexes           # all domains with vault path (JSON)
hylo knowledge list-indexes --table   # table format
```

### List knowledge notes

```bash
hylo knowledge list --kind knowledge                                          # knowledge notes, newest first
hylo knowledge list --kind knowledge --latest 7                               # updated in the last 7 days
hylo knowledge list --kind knowledge --start 2026-01-01 --end 2026-01-31      # date range
hylo knowledge list --kind knowledge --limit 20                               # capped
```

### Read a knowledge note

```bash
hylo knowledge read "/_knowledge/summary.md"   # vault-absolute path
hylo knowledge read summary.md                  # bare filename — most recent match
```

### Search knowledge notes

```bash
hylo knowledge search "machine learning"         # name + content (default)
hylo knowledge search topic --field name         # filename only
hylo knowledge search "key idea" --field content # content only
hylo knowledge search "project" --limit 10
```

Query syntax: bare words are OR-joined; `"quoted phrase"` matches exactly.