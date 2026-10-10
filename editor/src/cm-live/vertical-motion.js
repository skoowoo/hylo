// Arrow up/down that never skips a line. CM probes past a block widget using
// estimated heights when the far side isn't rendered, and can overshoot (WebKit; see e2e/).
import { keymap, BlockType } from '@codemirror/view';
import { EditorSelection, Prec } from '@codemirror/state';

// Collapsed hairline rows (table delimiter, setext underline, list gap) are
// meant to be stepped over, just like CM does when they're rendered.
function isStopLine(view, block) {
  return block.type === BlockType.Text && block.height >= view.defaultLineHeight / 3;
}

// Nearest visual row of `line` to where the motion came from, at the goal x.
function landOn(view, line, block, goal, forward) {
  const x = goal == null ? 0 : goal;
  if (line.from >= view.viewport.from && line.to <= view.viewport.to) {
    const half = view.defaultLineHeight / 2;
    const y = view.documentTop + (forward ? block.top + half : block.bottom - half);
    const pos = view.posAtCoords({ x: view.contentDOM.getBoundingClientRect().left + x, y }, false);
    if (pos != null && pos >= line.from && pos <= line.to) return EditorSelection.cursor(pos, forward ? -1 : 1, undefined, goal);
  }
  // Not rendered: no coordinates to resolve, estimate the column.
  const col = Math.round(x / view.defaultCharacterWidth);
  return EditorSelection.cursor(line.from + Math.min(col, line.length), forward ? -1 : 1, undefined, goal);
}

function guardedMove(view, range, forward) {
  const moved = view.moveVertically(range, forward);
  const doc = view.state.doc;
  const fromLine = doc.lineAt(range.head).number;
  const toLine = doc.lineAt(moved.head).number;
  if (Math.abs(toLine - fromLine) <= 1) return moved;
  for (let n = forward ? fromLine + 1 : fromLine - 1; forward ? n < toLine : n > toLine;) {
    const block = view.lineBlockAt(doc.line(n).from);
    if (isStopLine(view, block)) return landOn(view, doc.line(n), block, moved.goalColumn, forward);
    // A block covers whole lines; continue past its far edge.
    n = forward ? doc.lineAt(block.to).number + 1 : doc.lineAt(block.from).number - 1;
  }
  return moved;
}

function moveLines(forward, extend) {
  return (view) => {
    const sel = view.state.selection;
    const ranges = sel.ranges.map((range) => {
      // Like cursorLineUp: a plain arrow on a selection first collapses it to that edge.
      if (!extend && !range.empty) return EditorSelection.cursor(forward ? range.to : range.from);
      const moved = guardedMove(view, range, forward);
      return extend ? EditorSelection.range(range.anchor, moved.head, moved.goalColumn) : moved;
    });
    view.dispatch({ selection: EditorSelection.create(ranges, sel.mainIndex), scrollIntoView: true, userEvent: 'select' });
    return true;
  };
}

// Prec.high: defaultKeymap binds the same keys to cursorLineUp/Down.
export const verticalMotion = Prec.high(
  keymap.of([
    { key: 'ArrowUp', run: moveLines(false, false), shift: moveLines(false, true) },
    { key: 'ArrowDown', run: moveLines(true, false), shift: moveLines(true, true) },
  ])
);
