// ── GFM tables — each row line is a flex row of cell marks sized by shared
// column weights. Rows stay drawn as cells with the caret in them: the source
// line wraps nothing like its cells do, so showing it raw changed the row's
// height and every column's width. Inline syntax inside a cell still reveals
// around the caret like anywhere else; structure is edited with Tab/Enter or
// in source mode.
import { Decoration } from '@codemirror/view';
import { caretOnLine } from './selection.js';
import { TableEmptyCellWidget, TableDelimiterWidget, TableBreakWidget } from './widgets.js';
import { hideRange } from './decoration-helpers.js';
import { rowCells, cellText } from './table-rows.js';

function tableAlignments(tableNode, doc) {
  const delim = tableNode.getChild('TableDelimiter');
  if (!delim) return [];
  const text = doc.sliceString(delim.from, delim.to);
  const segs = text.split('|').map((s) => s.trim());
  if (segs.length && segs[0] === '') segs.shift();
  if (segs.length && segs[segs.length - 1] === '') segs.pop();
  return segs.map((seg) => {
    const left = seg.startsWith(':');
    const right = seg.endsWith(':');
    if (left && right) return 'center';
    if (right) return 'right';
    if (left) return 'left';
    return null;
  });
}

// Column weight — the widest cell's character count (+2 slack), across
// every row including the header. Drives two things per column: the
// flex-grow ratio (how much of the row's *leftover* width it claims) and,
// capped, a min-width floor in `em` (below).
//
// A pure percent-of-total-weight split (the old approach: width = weight /
// sum(weights) * 100%) ties a column's actual pixel width to every OTHER
// column's content — a short label column (e.g. 4 CJK chars, weight 6)
// sitting next to a column with a long sentence (weight 20+) gets squeezed
// to a sliver of its "share" once the heavy column inflates the total,
// often below the width its own 4 characters need to avoid wrapping one
// character per line. flex-grow + min-width fixes this the way a real
// <table> would: every column gets at least enough room for its own
// content first, and only the space left over after that gets divided by
// weight.
function tableColumnWeights(tableNode, doc) {
  const weights = [];
  const rows = [];
  const header = tableNode.getChild('TableHeader');
  if (header) rows.push(header);
  for (const row of tableNode.getChildren('TableRow')) rows.push(row);
  for (const row of rows) {
    let colIndex = -1;
    const cur = row.cursor();
    if (cur.firstChild()) {
      do {
        if (cur.name === 'TableDelimiter') {
          colIndex++;
        } else if (cur.name === 'TableCell') {
          const len = Math.max(...doc.sliceString(cur.from, cur.to).split(/<br\s*\/?>/i).map((s) => s.length));
          if (weights[colIndex] === undefined || len > weights[colIndex]) weights[colIndex] = len;
        }
      } while (cur.nextSibling());
    }
  }
  return weights.map((w) => Math.max((w || 0) + 2, 4));
}

// Capped at 6em: enough for a short label (~4-6 CJK glyphs, which are ~1em
// wide each) to never wrap character-by-character, without also handing a
// genuinely long column (weight 20+) a floor so big it starves everyone
// else before flex-grow even gets a say.
function tableColumnMinEm(weight) {
  return Math.min(weight, 6);
}

// Cache per table — rebuilds on selection change would recompute every row.
function getTableLayout(tableNode, doc, cache) {
  if (!cache) return { alignments: tableAlignments(tableNode, doc), weights: tableColumnWeights(tableNode, doc) };
  let layout = cache.get(tableNode.from);
  if (!layout) {
    layout = { alignments: tableAlignments(tableNode, doc), weights: tableColumnWeights(tableNode, doc) };
    cache.set(tableNode.from, layout);
  }
  return layout;
}

// Hairline via inline-block widget + shrunk line font-size (shrinks CM6
// widgetBuffers too). Plain hideRange / block widget leave a tall gap. Only
// typing reaches the row raw: caret motion steps over it (isHairlineLine).
export function decorateTableDelimiterRow(node, view, decos) {
  if (!node.node.parent || node.node.parent.name !== 'Table') return;
  const state = view.state;
  const line = state.doc.lineAt(node.from);
  if (caretOnLine(state, line)) {
    decos.push(Decoration.line({ class: 'cm-lp-table-delim-raw' }).range(line.from));
    return;
  }
  decos.push(Decoration.line({ class: 'cm-lp-table-row cm-lp-table-delim' }).range(line.from));
  decos.push(Decoration.replace({ widget: new TableDelimiterWidget() }).range(line.from, line.to));
}

function decorateTableRow(isHeader) {
  return (node, view, decos, options, tableCache) => {
    const state = view.state;
    const doc = state.doc;
    const line = doc.lineAt(node.from);
    const tableNode = node.node.parent;
    const last = tableNode && tableNode.lastChild;
    const isLastRow = !isHeader && !!last && last.from === node.from && last.to === node.to;

    const rowClasses = ['cm-lp-table-row'];
    if (isHeader) rowClasses.push('cm-lp-table-row-header');
    if (isLastRow) rowClasses.push('cm-lp-table-row-last');
    decos.push(Decoration.line({ class: rowClasses.join(' ') }).range(line.from));

    const layout = tableNode ? getTableLayout(tableNode, doc, tableCache) : { alignments: [], weights: [] };
    const { alignments, weights } = layout;
    const cells = rowCells(state, node.node);

    // Column count from header alignments — trailing empty cells still need boxes.
    const colCount = Math.max(alignments.length, weights.length, cells.length);
    let total = 0;
    for (let col = 0; col < colCount; col++) total += weights[col] || 4;
    hideRange(line.from, node.from, decos);
    let pos = node.from;
    for (let col = 0; col < colCount; col++) {
      const cls = ['cm-lp-table-cell'];
      if (col === 0) cls.push('cm-lp-table-cell-first');
      if (col === colCount - 1) cls.push('cm-lp-table-cell-last');
      const align = alignments[col];
      const weight = weights[col] || 4;
      // Every column first gets its own floor, then the leftover width goes by
      // weight; the floor gives way to the weighted share so the table always fits.
      const styleParts = ['flex:' + weight + ' 1 0%', `min-width:min(${tableColumnMinEm(weight)}em,${(100 * weight) / total}%)`];
      if (align) styleParts.push('text-align:' + align);
      const style = styleParts.join(';');
      const cell = cells[col];
      if (!cell) {
        decos.push(Decoration.widget({ widget: new TableEmptyCellWidget(cls.join(' '), style), side: 1 }).range(line.to));
        continue;
      }
      // The cell's box owns its pipes and padding too, so no stray flex item sits
      // between cells and a caret at a hidden edge is still drawn in its cell.
      const end = col === cells.length - 1 ? line.to : cell.to;
      const text = cellText(state, cell);
      hideRange(pos, text.from, decos);
      hideRange(text.to, end, decos);
      decos.push(Decoration.mark({ class: cls.join(' '), attributes: { style }, inclusive: true }).range(pos, end));
      pos = end;
    }
  };
}

export const decorateTableHeader = decorateTableRow(true);
export const decorateTableBodyRow = decorateTableRow(false);

export function decorateCellBreak(node, view, decos) {
  decos.push(Decoration.replace({ widget: new TableBreakWidget() }).range(node.from, node.to));
}
