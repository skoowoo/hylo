# Step: recall-bot

Give them a Recall bot that brings their own notes back to them, in their inbox, in the ways they pick.

**Condition:** skip it if `bot_list` already shows a bot named Recall.

## Do

1. In one sentence, say it brings their notes back to them on its own, and ask which of these they want. Offer them as multi-pick `choices`, one `- [ ] <name> — <when, what>` each, in this order:
   - **Weekly review**: every Monday morning, a short letter on what their week was about and how it ties to what they've thought before.
   - **Look back**: every afternoon, something from their history they've likely forgotten that speaks to where they are now.
   - **Short notes**: after each short note, a word on it in light of what they've written before; nothing when their notes have nothing to add.

   If one fits them especially (many short notes, a vault years deep), say so in the text above.
2. On their answer:
   - **None, or not now:** skip the step.
   - **One or more:** build it with the `hylo-bots` skill's create workflow, with these decided already, so skip its questions and its plan card (their pick was the go-ahead):
     - A bot of its own named Recall, with a one-line description naming what they picked, and this system prompt, word for word:

        ```
        You're an old friend who has read everything the user has written. You bring their own notes back to them, with what you notice about how they think and how they've changed. Your reply goes straight to their inbox:
        - Start with the content. No preamble, no account of what you looked up or how, no closing offer.
        - Write like a friend's note, not a report: the one or two things worth saying, in their own words where you can.
        - Everything you remember about them comes from their notes and /_memory/; never claim more.
        - Link every note and knowledge unit you lean on.
        - Never save notes or files; the reply is the only output.
        - Write in the language set in /_memory/_preferences.md.
        ```
     - The built-in `hylo-recall` skill turned on for it with `skills_enable`; it's added already, so no `skills_add`.
     - One trigger per pick, named as in the choices, with its prompt word for word:
       - **Weekly review**: schedule `weekly mon 09:05` (after the main bot's daily memory update).

          ```
          Use the hylo-recall skill to review everything I wrote this past week.
          ```
       - **Look back**: schedule `daily 15:00`.

          ```
          Use the hylo-recall skill for a random look back: a few things from my history that I have likely forgotten.
          ```
       - **Short notes**: event `short_note_created`.

          ```
          Use the hylo-recall skill to reflect on this short note of mine against everything I've written before.

          If my notes give you nothing worth saying about it, reply with exactly `[silent]` and nothing else.

          Short note content: `{Content}`
          ```
     - Run one scheduled trigger once now, so they see what it brings: Look back if they picked it and have notes older than 90 days (`list {"end": "<the date 90 days ago>", "limit": 1}`), else Weekly review if they picked it and wrote or changed notes in the past week (`list {"latest": 7, "limit": 1}`), else none.

     End with its receipt: what arrives when, and that short notes only get a reply when their notes have something to say.

If a call fails, the receipt says what didn't get set up; add that they can finish it by asking you or in Settings → Bots, then go on.

## Record

`- [x] recall-bot — <picks>`, or `- [-] recall-bot — skipped: <reason>`.
