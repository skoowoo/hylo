# Longform

Write an article with a point of view, beat by beat, from the knowledge base. Read `references/beats.md` first: it says what a beat is and how to write one.

## Phase 0 — Topic and palette

**Guess, don't quiz.** Take the topic from the request. Guess the reader and the stance from the request, `/_memory/_goals.md` and `/_memory/_beliefs.md`. Ask only when the topic itself is missing.

**No topic yet** ("I want to write something but don't know what"): look at knowledge units added or updated in the last 30 days (`knowledge_list` with `latest: 30`) and `/_memory/_goals.md`, and offer 3 topics as `choices`, one line each on why it's ripe now.

Then find material:

1. `knowledge_list_indexes`, pick the 1–3 most relevant domain indexes, `knowledge_read` them for matching units. With no indexes, scan `_knowledge/` directly.
2. If the units are thin, add uncompiled notes (`search`) and short notes (`short_list`).

**Present the palette for confirmation.** Don't go on to Phase 1 until the user signs off:

---
*Reader: first-time managers · Stance: async-first is usually the wrong default* (one line with your guesses; they can correct it)

**Sources found**

| # | Source | Type | What it contributes |
| --- | --- | --- | --- |
| 1 | [[unit-name]] | knowledge unit | one sentence on what it brings |
| 2 | [[note-name]] | note | … |
| 3 | [[2026-10-12]] | short note | … |

**Possible gaps:** what the article needs that the sources don't cover (if any).

Then `choices`: start writing with these; drop some; compile more notes first.
---

If they want more coverage, suggest `hylo-compile-note` on the relevant source, then re-scan and show the palette again. Never start with a palette they haven't confirmed.

Pick the save path yourself (a top-level folder) and say it in one line.

## Phase 1 — Starting beats

Write 2–3 candidate **starting beats**, each drawn from different sources in the palette: different entry points into the same topic. Show them before writing anything to the article. For each, name its sources and preview where it might lead. Offer them as `choices`, one short label per beat.

## Phase 2 — Beat by beat

Loop until the article reaches a natural end:

1. Write **only the chosen beat** to the article (in the editor: `editor_type` at `end`, one call per beat, default speed). Follow the voice in every beat.
2. Re-read the article (in the editor: `editor_read`).
3. Offer 2–3 candidate **next beats** as `choices`, naming the sources each would draw from, plus "finish the rest in one go" and "end here".
4. Repeat with their pick.

On "finish the rest in one go", write the remaining beats one call each without stopping to offer choices, then go to Phase 3.

## Phase 3 — Close

Write the final beat and stop. Then ask with `choices` whether to compile the article back into the knowledge base; if yes, run `hylo-compile-note` on it. End your reply with the article's wiki link (its filename without `.md`), e.g. `[[on-focus]]`, as the last line.
