// Node-name → decorator. live-preview.js walks the tree and dispatches here.
// hideRange collapses markers when caret leaves; styleRange is always-on.
// GFM tables and frontmatter live in their own files (tables-decorate.js,
// frontmatter-card.js) — this file keeps the smaller inline/list/quote/
// code decorators that don't carry enough independent state to justify one.
import { Decoration } from '@codemirror/view';
import { syntaxTree } from '@codemirror/language';
import { caretTouchesRange, caretOnLine, headInBlock, selectionInsideRange, readingMode } from './selection.js';
import { isRevealSuppressed } from './link-reveal-suppression.js';
import { HangMarkWidget, WikiLinkWidget, TaskCheckboxWidget, ListMarkerWidget, CodeHeaderWidget, HorizontalRuleWidget } from './widgets.js';
import { imageEditing } from './image-field.js';
import { wikiNodeInner, splitWikiLinkInner } from './wiki-syntax.js';
import { hideRange, styleRange, hangRange } from './decoration-helpers.js';
import { isSameNode } from './tree-utils.js';
import { isWidgetRevealed } from './widget-reveal.js';
import { imageFillsLink } from './image-nodes.js';
import { decorateTableHeader, decorateTableBodyRow, decorateTableDelimiterRow } from './tables-decorate.js';
import { decorateFrontmatterSource } from './frontmatter-card.js';
import { cardFence, fenceBody } from './fence-card/registry.js';
import { linkTarget } from './link-refs.js';

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

// textFrom/textTo: line numbers of the heading's text; nextNo: the line after the whole heading.
function pushHeadingLines(doc, level, textFrom, textTo, nextNo, decos) {
  for (let n = textFrom; n <= textTo; n++) {
    const lineSpec = { class: 'cm-lp-heading cm-lp-h' + level };
    // Blank-above first: AFTER_HR must not stack on blank-line height.
    const paddingTop =
      n !== textFrom
        ? '0'
        : n === 1
          ? HEADING_FIRST_LINE_PADDING_TOP[level]
          : isBlankLine(doc, n - 1)
            ? HEADING_BLANK_ABOVE_PADDING_TOP[level]
            : isAfterHorizontalRule(doc, n)
              ? HEADING_AFTER_HR_PADDING_TOP[level]
              : null;
    const paddingBottom = n !== textTo || isBlankLine(doc, nextNo) ? '0' : null;
    // !important: theme.js base padding is !important too.
    const styleDecls = [];
    if (paddingTop) styleDecls.push('padding-top:' + paddingTop + ' !important');
    if (paddingBottom) styleDecls.push('padding-bottom:' + paddingBottom + ' !important');
    if (styleDecls.length) lineSpec.attributes = { style: styleDecls.join(';') };
    decos.push(Decoration.line(lineSpec).range(doc.line(n).from));
  }
}

// A hung marker is a widget until the caret steps into it (← from the text,
// or a click on it); then the source shows in place, still hung, to edit.
function hangMarker(state, from, to, width, className, decos) {
  const editing = state.selection.ranges.some((r) => r.empty && r.head >= from && r.head < to);
  if (editing) {
    hangRange(from, to, width, className, decos);
    return;
  }
  decos.push(Decoration.replace({ widget: new HangMarkWidget(state.doc.sliceString(from, to), className) }).range(from, to));
}

function decorateHeading(level) {
  return (node, view, decos) => {
    const doc = view.state.doc;
    const line = doc.lineAt(node.from);
    const mark = node.node.getChild('HeaderMark');
    // A bare "#" is CommonMark's empty heading, but styling it before the
    // space is typed reads as "# " and turns "#text" into a paragraph.
    if (mark && mark.to === line.to) return;
    pushHeadingLines(doc, level, line.number, line.number, line.number + 1, decos);
    if (!mark) return;
    // Fold trailing space after "#"s — not part of HeaderMark.
    const extra =
      mark.to < line.to && view.state.doc.sliceString(mark.to, mark.to + 1) === ' ' ? 1 : 0;
    if (caretOnLine(view.state, line)) {
      // Caret on the line reveals the "#"s, muted so they read as syntax. At
      // the line start they hang in the margin; nested ("> # x") they stay inline.
      if (mark.from === line.from) {
        hangMarker(view.state, mark.from, mark.to + extra, (mark.to + extra - mark.from) * 0.75 + 'em', 'cm-lp-heading-mark', decos);
      } else {
        styleRange(mark.from, mark.to, 'cm-lp-heading-mark', decos);
      }
      return;
    }
    hideRange(mark.from, mark.to + extra, decos);
  };
}

