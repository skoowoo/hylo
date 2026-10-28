// Rendered table rows hide their pipes and padding; these keep the caret and
// deletions out of them. Live mode only: in source mode the pipes are text.
import { keymap, EditorView, ViewPlugin, Decoration } from '@codemirror/view';
import { EditorState, EditorSelection, Prec, RangeSetBuilder } from '@codemirror/state';
import { syntaxTree } from '@codemirror/language';
import { rowOnLine, rowTexts, isCellBreak } from './table-rows.js';
import { isHairlineLine } from './decorators.js';

// Where a caret in hidden source goes: on to the next cell's text in the
// direction it moved, so arrows step cell to cell and a click never lands where
// typing would go unseen. Only a one-character step (`leave`) goes on past
// the row's first or last cell to the next line.
function cellEdge(state, pos, forward, leave = true) {
  const { doc } = state;
  const line = doc.lineAt(pos);
  const row = rowOnLine(state, line);
  if (!row) return null;
  const texts = rowTexts(state, row);
  if (!texts.length || texts.some((t) => pos >= t.from && pos <= t.to)) return null;
  let before = null;
  let after = null;
  for (const t of texts) {
    if (t.to < pos) before = t.to;
    else if (after == null) after = t.from;
  }
  const inRow = forward ? after : before;
  if (inRow != null) return inRow;
  if (!leave) return forward ? before : after;
  let n = line.number + (forward ? 1 : -1);
  if (isHairlineLine(state, n)) n += forward ? 1 : -1;
  if (n < 1 || n > doc.lines) return forward ? before : after;
  const next = forward ? doc.line(n).from : doc.line(n).to;
  return cellEdge(state, next, forward, false) ?? next;
}

// Right after a <br> the caret belongs at the start of the next line, not at
// the end of the line the break closes.
function afterBreak(state, pos) {
  for (let n = syntaxTree(state).resolveInner(pos, -1); n && n.to === pos; n = n.parent) {
    if (isCellBreak(n, state.doc)) return true;
  }
  return false;
}

const keepCaretInCells = EditorState.transactionFilter.of((tr) => {
  if (!tr.selection || tr.docChanged || tr.isUserEvent('select.agent')) return tr;
  const { doc } = tr.startState;
  const prev = tr.startState.selection.main.head;
  const pointer = tr.isUserEvent('select.pointer');
  const sel = tr.newSelection;
  let moved = false;
  const ranges = sel.ranges.map((r) => {
    if (!r.empty) return r;
    // One step off a cell's text by arrow; Home/End and clicks stay in the row.
    const leave = !pointer && Math.abs(r.head - prev) <= 1 && doc.lineAt(prev).number === doc.lineAt(r.head).number;
    const to = cellEdge(tr.startState, r.head, r.head >= prev, leave) ?? r.head;
    const assoc = afterBreak(tr.startState, to) ? 1 : r.assoc;
    if (to === r.head && assoc === r.assoc) return r;
    moved = true;
    return EditorSelection.cursor(to, assoc, r.bidiLevel, r.goalColumn);
  });
  if (!moved) return tr;
  return [tr, { selection: EditorSelection.create(ranges, sel.mainIndex), sequential: true }];
});

// The row's text is blank: its cells hold nothing but padding.
function blankRow(state, row) {
  return rowTexts(state, row).every((t) => !state.doc.sliceString(t.from, t.to).trim());
}

// Backspace in a blank body row's first cell removes the row, as Enter there
// leaves the table; the caret goes to the end of the row above.
function deleteBlankRow(view, line) {
  const { state } = view;
  let n = line.number - 1;
  if (isHairlineLine(state, n)) n--;
  const above = state.doc.line(n).to;
  view.dispatch({
    changes: { from: line.from - 1, to: line.to },
    selection: { anchor: cellEdge(state, above, false, false) ?? above },
    scrollIntoView: true,
    userEvent: 'delete',
  });
  return true;
}

// Backspace at a cell's start (Delete at its end) would eat a hidden pipe and
// merge two cells out of sight; it stops there instead, except in a blank row.
function cellDelete(view, forward) {
  const { state } = view;
  const sel = state.selection;
  if (sel.ranges.length > 1 || !sel.main.empty) return false;
  const pos = sel.main.head;
  const line = state.doc.lineAt(pos);
  const row = rowOnLine(state, line);
  if (!row) return false;
  const texts = rowTexts(state, row);
  const text = texts.find((t) => pos >= t.from && pos <= t.to);
  if (!text) return false;
  if (!forward && row.name === 'TableRow' && text === texts[0] && line.number > 1 && blankRow(state, row)) return deleteBlankRow(view, line);
  return forward ? pos === text.to && pos < line.to : pos === text.from && pos > line.from;
}

// A drawn <br> is one step for the caret and one keystroke to delete.
const breakMark = Decoration.mark({});
const cellBreakAtoms = ViewPlugin.fromClass(class {
  constructor(view) {
    this.atoms = this.build(view);
  }

  update(u) {
    if (u.docChanged || u.viewportChanged || syntaxTree(u.state) !== syntaxTree(u.startState)) this.atoms = this.build(u.view);
  }

  build(view) {
    const builder = new RangeSetBuilder();
    const { doc } = view.state;
    for (const { from, to } of view.visibleRanges) {
      syntaxTree(view.state).iterate({ from, to, enter: (n) => {
        if (isCellBreak(n.node, doc)) builder.add(n.from, n.to, breakMark);
      } });
    }
    return builder.finish();
  }
}, { provide: (p) => EditorView.atomicRanges.of((view) => view.plugin(p)?.atoms ?? Decoration.none) });

export const tableCaret = [
  keepCaretInCells,
  cellBreakAtoms,
  Prec.high(keymap.of([
    { key: 'Backspace', run: (view) => cellDelete(view, false) },
    { key: 'Delete', run: (view) => cellDelete(view, true) },
  ])),
];
