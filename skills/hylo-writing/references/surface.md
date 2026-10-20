# Where the writing lands

Whether to touch a note depends on the intent. Which tool touches it: **the live editor first**, files only when the editor is unavailable. Load the `hylo-editor` skill before the first `editor_*` call.

The editor is unavailable when the `editor_*` tools aren't there even after searching your deferred tools for "hylo editor", or when the first editor call fails with "no editor is open in Hylo" (nothing was written). Then write the file as the right column says, and say so in one line first: "No editor open, so I'm writing the file directly." Don't ask the user to open Hylo.

| Case | Editor available | Editor unavailable |
| ---- | ---------------- | ------------------ |
| Revise: feedback, or candidates not yet picked | chat only | chat only |
| Revise: replacement, or a picked candidate | `editor_rewrite`, or `editor_type` with `at: "cursor"`; the user watches it change | read the whole note, write it back with `create` + `force` |
| Draft | `editor_type` to the new path (a missing note is created), `speed: "fast"`; the user watches the note open and fill | `create` |
| Longform | `editor_type`, one call per beat, default speed | `create`, then `append` per beat |
| A trigger running on its own (e.g. a scheduled weekly report) | `create`, never the editor: nobody is watching, and typing would take over the note they're editing | `create` |

Once something is written through the editor, the piece stays there (the `hylo-editor` skill's *Commit to the editor*): any later failure, such as "the editor disconnected…", is reported, never finished with file tools.

A whole-file write-back must leave every other paragraph byte-for-byte as it was.

**From WeChat, Discord, Telegram or another chat app:** a draft is saved as a note as usual, and the reply gives its name and a two-sentence summary. A revise only replies with the revised text and leaves the note alone: on a phone they can't check what changed.
