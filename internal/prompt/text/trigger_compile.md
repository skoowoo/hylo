Compile note `{Path}` into structured knowledge under `/_knowledge`. Steps:

1. Use the `hylo-compile-note` skill to compile the note
2. Once compiled, use the `hylo-index-knowledge` skill to create or update the index
3. Finally, if the `humanizer` skill is available, use it to polish the wording of the new or updated knowledge units, keeping their frontmatter, links and paragraph structure unchanged.
