# Update — Existing Knowledge Unit

Produce the best current distillation of the existing unit plus the new note. Treat it as editorial work, not appending:

- **Integrate.** Weave new content in where it belongs; no appended sections or "Updates" blocks.
- **Cite by paragraph.** After each paragraph with new content, add `[[note-stem]]`; list every source a paragraph draws on.
- **Reorganize** when the new content shifts what matters most.
- **Merge** overlaps, keeping the sharper version.
- **Cut** what is superseded, contradicted or outweighed. The unit isn't a historical record; the source notes are.
- **Rewrite for coherence**, so it reads as one document, not layers from different dates.
- **Split** a sub-topic that has its own core and keeps growing into a new unit, linked from this one; its `source_notes` are the notes its content came from.

Bookkeeping:

- Add this note to `## Sources` if it isn't listed.
- Frontmatter: add its path to `source_notes`, set `last_compiled_at` to now, increment `compile_count`. `perspective` stays when it equals the note's `source_perspective`, else becomes `mixed`; a unit without one gets it judged from its existing `source_notes` first.

Before writing, check the whole unit, not just the new parts: every paragraph on its core subject, sections ordered by importance with none tacked on at the end, nothing outdated, nothing restated. Fix what fails, then overwrite the file with `create` + `force`.
