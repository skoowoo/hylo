# Step: news-bot

Give them a News bot that brings a digest from the news sources they pick every morning, to their inbox.

**Condition:** skip it if `bot_list` already shows a bot named News.

## Do

1. Call `skills_catalog {"category": "news"}`. Ask which sources they want, and say the digests come at 08:00 unless they want another time. Offer the sources as multi-pick `choices`, one `- [ ] <skill name> — <what it covers, a few words>` each; if you know why one fits them, say so in the text above.
2. On their answer:
   - **None, or not now:** skip the step.
   - **One or more:**
     1. For each one they picked: `skills_add {"repo": "<repo>", "skill": "<name>"}` with the values from its catalog entry, unless it is already `added`; then `skills_enable {"name": "<name>"}`.
     2. `bot_create {"name": "News", "description": "<one line naming the sources>", "system_prompt": "<system prompt>"}`, with this system prompt, word for word:

        ```
        You bring the user today's news. Your reply is the digest itself, and it goes straight to their inbox:
        - Write it in Markdown: a heading per topic or source, short items, each with a link to the original.
        - Start with the news. No preamble, no account of what you looked up or how, no closing offer.
        - Never save notes or files; the reply is the only output.
        - Write in the language set in /_memory/_preferences.md.
        ```
     3. `trigger_add {"bot_id": "<bot id>", "name": "<skill name>", "schedule": "daily 08:00", "prompt": "Use the <skill name> skill to get today's news."}` once per source, in the order they picked. The first is at 08:00, or the time they chose; each next one is five minutes later (`daily 08:05`, `daily 08:10`, …).
     4. `trigger_run {"bot_id": "<bot id>", "trigger_id": "<first trigger id>"}`, so the first digest is on its way now.
     5. Tell them in one or two sentences: the first digest will be in their inbox shortly, then every morning at the time set.

If a call fails, say what didn't get set up in one line, and that they can finish it in Settings → Bots; then go on.

## Record

`- [x] news-bot — <skill names>, daily <time>`, or `- [-] news-bot — skipped: <reason>`.
