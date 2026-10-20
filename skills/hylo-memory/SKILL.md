---
name: hylo-memory
description: "Keeps the user's personal memory under /_memory/: extracts it from recent notes, or records one fact they just told you. Use for 'update my memory', 'refresh my memory from my notes', or when they share something lasting about themselves and want it kept ('remember that I…', '记住我…')."
---

# Hylo Memory

Two jobs: **extract** memories from recent notes into seven files under `/_memory/` (Steps 1–5), or **record one fact** the user just told you (last section). Both use the file format and update rules in Step 4. Items not reinforced fade, then go.

Tools named here are Hylo's MCP tools (server `hylo`, maybe shown as `mcp__hylo__read`). If they aren't in your tool list, search your deferred tools for "hylo"; if still missing, say Hylo's tools are unavailable and stop.

**Extraction runs to the end without stopping or asking**, often in the background where nobody can answer. Speak only in Step 5.

Inputs, both optional: a self-introduction in the request (name, role, projects), and extra vault paths to scan.

## Step 1 — Run mode and author profile

`read /_memory/_identity.md`. Missing: first run, `scan_window = 90` days. Present: `scan_window = 2`. Note today's date.

Build an **author profile** from `_identity.md`, `_goals.md` and any self-introduction: name and aliases, projects or products they run (these may show up as vault folders or unit titles), roles, people in their personal life. Step 3 uses it to tell their own material from external.

## Step 2 — Collect

In parallel:

```
short_list {"latest": <scan_window>, "limit": 100}
knowledge_list {"kind": "knowledge", "latest": <scan_window>, "limit": 50}
```

Plus `list {"path": "<path>", "latest": <scan_window>}` per extra path. If everything is empty, go to Step 4 (decay still runs).

## Step 3 — Extract

| Dimension | File | What qualifies |
| --- | --- | --- |
| **identity** | `_identity.md` | Stable facts: name, job, location, family, languages |
| **preferences** | `_preferences.md` | Likes and dislikes, tools of choice, tastes as their own |
| **goals** | `_goals.md` | Active projects, ambitions, what they're working toward |
| **beliefs** | `_beliefs.md` | Worldview, recurring opinions, values they endorse |
| **people** | `_people.md` | Named individuals and their relationship to the author |
| **state** | `_state.md` | Near-term mood, struggles, what's top of mind |
| **voice** | `_voice.md` | How they write: habits of expression and writing rules they state ("no em dashes") |

**Short notes** (`content` comes inline): the author's own words. All seven dimensions.

**Voice comes only from short notes**: they're typed as is, while units are rewritten by compiling and other notes may be clipped. Learn it from how the shorts are written, not just what they say: sentence length, punctuation, tone, word choice, mixing languages, how a thought opens and ends. Write each item as a rule a writer can follow ("短句，很少用连接词", "中英混写，术语保留英文"). A habit seen in one short note isn't voice yet.

**Knowledge units** (`knowledge_read` each): sort by `perspective`. Without one, judge from `source_notes`; a path under the author's own project (from the profile) is their own.

- `own`, or no `source_notes` at all: every dimension but voice.
- `external`, what they read or heard: no beliefs (the view is its source's), no people (guests and case-study figures aren't relationships). Only preferences, synthesized below.
- `mixed`: both. Its title and tags join the external pool; other dimensions only from what it attributes to the author's own sources.

**Preferences from external units**, after reading them all: pool their titles and tags, cluster by overlap, and write one item per cluster of two or more, as a trait, not a reading log. `[indie-dev, startup]`, `[ai, startup]`, `[ai, llm]` → `对 AI 驱动的独立技术创业有持续投入，偏工程实践侧`. Drop single-unit clusters.

**Extra paths**: the author's own notes. Every dimension but voice.

## Step 4 — Update the files

Read every existing memory file, apply the rules below with the new evidence, and write each file that changed: `create` for a new one, `create` + `force` to overwrite.

```markdown
---
decay_window: <Xd>
last_updated: <YYYY-MM-DD>
---

## Active
- <item text> `last:<YYYY-MM-DD> seen:<N>`

## Fading
- <item text> `last:<YYYY-MM-DD> seen:<N>`
```

Decay windows: `_identity` 365d · `_voice` 180d · `_beliefs`, `_preferences`, `_people` 90d · `_goals` 30d · `_state` 7d.

Rules, in order:

- **A** — evidence matches an Active item: `last` = today, `seen` + 1.
- **B** — evidence matches a Fading item: move it to Active, `last` = today, `seen` + 1.
- **C** — new item: add to Active with `last:<today> seen:1`.
- **D** — Active item without evidence: if `today − last > decay_window`, move it to Fading, `last` and `seen` unchanged.
- **E** — Fading item without evidence: if `today − last > 2 × decay_window`, delete it.
- **F** — same fact in other words: merge, keeping the clearer wording, the sum of `seen` and the later `last`.
- **G** — an item whose metadata ends in `pinned` was stated outright by the user: D and E never apply, and it keeps `pinned`.

## Step 5 — Summary

Run mode and window; items processed per source; files updated; counts added / refreshed / faded / deleted; deleted items by name, so the user can check.

## Record one fact

When the user tells you something lasting about themselves or asks you to remember it:

1. Pick its file from the dimension table (a rule about how to write goes to `_voice.md`), read it, and apply the rules to that one item, in their words where you can.
2. Add `pinned` only for a standing preference or fact ("always answer in English", "I'm vegetarian"), not for current work or moods, which should fade.
3. Write the file and say in one line what you kept.
