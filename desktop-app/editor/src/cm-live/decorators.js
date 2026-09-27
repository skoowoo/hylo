// Node-name → decorator. live-preview.js walks the tree and dispatches here.
// hideRange collapses markers when caret leaves; styleRange is always-on.
import { Decoration } from '@codemirror/view';
import { selectionTouchesRange, selectionTouchesLine, selectionInsideRange } from './selection.js';
import { isRevealSuppressed } from './link-reveal-suppression.js';
import {
  WikiLinkWidget,
  WikiImageWidget,
  TaskCheckboxWidget,
  MarkdownImageWidget,
  BulletWidget,
  TableEmptyCellWidget,
  TableDelimiterWidget,
  FrontmatterCollapsedLineWidget,
  FrontmatterMoreWidget,
  FrontmatterKeyIconWidget,
  FrontmatterListIndentWidget,
} from './widgets.js';
import { wikiNodeInner, splitWikiLinkInner } from './wiki-syntax.js';
import { frontmatterCollapseField } from './frontmatter-collapse.js';

function hideRange(from, to, decos) {
  if (from >= to) return;
  decos.push(Decoration.replace({}).range(from, to));
}

function styleRange(from, to, className, decos) {
  if (from >= to) return;
  decos.push(Decoration.mark({ class: className }).range(from, to));
}

// ── Headings ──────────────────────────────────────────────────────────────
// Inline padding overrides (not classes): depend on line position, not CSS
// selectors. Use padding not margin — CM6 height measurement ignores margin.
// h2-only first-line inset matches old note_editor_prose.css (commit 980af62).
const HEADING_FIRST_LINE_PADDING_TOP = { 1: '0', 2: '1em', 3: '0', 4: '0', 5: '0', 6: '0' };
const HEADING_AFTER_HR_PADDING_TOP = { 2: '0.5em', 3: '0.5em', 4: '0.4em', 5: '0.4em', 6: '0.4em' };

// Blank line above: residual after subtracting blank-line height (~1.75em body),
// not flat 0 — flat 0 erased h1/h2 hierarchy. h3–h6 already ≤ blank height → 0.
const HEADING_BLANK_ABOVE_PADDING_TOP = { 1: '0.5em', 2: '0.48em', 3: '0', 4: '0', 5: '0', 6: '0' };

const HR_LINE_TEXT_RE = /^(?:-{3,}|\*{3,}|_{3,})$/;

// Walk back past blank lines — CM6 keeps them; ProseMirror did not.
function isAfterHorizontalRule(doc, lineNumber) {
  for (let n = lineNumber - 1; n >= 1; n--) {
    const text = doc.line(n).text.trim();
    if (text === '') continue;
    return HR_LINE_TEXT_RE.test(text);
  }
  return false;
}

// Adjacent blank .cm-line already adds ~1.75em; zero padding to avoid stacking.
function isBlankLine(doc, lineNumber) {
  return lineNumber >= 1 && lineNumber <= doc.lines && doc.line(lineNumber).text.trim() === '';
}

function decorateHeading(level) {
  return (node, view, decos) => {
    const doc = view.state.doc;
    const line = doc.lineAt(node.from);
    const lineSpec = { class: 'cm-lp-heading cm-lp-h' + level };
    // Blank-above first: AFTER_HR must not stack on blank-line height.
    const paddingTop =
      line.number === 1
        ? HEADING_FIRST_LINE_PADDING_TOP[level]
        : isBlankLine(doc, line.number - 1)
          ? HEADING_BLANK_ABOVE_PADDING_TOP[level]
          : isAfterHorizontalRule(doc, line.number)
            ? HEADING_AFTER_HR_PADDING_TOP[level]
            : null;
    const paddingBottom = isBlankLine(doc, line.number + 1) ? '0' : null;
    // !important: theme.js base padding is !important too.
    const styleDecls = [];
    if (paddingTop) styleDecls.push('padding-top:' + paddingTop + ' !important');
    if (paddingBottom) styleDecls.push('padding-bottom:' + paddingBottom + ' !important');
    if (styleDecls.length) lineSpec.attributes = { style: styleDecls.join(';') };
    decos.push(Decoration.line(lineSpec).range(line.from));
    const mark = node.node.getChild('HeaderMark');
    if (!mark) return;
    if (selectionTouchesLine(view.state, line)) {
      // Caret on the line reveals the "#"s — style them muted (cm-lp-heading-mark)
      // instead of inheriting the heading's own color, so they read as syntax.
      styleRange(mark.from, mark.to, 'cm-lp-heading-mark', decos);
      return;
    }
    // Fold trailing space after "#"s — not part of HeaderMark.
    const extra =
      mark.to < line.to && view.state.doc.sliceString(mark.to, mark.to + 1) === ' ' ? 1 : 0;
    hideRange(mark.from, mark.to + extra, decos);
  };
}

