# Workflow: undo a change

The user clicked a receipt's undo button or asked to undo a recent automation change.

## 1. Find what to reverse

Take the `ids` line of the receipt whose `undo` text matches their message. If they asked in their own words, find the latest receipt that fits; if none fits, say you can't tell which change they mean and ask with `choices`.

## 2. Reverse it

No confirmation: the click was the go-ahead. Work through `ids` from last to first:

| Entry | Reverse with |
| ----- | ------------ |
| `trigger <bot>/<trigger>` | `trigger_delete` |
| `skill <name> on <bot>` | `skills_disable`; if marked `(added now)` and `skills_list` shows it on for no other bot, also `skills_remove` |
| `changed trigger <bot>/<trigger> from <field>=<old>` | `trigger_update` with the old value |
| `changed bot <bot> from <field>=<old>` | `bot_update` with the old value |
| `bot <bot>` | `bot_delete`, last |

Something already gone (a 404 or a missing id) counts as reversed.

## 3. Tell them

One or two sentences: what is back to how it was. If a step failed, say what is still there.
