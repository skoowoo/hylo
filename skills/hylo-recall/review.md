# Review

Review the requested period or take a random look back. Say the result in chat; don't write a file.

## Periodic review

Use the requested range or folder. Default: past 7 days; “this month”: 30 days.

1. Collect in parallel:

   ```
   knowledge_list {"kind": "knowledge", "latest": <window>}
   short_list {"latest": <window>, "limit": 100}
   list {"latest": <window>}
   ```

   Read Active items in `/_memory/_state.md` and `_goals.md`. If nothing changed, follow any silence instruction; otherwise say so briefly.
2. Read all short entries. For knowledge units, use `extract_tag` and `extract_segment` with `head: 20` to check content, `perspective`, and whether new (`created_at`) or updated (`last_compiled_at`). For regular notes, use `extract_outline` or `extract_segment`; fully read only central material. Check authorship before attributing any of it to the user.
3. Identify the main recorded threads, changes and unresolved plans. Search older notes only where an earlier answer, changed view or open question would help. An unrecorded outcome remains unknown.
4. Answer the request: dates, counts and examples for recording activity; a few paragraphs for a broader reflection. Link sources and distinguish the user's words from saved or generated material. Include connections only when supported, with at most one useful closing question. Sparse records need no personal story or lesson.

## Random look back

1. Call `sample {}`. It draws two of each kind across the vault, favoring material at least 90 days old when available. A unit's `created_at` is its earliest source date. If empty, follow any silence instruction; otherwise say there is nothing yet.
2. Give a short paragraph per item in returned order: its date, what it says, a brief quote when useful, and `[[filename stem]]`; short entries link their daily note. Name a unit's domain and check authorship using its `perspective` and sources. Read further when the excerpt leaves content or attribution unclear. Don't force a connection to this week. At most one closing question, only if a find raises it.