// ── Bold / italic / strikethrough

function decorateWrappedMark(markName, className) {
  return (node, view, decos) => {
    const marks = node.node.getChildren(markName);
    if (marks.length < 2) return;
    const first = marks[0];
    const last = marks[marks.length - 1];
    styleRange(first.to, last.from, className, decos);
    if (selectionTouchesRange(view.state, node.from, node.to)) return;
    hideRange(first.from, first.to, decos);
    hideRange(last.from, last.to, decos);
  };
}

const decorateStrong = decorateWrappedMark('EmphasisMark', 'cm-lp-strong');
const decorateEmphasis = decorateWrappedMark('EmphasisMark', 'cm-lp-em');
const decorateStrikethrough = decorateWrappedMark('StrikethroughMark', 'cm-lp-strike');
const decorateInlineCode = decorateWrappedMark('CodeMark', 'cm-lp-code');

// ── Blockquote — QuoteMark lives on leaf lines, not as Blockquote children.
// Depth + skip nested child ranges so each line gets one decoration with
// correct nesting class (cm-lp-quote-dN), capped at 4.

function decorateBlockquote(node, view, decos) {
  const doc = view.state.doc;
  const bqNode = node.node;
  const fromLine = doc.lineAt(bqNode.from).number;
  const toLine = doc.lineAt(Math.max(bqNode.from, bqNode.to - 1)).number;
  let depth = 0;
  let outer = bqNode;
  for (let p = bqNode; p; p = p.parent) if (p.name === 'Blockquote') { depth++; outer = p; }
  // Zero padding only on outermost quote's true first/last line.
  const outerFromLine = doc.lineAt(outer.from).number;
  const outerToLine = doc.lineAt(Math.max(outer.from, outer.to - 1)).number;
  const childRanges = bqNode.getChildren('Blockquote').map((c) => [
    doc.lineAt(c.from).number,
    doc.lineAt(Math.max(c.from, c.to - 1)).number,
  ]);
  const cls = depth > 1 ? 'cm-lp-quote cm-lp-quote-d' + Math.min(depth, 4) : 'cm-lp-quote';
  for (let n = fromLine; n <= toLine; n++) {
    if (childRanges.some(([a, b]) => n >= a && n <= b)) continue;
    const lineSpec = { class: cls };
    const styleDecls = [];
    if (n === outerFromLine && isBlankLine(doc, n - 1)) styleDecls.push('padding-top:0 !important');
    if (n === outerToLine && isBlankLine(doc, n + 1)) styleDecls.push('padding-bottom:0 !important');
    if (styleDecls.length) lineSpec.attributes = { style: styleDecls.join(';') };
    decos.push(Decoration.line(lineSpec).range(doc.line(n).from));
  }
}

function decorateQuoteMark(node, view, decos) {
  const doc = view.state.doc;
  const markLine = doc.lineAt(node.from);
  if (selectionTouchesLine(view.state, markLine)) return;
  // Fold space after ">" — not part of QuoteMark.
  const extra =
    node.to < markLine.to && doc.sliceString(node.to, node.to + 1) === ' ' ? 1 : 0;
  hideRange(node.from, node.to + extra, decos);
}

// ── [text](url)

function decorateLink(node, view, decos) {
  const marks = node.node.getChildren('LinkMark');
  const urlNode = node.node.getChild('URL');
  if (marks.length < 2 || !urlNode) return;
  const textFrom = marks[0].to;
  const textTo = marks[1].from;
  styleRange(textFrom, textTo, 'cm-lp-link', decos);
  // isRevealSuppressed: avoid one-frame raw flash on click-to-open.
  if (selectionTouchesRange(view.state, node.from, node.to) && !isRevealSuppressed(view.state, node.from, node.to)) return;
  hideRange(node.from, marks[0].to, decos);
  hideRange(textTo, node.to, decos);
}

// ── Autolinks — URL styles text; Autolink hides < >.

function decorateUrl(node, view, decos) {
  styleRange(node.from, node.to, 'cm-lp-link', decos);
}

function decorateAutolinkBrackets(node, view, decos) {
  const marks = node.node.getChildren('LinkMark');
  if (marks.length < 2) return;
  if (selectionTouchesRange(view.state, node.from, node.to) && !isRevealSuppressed(view.state, node.from, node.to)) return;
  hideRange(marks[0].from, marks[0].to, decos);
  hideRange(marks[1].from, marks[1].to, decos);
}

