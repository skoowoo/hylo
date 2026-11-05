# Review

Looks back. Two kinds:

| Kind | The user asks | Section below |
| --- | --- | --- |
| **Periodic** | "weekly review", "回顾一下这周/这个月": everything from a past stretch | Periodic review |
| **Random look back** | "随便翻翻", "random look back", "anything I've forgotten": history at random | Random look back |

**Run that kind's steps to completion without stopping. Only speak at the end.** Say it in the conversation; never write a file.

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

A walk through the whole vault: any note, any knowledge domain, any short entry, from any part of their history. `sample` has already chosen. Don't search for a fit with this week, and don't drop a find because it doesn't.

### Step 1 — Draw

```
sample {}
```

That draws two of each kind, written at least 90 days ago when anything is that old, spread across the span and across domains. A knowledge unit's `created_at` is its earliest source note, not the day it was compiled. If `items` is empty, say there is nothing written yet and stop.

### Step 2 — Answer

A `#` title, then a short paragraph per item, in the order `sample` returned: what it says and when, quoting `excerpt`, with the `[[link]]` (the filename stem); a short entry links its daily note. Whose words they are decides how you say it. A short entry is theirs. A knowledge unit goes by its `perspective` and names its `domain`. A note is theirs unless the excerpt reads as saved from elsewhere (an article, a transcript), which is "you saved…". Read a note or unit only when its excerpt is empty or a unit has no `perspective`. One question at most, at the end, and only when one of the finds actually raises it.
