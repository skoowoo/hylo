# Update — Existing Knowledge Unit

Your goal is not to append — it is to produce the best current distillation of the existing unit plus the new note.

Treat the update as editorial work:

- **Integrate, don't append.** Weave new content into the structure where it belongs — no section-appending, no "Updates" blocks.
- **Cite by paragraph.** After each paragraph introducing new content, append `[[note-stem]]`. For paragraphs drawing from multiple sources, list all relevant citations.
- **Reorganize when the weight has shifted.** If new content changes what's most important, restructure sections accordingly.
- **Merge redundancies.** When old and new content overlap, keep only the sharper version.
- **Remove what no longer earns its place.** Cut points that are superseded, contradicted, or outweighed by richer content. The unit is not a historical record — source notes are.
- **Rewrite for coherence.** The final unit should read as one document, not layered deposits from different dates.

Test: does the unit give the clearest possible picture of where the author's understanding stands today?

---

## Content organization

You have full freedom to choose and name sections. Let the entity dictate the structure — a person, a book, a company, and a technique each call for different organization. There is no prescribed template.

Ask yourself: what does the author need to know about this entity when they return to it later? What is their current understanding, and how should it be organized to grow?

**One principle**: a knowledge unit should contain only what's most important. Source notes are the permanent record — the unit is a living distillation.

---

## Two required sections

**`## Related Knowledge`** — wikilinks to other knowledge units with a brief note on the relationship. Omit if there are no meaningful connections.

**`## Sources`** — always the **last section** in the file. Append `- [[<current-note-stem>]]` only if not already listed — never duplicate. Stem is the source note's filename without path or extension (e.g. `/notes/2024-01-15 Weekly Review.md` → `- [[2024-01-15 Weekly Review]]`).

If the vault language is not English, translate section headings accordingly (e.g. `## 来源`).

---

## Frontmatter

```yaml
---
kind: knowledge
title: <Title>
entity_type: <freeform label>
tags: [tag1, tag2]
source_notes:
  - /path/to/note.md
created_at: YYYY-MM-DDTHH:MM:SSZ
last_compiled_at: YYYY-MM-DDTHH:MM:SSZ
compile_count: 1
---
```

Add the current note path to `source_notes`. Update `last_compiled_at` to today. Increment `compile_count`.

---

## Tag normalization

Before assigning tags, run:

```bash
hylo tag list
```

First determine the tags that accurately describe the unit. Then, for each tag, check the existing list for a semantically equivalent entry (same concept, different language or spelling — e.g. `独立开发` ↔ `indie-dev`). If one exists, replace your tag with it. Only keep a new tag when no equivalent exists.

---

## Pre-write review

Before writing, review the integrated unit against four criteria:

1. **Relevance** — Does every paragraph belong to this unit's core subject? Cut anything that drifted in from adjacent topics.

2. **Currency** — Is any content outdated or superseded by the new note? Cut or correct it. The unit reflects current understanding, not accumulated strata.

3. **Coherence** — Does it read as one document? Are section order and transitions immediately navigable?

4. **Concision** — Is everything load-bearing? Cut anything that restates what is already clear elsewhere.

Resolve all issues before writing.

---

## Write output

Overwrite the existing file with the full updated content.