// ── ![alt](url) — selectionInsideRange like wikiimage (atomic boundary).

function decorateImage(node, view, decos, atomics) {
  const marks = node.node.getChildren('LinkMark');
  const urlNode = node.node.getChild('URL');
  if (marks.length < 2 || !urlNode) return;
  const url = view.state.doc.sliceString(urlNode.from, urlNode.to);
  const alt = view.state.doc.sliceString(marks[0].to, marks[1].from);
  if (selectionInsideRange(view.state, node.from, node.to)) {
    styleRange(node.from, node.to, 'cm-lp-wikiimage-raw', decos);
    return;
  }
  const range = Decoration.replace({ widget: new MarkdownImageWidget(url, alt) }).range(node.from, node.to);
  decos.push(range);
  atomics.push(range);
}

// ── List markers: bullet → •; ordered keep number; task hide mark.
// Hanging indent via padding-left/text-indent on the item's first line only.

// Uniform per-level step, in ch — same value for bullet/ordered/task, matching
// how mainstream editors (Notion, Typora, Obsidian Live Preview) indent lists:
// one consistent step per depth regardless of marker kind, not a step sized to
// each marker's own text width (that made bullet vs. ordered nesting look
// inconsistent next to each other).
const LIST_STEP_CH = 2;

// Is `lineNumber` the start of some ListItem in the same document tree as
// `node`? Walks the tree root via .parent instead of importing syntaxTree()
// — node is already a live SyntaxNodeRef in that same tree.
function lineIsListItemLine(node, doc, lineNumber) {
  if (lineNumber < 1 || lineNumber > doc.lines) return false;
  let root = node.node;
  while (root.parent) root = root.parent;
  for (let n = root.resolve(doc.line(lineNumber).from, 1); n; n = n.parent) {
    if (n.name === 'ListItem') return true;
  }
  return false;
}

function decorateListMark(node, view, decos) {
  const doc = view.state.doc;
  const listItem = node.node.parent;
  const list = listItem && listItem.parent;
  const task = listItem && listItem.getChild('Task');
  const line = doc.lineAt(node.from);

  let depth = 0;
  for (let p = list; p; p = p.parent) if (p.name === 'BulletList' || p.name === 'OrderedList') depth++;

  // The raw indentation spaces that encode nesting in the source (2, 4,
  // tabs, whatever) would otherwise double up with the step below — hide
  // them so only LIST_STEP_CH controls the visual position.
  if (node.from > line.from) hideRange(line.from, node.from, decos);

  if (task) {
    hideRange(node.from, node.to, decos);
  } else if (list && list.name === 'BulletList') {
    decos.push(Decoration.replace({ widget: new BulletWidget() }).range(node.from, node.to));
  } else {
    styleRange(node.from, node.to, 'cm-lp-list-mark-ol', decos);
  }

  const indent = depth * LIST_STEP_CH;
  const lineSpec = { class: 'cm-lp-list-line' };
  const styleDecls = [];
  if (indent > 0) styleDecls.push('padding-left:' + indent + 'ch', 'text-indent:-' + LIST_STEP_CH + 'ch');
  // Don't stack item padding on adjacent blank-line height.
  if (isBlankLine(doc, line.number - 1)) styleDecls.push('padding-top:0 !important');
  if (isBlankLine(doc, line.number + 1)) styleDecls.push('padding-bottom:0 !important');
  if (styleDecls.length) lineSpec.attributes = { style: styleDecls.join(';') };
  decos.push(Decoration.line(lineSpec).range(line.from));

  // A "loose list" blank line directly between two items of this same list
  // is still conceptually one gap, not a deliberate extra paragraph break —
  // without this it renders at the default line height (a full text line),
  // which dwarfs cm-lp-list-line's own padding and makes list-item spacing
  // look untouched however tight that padding is set. Collapsed the same
  // hairline way frontmatter/table rows already do (checked from the item
  // above only, so a blank line between two items isn't decorated twice).
  if (isBlankLine(doc, line.number + 1) && lineIsListItemLine(node, doc, line.number + 2)) {
    const gap = doc.line(line.number + 1);
    decos.push(Decoration.line({ class: 'cm-lp-list-gap' }).range(gap.from));
    // 1px line class would leave a short ASCII run for CM's text-size probe.
    // textHeight 1 makes vertical motion's step (>> 1) zero, so the scan never ends.
    hideRange(gap.from, gap.to, decos);
  }
}

// HorizontalRule → horizontal-rule-field.js (needs block:true StateField).

// ── Fenced code — card chrome only when closed (≥2 CodeMarks); hide fences
// when caret leaves the whole block (not per-line).

