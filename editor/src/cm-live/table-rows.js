// A GFM table row's cells as source ranges, shared by the decorations, the
// keymap and the caret guards so they agree on where a cell's text is.
import { syntaxTree } from '@codemirror/language';
import { resolveAncestor } from './tree-utils.js';

export const isRow = (n) => n.name === 'TableHeader' || n.name === 'TableRow';

export function rowAt(state, pos) {
  return resolveAncestor(state, pos, isRow, -1) || resolveAncestor(state, pos, isRow, 1);
}

// The row starting on this line, if the line is a table's header or body row.
export function rowOnLine(state, line) {
  // Trailing blanks after the last pipe lie outside the row (and the table).
  const end = line.from + line.text.trimEnd().length;
  for (let n = syntaxTree(state).resolveInner(end, -1); n; n = n.parent) {
    if (isRow(n)) return n.from >= line.from ? n : null;
    if (n.name === 'Table') return null;
  }
  return null;
}

// A cell's source is one line, so a line break inside it is written <br>.
export const CELL_BREAK = '<br>';
const BREAK_RE = /^<br\s*\/?>$/i;

export function isCellBreak(node, doc) {
  return node.name === 'HTMLTag' && !!node.parent && node.parent.name === 'TableCell' && BREAK_RE.test(doc.sliceString(node.from, node.to));
}

// Content ranges between pipes; a row may omit its leading/trailing pipe.
export function rowCells(state, row) {
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

// What a rendered cell shows: its trimmed text, or for an empty cell its blank
// padding, so a caret placed there still has somewhere visible to sit.
export function cellText(state, cell) {
  const text = state.doc.sliceString(cell.from, cell.to);
  const core = text.trim();
  if (!core) return { from: cell.from, to: cell.to };
  const from = cell.from + text.length - text.trimStart().length;
  return { from, to: from + core.length };
}

// The visible text runs of a row, in order.
export function rowTexts(state, row) {
  return rowCells(state, row).map((c) => cellText(state, c));
}
