---
name: hylo-index-knowledge
description: "Adds knowledge units that aren't in any domain index to /_knowledge/_indexes/, creating domains as needed; safe to rerun. Use after compiling, or when the user asks to build or refresh the knowledge index."
---

# Hylo Index Knowledge

Scans the knowledge directory, finds units not yet in any index, and adds them. Running this skill repeatedly is safe and always converges — it only ever adds missing entries, never rewrites existing ones.

Domain index files live at `/_knowledge/_indexes/<Domain>.md`. The compile skill matches a note to a domain by the index filename alone (e.g. `AI.md`, `Business.md`).

A unit may appear in **at most 2** indexes: one primary domain, one secondary only when the unit genuinely straddles two.

Tools named here are Hylo's MCP tools (server `hylo`, maybe shown as `mcp__hylo__read`). If they aren't in your tool list, search your deferred tools for "hylo"; if still missing, say Hylo's tools are unavailable and stop.

**Run all steps to completion without stopping. Only speak at the end (Step 4 summary).**

## Step 1 — Find unindexed units

- All units: `knowledge_list {"kind": "knowledge"}`.
- All indexes: `knowledge_list_indexes`, then `knowledge_read` each and collect every `Path` value from its table.

A unit is **unindexed** if its vault-absolute path (e.g. `/_knowledge/Vertical Agent.md`) does not appear in any index table. Build a list of only these units.

If all units are already indexed, report and stop.

## Step 2 — Read unindexed units

For each unindexed unit, read its YAML front matter and first non-heading paragraph. Build an internal list:

`extract_tag` returns the front-matter tags and `extract_segment` with `head` the opening lines, without a full `read`.

```
title          | entity_type | tags          | summary (≤80 chars)                              | vault-absolute path
Vertical Agent | concept     | [ai, agent]   | AI product focused on a specific vertical domain | /_knowledge/Vertical Agent.md
Jane Smith     | person      | [founder, ai] | Founder of SophiaPro, AI startup                 | /_knowledge/Jane Smith.md
```

## Step 3 — Assign and update indexes

For each unindexed unit, determine domain assignment:

1. **Match existing domains first** by semantic understanding of the domain name (from existing index filenames) against the unit's title, entity_type, tags, and summary. Assign the strongest match.

2. **Secondary domain** only when the unit genuinely belongs to two domains — passing relevance is not enough. When in doubt, assign one.

3. **New domain** when no existing domain fits. Create a new domain if the topic is broad and stable enough to plausibly accumulate more units over time (e.g. `Physics`, `History`, `Health`, `Law`). The test is domain breadth, not how many unindexed units are present right now — a single unit can justify a new domain. Only fall back to "General" when the topic is genuinely narrow or one-off and unlikely to grow.

4. **Domain names** should be broad and stable (e.g. `AI`, `Business`, `Science`, `People`, `Philosophy`). Don't name a domain after a single entity unless it anchors a large cluster.

For each affected domain index, add the new rows, update `unit_count` and `updated` in the frontmatter, and write the whole file back with `create` + `force`. A new domain is a new file at `/_knowledge/_indexes/<Domain>.md`, written with `create`:

```markdown
---
kind: index
domain: <Domain>
unit_count: <N>
updated: <YYYY-MM-DD>
---

# <Domain>

| Title          | Type    | Tags          | Summary                                          | Path                          |
| -------------- | ------- | ------------- | ------------------------------------------------ | ----------------------------- |
| Vertical Agent | concept | [ai, agent]   | AI product focused on a specific vertical domain | /_knowledge/Vertical Agent.md |
| Jane Smith     | person  | [founder, ai] | Founder of SophiaPro, AI startup                 | /_knowledge/Jane Smith.md     |
```

Keep rows sorted alphabetically by Title within each index file.

## Step 4 — Report

- How many units were already indexed vs. newly added
- Which index files were updated or created, and how many entries were added to each