// "Text\n===" / "Text\n---". The underline is a hairline the caret steps
// over (isHairlineLine). Only typing reaches it, and then it is all left raw:
// a lone "-" typed under a paragraph is already an H2, and styling it would
// flash a heading on the way to "- item".
function decorateSetextHeading(level) {
  return (node, view, decos) => {
    const doc = view.state.doc;
    const mark = node.node.getChild('HeaderMark');
    if (!mark) return;
    const markLine = doc.lineAt(mark.from);
    if (caretOnLine(view.state, markLine)) return;
    const firstNo = doc.lineAt(node.from).number;
    pushHeadingLines(doc, level, firstNo, markLine.number - 1, markLine.number + 1, decos);
    decos.push(Decoration.line({ class: 'cm-lp-hidden-line' }).range(markLine.from));
    hideRange(markLine.from, markLine.to, decos);
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
    if (caretTouchesRange(view.state, node.from, node.to)) return;
    hideRange(first.from, first.to, decos);
    hideRange(last.from, last.to, decos);
  };
}

const decorateStrong = decorateWrappedMark('EmphasisMark', 'cm-lp-strong');
const decorateEmphasisMarks = decorateWrappedMark('EmphasisMark', 'cm-lp-em');
// Ideographs and kana only — punctuation doesn't take emphasis dots.
const CJK_RUN_RE = /[\u3040-\u30ff\u3400-\u4dbf\u4e00-\u9fff\uf900-\ufaff]+/g;

function decorateEmphasis(node, view, decos) {
  decorateEmphasisMarks(node, view, decos);
  const text = view.state.doc.sliceString(node.from, node.to);
  for (const m of text.matchAll(CJK_RUN_RE)) {
    styleRange(node.from + m.index, node.from + m.index + m[0].length, 'cm-lp-em-cjk', decos);
  }
}
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

const QUOTE_PREFIX_RE = /^(?:>[ \t]?)+/;
// Matches the theme's per-level quote padding.
const QUOTE_STEP_REM = 1.5;

function decorateQuoteMark(node, view, decos) {
  const doc = view.state.doc;
  const markLine = doc.lineAt(node.from);
  if (caretOnLine(view.state, markLine)) {
    // One hanging box for the whole "> > " prefix, placed by its first mark,
    // so the revealed markers sit in the quote's own padding.
    const prefix = node.from === markLine.from && QUOTE_PREFIX_RE.exec(markLine.text);
    // "> - item": the list marker hides the prefix to lay out its own column.
    if (prefix && syntaxTree(view.state).resolveInner(markLine.from + prefix[0].length, 1).name !== 'ListMark') {
      const depth = (prefix[0].match(/>/g) || []).length;
      hangMarker(view.state, markLine.from, markLine.from + prefix[0].length, depth * QUOTE_STEP_REM + 'rem', 'cm-lp-quote-mark', decos);
    }
    return;
  }
  // Fold space after ">" — not part of QuoteMark.
  const extra =
    node.to < markLine.to && doc.sliceString(node.to, node.to + 1) === ' ' ? 1 : 0;
  hideRange(node.from, node.to + extra, decos);
}

// ── Links: [text](url), [text][ref], [text][], [text]. The visible text
// carries the URL as a tooltip; editing (caret on the link) drops the
// pointer so the mouse reads as a text cursor.

