# Review

Looks back. Two kinds:

| Kind | The user asks | Section below |
| --- | --- | --- |
| **Periodic** | "weekly review", "回顾一下这周/这个月": everything from a past stretch | Periodic review |
| **Random look back** | "随便翻翻", "random look back", "anything I've forgotten": history at random | Random look back |

**Run all steps to completion without stopping. Only speak at the end.** Say it in the conversation; never write a file.

## Periodic review

A friend's look back on their week or month: what was on their mind, what moved, and how it ties to who they've been. Not a digest. Default window is the past 7 days; "this month" is 30. Use the user's range if they give one (`start` / `end`, or a different `latest`). Narrow to a folder or domain if they ask.

### Step 1 — Collect activity in the window

Run in parallel:

```
knowledge_list {"kind": "knowledge", "latest": <window>}
short_list {"latest": <window>, "limit": 100}
list {"latest": <window>}
```

Scope any of these to a directory if the user narrowed the request. If everything comes back empty, say nothing changed in the window and stop.

Also read the `## Active` parts of `/_memory/_state.md` and `_goals.md`.

### Step 2 — Read what changed

- **Short notes**: `short_list` returns `content` inline. These are the closest to what was actually on their mind; read them all.
- **Knowledge units**: `extract_tag` and `extract_segment` with `head: 20` for what each is about, its `perspective`, and whether it's new (`created_at` in the window) or grew (only `last_compiled_at`).
- **Regular notes**: `extract_outline` or `extract_segment` with `head: 20`; a full `read` only for a note central to the stretch.

### Step 3 — See the stretch

- **What occupied them**: one to three threads, by what kept coming back, not by tag counts.
- **What moved**: a thread they pushed forward, a view that shifted, something they finished, a decision made.
- **What didn't show up**: an active goal that got no attention, a worry that kept recurring in short notes.
- **The long arc**: for the main thread or two, `search` their older notes (a few targeted searches, no date filter) to see where it comes from: the third time they've circled it, a view that has turned around since 2023, an old plan this finally picks up.

Then pick what's worth saying, using "What a friend notices" in `SKILL.md`. A thin stretch gets a few honest lines, not padding.

### Step 4 — Answer

A short letter, not a report: a `#` title naming the stretch's main thread, then two to four short paragraphs of prose. No section headings, no stats line, no emoji labels.

1. What this stretch was about for them, in a sentence or two, linking the notes it rests on.
2. The long arc: how it ties to what they wrote before, with their earlier words quoted and dated.
3. What you noticed that they might not have: something that moved (say so when it's growth), or a gap, plainly.
4. Optionally, one question or one thing to carry into the next stretch.

## Random look back

Bring back something from their past that speaks to where they are now, the way a friend says "remember when you said…". Old alone isn't enough.

### Step 1 — Know where they are now

Read the `## Active` parts of `/_memory/_state.md` and `_goals.md`, and their recent short notes (`short_list {"latest": 14}`).

### Step 2 — Draw candidates

About 20 notes and units not touched for 90 days or more, at random, from the vault root (`cd` there first when your working directory is a bot's folder):

```bash
find . -name '*.md' -not -path './.*' -not -path './_knowledge/_indexes/*' -not -path './_memory/*' -not -path './_bots/*' -not -path './_setup/*' -mtime +90 | sort -R | head -20
```

Without a shell: pick two random months from the past few years and `list {"start": "<YYYY-MM-01>", "end": "<next month>", "limit": 15}` for each; try another month when one comes back empty.

Also `search` the main thing on their mind now (from Step 1) with no date filter, and add hits older than 90 days to the candidates.

### Step 3 — Keep one or two

Skim each candidate (`extract_segment` with `head: 20`, or the short note itself) and drop empty notes, logs and todo lists. Keep the one or two that say the most to them now: an earlier self that already answered today's question, a view they've since outgrown, a plan never followed up that fits what they're doing, a sharp idea they let go of. If nothing ties to now, keep the single sharpest one and say plainly it's just worth rereading.

### Step 4 — Answer

A `#` title, then a short paragraph per find: what they wrote and when, in their words where possible, with the `[[link]]`; then why it's worth seeing now, tied concretely to what's on their mind. If they've changed since, say how. One question at most, at the end.
