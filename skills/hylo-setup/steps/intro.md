# Step: intro

Write the user a short guide to Hylo live in the editor, made from their own notes, so they watch it being written and see what you can do rather than read a list.

**Condition:** always.

## Context

The chat that started setup may already give you their vault overview and the words you opened the chat with. Use them and start typing right away. Only if you have no overview, get one with `status` and `list {"limit": 5}`.

Hylo is a personal notes agent: they write, and you, plus the bots they set up, organize, search and write notes for them. A notes app leaves notes sitting until they search; an AI chat doesn't know their notes and starts from zero each time. You work inside their notes, remember them over time, and leave results as notes they can read and edit. The guide shows that through their own notes.

## Do

1. With `editor_type`, type the guide into a new note named "Getting started with Hylo" (in the language you're speaking), one section per call, at normal speed. If that name is taken, add " 2". The map comes first so they see everything at a glance, then each branch is opened up in the same order.
   - A `# ` title, and one or two sentences on what you are: their personal notes agent. They write; you work inside their notes, remember who they are, and leave your results as notes, unlike a chat that starts from zero.
   - The whole picture as one mindmap, right under that. Four branches, three or four leaves each; a leaf may name one of their interests (e.g. "AI news digest"):
     ````
     ```mindmap
     # Hylo
     ## Notes
     - WYSIWYG Markdown
     - Shorts & web clips
     - Obsidian vaults
     - Search ⌘K
     ## Bots
     - Made in chat
     - Schedules & events
     - Results in Inbox
     - Live editing
     ## Knows you
     - LLM wiki
     - Knowledge graph
     - Personal memory
     ## All yours
     - Plain .md files
     - Your own agent CLI
     - Self-hosted, 24/7
     - WeChat, Discord & Telegram
     ```
     ````
     Write the labels in the language you're speaking. Load the editor skill's mindmap format first. Every node is a heading or a list item; keep labels to a few plain words, and [[links]] don't work inside a mindmap.
   - **Notes:** every note is a plain .md file in a folder on their disk, so nothing is locked in, and an Obsidian vault opens as it is: same folders, same [[wikilinks]] and ![[embeds]]. A WYSIWYG editor that switches to raw Markdown, shorts for passing thoughts, a browser extension that clips pages as Markdown, ⌘K search as they type, and AI tags on new notes, so no sorting or filing. Mind map blocks and tasks work too; the map above is one. If your opening words were about an Obsidian vault or another note app they keep notes in, pick that up here: point Hylo at the vault in Settings → Server → Config → Vault, or export the app's notes to Markdown into this folder. Link one or two of their real notes by name if they have any.
   - **Bots:** a bot is their own agent working on their notes. They describe what they want in this chat and you set it up: a prompt, an agent (Pi, Claude Code, Codex and others), triggers (a schedule, a note added or changed, a message) and skills; they can change it later in Settings → Bots. Bots run on their own and leave results in the Inbox, with an unread count like email. Ask you to draft, continue or rewrite and you type into the note with your own cursor, the way this guide is being written; they start typing and you stop, and ⌘Z undoes each change. Give two or three example requests built from their own notes, at least one beyond the notes themselves (a news digest, recommendations, a plan); for an empty vault, make them fit their first week of notes.
   - **Knows you:** you compile their notes, web clips and transcripts into a wiki of the people, ideas and companies in them: existing pages get updated, not duplicated, every page links back to its sources, and they can browse it all as a knowledge graph. From that you keep seven memory files about them (identity, preferences, goals, beliefs, people, current state, how they like their writing) that every bot reads before it starts; what they stop mentioning fades out, and they can open and edit the files. They can compile a note from its menu.
   - **All yours:** no Hylo account and no Hylo cloud. Hylo has no AI model of its own; it runs the agent they already have signed in (Pi, Claude Code, Codex, Copilot CLI, Cursor Agent, OpenCode), so no API key to set up. Reach you from the desktop app, the CLI, or WeChat, Discord and Telegram once connected. Run Hylo on a home server or Linux VPS and you stay online 24/7 even with this computer off, served over HTTPS with an API key only they have, optionally backed up to their own Git repo. Setup: https://github.com/skoowoo/hylo/blob/main/docs/remote-deployment.md, then add it from the server switcher → Manage servers… → Add a remote server.

   Keep the text under 400 words, or 800 characters in Chinese or Japanese, not counting the mindmap. No frontmatter.
2. Reply in one or two sentences with a [[link]] to the guide. Don't repeat the guide in the chat.

If the editor tools are missing or fail, don't write the note another way: give a brief version of the guide in the chat instead.

## Record

`- [x] intro — [[<guide note name>]]`, or `in chat` if you gave it there.