function decorateFencedCode(node, view, decos) {
  const doc = view.state.doc;
  const fromLine = doc.lineAt(node.from).number;
  const toLine = doc.lineAt(Math.max(node.from, node.to - 1)).number;
  const marks = node.node.getChildren('CodeMark');
  // Unclosed fence extends to EOF — don't show card until truly closed.
  const info = node.node.getChild('CodeInfo');
  const editingBlock = selectionTouchesRange(view.state, node.from, node.to);
  if (marks.length >= 2) {
    for (let n = fromLine; n <= toLine; n++) {
      const classes = ['cm-lp-codeblock'];
      if (n === fromLine) classes.push('cm-lp-codeblock-first');
      if (n === toLine) classes.push('cm-lp-codeblock-last');
      // Once the fence marker is hidden, this line is empty dead space —
      // collapse it so codeblock-first/last's own padding is the only thing
      // controlling the gap, not this line's own font line-height stacking
      // on top of it too. A language label on the opening line doesn't need
      // it held open either: cm-lp-code-lang positions itself as a corner
      // badge instead of sitting in the line's normal text flow.
      if (!editingBlock && (n === fromLine || n === toLine)) {
        classes.push('cm-lp-codeblock-marker-line');
      }
      decos.push(Decoration.line({ class: classes.join(' ') }).range(doc.line(n).from));
    }
  }

  for (const mark of marks) {
    if (editingBlock) continue;
    hideRange(mark.from, mark.to, decos);
  }
  if (info) styleRange(info.from, info.to, 'cm-lp-code-lang', decos);
}

// ── GFM task marker

function decorateTaskMarker(node, view, decos) {
  const raw = view.state.doc.sliceString(node.from, node.to);
  const checked = /\[[xX]\]/.test(raw);
  decos.push(Decoration.replace({ widget: new TaskCheckboxWidget(checked, node.from) }).range(node.from, node.to));
}

// ── Wikilink / wikiimage — selectionInsideRange (strict) for atomicRanges.

function decorateWikiLink(node, view, decos, atomics, options) {
  const inner = wikiNodeInner(node.node, view.state.doc);
  const { target, alias } = splitWikiLinkInner(inner);
  if (!target) return;
  if (selectionInsideRange(view.state, node.from, node.to)) {
    styleRange(node.from, node.to, 'cm-lp-wikilink-raw', decos);
    return;
  }
  const broken = !!(options && options.isWikiLinkBroken && options.isWikiLinkBroken(target));
  const range = Decoration.replace({
    widget: new WikiLinkWidget(target, alias, options && options.onWikiLinkClick, broken),
  }).range(node.from, node.to);
  decos.push(range);
  atomics.push(range);
}

function decorateWikiImage(node, view, decos, atomics, options) {
  const filename = wikiNodeInner(node.node, view.state.doc);
  if (!filename) return;
  if (selectionInsideRange(view.state, node.from, node.to)) {
    styleRange(node.from, node.to, 'cm-lp-wikiimage-raw', decos);
    return;
  }
  const range = Decoration.replace({
    widget: new WikiImageWidget(filename, options && options.resolveImageSrc),
  }).range(node.from, node.to);
  decos.push(range);
  atomics.push(range);
}

// ── GFM tables — per-line decorations; cells are inline-block + % width
// (shared across rows). No block widget / contenteditable.

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
          const len = doc.sliceString(cur.from, cur.to).length;
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
// widgetBuffers too). Plain hideRange / block widget leave a tall gap.
function decorateTableDelimiterRow(node, view, decos) {
  if (!node.node.parent || node.node.parent.name !== 'Table') return;
  const state = view.state;
  const line = state.doc.lineAt(node.from);
  if (selectionTouchesLine(state, line)) {
    decos.push(Decoration.line({ class: 'cm-lp-table-row' }).range(line.from));
    return;
  }
  decos.push(Decoration.line({ class: 'cm-lp-table-row cm-lp-table-delim' }).range(line.from));
  decos.push(Decoration.replace({ widget: new TableDelimiterWidget() }).range(line.from, line.to));
}

