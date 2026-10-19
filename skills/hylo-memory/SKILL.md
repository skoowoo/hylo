---
name: hylo-memory
description: "Keeps the user's personal memory under /_memory/: extracts it from recent notes, or records one fact they just told you. Use for 'update my memory', 'refresh my memory from my notes', or when they share something lasting about themselves and want it kept ('remember that I…', '记住我…')."
---

# Hylo Memory

Two jobs: **extract** memories from recent notes (the steps below), or **record one fact** the user just told you (the last section). Both use the file format and update rules in Step 4.

Extraction pulls personal memories from short notes (`/_shorts`) and the knowledge base (`/_knowledge`) into six structured memory files under `/_memory/`. On first run (no memory files exist yet), scans the last 90 days for a rich initial snapshot. On subsequent runs, scans only the last 2 days. Memories not reinforced over time gradually fade and are eventually removed.

Calls below are Hylo's MCP tools (server `hylo`); your client may show them prefixed, e.g. `mcp__hylo__read`. If they are not in your tool list, search your deferred tools for "hylo".

**Run all steps to completion without stopping or asking for confirmation.** Only speak at the final summary step.

---

## Inputs

1. **Self-introduction** — optional. Whatever the user says about themselves in the request (name, role, projects). This often runs in the background, where nobody can answer, so never ask for it.
2. **Extra scan paths** — additional vault path prefixes to scan beyond the two defaults (optional).
3. **Memory directory** — where memory files live (default: `/_memory/`).

---

## Step 1 — Determine run mode

Check whether any memory file already exists:

```
read {"path": "/_memory/_identity.md"}
```

- **First run** (file not found): set `scan_window = 90` days.
- **Incremental run** (file exists): set `scan_window = 2` days.

Record today's date as `today`.

Build an **author profile** from `/_memory/_identity.md` and `/_memory/_goals.md` (whichever exist) plus any self-introduction in the request, to use as a lens throughout extraction:
- Name and known aliases/IDs
- Projects or products the author owns or runs (these may appear as vault directories or knowledge unit titles)
- Roles (creator, host, founder, etc.)
- People the author mentions as part of their personal life

The profile matters in the knowledge base step for units without a `perspective`: a `source_notes` path under the author's own project (e.g. their own podcast, their own product) is their own, not an external source.

---

## Step 2 — Collect content to process

Run both queries in parallel:

```
short_list {"latest": <scan_window>, "limit": 100}
knowledge_list {"kind": "knowledge", "latest": <scan_window>, "limit": 50}
```

Also call `list {"path": "<path>", "latest": <scan_window>}` for each extra scan path provided.

If all queries return zero results, skip to Step 4.

---

## Step 3 — Extract evidence

Apply different extraction rules depending on the source.

### Source A: Short notes (`/_shorts`)

`short_list` returns each entry's `content` inline — process directly, no file reads needed.

Short notes are the author's own unfiltered voice. Extract from **all six dimensions**:

| Dimension       | File              | What qualifies                                                                  |
| --------------- | ----------------- | ------------------------------------------------------------------------------- |
| **identity**    | `_identity.md`    | Stable facts: name, job title, location, family, languages                      |
| **preferences** | `_preferences.md` | Likes/dislikes, tools of choice, aesthetic tastes expressed as the author's own |
| **goals**       | `_goals.md`       | Active projects, ambitions, things the author is working toward                 |
| **beliefs**     | `_beliefs.md`     | Worldview, recurring opinions, values the author endorses                       |
| **people**      | `_people.md`      | Named individuals and their relationship to the author                          |
| **state**       | `_state.md`       | Near-term mood, current struggles, what is top of mind                          |

### Source B: Knowledge base (`/_knowledge`)

`knowledge_list` returns note paths. Read each unit:

```
knowledge_read {"path": "<path>"}
```

**Sort each unit by its `perspective`** (whose words it holds; see your instructions). A unit without one: judge from its `source_notes` the same way, using the author profile from Step 1 — paths under the author's own projects count as their own.

**`own`** — the author's own words or experience. Extract from **all six dimensions**, same as Source A.

**`external`** — a synthesis of what the author heard or read, not their own views:
- **Do not** extract beliefs from it; a view the unit reports belongs to its source.
- **Do not** extract people (third-party guests and case-study figures are not personal relationships).
- Only preferences, synthesized as below.

**`mixed`** — treat it as both: its title and tags join the external pool below, and the six dimensions come only from what the unit attributes to the author's own sources.

**No `source_notes` at all** — treat as personal. Extract from all six dimensions.

**Preferences from external sources — synthesize after all `external` and `mixed` units are read:**

Do not write one "interested in X" entry per unit. Instead:
1. Collect their titles and tags into a pool.
2. Group into theme clusters by overlapping tags or subject matter.
3. For each cluster of ≥2 units, write **one synthesized preference item** describing the author's underlying orientation or taste — not a list of topics. The item should read like a trait, not a reading log. Example: units tagged `[indie-dev, startup]`, `[ai, startup]`, `[ai, llm]` → `对 AI 驱动的独立技术创业有持续投入，偏工程实践侧`.
4. Discard single-unit clusters — one reading is not a preference.

### Source C: Extra scan paths

Treat as personal notes. Extract from all six dimensions.

---

## Step 4 — Update memory files

For each dimension with new evidence, read the current file (if it exists), apply the update rules, then write the result.

### File format

```markdown
---
decay_window: <Xd>
last_updated: <YYYY-MM-DD>
---

## Active
- <item text> `last:<YYYY-MM-DD> seen:<N>`

## Fading
- <item text> `last:<YYYY-MM-DD> seen:<N>`
```

### Decay windows

| File              | decay_window |
| ----------------- | ------------ |
| `_identity.md`    | 365d         |
| `_beliefs.md`     | 90d          |
| `_preferences.md` | 90d          |
| `_people.md`      | 90d          |
| `_goals.md`       | 30d          |
| `_state.md`       | 7d           |

### Update rules (apply in this order)

**A — New evidence matches an existing Active item:** Update `last` to today, increment `seen`.

**B — New evidence matches a Fading item:** Promote to Active, update `last` to today, increment `seen`.

**C — New item not yet in the file:** Add to `## Active` with `last:<today> seen:1`.

**D — Active item with no new evidence:** If `(today − last) > decay_window`, move to `## Fading`. Do not change `last` or `seen`.

**E — Fading item with no new evidence:** If `(today − last) > 2 × decay_window`, delete entirely.

**F — Same fact, different wording:** Merge into one item — keep the clearer wording, sum `seen`, use the more recent `last`.

**G — Pinned items:** An item whose metadata ends in `pinned` is something the user stated outright. Rules D and E never apply to it, and keep `pinned` when you update it.

### Writing the file

```
create {"path": "/_memory/_<dimension>.md", "content": "<content>"}                  # new
create {"path": "/_memory/_<dimension>.md", "content": "<content>", "force": true}   # overwrite
```

Only write files that actually changed.

---

## Step 5 — Summary

Report:
- Run mode (first run / incremental) and scan window used
- Items processed per source (shorts / knowledge / extra)
- Which memory files were updated
- Counts: added / refreshed / faded / deleted
- Deleted items by name (for user verification)

---

## Record one fact

When the user tells you something lasting about themselves, or asks you to remember it:

1. Pick its file by the dimension table in Step 3, read it, and apply the update rules in Step 4 to that one item, in their words where you can.
2. Add `pinned` only for something they state as a standing preference or fact ("always answer in English", "I'm vegetarian"); not for current work or moods, which should fade.
3. Write the file and say in one line what you kept.
