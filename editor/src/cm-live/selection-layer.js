// Selection background, in place of drawSelection's own layer (whose cursor layer stays).
// CM decides which visual lines a range's ends are on by hit-testing the line's left and
// right edges (posAtCoords). A line that starts or ends with hidden syntax ("## ", "> ",
// a table's pipes, a closing "**") has several zero-width pieces at that edge, and CM
// breaks the tie at random: a selection inside one line was drawn now as one line, now as
// two with the whole line filled, flashing on every mouse move. Here the ends' own
// coordinates decide, which don't tie.
import { layer, RectangleMarker, EditorView, BlockType } from '@codemirror/view';

const overlap = (a, b) => a.top < b.bottom && b.top < a.bottom;

function rectangles(view, range, className) {
  const { viewport, state } = view;
  if (range.to <= viewport.from || range.from >= viewport.to) return [];
  const from = Math.max(range.from, viewport.from);
  const to = Math.min(range.to, viewport.to);
  const content = view.contentDOM.getBoundingClientRect();
  const scroller = view.scrollDOM.getBoundingClientRect();
  const baseLeft = scroller.left - view.scrollDOM.scrollLeft;
  const baseTop = scroller.top - view.scrollDOM.scrollTop;
  const piece = (left, top, right, bottom) =>
    new RectangleMarker(className, left - baseLeft, top - baseTop, Math.max(0, right - left), bottom - top);

  // An end inside a block widget (card, frontmatter) sits past it, as in CM: a range
  // starting there begins at its bottom, one ending there stops at its top.
  const end = (pos, side) => {
    const block = view.lineBlockAt(pos);
    if (block.type !== BlockType.Text) {
      const y = content.top + (side > 0 ? block.bottom : block.top);
      return { top: y, bottom: y, left: content.left, right: content.left, widget: true };
    }
    return view.coordsAtPos(pos, side);
  };
  const a = end(from, 1);
  const b = end(to, -1);
  if (!a || !b) return [];
  const fromLine = state.doc.lineAt(from);
  const toLine = state.doc.lineAt(to);

  if (fromLine.number === toLine.number && !a.widget && overlap(a, b)) {
    return [piece(a.left, Math.min(a.top, b.top), b.right, Math.max(a.bottom, b.bottom))];
  }
  const top = { top: a.top, bottom: a.bottom };
  const bottom = { top: b.top, bottom: b.bottom };
  // A full-width band only where something lies between the two ends' visual lines:
  // a whole line, or the rest of a wrapped one. Otherwise the two pieces meet halfway.
  const lineEnd = fromLine.number === toLine.number ? null : view.coordsAtPos(fromLine.to, -1);
  const lineStart = fromLine.number === toLine.number ? null : view.coordsAtPos(toLine.from, 1);
  const between = fromLine.number === toLine.number
    ? b.top - a.bottom > view.defaultLineHeight / 2
    : toLine.number - fromLine.number > 1 || (lineEnd && !overlap(a, lineEnd)) || (lineStart && !overlap(b, lineStart));
  const out = [piece(a.left, top.top, content.right, top.bottom)];
  if (between && a.bottom < b.top) {
    out.push(piece(content.left, a.bottom, content.right, b.top));
  } else if (a.bottom < b.top) {
    top.bottom = bottom.top = (a.bottom + b.top) / 2;
    out[0] = piece(a.left, top.top, content.right, top.bottom);
  }
  out.push(piece(content.left, bottom.top, b.right, bottom.bottom));
  return out;
}

export const selectionLayer = [
  layer({
    above: false,
    class: 'cm-lp-selection-layer',
    markers(view) {
      const out = [];
      for (const r of view.state.selection.ranges) if (!r.empty) out.push(...rectangles(view, r, 'cm-selectionBackground'));
      return out.filter((m) => m.width > 0 && m.height > 0);
    },
    update(u) {
      return u.docChanged || u.selectionSet || u.viewportChanged;
    },
  }),
  EditorView.baseTheme({ '.cm-selectionLayer': { display: 'none' } }),
];
