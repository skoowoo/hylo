# Draft

Write a note from the user's material. Choose sensible defaults; ask only when ambiguity would change the result.

## Material

| They say | Take |
| -------- | ---- |
| "these notes", with `[[ ]]` in the message | those notes |
| "this week's short notes", "what I jotted lately" | `short_list` over the window they name, 7 days by default |
| "what we just discussed" | this conversation: only the conclusions and what the user agreed with, never the back-and-forth |
| "my notes on X" | find it in the order your instructions give |
| one note + a form ("make a WeChat version", "a TL;DR of [[X]]") | that note; leave it untouched and save a new one |

## Steps

1. Read the voice (`/_memory/_voice.md`) and settle the form (`references/forms.md`).
2. Gather the material. Attribute source claims and user comments accurately; label your suggestions in the note itself. Don't invent the user's beliefs or plans. Leave gaps out and name them in step 5.
3. Pick the title and the folder without asking. An adaptation keeps the original name plus the form: `Attention management (WeChat)`, `注意力管理（公众号）`.
4. Write it (where: `references/surface.md`). End the body with the source notes as `[[ ]]` links, the same way knowledge units link back to theirs.
5. In one or two sentences, say where it's saved, its sources and any gaps. Offer `choices` only for an unresolved decision.

Never write the piece in chat first. They asked for a note; writing it twice is slow and splits their attention. Changes happen on the note, in revise mode.

## Remembering a form

If the user has now used the same structure for this kind of piece twice or more (an earlier note of the same kind, written the same way), offer once per conversation to remember it: one `choices` option like "Remember this weekly-report format". On yes, record it in `/_memory/_voice.md` with the `hylo-memory` skill's *Record one fact* (form, sections, folder), `pinned`.
