# Bots

A bot is an AI agent you define in **Settings → Bots**: a name, a system prompt, a backing agent CLI, and optional triggers that make it run on its own.

## Backing agents

Hylo doesn't bundle a model. It finds these agent CLIs on your `PATH` at startup, with no configuration:

- Claude Code
- OpenCode
- Codex CLI
- Cursor Agent
- GitHub Copilot CLI
- Pi

## The Hylo bot

Onboarding in the desktop app creates the main **Hylo** bot. If you skipped it, or run Hylo on a server, open **Settings → Bots**; with no bots yet, it walks you through the same setup. It comes with these triggers:

| Trigger                         | What it does                                       |
| ------------------------------- | -------------------------------------------------- |
| `compile_requested`             | Compiles a note when you ask for it in the app     |
| `note_created` on `/Web Clips/` | Compiles new web clips automatically               |
| `scheduled` daily 09:00         | Updates your [personal memory](./context.md#personal-memory) |

The setup chat after onboarding also offers a **Recall** bot that brings your notes back to you: a review of your past week every Monday, a random walk through your history every afternoon, or both.

Edit or remove any of these, or click **New Bot** to build your own.

## Triggers

| Event                | When it fires                                        |
| -------------------- | ---------------------------------------------------- |
| `note_created`       | A new note is created in the vault                   |
| `note_updated`       | A note is modified                                   |
| `note_deleted`       | A note is deleted                                    |
| `short_note_created` | A short note is added                                |
| `scheduled`          | On an interval, at a daily time, or at a weekly time |
| `compile_requested`  | You compile a note from the app                      |

Note triggers can be narrowed with **Path Prefixes** (e.g. `/journal/`).

## WeChat, Discord, Telegram

Each bot can connect its own contact. A message from that contact is a turn in the bot's current chat, the same conversation as on the desktop. `/new` starts a new one, and the desktop follows it.

Open the bot under **Settings → Bots → Channels**:

- **WeChat:** click Connect and scan. One WeChat account stays on one bot; scanning again disconnects the previous bot.
- **Discord:** create a bot in the [Developer Portal](https://discord.com/developers/applications), enable **Message Content Intent**, and paste the bot token plus your user ID (Developer Mode → right-click your avatar → Copy User ID).
- **Telegram:** message [@BotFather](https://t.me/BotFather), send `/newbot`, paste the token, then open the link on your phone and tap Start.

Only the account that completed the connection is answered. No restart needed.

If a channel can't reach the network, set a proxy under **Settings → Server → Config → Proxy** (e.g. `http://127.0.0.1:7890`) and restart.

## Customizing prompts

The global system prompt is built into Hylo. Each bot adds two of its own:

- **System Prompt:** appended to the global one, separated by `---`.
- **Trigger Prompt:** the message sent to the agent when a trigger fires. Variables: `{Path}`, `{Name}`, `{Content}` for note events; `{Now}`, `{Date}`, `{Time}` for scheduled ones.

Built-in skills such as `hylo-compile-note` and `hylo-memory` are rewritten on every start, so edits to them don't last. To change how a bot works, use these prompts, or write your own skill (see [Skills](./skills.md#managing-skills)).
