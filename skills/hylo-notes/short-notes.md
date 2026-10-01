# Short Notes

Short notes are quick-capture entries appended to a daily file inside the `_shorts` directory. Use these for fleeting thoughts, quick todos, and daily log entries — not for structured notes with their own files.

## Create a short note

```bash
hylo short create --content "Quick thought"
```

Content goes to today's daily shorts file automatically — no path needed.

## List short notes

Each daily file is parsed into individual entries; you see one row per note, not one row per day.

```bash
hylo short list --latest 7                              # entries from the last 7 days
hylo short list --start 2026-01-01 --end 2026-01-31    # date range
hylo short list --limit 20                              # cap results
```

## When to use short notes vs regular notes

| Use short notes                 | Use regular notes                    |
| ------------------------------- | ------------------------------------ |
| Quick capture, fleeting thought | Structured content with its own file |
| Daily log entry                 | Journal entry at `/journal/`         |
| Temporary reminder              | Research or reference material       |