function decorateLink(node, view, decos, options, cache) {
  // The picture widget replaces the whole link unless its source is being edited.
  if (imageFillsLink(node.node) && !selectionInsideRange(view.state, node.from, node.to)) return;
  const target = linkTarget(view.state, node.node, cache);
  // An undefined reference is just bracketed text.
  if (!target) return;
  const editing = caretTouchesRange(view.state, node.from, node.to) && !isRevealSuppressed(view.state, node.from, node.to);
  if (target.textTo > target.textFrom) {
    decos.push(
      Decoration.mark(editing ? { class: 'cm-lp-link cm-lp-link-editing' } : { class: 'cm-lp-link', attributes: { title: target.url } })
        .range(target.textFrom, target.textTo)
    );
  }
  if (editing) return;
  hideRange(node.from, target.textFrom, decos);
  hideRange(target.textTo, node.to, decos);
}

// ── URLs: bare/<autolink> ones are links themselves; inside a link, image or
// reference definition they're the (revealed) target, styled but not clickable.

const URL_CONTAINERS = new Set(['Link', 'Image', 'LinkReference']);

function decorateUrl(node, view, decos) {
  const parent = node.node.parent;
  if (parent && URL_CONTAINERS.has(parent.name)) {
    styleRange(node.from, node.to, 'cm-lp-link-url', decos);
    return;
  }
  const editing = caretTouchesRange(view.state, node.from, node.to);
  styleRange(node.from, node.to, editing ? 'cm-lp-link cm-lp-link-editing' : 'cm-lp-link', decos);
}

function decorateAutolinkBrackets(node, view, decos) {
  const marks = node.node.getChildren('LinkMark');
  if (marks.length < 2) return;
  if (caretTouchesRange(view.state, node.from, node.to) && !isRevealSuppressed(view.state, node.from, node.to)) return;
  hideRange(marks[0].from, marks[0].to, decos);
  hideRange(marks[1].from, marks[1].to, decos);
}

// "[label]: url" definitions don't render in markdown; keep them quiet.
function decorateLinkReference(node, view, decos) {
  styleRange(node.from, node.to, 'cm-lp-linkref', decos);
}

// ── \* — the backslash is syntax; show it only while the caret is on it.

function decorateEscape(node, view, decos) {
  if (caretTouchesRange(view.state, node.from, node.to)) return;
  hideRange(node.from, node.from + 1, decos);
}

// ── Indented code (4 spaces / tab). Top level only: inside a list the
// indentation belongs to the item. The indent stays hidden even while
// editing so entering the block doesn't shift it sideways.

function decorateIndentedCode(node, view, decos) {
  if (!node.node.parent || node.node.parent.name !== 'Document') return;
  const doc = view.state.doc;
  const fromNo = doc.lineAt(node.from).number;
  const toNo = doc.lineAt(Math.max(node.from, node.to - 1)).number;
  for (let n = fromNo; n <= toNo; n++) {
    const line = doc.line(n);
    const classes = ['cm-lp-codeblock', 'cm-lp-codeblock-indented'];
    if (n === fromNo) classes.push('cm-lp-codeblock-first');
    if (n === toNo) classes.push('cm-lp-codeblock-last');
    decos.push(Decoration.line({ class: classes.join(' ') }).range(line.from));
    const indent = /^(?: {1,4}|\t)/.exec(line.text);
    if (indent) hideRange(line.from, line.from + indent[0].length, decos);
  }
}

// ── HTML is shown as source, never rendered (notes may come from anywhere).

function decorateHtmlBlock(node, view, decos) {
  const doc = view.state.doc;
  const fromNo = doc.lineAt(node.from).number;
  const toNo = doc.lineAt(Math.max(node.from, node.to - 1)).number;
  for (let n = fromNo; n <= toNo; n++) decos.push(Decoration.line({ class: 'cm-lp-htmlblock' }).range(doc.line(n).from));
}

function decorateHtmlTag(node, view, decos) {
  styleRange(node.from, node.to, 'cm-lp-html', decos);
}

function decorateHtmlComment(node, view, decos) {
  styleRange(node.from, node.to, 'cm-lp-html-comment', decos);
}

// ── ![alt](url) / ![[file]] render from image-field.js; here only the source
// shown while the caret is inside the picture (double-click) gets its color.

function decorateImageSource(node, view, decos) {
  if (imageEditing(view.state, node.node)) styleRange(node.from, node.to, 'cm-lp-wikiimage-raw', decos);
}

