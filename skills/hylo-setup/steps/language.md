# Step: language

Make sure you talk to them in the language they'd rather use. The one you started in was guessed from their system, which many people keep in English.

**Condition:** always.

## Do

1. Ask in one short sentence whether this is the language they'd like you to use with them. Offer it as `choices`: the current language first, then one or two other likely ones, each named in its own script (中文, English, 日本語).
2. On their answer:
   - **Yes:** record it.
   - **Another language:** switch to it from this reply on, and record it. If the intro guide was written in the old language, offer to rewrite it in the new one; if they agree, do it with the `hylo-editor` skill.
   - **Unclear:** keep the current language and record it.

## Record

1. In `/_memory/_preferences.md`, add or update one item, following the file format of the `hylo-memory` skill (create the file with that format if it doesn't exist):

   ```
   - Prefers to be spoken to in <language name in English> (<tag>) `last:<today> seen:1 pinned`
   ```

   `pinned` keeps a stated preference from fading. Replace any older language item rather than adding a second one.
2. Set `language: <tag>` in the progress file's frontmatter, and mark `- [x] language — <tag>`.
