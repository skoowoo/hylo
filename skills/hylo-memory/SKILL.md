---
name: hylo-memory
description: "Keeps the user's personal memory under /_memory/: extracts it from recent notes and conversations with all bots, or records one fact they just told you. Use for 'update my memory', 'refresh my memory from my notes or chats', or when they share something lasting about themselves and want it kept ('remember that I…', '记住我…')."
---

# Hylo Memory

Two jobs: **extract** memories from recent notes and bot conversations into eight files under `/_memory/` (Steps 1–5), or **record one fact** the user just told you (last section). Both use the file format and update rules in Step 4. Items not reinforced fade, then go.

Tools named here are Hylo's MCP tools (server `hylo`, maybe shown as `mcp__hylo__read`). If they aren't in your tool list, search your deferred tools for "hylo"; if still missing, say Hylo's tools are unavailable and stop.

**Extraction runs to the end without stopping or asking**, often woken by a trigger with nobody there to answer. Speak only in Step 5.

Inputs, both optional: a self-introduction in the request (name, role, projects), and extra vault paths to scan.

## Step 1 — Run mode and author profile

`read /_memory/_identity.md`. Missing: first run, `scan_window = 90` days. Present: `scan_window = 2`. Note today's date and calculate `chat_since` once as the local `YYYY-MM-DD HH:MM` at the start of the scan window. Use source dates for evidence; today is only the decay evaluation date.

Build an **author profile** from `_identity.md`, `_goals.md` and any self-introduction: name and aliases, projects or products they run (these may show up as vault folders or unit titles), roles, people in their personal life. Step 3 uses it to tell their own material from external.

## Step 2 — Collect

In parallel:

```
short_list {"latest": <scan_window>, "limit": 100}
knowledge_list {"kind": "knowledge", "latest": <scan_window>, "limit": 50}
bot_list {}
```

Plus `list {"path": "<path>", "latest": <scan_window>}` per extra path. For every bot, including the main bot and disabled bots, call `bot_chat {"bot_id": "<id>", "since": "<chat_since>", "limit": 50}`. This mode reads all retained conversations with a `chatId` on each message. User text is complete; bot replies are capped at 4000 characters and marked `truncated: true` when shortened. Do not infer missing options or conclusions from a truncated reply; skip claims that need the missing context. While `hasMore`, pass the first message's ID as `before`, keeping `since` fixed. Group messages by `chatId` and read each conversation in order, joining context across pages. Deleted conversations are simply unavailable; do not reconstruct them.

If all sources are empty, go to Step 4 (decay still runs).

## Step 3 — Extract

| Dimension | File | What qualifies |
| --- | --- | --- |
| **identity** | `_identity.md` | Stable facts: name, job, location, family, languages |
| **preferences** | `_preferences.md` | Likes and dislikes, tools of choice, tastes as their own |
| **goals** | `_goals.md` | Active projects, ambitions, what they're working toward |
| **beliefs** | `_beliefs.md` | Worldview, recurring opinions, values they endorse |
| **people** | `_people.md` | Named individuals and their relationship to the author |
| **state** | `_state.md` | Near-term mood, struggles, what's top of mind |
| **events** | `_events.md` | Important decisions, milestones and changes with lasting relevance; keep the event date and outcome, not a transcript |
| **voice** | `_voice.md` | How they write: habits of expression and writing rules they state ("no em dashes") |

Check authorship for every source, including short notes and extra paths. Skip uncertain attributions, tests, demo text, quotes and AI drafts; don't store them as “疑似”. Explicit user statements about the test environment or interaction preferences still qualify.

- **Short notes**: confirmed original writing can inform all eight dimensions. Learn voice only from repeated habits in these notes or explicit writing preferences, not from compiled prose. Record usable rules such as “短句，很少用连接词”.
- **Knowledge units**: read each and check `perspective` against `source_notes`. Original writing can inform all dimensions except voice. External material can support only narrow interests evidenced by explicit statements or repeated independent choices; several units from one article count once. It does not establish endorsement or experience. Mixed units require attributing each claim separately.
- **Bot conversations**: user statements are personal evidence; assistant messages only resolve context (such as what “the second option” refers to). Do not turn suggestions, generated drafts, automated digests, trigger tasks or the bot's own memory summaries into facts about the user. Silence is not agreement. Skip quoted, hypothetical and role-play claims. Keep explicit corrections, decisions, project progress and standing interaction preferences. Infer voice only from explicit preferences, not brief chat phrasing. If context is missing, skip the ambiguous claim. Preserve the user message ID and `chatId` as the source of each extracted fact.
- **Extra paths**: confirmed original writing can inform every dimension except voice.

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

Decay windows: `_identity` 365d · `_voice` 180d · `_beliefs`, `_preferences`, `_people` 90d · `_goals` 30d · `_state` 7d · `_events` 180d. Events keep their actual event date in the text; `last` is the latest supporting evidence date.

Use source statement dates, not extraction or compilation dates. Count reinforcement by distinct evidence day: rescanning the same material or seeing it copied into a note does not add another observation. For conversation-derived items, append `source:chat:<chatId>:<messageId>` inside the metadata before any trailing `pinned`, keeping the latest supporting user message. Existing items without a source remain valid.

Apply corrections before reinforcement: a newer explicit user correction replaces the old claim (including a pinned one). Close completed or canceled goals instead of leaving them active; retain an important outcome in `_events.md`. Older evidence must not reverse a newer correction. A request to forget removes the fact without creating an event that repeats it.

Process evidence chronologically within each fact, using message order for same-day corrections. Apply the age checks D and E after merging evidence even when old evidence was found again.

Rules, in order:

- **A** — evidence matches an Active item: `last` = the newer evidence date, `seen` + 1 only if that date is later than the existing `last`.
- **B** — evidence matches a Fading item: move it to Active only with evidence newer than `last`, updating `last` and adding 1 to `seen`.
- **C** — new item: add to Active with `last:<evidence date> seen:1`; use its age to place it in Active or Fading, and skip already-expired evidence.
- **D** — Active item: if `today − last > decay_window`, move it to Fading, `last` and `seen` unchanged.
- **E** — Fading item: if `today − last > 2 × decay_window`, delete it.
- **F** — same fact in other words: merge, keeping the clearer wording, the larger `seen` and the later `last` so duplicated evidence is not counted twice.
- **G** — an item whose metadata ends in `pinned` is an explicit standing fact or preference: D and E never apply, and it keeps `pinned`.

## Step 5 — Summary

Briefly describe meaningful changes, naming corrected or removed facts. Give processing counts only when asked. An automatic run with nothing worth reporting returns exactly `[silent]`.

## Record one fact

When the user tells you something lasting about themselves or asks you to remember it:

1. Pick its file from the dimension table (a rule about how to write goes to `_voice.md`), read it, and apply the rules to that one item, in their words where you can.
2. Add `pinned` only for a standing preference or fact ("always answer in English", "I'm vegetarian"), not for current work or moods, which should fade.
3. Write the file and say in one line what you kept.