// ── List markers. Each list gets a fixed-width marker column (bullet/task
// share one width; ordered widens with its digit count), so wrapped lines,
// continuation lines and nested lists all start at the item's text edge.

const BULLET_COLUMN_EM = 1.4;

function isListNode(n) {
  return n.name === 'BulletList' || n.name === 'OrderedList';
}

function markerColumnEm(doc, list, cache) {
  if (list.name !== 'OrderedList') return BULLET_COLUMN_EM;
  const key = 'ol' + list.from;
  if (cache && cache.has(key)) return cache.get(key);
  let digits = 1;
  for (let item = list.firstChild; item; item = item.nextSibling) {
    const mark = item.name === 'ListItem' && item.firstChild;
    if (mark && mark.name === 'ListMark') digits = Math.max(digits, mark.to - mark.from - 1);
  }
  const em = Math.max(BULLET_COLUMN_EM, digits * 0.6 + 0.85);
  if (cache) cache.set(key, em);
  return em;
}

// Left edge of this list's item text, summed over every enclosing list.
function contentIndentEm(doc, list, cache) {
  let em = 0;
  for (let p = list; p; p = p.parent) if (isListNode(p)) em += markerColumnEm(doc, p, cache);
  return +em.toFixed(3);
}

// End of marker + separator spaces (for a task: up to its "[ ]").
function markerEnd(doc, mark, line, taskMarker) {
  if (taskMarker) return taskMarker.from;
  let pos = mark.to;
  while (pos < line.to && doc.sliceString(pos, pos + 1) === ' ') pos++;
  return pos;
}

// Lines of `item` past its marker line that aren't inside a nested list.
function continuationLines(doc, item, markLineNo) {
  // By line number: a nested list starts at its marker, past the line's indentation.
  const nested = [];
  for (let c = item.firstChild; c; c = c.nextSibling) {
    if (isListNode(c)) nested.push([doc.lineAt(c.from).number, doc.lineAt(Math.max(c.from, c.to - 1)).number]);
  }
  const lastNo = doc.lineAt(Math.max(item.from, item.to - 1)).number;
  const out = [];
  for (let n = markLineNo + 1; n <= lastNo; n++) {
    const line = doc.line(n);
    if (!line.text.trim()) continue;
    if (nested.some(([a, b]) => n >= a && n <= b)) continue;
    out.push(line);
  }
  return out;
}

