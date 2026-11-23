---
name: hylo-curate-knowledge
description: "Curates existing knowledge units under /_knowledge/: trims repetition, merges duplicate units and deletes units without independent collection value, keeping original notes untouched. Use for 'curate my knowledge base', 'merge duplicate knowledge units', '整理知识库', '精简知识单元', or a weekly knowledge-curation trigger. Not for organizing raw notes, extracting new knowledge or recalling a past week."
---

# Hylo Curate Knowledge

Keep the knowledge base worth returning to. Edit existing units and their indexes; original notes are read-only. Before each write or deletion, verify the path is under `/_knowledge/` and its current kind is `knowledge` or `index`; only knowledge units may be deleted. A `source_notes` entry is provenance, never permission to modify that file. This skill does not archive, create replacement units, browse for newer facts or set up its own schedule.

Tools named here are Hylo's MCP tools (server `hylo`, maybe shown as `mcp__hylo__read`). If unavailable, search deferred tools for "hylo"; if still missing, report the limitation and stop.

A request to curate, or an explicitly configured curation trigger, authorizes the edits and deletions below. Run to completion without intermediate questions. A request for advice or a preview authorizes no writes. Use `knowledge_delete`, never the general `delete`, for approved removals.

## 1 — Select a batch

Optional inputs: a domain, unit paths or a preview-only request. Otherwise choose one domain, preferring the oldest `last_curated_at` on its index (missing comes first; tie-break by path). No indexes: run `hylo-index-knowledge`, then reload them. No knowledge units: stop without writing.

- `knowledge_list {"kind": "knowledge", "limit": 0}` gives existing units; `knowledge_list_indexes` gives indexes. Read the selected index and validate its unit paths against the listing.
- Sort the selected domain's unit paths. Start after the index's `curation_after` path, wrapping on the next run when none remain. Explicit unit inputs override the cursor.
- Fully read at most 20 candidate units per run. Find possible duplicates with `knowledge_search` on names and aliases, including other domains; a merge partner must also be fully read and counts toward 20. Do not decide from index summaries or search snippets.
- Read original sources as needed for evidence and attribution. Reference checks in Step 3 may inspect units outside the batch; these are not additional curation candidates.

Finish at most 3 merge groups and 5 standalone deletions in one run. These are limits, not targets; zero changes is valid. Do not expand the run into a whole-vault rewrite.

## 2 — Decide before writing

Build an internal plan: unit paths, action, evidence, distinctive content to preserve, surviving merge target if any, and affected references/indexes. Read every index, since a unit may appear in two domains. Include stale index rows whose paths are absent from the complete knowledge listing in the repair plan; remove those rows without recreating missing units.

| Action | When justified |
| --- | --- |
| Keep | A distinctive explanation, mechanism, case, counterexample, qualification or personal insight is worth finding again |
| Trim | Repeated explanations or peripheral detail obscure that value |
| Merge | Units describe the same entity or substantially the same core; one page can preserve their distinct contributions without conflating different concepts |
| Delete | The whole unit has no independent collection value and no distinctive content that should be kept or merged elsewhere |

- Definitions and generic overviews alone need not occupy a unit. A basic concept can still deserve one for its explanation or personal significance. Do not infer the user's interests or mastery.
- Age, length, low usage and few links are not deletion reasons. A historical case can retain value after its current facts expire.
- Call a claim superseded only when the sources establish it under comparable conditions. Preserve meaningful disagreements and dates; a newer source does not automatically win.
- Preserve the user's observations, annotations and changes of understanding. Explicit instructions to keep a unit protect it from deletion or absorption; if both merge candidates are protected, keep both.
- For trim and merge, follow the sibling compile skill's [editing and attribution rules](../hylo-compile-note/references/update.md). There is no new input note: use the existing units and their sources. Do not invoke the compile workflow or split long units.
- Choose a merge target among existing units, preferring accurate naming and stable existing references. Preserve useful old titles as aliases. All absorbed units point directly to this survivor in the plan; no chains or cycles.

When uncertain, keep the unit. If a source needed to verify loss or attribution is missing, defer that action rather than deleting its only remaining account. Recheck the plan as a whole: no unit scheduled for deletion may be a survivor or an unresolved source for another action.

For preview-only requests, present the proposed actions and reasons now, then stop without repairs, deletions or progress writes.

## 3 — Repair references, then delete

Deletion cleans database relations but does not repair Markdown links or index rows. Do not treat `rename` as a merge: it rewrites references vault-wide and can alter original notes.

Before any deletion:

1. Inspect incoming references across **all knowledge units**, not just this domain. Use the complete knowledge listing and `extract_link` on each unit; inspect `source_notes` frontmatter too. Search can discover candidates but its result count is not proof of a complete backlink scan. Fully read referrers needing edits. Match wikilinks (including labels and headings) and Markdown paths to the actual target; ambiguous matches must be resolved before deletion. If the audit cannot finish, skip deletions; already justified trims can still proceed.
2. Prepare the survivor and all affected knowledge/index edits. For a merge, retain the absorbed units' distinct insights and supporting original sources in the survivor. Keep paragraph citations, `source_notes`, `perspective` and `## Sources` consistent with retained content; do not union unrelated sources or leave a dependency on a unit about to be deleted.
3. Re-read files about to change; if a file changed since planning, defer the affected action rather than overwriting concurrent work. This check is best-effort, not a transaction.
4. Write and read back the survivor first with `create` + `force`. Verify the retained contributions and citations before proceeding. Preserve its `created_at`; update `last_compiled_at` only for a substantive edit, and preserve `compile_count` because curation did not compile a new source. Apply the same metadata rule to trims. Do not sum absorbed compilation counts.
5. Rewrite references only in knowledge units and indexes under `/_knowledge/`. Redirect absorbed-unit links to the survivor, preserving labels and only valid heading anchors. For a standalone deletion, remove a dispensable Related Knowledge entry; if a substantive claim relies on it, link to verified original evidence or keep/merge the unit instead. Do not merely strip the link from a dependent claim. Leave raw notes, shorts, personal memory and other vault content untouched.
6. Update affected index tables: remove deleted paths, refresh edited summaries, deduplicate surviving rows, keep at most two domain memberships, and recalculate `unit_count`. Preserve unrelated rows and index metadata. Do this directly; `hylo-index-knowledge` only adds missing entries and cannot perform these repairs.
7. Read back the changed units and indexes and confirm no retained knowledge reference or index row still requires a planned deletion. Then call `knowledge_delete` for each verified obsolete unit and verify it is absent. Never delete first or delete a source note.

On a failed write, failed verification or changed input, stop the affected deletion sequence and report what completed and what remains. Do not report a planned deletion as done. A later run re-reads actual files and repairs stale index rows for already-missing units without recreating them. Do not delete an empty domain index in this workflow.

## 4 — Progress and summary

For an automatic domain batch that completed successfully, update only that index's `last_curated_at` (current ISO timestamp) and `curation_after` (last candidate path examined in the sorted batch, excluding extra merge partners). Advance even for keep-only batches so later runs reach other units. Do not advance on preview or an incomplete run. No new progress files or per-unit review stamps.

No content improvement: leave unit bodies and compilation metadata unchanged. On explicit requests, briefly name meaningful trims, merges and deletions with their reasons; deleted titles are plain text, surviving units use wikilinks. Mention deferred work or failures accurately. An automatic run with no meaningful content changes returns exactly `[silent]`; updating the scan cursor alone is not worth a message.
