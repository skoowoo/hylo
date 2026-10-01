# Associative Recap

Given a piece of freshly written content, cross-check it against the knowledge base and surface two things: knowledge that **reinforces or extends** it, and knowledge that's in **tension** with it. This is a real-time recall aid, not a compile step — the output is always a conversational recap, never a saved file.

**Run all steps to completion without stopping. Only speak at the end (Step 5), presenting the recap.**

---

## Inputs

1. **Source content** — one of:
   - Inline text the user pasted directly into the request
   - A note reference (vault-absolute path or bare filename) — a short note, daily note, journal entry, or draft
2. **Knowledge directory** — default `_knowledge/` (relative to cwd, which is always the vault root)

---

## Step 1 — Get the source content

- **Inline text**: use it directly as `source_text`.
- **A named note**: `hylo read <path-or-name>`. If it's long (a draft, article), don't load it all — `hylo extract outline <path>` first, then pull only the sections carrying the main claims via `hylo extract section` or `hylo extract segment --head/--tail`. `references/extract.md` documents these commands.
- **"My latest short note" / "today's shorts"**: `hylo short list --latest 1 --limit 1` — the entry's `content` is returned inline, no file read needed.

---

## Step 2 — Distill search signal

From `source_text`, extract 3–8 short search phrases: the core named entities, concepts, and — critically — the **explicit claims or stances** the content takes (not just topics). Contradiction-checking depends on stance, not keyword overlap: "remote work" as a search term finds units about remote work, but only a stance ("remote work hurts collaboration") lets you judge whether a unit agrees or conflicts.

Skip generic words and boilerplate. If the source content has no clear claim (pure log entry, todo list, fact dump), search on its entities alone — tension-checking may simply come up empty, which is fine.

---

## Step 3 — Search the knowledge base

If the domain is obviously unclear or the vault is large, run `hylo knowledge list-indexes --table` first to see the domain landscape and narrow scope. Otherwise skip straight to search.

For each search phrase:

```bash
hylo knowledge search "<phrase>"
```

Collect and deduplicate candidate unit paths across all phrase searches.

---

## Step 4 — Read and classify candidates

For each candidate unit:

1. `hylo extract outline <unit>` — quick structural check; drop units that are obviously off-topic.
2. For plausible matches, read enough to judge the actual stance: `hylo extract section <unit> "<heading>"` if the outline pinpoints it, otherwise `hylo knowledge read <unit>`.

Classify each surviving unit against the source content:

- **Reinforces** — the unit's existing understanding aligns with or supports the source's claim.
- **Extends** — relevant background or adjacent context; deepens the picture without agreeing or disagreeing on stance.
- **Contradicts** — genuine tension: the unit holds a position, definition, or fact that conflicts with what the source asserts. Do not stretch weak overlaps into contradictions — a real contradiction should be paraphrasable in one sentence that makes the conflict obvious.
- **Associative leap** — not a direct topical match, but a surprising, non-obvious connection worth surfacing. Use sparingly (0–2 max) and only when it's genuinely thought-provoking, not filler.

Discard units that are only weakly related. Quality over quantity — 3–8 total surviving units across all categories is typical. If nothing survives, say so plainly in Step 5 rather than forcing weak connections.

---

## Step 5 — Present the recap

Output directly in the response — this is meant to be read now, while the thought is fresh. Never write it to a file. Format:

```markdown
# Recap: <one-line gist of the source content>

## 🔗 Reinforces / Extends
- **[[Unit Title]]** — <1-2 sentences: what it says, how it connects to the source>

## ⚡ Tension
- **[[Unit Title]]** — <what conflicts, stated concretely — paraphrase the unit's actual position, don't just say "conflicts"> — <optionally, a question back to the user: has their thinking shifted, or is this a one-off?>

## 💭 Associative
- **[[Unit Title]]** — <the unexpected link, and why it's worth noticing>
```

Omit any section with nothing in it. If no related or conflicting knowledge exists at all, state that directly instead of printing empty headings.
