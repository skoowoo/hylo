// Tab / Shift-Tab / Enter inside a GFM table: move between cells, append rows,
// and leave the table from an empty last row.
import { keymap } from '@codemirror/view';
import { Prec, EditorSelection } from '@codemirror/state';
import { resolveAncestor } from './tree-utils.js';

const isRow = (n) => n.name === 'TableHeader' || n.name === 'TableRow';

function rowAt(state, pos) {
  return resolveAncestor(state, pos, isRow, -1) || resolveAncestor(state, pos, isRow, 1);
}

// Content ranges between pipes; a row may omit its leading/trailing pipe.
function rowCells(state, row) {
  const pipes = row.getChildren('TableDelimiter');
  let i = 0;
  let start = row.from;
  if (pipes.length && pipes[0].from === row.from) start = pipes[i++].to;
  const cells = [];
  for (; i < pipes.length; i++) {
    cells.push({ from: start, to: pipes[i].from });
    start = pipes[i].to;
  }
  if (start < row.to && state.doc.sliceString(start, row.to).trim()) cells.push({ from: start, to: row.to });
  return cells;
}

// Select the cell's text so typing replaces it; an empty cell gets a caret after its pad space.
function cellSelection(state, cell) {
  const text = state.doc.sliceString(cell.from, cell.to);
  const lead = text.length - text.trimStart().length;
  const core = text.trim();
  if (core) return EditorSelection.range(cell.from + lead, cell.from + lead + core.length);
  return EditorSelection.cursor(Math.min(cell.from + 1, cell.to));
}

function cellIndexAt(cells, pos) {
  for (let i = 0; i < cells.length; i++) if (pos <= cells[i].to) return i;
  return cells.length - 1;
}

function siblingRow(row, dir) {
  for (let n = dir > 0 ? row.nextSibling : row.prevSibling; n; n = dir > 0 ? n.nextSibling : n.prevSibling) {
    if (isRow(n)) return n;
  }
  return null;
}

function newRowText(state, row, columns) {
  const line = state.doc.lineAt(row.from);
  const prefix = state.doc.sliceString(line.from, row.from);
  return prefix + '|' + '  |'.repeat(Math.max(1, columns));
}

function columnCount(state, row) {
  const header = row.parent && row.parent.getChild('TableHeader');
  return rowCells(state, header || row).length;
}

function insertRowAfter(view, row) {
  const state = view.state;
  const line = state.doc.lineAt(row.from);
  const text = newRowText(state, row, columnCount(state, row));
  const prefixLen = row.from - line.from;
  view.dispatch({
    changes: { from: line.to, insert: '\n' + text },
    selection: { anchor: line.to + 1 + prefixLen + 2 },
    scrollIntoView: true,
    userEvent: 'input',
  });
  return true;
}

function moveCell(view, dir) {
  const state = view.state;
  if (state.readOnly) return false;
  const head = state.selection.main.head;
  const row = rowAt(state, head);
  if (!row) return false;
  const cells = rowCells(state, row);
  const idx = cellIndexAt(cells, head);
  let target = cells[idx + dir];
  if (!target) {
    const next = siblingRow(row, dir);
    if (!next) return dir > 0 ? insertRowAfter(view, row) : true;
    const nextCells = rowCells(state, next);
    target = dir > 0 ? nextCells[0] : nextCells[nextCells.length - 1];
    if (!target) return true;
  }
  view.dispatch({ selection: cellSelection(state, target), scrollIntoView: true });
  return true;
}

export function tableEnter(view) {
  const state = view.state;
  if (state.readOnly) return false;
  const row = rowAt(state, state.selection.main.head);
  if (!row) return false;
  const empty = rowCells(state, row).every((c) => !state.doc.sliceString(c.from, c.to).trim());
  if (empty && row.name === 'TableRow') {
    // Empty row → leave the table on a blank line instead of growing it forever.
    const line = state.doc.lineAt(row.from);
    view.dispatch({ changes: { from: line.from, to: line.to }, selection: { anchor: line.from }, userEvent: 'delete' });
    return true;
  }
  return insertRowAfter(view, row);
}

export const nextCell = (view) => moveCell(view, 1);
export const previousCell = (view) => moveCell(view, -1);

export const tableKeymap = Prec.highest(
  keymap.of([
    { key: 'Tab', run: nextCell },
    { key: 'Shift-Tab', run: previousCell },
    { key: 'Enter', run: tableEnter },
  ])
);
