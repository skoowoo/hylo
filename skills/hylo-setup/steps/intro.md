# Step: intro

Write the user a short guide to Hylo live in the editor, made from their own notes, so they watch it being written and see what you can do rather than read a list.

**Condition:** always.

## Context

The chat that started setup may already give you their vault overview and the words you opened the chat with. Use them and start typing right away. Only if you have no overview, get one with `status` and `list {"limit": 5}`.

Hylo is a personal AI agent powered by their notes: they just write, you learn from their notes who they are, and you work for them around the clock, on their notes and well beyond them. The guide shows that promise through their own notes.

## Do

1. With `editor_type`, type the guide into a new note named "Getting started with Hylo" (in the language you're speaking), one section per call, at normal speed. If that name is taken, add " 2". The map comes first so they see everything at a glance, then each branch is opened up in the same order.
   - A `# ` title, and one sentence on what you are: their personal agent powered by their notes. They just write; you learn who they are from it and work for them around the clock.
   - The whole picture as one mindmap, right under that sentence. Four branches, three or four leaves each; a leaf may name one of their interests (e.g. "AI news digest"):
     ````
     ```mindmap
     # Hylo
     ## You write
     - Editor & shorts
     - Web clips
     - Instant search
     - Obsidian-compatible
     ## I know you
     - Knowledge compiling
     - Knowledge graph
     - Personal memory
     ## I work for you
     - Answers from your notes
     - Bots on schedules & events
     - News digest to inbox
     ## Always on
     - Self-hosted, 24/7
     - WeChat & Discord
     - Desktop & CLI
     ```
     ````
     Write the labels in the language you're speaking. Load the editor skill's mindmap format first. Every node is a heading or a list item; keep labels to a few plain words, and [[links]] don't work inside a mindmap.
   - **You write:** a full note app with nothing to manage. A WYSIWYG editor that switches to raw Markdown, shorts for passing thoughts, a browser extension that clips pages as Markdown, and search as you type. Use [[wikilinks]], skip sorting and tagging, keep folders flat. Plain Markdown in their own folder, so Obsidian keeps working on the same vault. If your opening words were about an Obsidian vault or another note app they keep notes in, pick that up here: point Hylo at the vault in Settings → Server → Config → Vault, or export the app's notes to Markdown into this folder. Link one or two of their real notes by name if they have any.
   - **I get to know you:** you compile their notes into structured knowledge, a linked LLM wiki they can browse as a knowledge graph, then distill it into personal memory: who they are, their goals, what they care about. They can compile a note from its menu and ask you to update their memory; every chat after starts out knowing them.
   - **I work for you:** because you know them, your work fits them, not just their notes. Ask here and you answer from their notes. Bots (Settings → Bots) run on any of 6 agent CLIs, on a schedule or when notes change or messages arrive: a morning news digest picked for their interests lands in the inbox, a weekly review of what they wrote, a daily memory update. Give two or three example requests built from their own notes, at least one beyond the notes themselves (news, recommendations, plans); for an empty vault, make them fit their first week of notes.
   - **Always on:** reach you from the desktop app, the CLI, or WeChat and Discord once connected. Run Hylo on a home or cloud server and you stay online even when this computer is off, with their data on a machine they own: bots keep running and you answer anytime. Setup: https://github.com/skoowoo/hylo/blob/main/docs/remote-deployment.md, then add it from the server switcher → Manage servers… → Add a remote server.

   Keep the text under 350 words, or 700 characters in Chinese or Japanese, not counting the mindmap. No frontmatter.
2. Reply in one or two sentences with a [[link]] to the guide. Don't repeat the guide in the chat.

If the editor tools are missing or fail, don't write the note another way: give a brief version of the guide in the chat instead.

## Record

`- [x] intro — [[<guide note name>]]`, or `in chat` if you gave it there.
