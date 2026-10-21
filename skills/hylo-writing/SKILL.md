---
name: hylo-writing
description: "Every writing request: rewriting notes, short notes or this conversation into a piece ('write up these notes', 'turn what we discussed into a note', 'draft my weekly report', 'make a WeChat version of [[X]]'); revising existing text — polish, shorten, expand, rephrase, continue, translate, title it, or just give feedback ('polish this', 'what's weak here?'); writing an article from the knowledge base; and writing rules ('never use em dashes'). '整理成一篇', '把刚才聊的写成笔记', '写个周报', '改成公众号版', '润色一下', '缩短', '换个说法', '接着写', '起个标题', '帮我看看哪里不好', '写一篇关于 X 的文章', '以后别用破折号'."
---

# Hylo Writing

One entry for all writing. Pick the mode here, then read its workflow.

Tools named here are Hylo's MCP tools (server `hylo`, maybe shown as `mcp__hylo__read`). If they aren't in your tool list, search your deferred tools for "hylo"; if still missing, say Hylo's tools are unavailable and stop.

## Pick the mode

First: **is there existing text to change?** If yes, it's revise. If no, ask: **where does the piece's point of view come from?** If it's already in the material (notes, a conversation, a note to adapt), it's draft. If it's still in the user's head, it's write, and only they can supply it.

| The user says | Mode | Because | Read |
| ------------- | ---- | ------- | ---- |
| "polish this", "shorten this paragraph", "say it another way", "continue", "what's weak here?" | **Revise** | there's text to change (quote, selection, current note) | `workflows/revise.md` |
| "write up these notes", "turn what we discussed into a note", "draft my weekly report", "make a WeChat version", "summarize my notes on X" | **Draft** | they point at the material or name a form, and want the finished piece | `workflows/draft.md` |
| "write an article on X", "help me write a piece arguing Y", "I want to write something but don't know what" | **Write** | a topic or stance to develop: settle their point and reader first, then write beat by beat with them | `workflows/write.md` |
| "never use em dashes", "all my articles use short sentences" | none | a writing rule | record it (see Voice below) |

When you can't tell draft from write, ask with `choices`: write it up from the material now, or write it together starting from their point.

During a write session, replies (a pick, "continue", "rewrite that beat") stay in write.

## The existing text

For revise, take the first of these that exists:

1. A quote in the message: the selection chip above the composer inserts `[[note]]` + `> the selected text`. This is the most reliable snapshot.
2. The user says "the selected part" and the message has no quote: `editor_read`'s `selection`.
3. A `[[note]]` in the message: the whole note.
4. The note open in the editor: only when they say "this one", "here".

## Rules for every mode

- **Read the voice before writing a word.** `read /_memory/_voice.md` (skip if missing). It holds how the user writes (learned from their short notes), the rules they've stated, and their recurring forms. Its rules beat your own taste.
- **Use only the material.** Don't invent facts, quotes or examples. Say what's missing instead of filling it.
- **Don't ask where to save.** Pick a fitting top-level folder yourself, never a nested one.
- **Forms** (weekly report, minutes, WeChat article, thread…): `references/forms.md`.
- **Where the writing lands**: `references/surface.md`, for every mode. Notes are written in the live editor whenever it's available, and as files only when it isn't.

## When the user states a writing rule

When the user states one ("from now on…", "my articles always…"), record it there with the `hylo-memory` skill's *Record one fact*, as `pinned`, and say in one line what you kept. Then carry on with any writing they asked for in the same message.
