# Update — Existing Knowledge Unit

Produce the best current distillation of the existing unit plus the new note. Re-select what deserves space rather than growing the page with every source:

- **No contribution, no write.** If the note repeats existing content and offers no meaningful editorial improvement, leave the unit, sources and metadata unchanged.
- **Integrate or replace.** Put a new mechanism or boundary where it belongs. A clearer explanation replaces the weaker one; a stronger example replaces a redundant example. No appended sections or "Updates" blocks.
- **Compress.** Merge overlapping explanations and repeated conclusions. Keep detail only when it changes understanding; link to source notes for full cases and arguments. A revision may be shorter even with more sources.
- **Preserve substance.** Keep distinctive user observations, essential causal steps, attribution and qualifications. Do not flatten the unit into a generic summary to save space.
- **Resolve carefully.** Replace a claim only when the sources establish a correction or supersession under comparable conditions. A newer publication or disagreement alone does not establish this. Retain meaningful disagreement with attribution; date time-sensitive claims rather than declaring old material false.
- **Cite by paragraph.** After each rewritten paragraph, add the `[[note-stem]]` links supporting its claims. Preserve existing citations on retained text. Read the relevant original sources when the unit does not support the proposed rewrite; omit unsupported additions.
- **Rewrite for coherence.** Order sections by importance and remove repetition across the whole unit. Length triggers editing, not automatic splitting. Any new unit must first pass Step 3's four criteria and be added to the manifest; an example or sub-point does not become a page just because the parent is long.

Bookkeeping, only when the content meaningfully changes:

- Keep `## Sources` and `source_notes` aligned with the sources supporting the retained content, without duplicates. Add the new note only if its contribution survives the edit; remove a source only when none of the retained content relies on it. Never modify or delete the original notes.
- Preserve `created_at`, set `last_compiled_at` to now and increment `compile_count`. Determine `perspective` from the retained content's sources: `own`, `external` or `mixed`. Read the relevant sources when attribution is unclear.

Before writing, compare the revision with the old unit: identify what became clearer, what changed or what was removed, and check that no distinct insight or necessary qualification was lost. If the revision offers no meaningful improvement, do not write. Otherwise overwrite with `create` + `force`.