function decorateTableRow(isHeader) {
  return (node, view, decos, atomics, options, tableCache) => {
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

    if (selectionTouchesLine(state, line)) return;

    const layout = tableNode ? getTableLayout(tableNode, doc, tableCache) : { alignments: [], weights: [] };
    const alignments = layout.alignments;
    const weights = layout.weights;

    const colCells = [];
    let colIndex = -1;
    const cur = node.node.cursor();
    if (cur.firstChild()) {
      do {
        if (cur.name === 'TableDelimiter') {
          colIndex++;
        } else if (cur.name === 'TableCell') {
          colCells[colIndex] = { from: cur.from, to: cur.to };
        }
      } while (cur.nextSibling());
    }

    // Column count from header alignments — trailing empty cells still need boxes.
    const colCount = Math.max(alignments.length, weights.length, colCells.length);
    let pos = line.from;
    for (let col = 0; col < colCount; col++) {
      const cls = ['cm-lp-table-cell'];
      if (col === 0) cls.push('cm-lp-table-cell-first');
      if (col === colCount - 1) cls.push('cm-lp-table-cell-last');
      const align = alignments[col];
      const weight = weights[col] || 4;
      // flex-grow:weight, flex-shrink:1, flex-basis:0% — divide the row's
      // width by weight ratio same as before, but only after every column
      // has already claimed its own min-width floor (tableColumnMinEm), so
      // a short column can never be squeezed narrower than its own content
      // needs just because a sibling column is much longer.
      const styleParts = ['flex:' + weight + ' 1 0%', 'min-width:' + tableColumnMinEm(weight) + 'em'];
      if (align) styleParts.push('text-align:' + align);
      const style = styleParts.join(';');
      const cell = colCells[col];
      if (cell) {
        hideRange(pos, cell.from, decos);
        decos.push(Decoration.mark({ class: cls.join(' '), attributes: { style } }).range(cell.from, cell.to));
        pos = cell.to;
      } else {
        decos.push(Decoration.widget({ widget: new TableEmptyCellWidget(cls.join(' '), style), side: 1 }).range(pos));
      }
    }
    hideRange(pos, line.to, decos);
  };
}

const decorateTableHeader = decorateTableRow(true);
const decorateTableBodyRow = decorateTableRow(false);

// ── Frontmatter — in-place line decorations (no card widget). Regex only
// for flat key:value / flow arrays / indented continuations.

const FM_KEY_LINE_RE = /^([A-Za-z0-9_.-]+)(:)(\s*)([\s\S]*)$/;
const FM_URL_RE = /^https?:\/\//i;
const FM_CONTINUATION_RE = /^[ \t]/;
const FM_NUMBER_RE = /^-?\d+(\.\d+)?$/;
// Arrays/lists over this many items collapse behind "+N more" (whole
// frontmatter block always renders in full now — only long array/list
// values get truncated, so a note with lots of tags stays scannable).
const FM_ARRAY_VISIBLE_ITEMS = 3;

// One group per top-level field (continuations fold in) so array/list groups
// can be told apart from plain scalar fields for icon choice + truncation.
function fmFieldGroups(doc, fromLineNo, toLineNo) {
  const groups = [];
  let current = null;
  for (let n = fromLineNo; n <= toLineNo; n++) {
    const text = doc.line(n).text;
    // A blank line (some YAML list authors leave one between "- item"
    // entries) folds into whatever group is open instead of starting a new
    // one-line "field" — otherwise it renders as a phantom full-height row
    // with a stray key icon and nothing next to it.
    const isBlank = text.trim().length === 0;
    if (current && (isBlank || FM_CONTINUATION_RE.test(text))) {
      current.toNo = n;
    } else {
      current = { fromNo: n, toNo: n };
      groups.push(current);
    }
  }
  return groups;
}

function decorateFmArrayValue(rawValue, valueFrom, valueTo, decos, state) {
  const m = /^\[([\s\S]*)\]\s*$/.exec(rawValue);
  if (!m) return false;
  const closeIdx = rawValue.lastIndexOf(']');
  const inner = rawValue.slice(1, closeIdx);
  hideRange(valueFrom, valueFrom + 1, decos);
  const parts = inner.split(',');
  // First pass: item ranges only, so overflow/reveal can be decided before
  // any decoration is pushed.
  const items = [];
  let pos = valueFrom + 1;
  for (let i = 0; i < parts.length; i++) {
    const raw = parts[i];
    const trimmed = raw.trim();
    const leadingWs = raw.length - raw.trimStart().length;
    const itemFrom = pos + leadingWs;
    const itemTo = itemFrom + trimmed.length;
    const segEnd = pos + raw.length;
    items.push({ itemFrom, itemTo });
    pos = segEnd + (i < parts.length - 1 ? 1 : 0); // +1 for the comma
  }
  const overflow = items.length > FM_ARRAY_VISIBLE_ITEMS;
  const revealed = overflow && selectionTouchesRange(state, items[FM_ARRAY_VISIBLE_ITEMS].itemFrom, valueTo);
  const visibleCount = overflow && !revealed ? FM_ARRAY_VISIBLE_ITEMS : items.length;

  pos = valueFrom + 1;
  for (let i = 0; i < visibleCount; i++) {
    const it = items[i];
    hideRange(pos, it.itemFrom, decos);
    if (it.itemTo > it.itemFrom) styleRange(it.itemFrom, it.itemTo, 'cm-lp-fm-tag', decos);
    pos = it.itemTo;
  }
  if (overflow && !revealed) {
    decos.push(
      Decoration.widget({
        widget: new FrontmatterMoreWidget(items.length - visibleCount, items[visibleCount].itemFrom),
        side: 1,
      }).range(pos)
    );
  }
  hideRange(pos, valueTo, decos);
  return true;
}

