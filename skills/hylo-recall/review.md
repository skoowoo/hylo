# Review

Review the requested period or take a random look back. Let original passages carry the review; your role is to select and briefly connect them. Reply in chat, not a file.

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
4. Group representative original passages under the main threads, with dates and source links. Use at most one short sentence to introduce each selection or connect a supported change; keep excerpts dominant. Preserve wording and context, mark omissions, and distinguish the user's words from saved or generated material. Sparse records need no personal story or lesson.

## Random look back

1. From visible replies in the past 30 days, collect previously shown paths (`resolve` ambiguous links). Load `sample` if needed, then call `sample {"each": 6, "older_than": -1, "exclude_paths": [<paths>]}` for candidates across the whole vault. A daily short path excludes that whole day. If needed, sample once more, also excluding the first batch.
2. Select 3–4 worthwhile discoveries across subjects and periods, favoring forgotten material. Skip recent repeats, tests, credentials and empty fragments; choose fewer rather than fill slots. Read selections with `short_get`, `read` or extraction tools; never quote a truncated sample preview as the original. For compiled units, prefer a passage from the source note. Check authorship and choose only one item per source family.
3. Present each item as a date and source link, an optional one-sentence introduction, then a blockquote of the original. Show a short entry in full or a self-contained passage from a longer note; preserve wording and mark omissions. Label compiled or external material accurately. Use verified original dates; import dates are not writing dates. Let the passage speak without paraphrasing it afterward, drawing a lesson or forcing a tie to this week. No process commentary or closing question for scheduled delivery. If nothing qualifies, follow any silence instruction or say so in one line.
