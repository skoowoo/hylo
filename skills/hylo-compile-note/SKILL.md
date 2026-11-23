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
- `source_tags`, the `#tags` and front matter tags the author put on it (`extract_tag`).
- `source_perspective`, whose words it holds. `own`: the author's writing (short notes, journal, drafts, notes on their own projects, their own podcast or product). `external`: someone else's (clipped articles, transcripts of others' podcasts or talks, book excerpts). Judge by content, not folder; a clip with the author's comments stays `external` unless the comments carry the note.

## Step 2 — Load the relevant index

`knowledge_list_indexes`, pick the 1–2 domains closest to the note, and read only those indexes for their unit tables. No indexes: `knowledge_list {"kind": "knowledge"}`.

## Step 3 — Pre-compile (internal, never shown)

List the units to create or update, without writing content yet.

A unit is a wiki page for a **key entity** the note treats with substance: a person, company, book, concept, technique, place, event, product, method, anything. It qualifies only if all four hold; otherwise leave it in the source note, or integrate it into an existing unit when it improves that unit:

1. **A stable name**: a name the note itself uses, or an established term. Never a coined phrase, a claim ("X matters more than Y") or a topic; a claim goes into the unit of the entity it's about.
2. **Depth to accumulate**: mechanisms, tensions, implications, not just a definition. One clear defining sentence, and more to say beyond it.
3. **It will recur** across notes from different contexts and dates.
4. **Worth finding again**: the source offers a distinctive mechanism, concrete case, useful counterexample or boundary, the author's experience or changed understanding, or an explanation they explicitly value. Name that contribution internally. A familiar entity, a definition, a generic overview or repetition alone does not qualify. A basic concept can still qualify when the source gives it this personal or explanatory value; do not invent the author's interest.

Calibrate:

- **Too broad**: the title is a domain ("AI Trends"). Find the specific entity the note advances.
- **Too narrow**: a sub-point, data point, metric, dependent sub-argument or passing mention. Fold it in.
- **Two units or one**: separate only when each has an independent core and passes all four criteria. Length alone is not a reason to split; keep an example or implication within its parent, or leave detail in the source.

Before marking a unit `create`, `knowledge_search` its name, aliases and other-language names: indexes miss units in other domains or not yet indexed. A matching entity is an `update` candidate. Read that unit before deciding whether the note improves it: a new mechanism, boundary, better explanation or example can qualify; another source repeating the same content does not. Similar names alone do not establish the same entity.

Manifest:

| unit name | entity_type | operation | contribution |
| --- | --- | --- | --- |
| <unit title> | <freeform> | create | <why this deserves its own page> |
| <unit title> | <freeform> | update | <what becomes clearer, changes or can be removed> |

`entity_type` is a short freeform label (`person`, `company`, `book`, `concept`, `event`, `technique`…). Before Step 4, read the file of every `update` unit at `/_knowledge/<unit-name>.md`.

Remove candidates with no meaningful content improvement from the manifest. An empty manifest is a valid result: when the note adds nothing the units lack, or nothing qualifies, write nothing, say why in one line and stop. Never force a unit out of a thin note.

## Step 4 — Compile

Each unit follows *Unit format* below plus its operation's file: `references/create.md` or `references/update.md`. Finish one unit before the next. Then say which were created, which updated, and what changed in each.

## Language

Match the language of the existing units, or of the note when there are none. The title uses that language alone, with no translation in parentheses; other names go in `aliases`, and wikilinks use the exact title. Required sections take exactly the names below, or `## 关联知识` / `## 来源` in Chinese. Dates are `YYYY-MM-DD`.

## Fidelity

A unit holds only what its source notes say. Add no background or facts from your own knowledge, even to round out an entity.

- Locate every number, date and quote in the note before writing it, and keep its precision: "三千多" stays 三千多, never 3,600. Computed values show their inputs. Can't find it: leave it out.
- In `external` notes, attribute opinions to whoever holds them.

## Time

Anchor time to the source's **original date**, not the compile date: write "as of <source_date>" for anything time-sensitive. When `source_date` is unknown, leave the time out or say "date unknown"; never use today's date.

## Unit format

**Content.** Choose and name sections freely; let the entity dictate the structure. Ask what the author needs to know about it when they return later. A unit holds only what's most important: source notes are the permanent record, the unit is a living distillation. Aim for a compact main text readable in a few minutes, with only the background needed to understand its distinctive contribution. More sources need not mean more text. Compress repetition and send readers to the source for full detail; preserve essential mechanisms, attribution and qualifications rather than meeting a hard word limit.

**Two required sections:**

- `## Related Knowledge` — wikilinks to other knowledge units, each with a brief note on the relationship. Omit it when there are none.
- `## Sources` — always the last section: one `- [[<note-stem>]]` per source note, never duplicated. The stem is the filename without path or extension (`/notes/2024-01-15 Weekly Review.md` → `[[2024-01-15 Weekly Review]]`).

**Frontmatter:**

```yaml
---
kind: knowledge
title: <Title>
aliases: [other names, other-language names]   # omit when none
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

**Tags.** Call `tag_list` first. Decide the tags that describe the unit, starting from the `source_tags` that fit it (the author chose them), then swap each other tag for an existing equivalent (same concept, other language or spelling: `独立开发` ↔ `indie-dev`). Keep a new tag only when no equivalent exists. Write them lowercase, without `#`, spaces as `-`.
