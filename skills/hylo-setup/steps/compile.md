# Step: compile

Turn a few of their notes into knowledge, so they see their knowledge base and graph start to grow.

**Condition:** skip it if they have fewer than 3 notes.

## Do

1. Pick three to five notes worth compiling: substantial notes they wrote or clipped, recent or central to what they care about. Leave out short notes (`/_shorts`), anything under a folder starting with `_`, and near-empty notes. `list {"limit": 30}` and their titles are enough to choose; don't read every note.
2. Name the notes you picked and ask in one sentence whether to compile them now. Say it runs in the background. Offer `choices`: compile them, pick other notes, not now.
3. On their answer:
   - **No, or not now:** skip the step.
   - **Yes, or other notes they name:** call `compile {"path": "<path>"}` for each. Skip the ones that come back `already_compiled`.
   - Tell them in one or two sentences: the results land in Knowledge and the knowledge graph over the next few minutes, each with a note in their inbox; their memory picks it up on its daily update.

## Record

`- [x] compile — <n> notes`, or `- [-] compile — skipped: <reason>`.