function decorateListMark(node, view, decos, options, cache) {
  const state = view.state;
  const doc = state.doc;
  const listItem = node.node.parent;
  const list = listItem && listItem.parent;
  if (!list || !isListNode(list)) return;
  const task = listItem.getChild('Task');
  const taskMarker = task && task.getChild('TaskMarker');
  const line = doc.lineAt(node.from);
  // Bare "-" / "1." is an empty item to CommonMark; same trap as a bare "#".
  if (node.to === line.to && listItem.to <= line.to) return;
  const columnEm = markerColumnEm(doc, list, cache);
  const indentEm = contentIndentEm(doc, list, cache);
  const end = markerEnd(doc, node, line, taskMarker);

  // Source indentation encodes nesting; the computed indent replaces it.
  if (node.from > line.from) hideRange(line.from, node.from, decos);

  // Whole visual marker: bullet/number + spaces, or "- [ ] " for a task.
  let markerTo = end;
  if (taskMarker) markerTo = taskMarker.to + (doc.sliceString(taskMarker.to, taskMarker.to + 1) === ' ' ? 1 : 0);
  // Caret stepped into the marker itself (← from the item text): show the source.
  const editingMarker = !state.facet(readingMode) && state.selection.ranges.some((r) => r.empty && r.head > node.from && r.head < markerTo);
  if (editingMarker) {
    styleRange(node.from, markerTo, 'cm-lp-list-mark-raw', decos);
  } else if (taskMarker) {
    hideRange(node.from, end, decos);
    const checked = /\[[xX]\]/.test(doc.sliceString(taskMarker.from, taskMarker.to));
    decos.push(Decoration.replace({ widget: new TaskCheckboxWidget(checked, columnEm) }).range(taskMarker.from, markerTo));
  } else {
    const label = list.name === 'BulletList' ? null : doc.sliceString(node.from, node.to);
    decos.push(Decoration.replace({ widget: new ListMarkerWidget(label, columnEm) }).range(node.from, end));
  }

  const lineSpec = { class: 'cm-lp-list-line' };
  const styleDecls = ['padding-left:' + indentEm + 'em', 'text-indent:-' + columnEm + 'em'];
  // Don't stack item padding on adjacent blank-line height.
  if (isBlankLine(doc, line.number - 1)) styleDecls.push('padding-top:0 !important');
  if (isBlankLine(doc, line.number + 1)) styleDecls.push('padding-bottom:0 !important');
  lineSpec.attributes = { style: styleDecls.join(';') };
  decos.push(Decoration.line(lineSpec).range(line.from));

  // Lazy/indented continuation lines, later paragraphs and code inside the
  // item. margin (not padding) so code/quote lines keep their own padding.
  const contentCol = end - line.from;
  for (const cont of continuationLines(doc, listItem, line.number)) {
    decos.push(Decoration.line({ attributes: { style: 'margin-left:' + indentEm + 'em' } }).range(cont.from));
    const ws = cont.text.length - cont.text.trimStart().length;
    if (ws > 0) hideRange(cont.from, cont.from + Math.min(ws, contentCol), decos);
  }

  // A "loose list" blank line directly between two items of this same list
  // is still conceptually one gap, not a deliberate extra paragraph break —
  // without this it renders at the default line height (a full text line),
  // which dwarfs cm-lp-list-line's own padding and makes list-item spacing
  // look untouched however tight that padding is set. Collapsed the same
  // hairline way frontmatter/table rows already do (checked from the item
  // above only, so a blank line between two items isn't decorated twice).
  if (isListGapLine(state, line.number + 1)) {
    const gap = doc.line(line.number + 1);
    decos.push(Decoration.line({ class: 'cm-lp-list-gap' }).range(gap.from));
    hideRange(gap.from, gap.to, decos);
  }
}

// The node starting line `n`, past any indent or "> " prefix.
function lineStartNode(state, n) {
  const line = state.doc.line(n);
  const lead = /^[\s>]*/.exec(line.text)[0].length;
  return syntaxTree(state).resolveInner(line.from + lead, 1);
}

// The marker-line ListItem starting on line `n`, else null.
function itemStartingOn(state, n) {
  const node = lineStartNode(state, n);
  return node.name === 'ListMark' ? node.parent : null;
}

// A blank line between a one-line item and the next item of the same list.
export function isListGapLine(state, n) {
  const doc = state.doc;
  if (n <= 1 || n >= doc.lines || doc.line(n).text.trim()) return false;
  const above = itemStartingOn(state, n - 1);
  const below = above && itemStartingOn(state, n + 1);
  return !!below && isSameNode(above.parent, below.parent);
}

// Rows drawn as a 1px hairline whenever the caret is elsewhere: a list gap, a
// table's delimiter row, a setext underline. skipHairlines carries the caret
// over them, since showing one's source would move every line below.
export function isHairlineLine(state, n) {
  if (n < 1 || n > state.doc.lines) return false;
  if (isListGapLine(state, n)) return true;
  const node = lineStartNode(state, n);
  const parent = node.parent;
  if (!parent) return false;
  if (node.name === 'TableDelimiter') return parent.name === 'Table';
  return node.name === 'HeaderMark' && /^SetextHeading/.test(parent.name) && parent.from < state.doc.line(n).from;
}

// ── Thematic break: one line either way, the rule or its "---" source, at the
// same height, so the caret passing through never moves the text below.
function decorateHorizontalRule(node, view, decos) {
  const line = view.state.doc.lineAt(node.from);
  decos.push(Decoration.line({ class: 'cm-lp-hr' }).range(line.from));
  if (caretOnLine(view.state, line)) return;
  decos.push(Decoration.replace({ widget: new HorizontalRuleWidget() }).range(node.from, node.to));
}

// ── Fenced code — card chrome only when closed (≥2 CodeMarks). The fence
// lines keep a fixed height whether shown raw (caret in the block) or not,
// so entering/leaving a block doesn't shift the text below it.

