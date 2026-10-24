# Step: intro

Write the user a short guide to Hylo live in the editor, made from their own notes, so they watch it being written and see what you can do rather than read a list.

**Condition:** always.

## Context

The chat that started setup may already give you their vault overview and the words you opened the chat with. Use them and start typing right away. Only if you have no overview, get one with `status` and `list {"limit": 5}`.

Hylo brings three libraries together. The notes library holds their ideas, thoughts, sparks and scraps, written down fast. The knowledge library holds the good articles, podcasts and papers they come across, compiled into a wiki. When they need inspiration or sit down to write, both become their material library. What drives all three is the AI bot: you and the bots they set up, working inside their notes and remembering them over time. The guide shows that through their own notes.

## Do

1. With `editor_type`, type the guide into a new note named "Meet Hylo" (in the language you're speaking, e.g. "认识 Hylo"), one section per call, at normal speed. If that name is taken, add " 2". The map comes first so they see everything at a glance, then each branch is opened up in the same order.
   - A `# ` title, then one line that ties the three libraries together, e.g. "What you jot down and what you read come back when you create." / "随手记下的、认真读过的，都会在你要创作的时候回来。"
   - The whole picture as one mindmap, right under that. Four branches, three or four leaves each; a leaf may name one of their interests (e.g. "AI news digest"):
     ````
     ```mindmap
     # Hylo
     ## Notes
     - Ideas & sparks
     - Shorts & web clips
     - AI tags, no filing
     - Search ⌘K
     ## Knowledge
     - Articles, podcasts, papers
     - Compiled into a wiki
     - Links back to sources
     - Knowledge graph
     ## Material
     - Finds what you noted & read
     - Writes with you in the note
     - Cites its sources
     ## AI bots
     - Made in chat
     - Schedules & events
     - Results in Inbox
     - Knows you
     ```
     ````
     Write the labels in the language you're speaking ("笔记库 / 知识库 / 素材库 / AI Bot" in Chinese). Load the editor skill's mindmap format first. Every node is a heading or a list item; keep labels to a few plain words, and [[links]] don't work inside a mindmap.
   - Then each branch in two or three short sentences:
     - **Notes:** jot down ideas and sparks the moment they come, as shorts or web clips; AI tags them, so nothing to file. If your opening words were about an Obsidian vault or another note app, say how to bring it in: point Hylo at the vault in Settings → Server → Config → Vault, or export to Markdown into this folder. Link one or two of their real notes if they have any.
     - **Knowledge:** good articles, podcasts and papers get compiled into a wiki of the people, ideas and companies in them, each page linked back to its sources and browsable as a knowledge graph.
     - **Material:** when they write or need inspiration, you pull what's relevant from their notes and knowledge, with sources, or write with them in the note, like this guide. Give one example request built from their notes.
     - **AI bots:** what drives the three. Describe a bot in this chat and you set it up; it runs on a schedule or on events, knows them through their memory, and leaves results in the Inbox. Give one example bot (a news digest, a plan).
   - One closing line: plain .md files on their disk, no Hylo account, and it runs the agent they already use.

   Keep the text under 250 words, or 500 characters in Chinese or Japanese, not counting the mindmap. No frontmatter.
2. Reply in one or two sentences with a [[link]] to the guide. Don't repeat the guide in the chat.

If the editor tools are missing or fail, don't write the note another way: give a brief version of the guide in the chat instead.

## Record

`- [x] intro — [[<guide note name>]]`, or `in chat` if you gave it there.
