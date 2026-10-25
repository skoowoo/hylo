# Find

Find what the user is looking for in their notes: a resource, a topic, a view, every note that touches something. Old matters as much as new; what they wrote years ago and forgot is often the best find.

**Search to completion, then answer once.**

## Step 1 — Know what you're looking for

| They ask for | You're after |
| --- | --- |
| A resource ("that article on X", "the book Y recommended") | one or a few notes; stop when found |
| A topic ("everything on X") | every unit and note on it, across all time |
| A view ("what do I think about X", "what did I say about Y") | their own statements on it, with dates |
| What relates to a piece of content (a note, draft, link or pasted text) | units that back it up, and units that conflict with it; for your take on it rather than a list, use Reflect |

For a piece of content, first pull 3–8 search phrases from it: its entities and, above all, the **claims** it makes. Conflict depends on stance, not keyword overlap. A link: fetch it with your own web tool, or ask them to paste it.

## Step 2 — Search wide, not just recent

Search in the order your instructions give, and don't stop at the first hits:

- Run `search` and `knowledge_search` with several phrasings: synonyms, the other language the vault uses, the names of people or projects involved. Use `field: "tag"` when a tag fits.
- Don't filter by date unless they asked for a period. Rank by fit, not by how recent it is.
- Follow `source_notes` and links one hop out; old notes often sit behind a unit that was compiled from them.
- Read only enough of each hit to know it fits: `extract_outline`, then `extract_section`, or the `search` snippet.

Note each unit's `perspective`, and each note's date (frontmatter `date`, a date in its name, or when it was created).

## Step 3 — Answer

**A resource**: the note, one line on what it is, its date. Stop.

**A topic**: group the finds (knowledge units first, then notes, then short notes), one line each with `[[link]]` and date. Flag the ones they likely forgot, such as untouched for a year or more ("2023-04, not opened since"). Many hits: the 10–15 that matter most, and say how many more there are.

**A view**: their own view first, in 1–3 sentences, from their own writing only and dated ("as of 2025-03, you thought…"). Then the grounds, one per line with link and date. What they read goes on its own lines: "you read in [[…]] that…". If their views disagree over time, give both with dates and don't pick one. A judgment call ("should I…"): lay out what their notes say for and against; don't decide for them.

**A piece of content**: what in their notes backs it up and what pushes against it, one line each with the link. Say whose view a conflict is with: theirs, or a source they read. At most two surprising links from elsewhere, only when genuinely worth a look.

```markdown
# <one-line gist of the content>

## 🔗 Backs it up / builds on it
- **[[Unit Title]]** — <what it says, how it connects>

## ⚡ Pushes against it
- **[[Unit Title]]** — <what conflicts, concretely> — <yours, or a source you read>

## 💭 A leap
- **[[Unit Title]]** — <the unexpected link>
```

Nothing found: say so in one line, and what you searched for. If the request tells you how to reply when there is nothing, do exactly that.

Close with a `sources` block when you leaned on two or more notes.
