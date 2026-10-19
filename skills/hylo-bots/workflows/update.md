# Workflow: update a bot

Change an existing bot. Everything starts from its current state.

## 1. Find the bot and read it

Resolve the bot (see the shared rules), then `bot_get` for its settings and triggers, and `skills_list` for the skills it has (`dirs` includes its `cwd`). For a new automation with no bot named, use the main bot. On the main bot only the skill and trigger rows below apply.

If the user didn't say what to change, show the bot in a few lines (job, skills, triggers with their schedules) and ask with `choices` what to change: name or description, system prompt, skills, triggers, turn on/off.

## 2. Make the change

| Change                     | How                                                                                      |
| -------------------------- | ---------------------------------------------------------------------------------------- |
| Name, description          | `bot_update {"bot_id", "name", "description"}`                                           |
| System prompt              | Read `references/system-prompt.md`. Start from the current prompt and edit it; `bot_update {"bot_id", "system_prompt"}` replaces the whole text, so send it complete. Show the diff in plain words. |
| Turn on / off              | `bot_update {"bot_id", "enabled"}`; off stops its triggers from firing                    |
| Add a skill                | Read `references/skills.md`                                                              |
| Remove a skill from the bot | `skills_disable {"name", "bot_id"}`; its files stay                                      |
| Add a trigger              | Pick its skills with `references/skills.md` (adding them is the row above), then read `references/triggers.md` and `trigger_add` |
| Change a trigger's label, prompt, schedule, or pause it | `trigger_update`                                             |
| Change a trigger's events or folders | `trigger_update` can't; `trigger_delete` it and `trigger_add` the new one, and say so   |
| Remove a trigger           | `trigger_delete`                                                                         |

Several changes in one request are fine: one plan, one confirmation, then apply them in the order of the table.

## 3. Confirm, then apply

Show only what changes (before → after, in `details`) in the plan card, as in the shared rules. Keep the old values: the receipt needs them for undo. After applying, `bot_get` once to check, then end with the receipt. If they picked running it once first, `trigger_run` each new or changed scheduled trigger before the receipt.
