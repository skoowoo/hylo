# Step: news-bot

Give them a News bot that brings a digest from the news sources they pick every morning, to their inbox.

**Condition:** skip it if `bot_list` already shows a bot named News.

## Do

1. Call `skills_catalog {"category": "news"}`. Ask which sources they want, and say the digests come at 08:00 unless they want another time. Offer the sources as multi-pick `choices`, one `- [ ] <skill name> — <what it covers, a few words>` each; if you know why one fits them, say so in the text above.
2. On their answer:
   - **None, or not now:** skip the step.
   - **One or more:** build it with the `hylo-bots` skill's create workflow, with these decided already, so skip its questions and its plan card (picking sources was the go-ahead):
     - A bot of its own named News, with a one-line description naming the sources, and this system prompt, word for word:

        ```
        You bring the user today's news. Your reply is the digest itself, and it goes straight to their inbox:
        - Write it in Markdown: a heading per topic or source. Each item is the original article as an embed card, then at most one sentence on why it matters; articles that belong together share one embed block.
        - Start with the news. No preamble, no account of what you looked up or how, no closing offer.
        - Never save notes or files; the reply is the only output.
        - Write in the language set in /_memory/_preferences.md.
        ```
     - The sources they picked as its skills, one scheduled trigger per source with the prompt "Use the <skill name> skill to get today's news.", in the order they picked, from the time they chose (08:00 by default) and staggered as `references/triggers.md` says.
     - Run the first trigger once now, so the first digest is on its way.

     End with its receipt: the first digest arrives shortly, then every morning at the time set.

If a call fails, the receipt says what didn't get set up; add that they can finish it by asking you or in Settings → Bots, then go on.

## Record

`- [x] news-bot — <skill names>, daily <time>`, or `- [-] news-bot — skipped: <reason>`.
