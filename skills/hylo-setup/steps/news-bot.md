# Step: news-bot

Give them a News bot that brings a digest from the news sources they pick every morning, to their inbox.

**Condition:** skip it if `hylo bot list` already shows a bot named News.

## Do

1. Run `hylo skills list --category news`. Ask which sources they want, and say the digests come at 08:00 unless they want another time. Offer the sources as multi-pick `choices`, one `- [ ] <skill name> — <what it covers, a few words>` each; if you know why one fits them, say so in the text above.
2. On their answer:
   - **None, or not now:** skip the step.
   - **One or more:**
     1. `hylo skills install <name>` for each one they picked.
     2. `hylo bot create News --description "<one line naming the sources>" --system-prompt "<system prompt>"`, with this system prompt, word for word:

        ```
        You bring the user today's news. Your reply is the digest itself, and it goes straight to their inbox:
        - Write it in Markdown: a heading per topic or source, short items, each with a link to the original.
        - Start with the news. No preamble, no account of what you looked up or how, no closing offer.
        - Never save notes or files; the reply is the only output.
        - Write in the language set in /_memory/_preferences.md.
        ```
     3. `hylo trigger add <bot-id> --name <skill name> --schedule "daily 08:00" --prompt "Use the <skill name> skill to get today's news."` once per source, in the order they picked. The first is at 08:00, or the time they chose; each next one is five minutes later (`daily 08:05`, `daily 08:10`, …).
     4. `hylo trigger run <bot-id> <first trigger-id>`, so the first digest is on its way now.
     5. Tell them in one or two sentences: the first digest will be in their inbox shortly, then every morning at the time set.

If a command fails, say what didn't get set up in one line, and that they can finish it in Settings → Bots; then go on.

## Record

`- [x] news-bot — <skill names>, daily <time>`, or `- [-] news-bot — skipped: <reason>`.
