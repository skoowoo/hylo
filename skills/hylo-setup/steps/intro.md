# Step: intro

Write the user a short guide to Hylo live in the editor, made from their own notes, so they watch it being written and see what you can do rather than read a list.

**Condition:** always.

## Context

The chat that started setup may already give you their vault overview and the words you opened the chat with. Use them and start typing right away. Only if you have no overview, get one with `hylo status` and `hylo list --limit 5`.

Hylo is a personal AI agent powered by their notes: they just write, you learn from their notes who they are, and you work for them around the clock, on their notes and well beyond them. The guide shows that promise through their own notes.

## Do

1. With `editor_type`, type the guide into a new note named "Getting started with Hylo" (in the language you're speaking), one section per call, at normal speed. If that name is taken, add " 2".
   - A `# ` title, and one sentence on what you, their personal agent, will do for them, tied to their notes.
   - **You write:** short notes for passing thoughts, clipped web pages, and [[wikilinks]] between notes. No sorting or tagging, and keep folders flat. Link one or two of their real notes by name if they have any.
   - **I get to know you:** you compile their notes into a knowledge base, a linked wiki they can browse as a knowledge graph, and distill it into personal memory: who they are, what they're working on, what they care about. They can compile a note from its menu and ask you to update their memory; every chat after starts out knowing them.
   - **I work for you:** because you know them, your work fits them, not just their notes. Agent bots (Settings → Bots) run on a schedule or when notes change: a morning news digest picked for their interests, a weekly review of what they wrote, a daily memory update. Give two or three example requests built from their own notes, at least one beyond the notes themselves (news, recommendations, plans); for an empty vault, make them fit their first week of notes.
   - **Always on:** ask here in chat, or from WeChat or Discord once connected. Run Hylo on a home or cloud server and you stay online even when this computer is off: bots keep running and you answer anytime. Setup: https://github.com/skoowoo/hylo/blob/main/docs/remote-deployment.md, then add it from the server switcher → Manage servers… → Add a remote server.
   - A small mindmap of how their notes and Hylo fit together, as a fenced block:
     ````
     ```mindmap
     # Hylo
     ## You write
     - quick thoughts
     ## I know you
     - memory
     ## I work for you
     - news digest
     ## Always on
     - WeChat
     ```
     ````
     Every node is a heading or a list item. Keep labels to a few plain words; [[links]] don't work inside a mindmap.

   Keep the guide under 350 words, or 700 characters in Chinese or Japanese. No frontmatter.
2. Reply in one or two sentences with a [[link]] to the guide. Don't repeat the guide in the chat.

If the editor tools are missing or fail, don't write the note another way: give a brief version of the guide in the chat instead.

## Record

`- [x] intro — [[<guide note name>]]`, or `in chat` if you gave it there.
