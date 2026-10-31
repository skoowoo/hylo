# Writing a bot's system prompt

The system prompt is the bot's standing instructions for every run. Triggers add the per-run request on top of it.

## Shape

A short opening line saying what the bot is for, then a few bullets of rules. Aim for under ten lines.

Cover, in this order, only what applies:

1. **Job**: one sentence, in the second person ("You bring the user today's news.").
2. **Output**: a trigger wakes the bot in its own chat and its final reply is the message the user gets, so the reply is the deliverable. Say its format (Markdown, headings) and to start with the content, with no preamble, no account of how it was made and no closing offer. When it hands over pages, videos or episodes to open, say each goes in an embed card; every run already knows the block, so name it, don't explain it.
3. **Side effects**: by default tell it not to save notes or files and that the reply is the only output. Allow writing to the vault only if the job is to write notes, and then name where (the bot's own folder unless the user says otherwise).
4. **Language**: "Write in the language set in /_memory/_preferences.md." unless the user named a language.
5. **Boundaries** the user stated (sources to avoid, length, tone).

## Don't

- Don't list skills by name; a trigger prompt or the job decides when a skill is used.
- Don't restate the schedule; triggers own that.
- Don't pad with generic advice ("be helpful", "be accurate").

## Example

```
You keep the user's reading list tidy. Your reply is the report the user reads in your chat:
- Write it in Markdown: one heading per folder, one line per note with a [[link]].
- Start with the report. No preamble, no account of what you looked at, no closing offer.
- Never save notes or files; the reply is the only output.
- Write in the language set in /_memory/_preferences.md.
```
