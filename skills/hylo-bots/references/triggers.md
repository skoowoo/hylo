# Triggers

A trigger tells a bot when to wake and what to do. The bot does it as a turn in its own chat, and the user reads the reply as a message the bot sent on its own. `trigger_add` takes a `prompt` plus either a `schedule` or `events`, never both.

## Scheduled

`schedule`, server local time:

- `every 1h`, `every 30m`: on an interval
- `daily 08:00`
- `weekly mon,wed 09:00`

Offer the common choices as `choices` (every morning 08:00, every evening 21:00, weekly on Monday morning, other) rather than asking for the syntax. Convert what they say ("每周五晚上八点") to the syntax yourself.

Several scheduled triggers on one bot: stagger them five minutes apart (`daily 08:00`, `daily 08:05`, …) so runs don't pile up.

## On events

`events` is a list of event types; the bot runs each time one happens. WeChat, Discord and Telegram are not events: each is connected on the bot and writes into its chat.

| Event                | Fires when                                  | Prompt placeholders          |
| -------------------- | ------------------------------------------- | ---------------------------- |
| `note_created`       | a note is created                           | `{Path}` `{Name}`            |
| `note_updated`       | a note is modified                          | `{Path}` `{Name}`            |
| `note_deleted`       | a note is deleted                           | `{Path}` `{Name}`            |
| `short_note_created` | a short note is appended                    | `{Path}` `{Name}` `{Content}`|
| `compile_requested`  | the user asks to compile a note             | `{Path}` `{Name}`            |

Rules:

- `path_prefixes` filters note events to folders and each must start with `/` (e.g. `/Web Clips/`). Ask which folder when the event is a note event, and recommend a folder over the whole vault; a bot on every note change runs a lot, and each run is a turn in its chat.
- A trigger's events and folders can't be edited afterwards; change them by deleting and re-adding.
- Scheduled events can't be mixed with the others in one trigger.

## The trigger prompt

What the bot is asked each time. Say the task, name the skill to use if there is one ("Use the hackernews skill to get today's top stories."), and use placeholders for what the event carries ("Summarize the note at {Path}."). Never put curl commands, URLs or API parameters in it; fetching belongs to a skill (`references/skills.md`). Keep it to a sentence or two; standing rules belong in the system prompt. Every woken turn is already told it's reaching out on its own, to start with the content with no closing offer, and that it may reply `[silent]` to send nothing; say when silence is right only if the job needs it ("nothing when my notes have nothing to add"). On the main bot, whose system prompt is for chatting, the trigger prompt also says to save nothing unless that is the job.

`name` is a short label shown in the bot's trigger list; use the skill or task name.
