# Revise

Before touching anything, decide whether the user wants **feedback**, **candidates** or a **replacement**. Only a replacement changes their text without them picking first.

| Intent | Typical words | Touches the text | Do |
| ------ | ------------- | ---------------- | -- |
| Feedback | "take a look", "what's weak", "does the logic hold", "would an outsider get it", "does this contradict what I thought before", 帮我看看、哪里不好 | never | In chat only: 3–5 points, each quoting the sentence (`>`), the problem and a suggestion. To check it against their earlier notes, use the `hylo-recall` skill's Find mode (what relates to this content). End with `choices`: apply the suggestions, apply only some, leave it. |
| Candidates | "say it another way", "give it a title", "write an opening", "how do I get from here to there", "what comes next" | only once picked | 2–3 numbered versions in chat; `choices` holds only short labels like "Use version 1". Write the picked one in. |
| Replacement | "polish", "make it flow", "shorten", "expand", "more casual", "translate into English", "continue" | yes | Write it in without confirming; each editor write is one undo step (Cmd+Z). Then say in one sentence what changed. |

When it's unclear ("this paragraph isn't great"), treat it as feedback and then ask whether to change it. Lean conservative.

## How far a replacement goes

- Change only the given range. Not one character outside it, not even a typo you noticed.
- Keep the user's views, facts and `[[ ]]` links. Polishing doesn't change the stance; shortening doesn't drop an argument.
- Revise toward the voice (`/_memory/_voice.md`), not your own taste. Where the voice is silent, match how this text is already written.
- If the `humanizer` or `humanizer-zh` skill is on and they ask to make it sound less AI-written, use it.

## Pointing at the range

With a quote snapshot in the message, use `editor_rewrite` with `from_text` / `to_text` set to the quote's first and last words, not `target: "selection"`: the user may have clicked elsewhere since sending, so the live selection may no longer be that passage.

Without a quote, follow the `hylo-editor` skill's *Revise part of the note*. Where the result goes, and what to do without an editor, is in `references/surface.md`.
