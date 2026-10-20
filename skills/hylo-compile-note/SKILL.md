---
name: hylo-compile-note
description: "Compiles one raw note into knowledge units under /_knowledge/ (triage, then create or update each unit). Use when the user wants a note compiled or turned into knowledge ('compile this note', '编译这篇笔记'), or when a compile trigger runs."
---

# Hylo Compile Note

Compiles one raw note into **knowledge units**: wiki-style pages holding the author's growing understanding of the entities that matter to them.

Tools named here are Hylo's MCP tools (server `hylo`, maybe shown as `mcp__hylo__read`). If they aren't in your tool list, search your deferred tools for "hylo"; if still missing, say Hylo's tools are unavailable and stop.

**Run all four steps without stopping**: no intermediate results, confirmations or pauses. Speak only at the end of Step 4.

The input is the note to compile (a vault-absolute or filesystem path). Units live in `/_knowledge/`.

## Step 1 — Read the note

Read it in full. Record:

- `source_date`, its **original publication date**, from the first of: frontmatter `date` / `published`, a date prefix in the filename, an article or podcast header, a timestamp in the body. None found: unknown.
- `source_perspective`, whose words it holds. `own`: the author's writing (short notes, journal, drafts, notes on their own projects, their own podcast or product). `external`: someone else's (clipped articles, transcripts of others' podcasts or talks, book excerpts). Judge by content, not folder; a clip with the author's comments stays `external` unless the comments carry the note.

## Step 2 — Load the relevant index

`knowledge_list_indexes`, pick the 1–2 domains closest to the note, and read only those indexes for their unit tables. No indexes: `knowledge_list {"kind": "knowledge"}` and build a compact table yourself (title | entity_type | tags | first paragraph, ≤80 chars).

## Step 3 — Pre-compile (internal, never shown)

List the units to create or update, without writing content yet.

A unit is a wiki page for a **key entity** the note treats with substance: a person, company, book, concept, technique, place, event, product, method, anything. It qualifies only if all three hold; otherwise fold the content into another unit:

1. **A stable name**: a recognized term or proper noun, not a sentence ("the argument that…").
2. **Depth to accumulate**: mechanisms, tensions, implications, not just a definition. One clear defining sentence, and more to say beyond it.
3. **It will recur** across notes from different contexts and dates.

Calibrate:

- **Too broad**: the title is a domain ("AI Trends"). Find the specific entity the note advances.
- **Too narrow**: a sub-point, data point, metric, dependent sub-argument or passing mention. Fold it in.
- **Two units or one**: split when they have different cores that would grow independently; keep one as a sub-point when it's an example or implication of the other.

Manifest (may be empty):

| unit name | entity_type | operation |
| --- | --- | --- |
| <unit title> | <freeform> | create |
| <unit title> | <freeform> | update |

`entity_type` is a short freeform label (`person`, `company`, `book`, `concept`, `event`, `technique`…), with no templates tied to it. Before Step 4, read the file of every `update` unit at `/_knowledge/<unit-name>.md`.

## Step 4 — Compile

Each unit follows *Unit format* below plus its operation's file: `references/create.md` or `references/update.md`. Finish one unit before the next. Then say which were created, which updated, and what changed in each.

## Language

Match the language of the existing units, or of the note when there are none; section names follow it (`## 来源`). Dates are `YYYY-MM-DD`.

## Time

Anchor time to the source's **original date**, not the compile date: write "as of <source_date>" for anything time-sensitive. When `source_date` is unknown, leave the time out or say "date unknown"; never use today's date.

## Unit format

**Content.** Choose and name sections freely; let the entity dictate the structure (a person, a book, a company and a technique each call for different organization). Ask what the author needs to know about it when they return later. A unit holds only what's most important: source notes are the permanent record, the unit is a living distillation.

**Two required sections:**

- `## Related Knowledge` — wikilinks to other knowledge units, each with a brief note on the relationship. Omit it when there are none.
- `## Sources` — always the last section: one `- [[<note-stem>]]` per source note, never duplicated. The stem is the filename without path or extension (`/notes/2024-01-15 Weekly Review.md` → `[[2024-01-15 Weekly Review]]`).

**Frontmatter:**

```yaml
---
kind: knowledge
title: <Title>
entity_type: <freeform label>
tags: [tag1, tag2]
source_notes:
  - /path/to/note.md
perspective: own   # own | external | mixed
created_at: YYYY-MM-DDTHH:MM:SSZ
last_compiled_at: YYYY-MM-DDTHH:MM:SSZ
compile_count: 1
---
```

**Tags.** Call `tag_list` first. Decide the tags that describe the unit, then swap each for an existing equivalent (same concept, other language or spelling: `独立开发` ↔ `indie-dev`). Keep a new tag only when no equivalent exists.
