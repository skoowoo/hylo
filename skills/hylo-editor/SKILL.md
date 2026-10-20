---
name: hylo-editor
description: "How to use Hylo's live editor tools (editor_type, editor_read, editor_rewrite), so the user watches the text being typed. Read it before any editor_* call, or when they ask for writing to happen in the editor ('write it in my note', '写到编辑器里'). What and how to write is the hylo-writing skill's job. Also covers Hylo's markdown dialect."
---

# Hylo Editor — live writing through MCP

The editor tools drive the Hylo editor the user has open: the note opens in front of them, a purple "Agent" caret types in human rhythm, and each call is **one undo step** (Cmd/Ctrl+Z). They need the Hylo app open.

## Environment check

You need `editor_type`, `editor_read` and `editor_rewrite` (maybe shown as `mcp__hylo__editor_type` or `hylo_editor_type`). If they aren't in your tool list, search your deferred tools for "hylo editor". If they still aren't there, live editing is unavailable: do what the skill that sent you here says (`hylo-writing` writes the file and says so in one line); otherwise ask the user whether to write the file directly.

## When to use it

Use the editor for writing the user is there to see: whatever `hylo-writing` writes or revises in a conversation, or when they ask to watch. Use the note tools for bulk or background work: many files, imports, a trigger running on its own, notes the user isn't looking at.

**Commit to the editor.** Once something for a request is written through the editor, every later write for it goes through the editor too. If an editor call fails, report it and let the user decide; never finish with `create`, `append` or another file write, which would race the editor still typing and duplicate text. The exception is "no editor is open in Hylo": it comes before anything is written.

## Tools

### editor_type: insert new text

| Param | Meaning |
| ----- | ------- |
| `path` | Vault-absolute path, e.g. `/journal/2026-10-10.md`. A missing note is created empty. |
| `content` | Markdown to type; trailing whitespace is trimmed. |
| `at` | `end` (default), or `cursor` for the user's caret. |
| `heading` | Types at the end of that section instead (case-insensitive, with or without `#`). Wins over `at`. |
| `speed` | `slow` (short showcase), `normal` (default), `fast` (long sections) or `instant` (large reference material, tables, or what they want right away). |

- Content lands as its own block, with blank lines added around it, so don't pad it with newlines. The exception is `at: "cursor"` mid-line, which inserts inline.
- A new path fails if a note with that file name exists anywhere in the vault, because wikilinks resolve by name. Pick a distinct name.

### editor_read: see what the user sees

Pass `path`. Returns the note as it is **in the editor**, unsaved edits included:

- `blocks`: `[{id, kind, text}]`, numbered `b1…bn` in document order, frontmatter excluded. `kind` is `heading`, `paragraph`, `list`, `code`, `quote`, `table`, `rule` or `htmlblock`; `text` is raw markdown.
- `selection`: `{text, blocks}` when the user has text selected.

Block ids are **positional**, valid only until the note changes. Read again before every rewrite: the user may have edited since.

### editor_rewrite: replace or delete a region

Highlights the region, deletes it, and types `content` in its place; empty `content` deletes it. Point at the region with exactly one of:

| Target | How |
| ------ | --- |
| `target: "selection"` | What the user has selected *at the moment of the call*. |
| `target: "b3"` or `"b3-b5"` | Whole blocks from `editor_read`. **Requires `expect`**: the region's first few words, copied from `editor_read`. |
| `from_text` (+ `to_text`) | Exact raw text where the region starts, occurring **once** in the note; with `to_text`, the region runs through its end. |

For block targets, `content` is trimmed, and deleting whole blocks also removes the blank line between them.

## Results and errors

| You get | Do |
| ------- | -- |
| `interrupted: true` | The user typed or cancelled and took over. Don't retype the rest or finish the job unasked; say where you stopped and ask how to proceed. After an interrupted rewrite, `editor_read` first: the region may be half-written. |
| "no editor is open in Hylo" | Nothing was written. If the user asked to watch, ask them to open Hylo, then retry. Coming from `hylo-writing`, write the file as its `references/surface.md` says. |
| "the editor disconnected…" | Part of the text may be in the note. Once the editor is back, `editor_read` and continue from what's there. |
| "the note changed since it was read" | `editor_read` again; recompute ids and `expect`. |
| "heading not found" | `editor_read` for the real headings; use the exact text, or drop `heading`. |
| "from_text occurs more than once" | Quote more surrounding text. |
| "nothing is selected in the editor" | Ask them to select it, or target it by block or `from_text`. |
| "the note would start with `---`…" | That would create frontmatter. Start with a heading or text. |
| "editor switched to another note" | An interruption: ask before resuming. |

## Workflows

- **Write a new note.** `editor_type` the title and first section, then one `editor_type` at `end` per further section.
- **Add to a section.** `editor_type` with `heading`. A new section: include its heading line in `content`, at `end` or under its parent via `heading`.
- **Continue where the user is.** `editor_type` with `at: "cursor"`, after an `editor_read` if you need the surrounding text.
- **Revise blocks.** `editor_read`, pick the blocks, `editor_rewrite` with `target: "bN"` or `"bN-bM"` and `expect` set to the first words of `bN`.
- **Rewrite the selection.** `editor_read` for `selection.text`, then `target: "selection"` right away. If other calls come in between, the selection may have moved: use `from_text`/`to_text` copied from `selection.text`.
- **Several edits in one note.** Work bottom to top so the ids above stay valid, or `editor_read` after each.

## Best practices

- **One call at a time.** Ops run in order; wait for each result.
- **Chunk long writing by section.** `normal` types about 40–70 characters a second, so 3,000 characters take a minute. Sections show progress, make interruptions cheap and avoid tool timeouts.
- **Typing steals focus.** Every op opens its note and scrolls to the caret. Stay on the note the user is in unless asked, and don't hop between notes.
- **Write complete constructs.** Close every code fence (the editor auto-closes one as you open it), give tables their separator row, and put a blank line before a `---` rule, or it turns the paragraph above into a heading.
- **Link with care.** Links to missing notes render broken; check a target with `resolve` or `search` first, unless it's a placeholder for a note to write later.

## Frontmatter

The editor tools never touch frontmatter: `editor_read` leaves it out, inserts land below it, rewrites refuse regions inside it, and content that would start a note with `---` is rejected. Never put frontmatter or tags in `content`; for metadata or tag changes, tell the user the editor tools don't touch them.

## Markdown dialect

Hylo renders GFM plus wiki syntax live. `references/formats.md` covers wikilinks, wiki images, mindmap blocks, tasks, tables, code languages, quotes and what to avoid. Load it when the content goes beyond paragraphs, headings and lists, and **always** before typing a mindmap.
