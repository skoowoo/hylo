# Line icons

One file per icon, named by what it means in Hylo (`pinned`, not `star`).
Each is a plain 24×24 SVG in Lucide coordinates, so it opens in any viewer
and diffs cleanly against upstream. The root carries:

- `data-base`: the Lucide icon it starts from, e.g. `lucide/star`
  (`hylo/...` for the few drawn here)
- `data-zoom`: optical scale about the centre, applied at render (1–1.5);
  the stroke shrinks by the same factor, so every icon draws the same line
- `data-kind="glyph"`: a small UI mark (chevrons, close, check, plus, the
  draft pencil), drawn smaller than icons on purpose; not size-checked
- `data-changes`: what was altered from the base, if anything

House style: stroke 2 (`STROKE` in editor/gen-icons.mjs); square box
corners at radius 4 (`corners r4`); every other corner, from arrow and
chevron tips to star points and folds, rounded so it trims 2 units off each
side (radius 2 at a right angle, smaller at sharper angles, so tips round
off without being cut short) (`tips rounded`). Where another stroke ends on
a rounded corner (an arrow shaft, a fold line), it ends on the new arc, not
the old point. Box corners are circular (`ry` equals `rx`), and a zoom never
pushes ink past the box edge.

`assets/icons_geometry.test.js` (part of `make test-js`) enforces these:
no sharp corner, no stroke end poking past another stroke, no elliptical box
corner, no clipped ink.

`make icons` writes `shared_icons_line.go` (templates) and
`assets/shared_icons_line.js` (page scripts); a Go test keeps them equal.

- Templates: `{{icon "name"}}`, `{{icon "name" "class"}}`, or raw
  attributes `{{icon "name" `class="x" width="13" height="13"`}}`. A
  `fill` or `stroke` there replaces the line defaults (a solid badge).
- Scripts: `hyloIcon('name', attrs)`, same rules.
- CSS: `var(--icon-<name>)` as a mask, for names listed in
  `lineIconsCSS(...)` in assets.go.

Check `/dev/icons` after a change: navigation icons are size-checked
against each other; the rest are shown with their measurements.

Lucide is ISC-licensed; see THIRD_PARTY_NOTICES.
