# Workflow: create a bot

Turn what the user wants into a working bot: name, system prompt, skills, triggers.

## 1. Understand the job

Work out the job in one sentence from what the user said and the conversation so far. Ask in free text only if you can't tell what the bot should do at all; everything else you decide, and the plan shows it so they can change it.

Call `capabilities`. If a bot with a similar name or job exists, ask with `choices`: update that one, or create a new one.

## 2. Decide, don't ask

| Item | Default |
| ---- | ------- |
| Name | A short one from the job |
| Skills | Picked with `references/skills.md`; none when the job doesn't call for one |
| When it runs | What they said ("every morning" → `daily 08:00`); for a digest or report with no time named, every morning; for work on new notes, the event and the folder it plainly concerns (`references/triggers.md`); chat only when nothing in the job repeats |

When several skills each produce their own output on a timer, make one trigger per skill and stagger them. Only an open question with no sensible default (which folder, when the vault has several that fit) is asked, with `choices`, before the plan.

## 3. Trigger prompts

Write each with `references/triggers.md`.

## 4. System prompt

Read `references/system-prompt.md` and write the prompt. Don't ask the user to write it; show it in the plan and let them ask for changes.

## 5. Confirm, then build

Show the plan (see the shared rules). On go-ahead, in this order:

1. `bot_create {"name", "description": "<one line>", "system_prompt"}`. Keep the returned `id`.
2. Add and turn on each skill, as in `references/skills.md`.
3. `trigger_add` per trigger.
4. If they picked running it once first: `trigger_run` each scheduled trigger.

If step 1 fails, nothing was made. If a later step fails, the bot exists: say so in the receipt, and offer to finish with the update workflow.

## 6. Tell them

The receipt: the bot's name, what it will do and when, and that results arrive in the inbox.
