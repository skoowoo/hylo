# Explore

Show the user the shape of what they know: which areas it falls into and how big each is, where it comes from, what has been growing or gone quiet, and how it lines up with their goals. Work from the indexes, the file tree and metadata; don't read unit bodies, so it stays quick on a large vault.

`/_knowledge` itself is flat, one file per unit, so listing it shows only names. The structure lives in the domain indexes, `/_knowledge/_indexes/<Domain>.md`: frontmatter `unit_count`, then a table row per unit (Title, Type, Tags, Summary, Path).

Run the commands below from the vault root: your instructions give its path, so `cd` there first when your working directory is a bot's folder. Without a shell, use the tool noted after each one.

**Run all steps to completion without stopping. Only speak at the end (Step 5).**

## Step 1 — Skeleton: areas and their size

`knowledge_list_indexes`, then the frontmatter of each index for its `unit_count`:

```bash
grep -H '^unit_count:' _knowledge/_indexes/*.md
```

(Without a shell: `extract_segment {"path": "<index>", "head": 6}` per index.)

Check coverage: units on disk against units indexed.

```bash
find _knowledge -maxdepth 1 -name '*.md' | wc -l
```

(Without a shell: `status`, whose `knowledge_notes` also counts the index files.) If noticeably more units exist than the indexes hold, note it for Step 5; don't fix the indexes yourself.

## Step 2 — Sources: where it comes from

Notes per top-level folder, leaving out Hylo's own folders except short notes:

```bash
find . -name '*.md' -not -path './.*' -not -path './_knowledge/*' -not -path './_memory/*' -not -path './_bots/*' -not -path './_setup/*' -not -path './_assets/*' | cut -d/ -f2 | sort | uniq -c | sort -rn | head -15
```

And whose words the knowledge holds:

```bash
grep -h '^perspective:' _knowledge/*.md | sort | uniq -c
```

Units with no `perspective` were compiled before it existed; judge the mix from the folder names instead (a clippings folder is what they read, short notes and a journal are their own). Without a shell, skip this step and say the sources weren't counted.

## Step 3 — Time: what is growing, what went quiet

```
knowledge_list {"kind": "knowledge", "latest": 90}
```

Map those units to domains in one pass, by their paths:

```bash
grep -oF -e '/_knowledge/<Title A>.md' -e '/_knowledge/<Title B>.md' … _knowledge/_indexes/*.md | cut -d: -f1 | sort | uniq -c
```

(Without a shell: read the indexes of the larger domains and match paths.)

- **Growing**: the domains with the most units updated in the last 30 days (`updated_at`).
- **Quiet**: domains with 5 or more units and none updated in the last 90 days.

## Step 4 — Goals: does it line up

Read the `## Active` items of `/_memory/_goals.md`. For each goal, find the domain or units it draws on (index names, or `knowledge_search` on its key words). Mark it **well covered**, **thin** (1–2 units) or **nothing yet**. No goals file: skip this step.

## Step 5 — Answer

1. Two to four sentences, each a finding with its numbers: the largest areas, the main sources and the split between their own writing and what they read, what's growing and what went quiet, and the gap between goals and knowledge. Link the units or notes you name.
2. A `mindmap`: root `# My knowledge`, one `##` per domain labelled with its count (`## AI · 214`), mark the growing ones (`· growing`) and quiet ones (`· quiet`), and under each 3–5 of its units by title: for a growing domain, its recently updated units; otherwise, the first rows of its index. Show the largest 8–10 domains; fold the rest into `## Other · N`. Plain labels, no wikilinks.
3. If the indexes were behind (Step 1), one line saying how many units aren't indexed yet and offering to index them.
4. A `choices` block with 3–4 next steps in the user's language, such as: open up one domain (name the largest), what went quiet, how this lines up with my goals, index the missing units (only when they were behind).

A pick of one domain: read its index and show its units grouped by type or tag, with the summaries. A pick of a quiet area or a unit: hand over to Find (a topic, or how a view changed over time). Index the missing units: use the `hylo-index-knowledge` skill.
