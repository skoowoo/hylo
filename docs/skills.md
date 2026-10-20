# Skills Guide

Hylo ships with a set of built-in skills. Agents pick the right skill automatically based on what you ask for — you don't need to name a skill explicitly, just describe what you want in plain language (English or Chinese both work, since every skill triggers on both).

This guide covers what each built-in skill does and how to talk to it. For how skills are installed, managed, and customized, see the [Skills](../README.md#skills) section in the main README.

## Table of Contents

- [Skills Guide](#skills-guide)
  - [Table of Contents](#table-of-contents)
  - [hylo-notes — save and change notes](#hylo-notes--save-and-change-notes)
  - [hylo-editor — live writing in the editor](#hylo-editor--live-writing-in-the-editor)
  - [hylo-compile-note — compile a note into knowledge](#hylo-compile-note--compile-a-note-into-knowledge)
  - [hylo-index-knowledge — build the knowledge index](#hylo-index-knowledge--build-the-knowledge-index)
  - [hylo-memory — extract personal memory](#hylo-memory--extract-personal-memory)
  - [hylo-recall — find, inspire, review, explore](#hylo-recall--find-inspire-review-explore)
  - [hylo-setup — guided setup in chat](#hylo-setup--guided-setup-in-chat)
  - [hylo-writing — write, revise, and long-form articles](#hylo-writing--write-revise-and-long-form-articles)

## hylo-notes — save and change notes

Saves and changes your notes: creating notes and short notes, editing, moving, renaming and deleting them. Looking things up is `hylo-recall`'s job.

**Use it for:** saving a new note, quick-jotting a thought, or tidying up notes you already have.

**Example prompts**

```
Save this as a note under /reading: "..."
Jot down a quick note: remember to follow up with Alex about the partnership
Rename my "Q3 plan" note to "Q3 launch plan"
```

## hylo-editor — live writing in the editor

Writes and revises notes inside the editor you have open, so you watch the text being typed: the note opens in front of you and an "Agent" caret types in a natural rhythm. It can add to the end of a note, under a given heading or at your cursor, and it can rewrite or delete a paragraph, a range of blocks, or exactly what you have selected. Each write is one undo step, and typing anything yourself stops the agent and hands control back. The skill also teaches the agent Hylo's markdown dialect — wikilinks, wiki images, mindmap blocks, tasks and tables — so what it types renders correctly. It needs the Hylo app to be open. If it isn't, writing done by `hylo-writing` goes straight to the file and the agent says so in one line; if you asked to watch, it asks you to open Hylo instead.

**Use it for:** watching writing happen in your note. What to write and how is `hylo-writing`'s job; this skill is how the agent types it into the editor.

**Example prompts**

```
Write an intro for this note in the editor.
Continue from my cursor with three more examples.
Rewrite the paragraph I selected to be more concise.
Turn the "Plan" section into a mindmap.
```

## hylo-compile-note — compile a note into knowledge

Turns a single raw note (a clip, a journal entry, a transcript) into one or more **knowledge units** — durable wiki-style pages for the people, concepts, companies, or ideas the note discusses. Existing units are updated rather than duplicated; only entities with real staying power become units, so throwaway details stay in the source note.

**Use it for:** turning a web clip or long note into structured, reusable knowledge instead of leaving it buried in a single file.

**Example prompts**

```
Compile the note /clips/network-effects.md into knowledge
Extract knowledge from this podcast transcript and update the knowledge base
Run compile on /journal/2026-07-01.md
```

## hylo-index-knowledge — build the knowledge index

Scans the knowledge base for units that aren't in any domain index yet and files them in (creating a new domain index if needed). Safe to run repeatedly — it only ever adds missing entries.

**Use it for:** keeping the knowledge base's domain indexes (`AI.md`, `Business.md`, etc.) up to date after compiling new notes.

**Example prompts**

```
Rebuild the knowledge index
Refresh the domain indexes — I've compiled a bunch of new notes
Update the knowledge index to include anything I'm missing
```

## hylo-memory — extract personal memory

Extracts personal memory from your short notes and knowledge base into seven structured files under `/_memory/` — identity, preferences, goals, beliefs, people, current state, and voice (how you write, learned from your short notes, plus the writing rules you've stated and the formats of your recurring pieces). The first run scans the last 90 days; later runs scan incrementally. Memories that stop being reinforced fade out over time.

See the [Personal Memory](../README.md#personal-memory) section in the main README for the full setup (including a daily scheduled bot trigger).

**Use it for:** giving every future conversation persistent context about who you are, without repeating your background each time.

**Example prompts**

```
Please update my personal memory. I'm Skoo, currently building an AI-native notes app.
Run memory extraction — also scan /journal/ this time.
Refresh my personal memory base.
```

## hylo-recall — find, inspire, review, explore

Gets things back out of your notes and knowledge base. Four modes:

- **Find** — a resource, a topic, a view, or every note that touches something, across all time, including what you wrote long ago and forgot. Ask what you think about something and your own views come first, dated; what you read is labelled as such. Give it a note or draft and it shows what backs it up and what pushes against it.
- **Inspire** — ideas and material for something you're thinking about or writing: your own past thinking, quotes and cases from what you read, links from other areas, and questions you left open.
- **Review** — the past week or month as a digest of themes and tensions, or a random look back that surfaces a few forgotten notes worth seeing again.
- **Explore** — the shape of your knowledge: its areas and their size (from the domain indexes), where it comes from, what has been growing or gone quiet, and how it lines up with your goals, as a mind map with next steps.

All answer in the conversation; nothing is saved unless you pick that afterwards. Your own views and what you read are always kept apart.

**Example prompts**

```
Find everything I've written about pricing.
What do I think about remote work?
Give me material for a post on onboarding.
Give me a review of this month.
Show me something I've forgotten.
What's in my knowledge base?
```

## hylo-setup — guided setup in chat

Runs on your first chat after onboarding. Hylo walks you through setup one step at a time: it writes you a getting-started guide live in the editor, checks which language you'd like it to use and remembers it, asks a little about you if your vault is still empty, offers a News bot that brings a morning digest from news sources you pick, compiles a few of your notes into knowledge, then sums up what's in place. Progress is kept in `/_setup/progress.md`, so you can stop anytime and pick up later.

**Use it for:** finishing setup you skipped, or redoing a step. Name Hylo setup when you ask, so everyday requests to set something up don't start it.

**Example prompts**

```
Continue Hylo setup
Run Hylo setup again
```

## hylo-writing — write, revise, and long-form articles

The one entry point for writing, always on for the main bot. It first decides whether there's existing text to change, then works in one of three modes:

- **Draft** — turns material you already have (a few notes, this week's short notes, the conversation you just had, or one note in a new format) into a new note. No questions: it picks the title and folder, writes the note (live in the editor whenever Hylo is open, to the file otherwise), lists the source notes at the end, and replies with a line or two plus next steps. The original is never touched.
- **Revise** — works on existing text: a quoted selection, your current selection, or a `[[note]]`. It first tells apart what you want: *feedback* (3–5 quoted points in chat, your text untouched), *candidates* (2–3 numbered versions, written in only when you pick one), or a *replacement* (polish, shorten, expand, translate, continue — written straight in, one undo step). It changes only the range you gave.
- **Long-form** — an article with a point of view from your knowledge base. It guesses the reader and stance from your request and memory, shows a palette of sources (knowledge units first, then notes and short notes) for you to confirm, then writes beat by beat, with an option to finish the rest in one go. With no topic, it proposes three from what you've been compiling lately. The finished article can be compiled back into the knowledge base.

All modes follow your voice in `/_memory/_voice.md`. Say a rule once ("never use em dashes") and it's kept there for every future piece. Recurring formats like weekly reports follow your last note of the same kind, falling back to a short built-in default.

**Use it for:** any writing — pulling notes together, writing up a discussion, weekly reports and minutes, adapting a note for another platform, polishing or getting feedback on a passage, or a long-form piece grounded in what you know.

**Example prompts**

```
Turn this week's short notes into one piece.
Write up what we just discussed as a note.
Draft my weekly report.
Make a Xiaohongshu version of [[Attention management]].
Polish this. / What's weak about this paragraph? / Give it a title.
Write an article on why SEO is dying, for indie hackers.
From now on, never use em dashes in my writing.
```
