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
  - [hylo-writing — beat-by-beat article writing](#hylo-writing--beat-by-beat-article-writing)

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

Writes and revises notes inside the editor you have open, so you watch the text being typed: the note opens in front of you and an "Agent" caret types in a natural rhythm. It can add to the end of a note, under a given heading or at your cursor, and it can rewrite or delete a paragraph, a range of blocks, or exactly what you have selected. Each write is one undo step, and typing anything yourself stops the agent and hands control back. The skill also teaches the agent Hylo's markdown dialect — wikilinks, wiki images, mindmap blocks, tasks and tables — so what it types renders correctly. It needs the Hylo app to be open; if it isn't, the agent tells you instead of quietly writing the file.

**Use it for:** drafting while you watch, continuing from where your cursor is, or polishing a passage you've selected.

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

Extracts personal memory from your short notes and knowledge base into six structured files under `/_memory/` — identity, preferences, goals, beliefs, people, and current state. The first run scans the last 90 days; later runs scan incrementally. Memories that stop being reinforced fade out over time.

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

## hylo-writing — beat-by-beat article writing

Leads a choose-your-own-adventure writing process: you state the topic, the intended reader, and the stance; the skill scans the knowledge base for relevant units, proposes a palette of sources for your approval, then writes the article one beat (one paragraph-sized move) at a time, offering a few candidate next beats after each one. When the article is done, it can be compiled back into the knowledge base, closing the loop. If the Hylo editor is open, each beat is typed live into it (via `hylo-editor`), so you watch the article grow.

**Use it for:** writing something grounded in your own accumulated understanding, rather than starting from a blank page or copy-pasting raw notes together.

**Example prompts**

```
I want to write an article about why SEO is dying, for indie hackers, arguing that content moats matter more than ever. Save it to /essays/seo-is-dying.md.
Help me write a piece on remote work culture, aimed at first-time managers, taking the position that async-first is usually the wrong default.
```
