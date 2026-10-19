# Triggers

A trigger tells a bot when to run and what to do. Its output goes to the user's inbox. `trigger_add` takes a `prompt` plus either a `schedule` or `events`, never both.

## Scheduled

`schedule`, server local time:

- `every 1h`, `every 30m`: on an interval
- `daily 08:00`
- `weekly mon,wed 09:00`

Offer the common choices as `choices` (every morning 08:00, every evening 21:00, weekly on Monday morning, other) rather than asking for the syntax. Convert what they say ("每周五晚上八点") to the syntax yourself.

Several scheduled triggers on one bot: stagger them five minutes apart (`daily 08:00`, `daily 08:05`, …) so runs don't pile up.

## On events

`events` is a list of event types; the bot runs each time one happens.

| Event                | Fires when                                  | Prompt placeholders          |
| -------------------- | ------------------------------------------- | ---------------------------- |
| `note_created`       | a note is created                           | `{Path}` `{Name}`            |
| `note_updated`       | a note is modified                          | `{Path}` `{Name}`            |
| `note_deleted`       | a note is deleted                           | `{Path}` `{Name}`            |
| `short_note_created` | a short note is appended                    | `{Path}` `{Name}` `{Content}`|
| `wechat_message`     | a WeChat DM arrives                         | `{Content}` `{WechatUserID}` |
| `discord_message`    | a Discord DM arrives                        | `{Content}` `{DiscordChannelID}` `{DiscordUserID}` |
| `compile_requested`  | the user asks to compile a note             | `{Path}` `{Name}`            |
| `agent_run_completed`| another bot's run succeeds                  | `{Name}` `{Content}` `{Now}` |

Rules:

- `path_prefixes` filters note events to folders and each must start with `/` (e.g. `/Web Clips/`). For `agent_run_completed` it holds bot names instead, without a slash. Ask which folder when the event is a note event, and recommend a folder over the whole vault; a bot on every note change runs a lot.
- A trigger's events and folders can't be edited afterwards; change them by deleting and re-adding.
- Scheduled events can't be mixed with the others in one trigger.

## The trigger prompt

What the bot is asked each time. Say the task, name the skill to use if there is one ("Use the hackernews skill to get today's top stories."), and use placeholders for what the event carries ("Summarize the note at {Path}."). Never put curl commands, URLs or API parameters in it; fetching belongs to a skill (`references/skills.md`). Keep it to a sentence or two; standing rules belong in the system prompt. On the main bot, whose system prompt is for chatting, the trigger prompt also carries the output rules from `references/system-prompt.md` (start with the content, no closing offer, save nothing unless that is the job).

`name` is a short label shown in the bot's trigger list; use the skill or task name.
