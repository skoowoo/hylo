---
name: hylo-recall
description: "Gets things back out of the user's notes and knowledge base: finds resources, topics, views and every note touching something (old and forgotten ones included), reflects on something they just wrote against their own history, digs up ideas and material, reviews a past week or month or a random slice of history, and maps their knowledge as a whole. Use for 'find my notes on X', 'what do I know/think about X', 'what relates to this', 'what do you make of this', 'give me material for X', 'inspire me', 'weekly review', 'show me something I've forgotten', 'what's in my knowledge base', '找一下关于 X 的笔记', '我对 X 怎么看', '你怎么看我这个想法', '找点素材/灵感', '周回顾', '随便翻翻', '我的知识库都有什么'."
---

# Hylo Recall

Gets things back out of the user's notes. Everything here is said in the conversation; nothing is saved unless they pick that afterwards.

Tools named here are Hylo's MCP tools (server `hylo`, maybe shown as `mcp__hylo__read`). If they aren't in your tool list, search your deferred tools for "hylo"; if still missing, say Hylo's tools are unavailable and stop.

## Pick the mode

| Mode | The user wants | Read |
| --- | --- | --- |
| **Find** | a resource, a topic, a view, every note touching something, or a list of what relates to a piece of content | `find.md` |
| **Reflect** | your take on something they just wrote (a short note, a thought, a decision), seen against what they've written before | `reflect.md` |
| **Inspire** | ideas and material for something they're thinking about or about to write | `inspire.md` |
| **Review** | a look back over the past week or month, or at random through their history | `review.md` |
| **Explore** | the whole picture: what their knowledge covers, where it comes from, what's growing or quiet | `explore.md` |

Read the matching file and follow it. If you can't tell which they mean, ask with `choices`.

## What to bring back

Find material that helps the request: an earlier answer, a changed view, an open plan, a useful counterargument, or a connection across topics. A factual list is right when they ask what was recorded; a shared topic alone does not warrant a reflection.

- Follow the instructed retrieval order. Link sources and use their original dates.
- Check authorship, including short notes and `own` units. Quotes, tests, demo text and AI drafts do not establish the user's views or life. Their corrections override earlier interpretations.
- Quote briefly when wording matters. Distinguish your suggestions and outside knowledge from theirs.
- Discuss ideas, not the user's character. Check that an apparent contradiction compares the same claim in the same context. An unrecorded outcome is unknown, not evidence of avoidance or failure.
- Follow interaction preferences; use `_voice.md` lightly. Keep questions to one that helps the request, or none.
- If the notes add nothing, follow any silence instruction; otherwise say so in one line. Don't invent a personal connection.
