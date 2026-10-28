# Line icons

One file per icon, named by what it means in Hylo (`pinned`, not `star`).
Each is a plain 24×24 SVG in Lucide coordinates, so it opens in any viewer
and diffs cleanly against upstream. The root carries:

- `data-base`: the Lucide icon it starts from, e.g. `lucide/star`
- `data-zoom`: optical scale about the centre, applied at render (1–1.5);
  the stroke shrinks by the same factor, so every icon draws the same line
- `data-kind="glyph"`: a small UI mark (chevron, close, the draft pencil),
  drawn smaller than nav icons on purpose; the specimen lists glyphs apart
  and doesn't check their size
- `data-changes`: what was altered from the base, if anything

Paths keep Lucide's stroke attributes; the drawn width is
`lineIconStroke` in `icons_line.go`.

After editing, run `make icons` (writes `shared_icons_line.go`) and check
`/dev/icons`. It shows every icon in the sidebar and at each size, with its
optical size and centre offset; flagged numbers are outside the sidebar
set's range. In templates use `{{icon "name"}}`, or `{{icon "name" "class"}}`.

Lucide is ISC-licensed; see THIRD_PARTY_NOTICES.