function decorateFencedCode(node, view, decos) {
  const doc = view.state.doc;
  const fromLine = doc.lineAt(node.from);
  const toLineNo = doc.lineAt(Math.max(node.from, node.to - 1)).number;
  const marks = node.node.getChildren('CodeMark');
  const info = node.node.getChild('CodeInfo');
  // Same test as card-field.js, so a card and its source never disagree.
  const editingBlock = headInBlock(view.state, node.from, node.to);
  // fence-card/card-field.js replaces the whole block with a widget.
  if (!editingBlock && cardFence(doc, node.node)) return;
  // Unclosed fence extends to EOF — no card chrome until it's closed.
  if (marks.length < 2) return;
  for (let n = fromLine.number; n <= toLineNo; n++) {
    const classes = ['cm-lp-codeblock'];
    if (n === fromLine.number) classes.push('cm-lp-codeblock-first', 'cm-lp-codeblock-fence');
    if (n === toLineNo) classes.push('cm-lp-codeblock-last', 'cm-lp-codeblock-fence');
    decos.push(Decoration.line({ class: classes.join(' ') }).range(doc.line(n).from));
  }
  if (editingBlock) {
    if (info) styleRange(info.from, info.to, 'cm-lp-code-lang-edit', decos);
    return;
  }
  const lang = info ? doc.sliceString(info.from, info.to).trim().split(/\s+/)[0] : '';
  decos.push(
    Decoration.replace({ widget: new CodeHeaderWidget(lang, fenceBody(doc, node.node)) }).range(marks[0].from, fromLine.to)
  );
  const close = marks[marks.length - 1];
  hideRange(close.from, close.to, decos);
}

// ── Wikilink — not atomic, so arrow keys can step inside and reveal the source.

function decorateWikiLink(node, view, decos, options) {
  const inner = wikiNodeInner(node.node, view.state.doc);
  const { target, heading, alias } = splitWikiLinkInner(inner);
  if (!target && !heading) return;
  if (selectionInsideRange(view.state, node.from, node.to) || isWidgetRevealed(view.state, node.from, node.to)) {
    styleRange(node.from, node.to, 'cm-lp-wikilink-raw', decos);
    return;
  }
  const broken = !!(target && options && options.isWikiLinkBroken && options.isWikiLinkBroken(target));
  decos.push(
    Decoration.replace({
      widget: new WikiLinkWidget(target, heading, alias, options && options.onWikiLinkClick, broken),
    }).range(node.from, node.to)
  );
}

export const nodeDecorators = {
  ATXHeading1: decorateHeading(1),
  ATXHeading2: decorateHeading(2),
  ATXHeading3: decorateHeading(3),
  ATXHeading4: decorateHeading(4),
  ATXHeading5: decorateHeading(5),
  ATXHeading6: decorateHeading(6),
  SetextHeading1: decorateSetextHeading(1),
  SetextHeading2: decorateSetextHeading(2),
  StrongEmphasis: decorateStrong,
  Emphasis: decorateEmphasis,
  Strikethrough: decorateStrikethrough,
  InlineCode: decorateInlineCode,
  Blockquote: decorateBlockquote,
  QuoteMark: decorateQuoteMark,
  Link: decorateLink,
  Autolink: decorateAutolinkBrackets,
  URL: decorateUrl,
  LinkReference: decorateLinkReference,
  Escape: decorateEscape,
  CodeBlock: decorateIndentedCode,
  HTMLBlock: decorateHtmlBlock,
  HTMLTag: decorateHtmlTag,
  Comment: decorateHtmlComment,
  CommentBlock: decorateHtmlComment,
  Image: decorateImageSource,
  ListMark: decorateListMark,
  HorizontalRule: decorateHorizontalRule,
  FencedCode: decorateFencedCode,
  WikiLink: decorateWikiLink,
  WikiImage: decorateImageSource,
  TableHeader: decorateTableHeader,
  TableRow: decorateTableBodyRow,
  TableDelimiter: decorateTableDelimiterRow,
  Frontmatter: decorateFrontmatterSource,
};