// ×1.15: letter-spacing + uppercase widen beyond plain ch count.
function frontmatterLabelWidthCh(doc, fromLineNo, toLineNo) {
  let maxLen = 0;
  for (let n = fromLineNo; n <= toLineNo; n++) {
    const m = FM_KEY_LINE_RE.exec(doc.line(n).text);
    if (m) maxLen = Math.max(maxLen, m[1].length + m[2].length);
  }
  return Math.ceil(maxLen * 1.15) + 1;
}

// System-defined keys get a fixed icon regardless of value shape — value-type
// icons (tag/list/number/text) below are the fallback for everything else.
const FM_KEY_ICON_OVERRIDES = {
  title: 'heading',
  kind: 'layers',
  tags: 'tag',
  author: 'user',
  source: 'link',
  source_notes: 'link',
  clipped: 'clock',
  clipped_at: 'clock',
  created_at: 'clock',
  last_compiled_at: 'clock',
};

// Type drives which icon renders next to the key — 'tags' is special-cased
// (Obsidian does the same), other arrays/lists get the generic list icon.
function fmFieldType(key, rawValue, isArrayGroup) {
  const override = FM_KEY_ICON_OVERRIDES[key.trim().toLowerCase()];
  if (override) return override;
  if (isArrayGroup) return key.trim().toLowerCase() === 'tags' ? 'tag' : 'list';
  return FM_NUMBER_RE.test(rawValue.trim()) ? 'number' : 'text';
}

function decorateFmFieldLine(text, lineFrom, decos, labelWidthCh, type, state) {
  const m = FM_KEY_LINE_RE.exec(text);
  if (!m) {
    decos.push(Decoration.widget({ widget: new FrontmatterKeyIconWidget(type || 'text'), side: -1 }).range(lineFrom));
    if (text.trim().length > 0) styleRange(lineFrom, lineFrom + text.length, 'cm-lp-fm-val', decos);
    return;
  }
  const [, key, colon, gap, rawValue] = m;
  const keyTo = lineFrom + key.length;
  const colonTo = keyTo + colon.length;
  decos.push(Decoration.widget({ widget: new FrontmatterKeyIconWidget(type || 'text'), side: -1 }).range(lineFrom));
  decos.push(
    Decoration.mark({ class: 'cm-lp-fm-label', attributes: { style: 'width:' + labelWidthCh + 'ch' } }).range(
      lineFrom,
      colonTo
    )
  );
  styleRange(lineFrom, keyTo, 'cm-lp-fm-key', decos);
  styleRange(keyTo, colonTo, 'cm-lp-fm-colon', decos);
  const valueFrom = colonTo + gap.length;
  const valueTo = lineFrom + text.length;
  if (valueFrom >= valueTo) return;
  if (decorateFmArrayValue(rawValue, valueFrom, valueTo, decos, state)) return;
  const cls = FM_URL_RE.test(rawValue.trim()) ? 'cm-lp-fm-val cm-lp-fm-link' : 'cm-lp-fm-val';
  styleRange(valueFrom, valueTo, cls, decos);
}

function decorateFmContinuationLine(text, lineFrom, decos, labelWidthCh) {
  if (text.trim().length === 0) return;
  const m = /^(\s*)(-\s?)?([\s\S]*)$/.exec(text);
  const indent = m[1];
  const bullet = m[2] || '';
  const pos = lineFrom + indent.length + bullet.length;
  // Replace the raw indent+"- " with an invisible icon+label-shaped spacer
  // instead of hiding it and padding-left-ing the line to compensate — the
  // real label renders at a smaller font-size than the line itself, so a
  // ch-based calc() on the line's padding (sized in the line's own font)
  // can't reproduce the label's actual (smaller-font) width. Reusing the
  // identical icon/label markup, just invisible, measures itself the same
  // way the real one does and lines up with it pixel-for-pixel.
  decos.push(Decoration.replace({ widget: new FrontmatterListIndentWidget(labelWidthCh) }).range(lineFrom, pos));
  const textEnd = lineFrom + text.length;
  if (pos < textEnd) styleRange(pos, textEnd, 'cm-lp-fm-val cm-lp-fm-list-item', decos);
}

