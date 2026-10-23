# Hylo markdown formats

Hylo's editor renders GitHub-flavored markdown plus Obsidian-style wiki syntax, live, while the text is typed. Raw markdown shows again only where the user's caret is. Everything below is what the editor actually understands; anything else stays plain text.

## Basics

| Format | Syntax | Notes |
| --- | --- | --- |
| Headings | `#` … `######` | Prefer ATX (`#`). Setext (`===`/`---` underlines) works but is easy to trigger by accident. |
| Bold / italic | `**bold**`, `*italic*` |  |
| Strikethrough | `~~text~~` |  |
| Inline code | `` `code` `` |  |
| Link | `[text](https://…)`, bare `https://…`, `<https://…>` | Prefer `[text](…)` for anything the reader shouldn't see as a raw URL. |
| Quote | `> text`, nested `> >` | Fences and lists inside quotes work. |
| Rule | `---` on its own line | Needs a blank line above, or it underlines the paragraph into a heading. Never as the note's first line: that reads as frontmatter. |
| HTML | allowed | Shown as tinted markup, not rendered. Avoid it; use markdown. |

There is no `==highlight==`, no inline `#tag` (tags live in frontmatter, which the editor tools never touch), no callouts (`> [!note]`), no math and no footnotes. They stay as literal text.

## Lists and tasks

```markdown
- bullet
  - nested (indent 2 spaces)
1. ordered
2. ordered

- [ ] open task
- [x] done task
```

Task checkboxes are clickable in the editor, so the user can tick what you write. Keep one task per line.

## Tables

```markdown
| Option | Cost | Notes |
| ------ | ---: | ----- |
| A      |   10 | cheap |
| B      |   25 | fast  |
```

- Always type the header and separator rows; without the separator it is not a table.
- Escape a literal pipe in a cell as `\|`. Don't put wikilink aliases (`[[a|b]]`) inside tables, because the `|` splits the cell.
- For a big table, use `speed: "fast"` or `"instant"`: a half-typed table flickers between layouts.

## Code blocks

````markdown
```python
print("hi")
```
````

Syntax highlighting covers JavaScript, TypeScript, JSON, Python, Go, Rust, Java, C++, HTML, CSS, XML, YAML, SQL, PHP, Shell, TOML, Diff, Dockerfile, Ruby, Swift, Kotlin and C#. Other info strings render as plain code. `~~~` fences work too. Always close the fence.

## Wikilinks

| Syntax | Meaning |
| --- | --- |
| `[[Note name]]` | Link to the note whose file is `Note name.md`, anywhere in the vault |
| `[[Note name#Heading]]` | Link to a heading in that note |
| `[[Note name\|shown text]]` | Link with different display text |
| `[[#Heading]]` | Jump to a heading in the current note |
| `[[folder/Note name]]` | Vault path, only when two notes share a name |

- The target is the **file name without `.md`**, not the title inside the note. Hylo enforces unique note names, so a bare name is almost always enough.
- Links to notes that don't exist render as broken. Confirm targets with `resolve` (or `search` for fuzzy matches) before linking.
- Typing `[[` doesn't pop autocomplete for the agent. Write the full link.
- Don't use `[`, `]` or `|` inside the target or alias.

## Wiki images

```markdown
![[diagram.png]]
```

- Embeds an image from the vault's image store, by file name.
- The MCP tools cannot upload images. Only embed images you know exist, for example ones already referenced by other notes.
- Standard `![alt](https://…)` images also render.

## Mindmap blocks

A fenced block with info string `mindmap` renders as an interactive mind map card. The user can pan, zoom, fold, and open it picture-in-picture or full screen. The body is a markdown **outline**:

````markdown
```mindmap
# Pricing strategy
## Value metric
- per seat
- per usage
  - API calls
  - storage
## Packaging
- Free tier
- Pro
- Enterprise
```
````

How the outline becomes a tree:

- **Headings** set depth: `#` is shallower than `##`, and so on.
- **List items** (`-`, `*`, `+`, `1.`) nest by indentation under the nearest heading or shallower item. Task markers `[ ]`/`[x]` are dropped.
- **Paragraph lines are ignored.** Every node must be a heading or a list item.
- One top-level node becomes the root. If there are several, they hang off an empty root. So start with a single `# Topic`.
- Inline formatting inside a node: `` `code` ``, `**bold**`, `*italic*`, `~~strike~~`, `[text](https://…)`.
- Wikilinks are **not** linked inside a mindmap and show as raw `[[…]]`. Use plain text labels.

Tips:

- Keep labels short (a few words, under ~40 characters). Detail belongs in the note body, not the map.
- Two to four levels read best. Balance branches. Root children each get their own color.
- Indent consistently (2 spaces per level).
- To build a map from existing content, `editor_read` the note, then type the mindmap as a new block or `editor_rewrite` an existing outline into one. A fence lands as a single block, so a later revision is one `editor_rewrite` on that block id.

## Embed cards

A fenced block with info string `embed` renders each URL in its body as a card. One URL is a full-width card. Several URLs, one per line, share a grid of equal cards, at most three across; further URLs wrap onto the next row. Two URLs fill the row. A web page shows its title, description and image. An audio or video file plays in the card. An image shows the image. Anything else is a file row.

````markdown
```embed
https://example.com/story
```
```embed
https://example.com/one
https://example.com/two
```
````

- Every non-empty line is one `http` or `https` URL. A sentence about it stays outside the fence, as a normal paragraph.
- Use it where the link is what the reader opens: a reading list, resources, a clip's original. A source cited inside a sentence stays a normal link, and a note is always a [[wiki link]].
- The cards are filled in when the note is open; the URLs are all that's stored.
