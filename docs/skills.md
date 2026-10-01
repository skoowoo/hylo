# Skills Guide

Hylo ships with a set of built-in skills. Agents pick the right skill automatically based on what you ask for — you don't need to name a skill explicitly, just describe what you want in plain language (English or Chinese both work, since every skill triggers on both).

This guide covers what each built-in skill does and how to talk to it. For how skills are installed, managed, and customized, see the [Skills](../README.md#skills) section in the main README.

## Table of Contents

- [Skills Guide](#skills-guide)
  - [Table of Contents](#table-of-contents)
  - [hylo-notes — everyday note operations](#hylo-notes--everyday-note-operations)
  - [hylo-compile-note — compile a note into knowledge](#hylo-compile-note--compile-a-note-into-knowledge)
  - [hylo-index-knowledge — build the knowledge index](#hylo-index-knowledge--build-the-knowledge-index)
  - [hylo-memory — extract personal memory](#hylo-memory--extract-personal-memory)
  - [hylo-quote-extract — pull out key quotes](#hylo-quote-extract--pull-out-key-quotes)
  - [hylo-recap — associative and periodic review](#hylo-recap--associative-and-periodic-review)
  - [hylo-writing — beat-by-beat article writing](#hylo-writing--beat-by-beat-article-writing)

## hylo-notes — everyday note operations

The general-purpose router for day-to-day note work: creating, reading, editing, deleting, and searching regular notes; capturing short notes; and querying the knowledge base. Most conversational note requests land here first, and it hands off to a more specific skill (like `hylo-compile-note` or `hylo-recap`) when your request needs one.

**Use it for:** saving a new note, finding an existing note, quick-jotting a thought, or asking what you already know about a topic.

**Example prompts**

```
Save this as a note under /reading: "..."
Find my notes about pricing strategy
Jot down a quick note: remember to follow up with Alex about the partnership
What do I know about network effects?
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

See the [Personal Memory](../README.md#personal-memory) section in the main README for the full setup (including a daily scheduled Agent Bot trigger).

**Use it for:** giving every future conversation persistent context about who you are, without repeating your background each time.

**Example prompts**

```
Please update my personal memory. I'm Skoo, currently building an AI-native notes app.
Run memory extraction — also scan /journal/ this time.
Refresh my personal memory base.
```

## hylo-quote-extract — pull out key quotes

Pulls high-value verbatim passages out of a note — like pull quotes in a magazine, not just a punchline sentence but enough surrounding context that the insight stands on its own — and saves each one as a short note with a backlink to the source.

**Use it for:** capturing the 3-8 best passages from an article or clip as standalone, re-discoverable short notes instead of re-reading the whole source later.

**Example prompts**

```
Extract the best quotes from /clips/deep-work.md
Highlight the important passages in this note
Pull out the key quotes from today's article and save them
```

## hylo-recap — associative and periodic review

Cross-checks content against your knowledge base and surfaces what's related or in tension with it. Two modes:

- **Associative recap** — check a single note, draft, or pasted passage right now, while the thought is fresh.
- **Periodic review** — recap everything created or updated over a recent window (e.g. the past week or month).

Both modes produce a conversational summary only; nothing is saved to a file.

**Use it for:** catching contradictions before they calcify, or getting a digest of what you've been thinking about lately.

**Example prompts**

```
Recap this draft against my knowledge base — anything related or conflicting?
Does this note conflict with anything I've written before?
Give me a review of everything I wrote this week.
```

## hylo-writing — beat-by-beat article writing

Leads a choose-your-own-adventure writing process: you state the topic, the intended reader, and the stance; the skill scans the knowledge base for relevant units, proposes a palette of sources for your approval, then writes the article one beat (one paragraph-sized move) at a time, offering a few candidate next beats after each one. When the article is done, it can be compiled back into the knowledge base, closing the loop.

**Use it for:** writing something grounded in your own accumulated understanding, rather than starting from a blank page or copy-pasting raw notes together.

**Example prompts**

```
I want to write an article about why SEO is dying, for indie hackers, arguing that content moats matter more than ever. Save it to /essays/seo-is-dying.md.
Help me write a piece on remote work culture, aimed at first-time managers, taking the position that async-first is usually the wrong default.
```