function decorateFmDelimiterLine(line, decos) {
  decos.push(Decoration.line({ class: 'cm-lp-fm-line cm-lp-fm-collapsed' }).range(line.from));
  if (line.length > 0) {
    decos.push(Decoration.replace({ widget: new FrontmatterCollapsedLineWidget() }).range(line.from, line.to));
  }
}

function decorateFrontmatter(node, view, decos) {
  const state = view.state;
  const doc = state.doc;
  const fmFrom = node.from;
  const fmTo = node.to;
  const firstLine = doc.lineAt(fmFrom);
  const lastLine = doc.lineAt(fmTo);

  if (state.field(frontmatterCollapseField, false)) {
    // Whole block collapsed via the "Metadata" header toggle — hairline
    // every line individually rather than one multi-line replace (that's
    // the known CM6 height-map crash this file works around everywhere
    // else too; see fmFieldGroups' blank-line handling above).
    for (let n = firstLine.number; n <= lastLine.number; n++) {
      const line = doc.line(n);
      decos.push(Decoration.line({ class: 'cm-lp-fm-line cm-lp-fm-collapsed' }).range(line.from));
      if (line.length > 0) {
        decos.push(Decoration.replace({ widget: new FrontmatterCollapsedLineWidget() }).range(line.from, line.to));
      } else {
        decos.push(Decoration.widget({ widget: new FrontmatterCollapsedLineWidget(), side: 1 }).range(line.from));
      }
    }
    return;
  }

  // Unclosed → no closing "---"; rest is interior.
  const hasClosingDelimiter = lastLine.number > firstLine.number && doc.line(lastLine.number).text.trim() === '---';
  const interiorFromNo = firstLine.number + 1;
  const interiorToNo = hasClosingDelimiter ? lastLine.number - 1 : lastLine.number;
  const hasInterior = interiorToNo >= interiorFromNo;
  const groups = hasInterior ? fmFieldGroups(doc, interiorFromNo, interiorToNo) : [];
  const labelWidthCh = hasInterior ? frontmatterLabelWidthCh(doc, interiorFromNo, interiorToNo) : 0;

  // Frontmatter is read-only in this view now (frontmatter-readonly.js) —
  // always the decorated/collapsed form, never raw syntax on caret entry;
  // editing happens through the header's edit button (an app-level dialog).
  decos.push(Decoration.line({ class: 'cm-lp-fm-line cm-lp-fm-first cm-lp-fm-collapsed' }).range(firstLine.from));
  decos.push(Decoration.replace({ widget: new FrontmatterCollapsedLineWidget() }).range(firstLine.from, firstLine.to));

  groups.forEach((group) => {
    const keyLineText = doc.line(group.fromNo).text;

    // A blank line with nothing open before it (rare — e.g. right after the
    // opening "---") ends up as its own one-line "group"; render it as a
    // hairline instead of a field row so it doesn't get a stray key icon.
    if (keyLineText.trim().length === 0) {
      const blankLine = doc.line(group.fromNo);
      const classes = ['cm-lp-fm-line', 'cm-lp-fm-collapsed'];
      if (!hasClosingDelimiter && group.fromNo === interiorToNo) classes.push('cm-lp-fm-last');
      decos.push(Decoration.line({ class: classes.join(' ') }).range(blankLine.from));
      // A truly empty line has nothing for CM6 to measure against, so its
      // height falls back to a full line instead of the shrunk hairline —
      // same widgetBuffer anchor the non-empty collapse branches use below.
      decos.push(Decoration.widget({ widget: new FrontmatterCollapsedLineWidget(), side: 1 }).range(blankLine.from));
      return;
    }

    const keyMatch = FM_KEY_LINE_RE.exec(keyLineText);
    const isMultilineList = group.toNo > group.fromNo;
    const isFlowArray = keyMatch ? /^\[[\s\S]*\]\s*$/.test(keyMatch[4].trim()) : false;
    const isArrayGroup = isMultilineList || isFlowArray;
    const type = keyMatch ? fmFieldType(keyMatch[1], keyMatch[4], isArrayGroup) : 'text';

    // Key line (always rendered — no more whole-block collapsing).
    const keyLine = doc.line(group.fromNo);
    const keyLineClasses = ['cm-lp-fm-line'];
    if (!hasClosingDelimiter && group.fromNo === interiorToNo) keyLineClasses.push('cm-lp-fm-last');
    decos.push(Decoration.line({ class: keyLineClasses.join(' ') }).range(keyLine.from));
    decorateFmFieldLine(keyLine.text, keyLine.from, decos, labelWidthCh, type, state);

    if (!isMultilineList) return;

    // Block-list items (indented "- item" continuation lines) — collapse
    // past FM_ARRAY_VISIBLE_ITEMS, same "+N more" affordance as flow arrays.
    // Blank lines a list author left between entries render as a hairline
    // (allLineNos) but don't count toward the visible-item cutoff (itemLineNos).
    const allLineNos = [];
    for (let n = group.fromNo + 1; n <= group.toNo; n++) allLineNos.push(n);
    const itemLineNos = allLineNos.filter((n) => doc.line(n).text.trim().length > 0);
    const overflow = itemLineNos.length > FM_ARRAY_VISIBLE_ITEMS;
    const revealed =
      overflow && selectionTouchesRange(state, doc.line(itemLineNos[FM_ARRAY_VISIBLE_ITEMS]).from, doc.line(group.toNo).to);
    const visibleCount = overflow && !revealed ? FM_ARRAY_VISIBLE_ITEMS : itemLineNos.length;

    allLineNos.forEach((n) => {
      const line = doc.line(n);
      const isLastRenderedLine = !hasClosingDelimiter && n === interiorToNo;
      const lineClasses = ['cm-lp-fm-line'];
      if (isLastRenderedLine) lineClasses.push('cm-lp-fm-last');
      const isBlank = line.text.trim().length === 0;
      const idx = isBlank ? -1 : itemLineNos.indexOf(n);

      // First hidden item's own line carries "+N more" as a normal,
      // left-aligned row (same invisible icon+label indent as a real item)
      // instead of collapsing to a hairline like the rest of the overflow.
      if (overflow && !revealed && idx === visibleCount) {
        decos.push(Decoration.line({ class: lineClasses.join(' ') }).range(line.from));
        const m = /^(\s*)(-\s?)?([\s\S]*)$/.exec(line.text);
        const prefixEnd = line.from + m[1].length + (m[2] || '').length;
        decos.push(
          Decoration.replace({ widget: new FrontmatterListIndentWidget(labelWidthCh) }).range(line.from, prefixEnd)
        );
        decos.push(
          Decoration.replace({
            widget: new FrontmatterMoreWidget(itemLineNos.length - visibleCount, line.from, true),
          }).range(prefixEnd, line.to)
        );
        return;
      }

      if (isBlank || idx >= visibleCount) {
        decos.push(Decoration.line({ class: lineClasses.concat('cm-lp-fm-collapsed').join(' ') }).range(line.from));
        if (line.length > 0) {
          decos.push(Decoration.replace({ widget: new FrontmatterCollapsedLineWidget() }).range(line.from, line.to));
        } else {
          // Zero-length (blank) line — same widgetBuffer anchor, as a point
          // widget instead of a replace since there's no range to consume.
          decos.push(Decoration.widget({ widget: new FrontmatterCollapsedLineWidget(), side: 1 }).range(line.from));
        }
        return;
      }

      decos.push(Decoration.line({ class: lineClasses.join(' ') }).range(line.from));
      decorateFmContinuationLine(line.text, line.from, decos, labelWidthCh);
    });
  });

  if (hasClosingDelimiter) {
    decos.push(Decoration.line({ class: 'cm-lp-fm-line cm-lp-fm-last cm-lp-fm-collapsed' }).range(lastLine.from));
    decos.push(Decoration.replace({ widget: new FrontmatterCollapsedLineWidget() }).range(lastLine.from, lastLine.to));
  }
}

export const nodeDecorators = {
  ATXHeading1: decorateHeading(1),
  ATXHeading2: decorateHeading(2),
  ATXHeading3: decorateHeading(3),
  ATXHeading4: decorateHeading(4),
  ATXHeading5: decorateHeading(5),
  ATXHeading6: decorateHeading(6),
  StrongEmphasis: decorateStrong,
  Emphasis: decorateEmphasis,
  Strikethrough: decorateStrikethrough,
  InlineCode: decorateInlineCode,
  Blockquote: decorateBlockquote,
  QuoteMark: decorateQuoteMark,
  Link: decorateLink,
  Autolink: decorateAutolinkBrackets,
  URL: decorateUrl,
  Image: decorateImage,
  ListMark: decorateListMark,
  TaskMarker: decorateTaskMarker,
  // HorizontalRule → horizontal-rule-field.js
  FencedCode: decorateFencedCode,
  WikiLink: decorateWikiLink,
  WikiImage: decorateWikiImage,
  TableHeader: decorateTableHeader,
  TableRow: decorateTableBodyRow,
  TableDelimiter: decorateTableDelimiterRow,
  Frontmatter: decorateFrontmatter,
};
