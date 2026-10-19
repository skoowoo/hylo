# Workflow: delete a bot

## 1. Find the bot

Resolve the bot (see the shared rules) and `bot_get` it.

## 2. Confirm

Say what goes: the bot's name, its triggers (count and what they do) and its chats. Say what stays: the bot's folder and notes in the vault, and the skills in `~/.hylo/skills`. Say there is no undo.

Show it as a `proposal` card (`references/cards.md`) with the buttons delete it, turn it off instead (keeps everything, stops its triggers), cancel. Recommend turning it off if the user seems unsure or only wants it quiet.

## 3. Act

- Delete: `bot_delete {"bot_id"}`.
- Turn off: `bot_update {"bot_id", "enabled": false}`.

Then end with a receipt. Turning off can be undone (`changed bot <bot_id> from enabled=true`); a delete can't.
