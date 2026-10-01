# Create — New Knowledge Unit

Synthesize the content from the current note into a clean knowledge unit written as a fresh document. There is no existing unit to preserve or reference.

Do not add inline citations anywhere in the body. All content originates from this note; the Sources section is the sole record.

---

## Content organization

You have full freedom to choose and name sections. Let the entity dictate the structure — a person, a book, a company, and a technique each call for different organization. There is no prescribed template.

Ask yourself: what does the author need to know about this entity when they return to it later? What is their current understanding, and how should it be organized to grow?

**One principle**: a knowledge unit should contain only what's most important. Source notes are the permanent record — the unit is a living distillation.

---

## Two required sections

**`## Related Knowledge`** — wikilinks to other knowledge units with a brief note on the relationship. Omit if there are no meaningful connections.

**`## Sources`** — always the **last section** in the file. Initialize with `- [[<current-note-stem>]]` where stem is the source note's filename without path or extension (e.g. `/notes/2024-01-15 Weekly Review.md` → `- [[2024-01-15 Weekly Review]]`).

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
  - /path/to/current-note.md
created_at: YYYY-MM-DDTHH:MM:SSZ
last_compiled_at: YYYY-MM-DDTHH:MM:SSZ
compile_count: 1
---
```

Set `created_at` and `last_compiled_at` to today. `compile_count: 1`.

---

## Tag normalization

Before assigning tags, run:

```bash
hylo tag list
```

First determine the tags that accurately describe the unit. Then, for each tag, check the existing list for a semantically equivalent entry (same concept, different language or spelling — e.g. `独立开发` ↔ `indie-dev`). If one exists, replace your tag with it. Only keep a new tag when no equivalent exists.

---

## Write output

Create `<knowledge-dir>/<Title>.md` with the full unit content.
