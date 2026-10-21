# Write

Write an article with the user, not for them. Its point of view is in their head, not in the notes, so draw it out first, then write beat by beat with them steering. Read `references/beats.md` first: it says what a beat is and how to write one.

## Phase 0 — Intent

Settle these before looking for material, in this order:

1. **The point**: what the article claims, in one sentence. Only the user can settle it; never write on a guessed one.
2. **The reader**: who it's for, and what they should think or do once they've read it.
3. **Why now**: what set it off: an event, an argument, a view that changed.
4. **The other side**: what someone who disagrees would say, and whether the article answers them.

The point and the reader are required. Ask why now and the other side only when the answer would change the article. Take form and length from the request or `references/forms.md`; don't ask.

How to ask:

- One question per turn, never a questionnaire. Don't re-ask what the request already says.
- Lead with your guesses. Before the first question, read `/_memory/_beliefs.md` and `/_memory/_goals.md` and `search` their notes on the topic. Offer 2–3 candidate answers as `choices`, best guess first, each grounded in what they've written (name the note in the text above). They pick one, tweak it, or type their own.
- A topic isn't a point. "AI and writing" is a topic; "AI makes first drafts worthless, so the craft moves to deciding what to say" is a point. When the answer is still a topic, offer sharper versions to pick from.

**No topic yet** ("I want to write something but don't know what"): look at knowledge units added or updated in the last 30 days (`knowledge_list` with `latest: 30`) and `/_memory/_goals.md`, and offer 3 topics as `choices`, one line each on why it's ripe now. Then ask for the point.

End the phase with a brief for them to confirm:

---
**Point:** async-first is the wrong default for teams under ten
**Reader:** first-time managers → stop copying big-company remote playbooks
**Why now:** the planning doc nobody read last week
**The other side:** "meetings kill focus": agree, but the fix is fewer, not none

`choices`: that's it; change something.
---

The brief lives only in this conversation. To pick up an article in a new conversation, read it, say in one line the point you read in it, and confirm before the next beat.

## Phase 1 — Material and direction

Look for material on the point, not just the topic: what supports it and what cuts against it.

1. `knowledge_list_indexes`, pick the 1–3 most relevant domain indexes, `knowledge_read` them for matching units. With no indexes, scan `_knowledge/` directly.
2. If the units are thin, add uncompiled notes (`search`) and short notes (`short_list`).

Then show the **direction**: 3–6 beats, one line each, the moves the argument makes from opening to landing, with each beat's sources. Mark a beat that needs what only the user has (their own example, a judgment the notes don't make):

---
1. Opening: the planning doc nobody read ([[2026-10-12]])
2. Why small teams copied async: [[remote playbooks]]
3. What it costs them: decisions that drift for days ([[team retro 09]])
4. The other side: meetings do kill focus ([[deep work]])
5. Landing: one daily sync, everything else async *(no material: your own team's version)*

`choices`: go with this direction; change it.
---

Pick the save path yourself (a top-level folder) and say it in one line.

The direction is a heading, not a contract. When the writing finds something better, follow it, and show the updated direction if it changes much.

## Phase 2 — Beat by beat

Loop until the point has landed:

1. Write **only the chosen beat** to the article (in the editor: `editor_type` at `end`, one call per beat, default speed). Follow the voice in every beat.
2. Re-read the article (in the editor: `editor_read`).
3. Offer 2–3 candidate **next beats** as `choices`. Each is a move (an example, answering the other side, pushing the point further, a turn, the landing) with one line of what it would say; the beat that follows the direction comes first. Name each one's sources in the text above. Once the point has landed, add "end here".
4. Repeat with their pick.

When a beat needs what only the user has, ask before writing it: one question, then write the beat from their answer.

## Phase 3 — Close

Write the final beat. Re-read the whole article against the brief: does it land the point for that reader? If it drifts, say where in one line and offer a beat to fix it.

Then ask with `choices` whether to compile the article back into the knowledge base; if yes, run `hylo-compile-note` on it. End your reply with the article's wiki link (its filename without `.md`), e.g. `[[on-focus]]`, as the last line.
