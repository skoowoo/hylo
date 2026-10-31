# Plan and receipt cards

Hylo's chat draws these fenced blocks as cards. In a message from WeChat, Discord or Telegram, say the same in plain text instead. Write every visible line in the user's language, as results they'd notice ("every day at 08:00, 5 AI news items from the main bot"), not as tool calls.

## Plan: `proposal`

Shown before the first call that changes anything. Its buttons answer like `choices`: a click sends that line back word for word.

```proposal
title: Daily AI briefing
summary: Every day at 08:00, the main bot sends you 5 AI news items that matter to your RAG project
details: Main bot · daily-ai-news skill (to add) · daily 08:00
- Go ahead
- Run it once first
- Change something
- Not now
```

- `title`: a few words. `summary`: one sentence on what will happen, when, and where results go.
- `details`: the mechanics, folded until clicked: which bot, skills (marked "to add" when they aren't), schedule or events.
- Buttons, in this order: go ahead; run it once first (only when a trigger is part of the plan); give it a bot of its own (only when the plan puts a new automation on an existing bot); change something; not now.
- "Change something" gets two or three concrete alternatives as `choices` (time, how many, which sources), not a blank question.
- For a delete, the buttons are delete, turn it off instead, cancel, and `summary` says what goes and that there is no undo.

## Done: `receipt`

Ends the reply after the calls. It replaces a written report.

```receipt
title: Daily AI briefing is on
- Main bot sends it every day at 08:00, in its chat
- Added the daily-ai-news skill
- Ran it once: the first briefing is on its way in the main bot's chat
undo: Undo the daily AI briefing
ids: trigger 9f2c/a81d; skill daily-ai-news on 9f2c (added now)
```

- `title`: what is now true. Items: what changed and where results arrive, one line each.
- `undo`: the button text, naming what it undoes. A click sends it back word for word and starts `workflows/undo.md`.
- `ids`: hidden from the user; it is what the undo reverses. List everything this change made, with ids from the tool results, separated by `;`:
  - `bot <bot_id>`
  - `trigger <bot_id>/<trigger_id>`
  - `skill <name> on <bot_id>`, plus `(added now)` when this change also ran `skills_add` for it
  - `changed trigger <bot_id>/<trigger_id> from <field>=<old value>` or `changed bot <bot_id> from <field>=<old value>`, one per field changed
- Leave out `undo` and `ids` after a delete, which can't be undone.
- If a call failed, the title says it's partly done and the items say which step didn't happen; `ids` lists only what was made.
