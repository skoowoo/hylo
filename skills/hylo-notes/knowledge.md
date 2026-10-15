# Knowledge

The knowledge base is three layers deep:

```
Domain Index  →  Knowledge Units  →  Raw Notes (sources)
```

- **Domain Index** — one index per domain; lists all knowledge units in that domain; the entry point.
- **Knowledge Units** — compiled, synthesized notes distilled from raw notes; usually sufficient to answer a question. Each unit contains links to its source raw notes.
- **Raw Notes** — original source notes; follow the source links inside a knowledge unit to read them directly when more detail is needed.

Retrieval path: `knowledge_list_indexes` → read domain index → read matching knowledge units → follow source links to raw notes → `search` as last-resort fallback only.

Knowledge notes are auto-generated and are read-only (except for explicit deletes).

## Retrieval Workflow

1. `knowledge_list_indexes` — identify relevant domains
2. `knowledge_read {"path": "<index-path>"}` — read the domain index to find matching knowledge units
3. `extract_outline {"path": "<unit-path>"}` — preview the knowledge unit's structure before reading in full
4. `extract_section {"path": "<unit-path>", "heading": "<heading>"}` — read only the relevant section if the outline is sufficient to pinpoint it; otherwise `knowledge_read {"path": "<unit-path>"}`
5. If more detail is needed: follow the source links inside the knowledge unit; use `extract_outline` on the source first, then `read` for full content
6. Last resort only: `search {"query": "X"}` — use when source links are absent or the above steps don't cover the question

If the domain is unclear, skip steps 1–2 and go straight to `knowledge_search {"query": "X"}`.

## Tool Reference

### List domain indexes

```
knowledge_list_indexes {}   # all domains with their vault path
```

### List knowledge notes

```
knowledge_list {"kind": "knowledge"}                                          # knowledge notes, newest first
knowledge_list {"kind": "knowledge", "latest": 7}                             # updated in the last 7 days
knowledge_list {"kind": "knowledge", "start": "2026-01-01", "end": "2026-01-31"}  # date range
knowledge_list {"kind": "knowledge", "limit": 20}                             # capped
```

### Read a knowledge note

```
knowledge_read {"path": "/_knowledge/summary.md"}   # vault-absolute path
knowledge_read {"path": "summary.md"}               # bare filename — most recent match
```

### Search knowledge notes

```
knowledge_search {"query": "machine learning"}                # name + content (default)
knowledge_search {"query": "topic", "field": "name"}          # filename only
knowledge_search {"query": "key idea", "field": "content"}    # content only
knowledge_search {"query": "project", "limit": 10}
```

Query syntax: bare words are OR-joined; `"quoted phrase"` matches exactly.
