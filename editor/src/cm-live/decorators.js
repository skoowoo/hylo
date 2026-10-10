// Node-name → decorator. live-preview.js walks the tree and dispatches here.
// hideRange collapses markers when caret leaves; styleRange is always-on.
// GFM tables and frontmatter live in their own files (tables-decorate.js,
// frontmatter-decorate.js) — this file keeps the smaller inline/list/quote/
// code decorators that don't carry enough independent state to justify one.
import { Decoration } from '@codemirror/view';
import { syntaxTree } from '@codemirror/language';
import { selectionTouchesRange, selectionTouchesLine, selectionInsideRange, readingMode } from './selection.js';
import { isRevealSuppressed } from './link-reveal-suppression.js';
import { WikiLinkWidget, ImageWidget, TaskCheckboxWidget, ListMarkerWidget, CodeHeaderWidget } from './widgets.js';
import { wikiNodeInner, splitWikiLinkInner } from './wiki-syntax.js';
import { hideRange, styleRange, hangRange, excludeFromTextSample } from './decoration-helpers.js';
import { isSameNode } from './tree-utils.js';
import { isWidgetRevealed } from './widget-reveal.js';
import { decorateTableHeader, decorateTableBodyRow, decorateTableDelimiterRow } from './tables-decorate.js';
import { decorateFrontmatter } from './frontmatter-decorate.js';
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
    excludeFromTextSample(doc.line(n), decos);
  }
}

function decorateHeading(level) {
  return (node, view, decos) => {
    const doc = view.state.doc;
    const line = doc.lineAt(node.from);
    pushHeadingLines(doc, level, line.number, line.number, line.number + 1, decos);
    const mark = node.node.getChild('HeaderMark');
    if (!mark) return;
    // Fold trailing space after "#"s — not part of HeaderMark.
    const extra =
      mark.to < line.to && view.state.doc.sliceString(mark.to, mark.to + 1) === ' ' ? 1 : 0;
    if (selectionTouchesLine(view.state, line)) {
      // Caret on the line reveals the "#"s, muted so they read as syntax. At
      // the line start they hang in the margin; nested ("> # x") they stay inline.
      if (mark.from === line.from) {
        hangRange(mark.from, mark.to + extra, (mark.to + extra - mark.from) * 0.75 + 'em', 'cm-lp-heading-mark', decos);
      } else {
        styleRange(mark.from, mark.to, 'cm-lp-heading-mark', decos);
      }
      return;
    }
    hideRange(mark.from, mark.to + extra, decos);
  };
}

// "Text\n===" / "Text\n---". Left raw while the caret is on the underline:
// a lone "-" typed under a paragraph is already an H2, and styling it would
// flash a heading on the way to "- item".
function decorateSetextHeading(level) {
  return (node, view, decos) => {
    const doc = view.state.doc;
    const mark = node.node.getChild('HeaderMark');
    if (!mark) return;
    const markLine = doc.lineAt(mark.from);
    if (selectionTouchesLine(view.state, markLine)) return;
    const firstNo = doc.lineAt(node.from).number;
    pushHeadingLines(doc, level, firstNo, markLine.number - 1, markLine.number + 1, decos);
    if (selectionTouchesRange(view.state, node.from, node.to)) {
      styleRange(markLine.from, markLine.to, 'cm-lp-heading-mark', decos);
      return;
    }
    // Same hairline collapse as a hidden fence line.
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
    if (selectionTouchesRange(view.state, node.from, node.to)) return;
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
    excludeFromTextSample(doc.line(n), decos);
  }
}

const QUOTE_PREFIX_RE = /^(?:>[ \t]?)+/;
// Matches the theme's per-level quote padding.
const QUOTE_STEP_REM = 1.5;

function decorateQuoteMark(node, view, decos) {
  const doc = view.state.doc;
  const markLine = doc.lineAt(node.from);
  if (selectionTouchesLine(view.state, markLine)) {
    // One hanging box for the whole "> > " prefix, placed by its first mark,
    // so the revealed markers sit in the quote's own padding.
    const prefix = node.from === markLine.from && QUOTE_PREFIX_RE.exec(markLine.text);
    if (prefix) {
      const depth = (prefix[0].match(/>/g) || []).length;
      hangRange(markLine.from, markLine.from + prefix[0].length, depth * QUOTE_STEP_REM + 'rem', 'cm-lp-quote-mark', decos);
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
  const target = linkTarget(view.state, node.node, cache);
  // An undefined reference is just bracketed text.
  if (!target) return;
  const editing = selectionTouchesRange(view.state, node.from, node.to) && !isRevealSuppressed(view.state, node.from, node.to);
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
  const editing = selectionTouchesRange(view.state, node.from, node.to);
  styleRange(node.from, node.to, editing ? 'cm-lp-link cm-lp-link-editing' : 'cm-lp-link', decos);
}

function decorateAutolinkBrackets(node, view, decos) {
  const marks = node.node.getChildren('LinkMark');
  if (marks.length < 2) return;
  if (selectionTouchesRange(view.state, node.from, node.to) && !isRevealSuppressed(view.state, node.from, node.to)) return;
  hideRange(marks[0].from, marks[0].to, decos);
  hideRange(marks[1].from, marks[1].to, decos);
}

// "[label]: url" definitions don't render in markdown; keep them quiet.
function decorateLinkReference(node, view, decos) {
  styleRange(node.from, node.to, 'cm-lp-linkref', decos);
}

// ── \* — the backslash is syntax; show it only while the caret is on it.

function decorateEscape(node, view, decos) {
  if (selectionTouchesRange(view.state, node.from, node.to)) return;
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
    excludeFromTextSample(line, decos);
    const indent = /^(?: {1,4}|\t)/.exec(line.text);
    if (indent) hideRange(line.from, line.from + indent[0].length, decos);
  }
}

// ── HTML is shown as source, never rendered (notes may come from anywhere).

function decorateHtmlBlock(node, view, decos) {
  const doc = view.state.doc;
  const fromNo = doc.lineAt(node.from).number;
  const toNo = doc.lineAt(Math.max(node.from, node.to - 1)).number;
  for (let n = fromNo; n <= toNo; n++) {
    const line = doc.line(n);
    decos.push(Decoration.line({ class: 'cm-lp-htmlblock' }).range(line.from));
    excludeFromTextSample(line, decos);
  }
}

function decorateHtmlTag(node, view, decos) {
  styleRange(node.from, node.to, 'cm-lp-html', decos);
}

function decorateHtmlComment(node, view, decos) {
  styleRange(node.from, node.to, 'cm-lp-html-comment', decos);
}

// ── ![alt](url) — strict inside like wikiimage: arrow keys step in to edit the source.

function decorateImage(node, view, decos, options) {
  const marks = node.node.getChildren('LinkMark');
  const urlNode = node.node.getChild('URL');
  if (marks.length < 2 || !urlNode) return;
  const url = view.state.doc.sliceString(urlNode.from, urlNode.to);
  const alt = view.state.doc.sliceString(marks[0].to, marks[1].from);
  if (selectionInsideRange(view.state, node.from, node.to) || isWidgetRevealed(view.state, node.from, node.to)) {
    styleRange(node.from, node.to, 'cm-lp-wikiimage-raw', decos);
    return;
  }
  // Relative paths name files in the vault, not URLs on the app's own page.
  const resolve = options && options.resolveMarkdownImageSrc;
  const src = resolve ? resolve(url) : url;
  decos.push(Decoration.replace({ widget: new ImageWidget(src, alt || url, 2) }).range(node.from, node.to));
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
    // 1px line class would leave a short ASCII run for CM's text-size probe.
    // textHeight 1 makes vertical motion's step (>> 1) zero, so the scan never ends.
    hideRange(gap.from, gap.to, decos);
  }
}

// The marker-line ListItem starting on line `n` (past any indent or "> " prefix), else null.
function itemStartingOn(state, n) {
  const line = state.doc.line(n);
  const lead = /^[\s>]*/.exec(line.text)[0].length;
  const node = syntaxTree(state).resolveInner(line.from + lead, 1);
  return node.name === 'ListMark' ? node.parent : null;
}

// A blank line between a one-line item and the next item of the same list;
// decorateListMark collapses it and skipListGaps steps over it.
export function isListGapLine(state, n) {
  const doc = state.doc;
  if (n <= 1 || n >= doc.lines || doc.line(n).text.trim()) return false;
  const above = itemStartingOn(state, n - 1);
  const below = above && itemStartingOn(state, n + 1);
  return !!below && isSameNode(above.parent, below.parent);
}

// HorizontalRule → horizontal-rule-field.js (needs block:true StateField).

// ── Fenced code — card chrome only when closed (≥2 CodeMarks). The fence
// lines keep a fixed height whether shown raw (caret in the block) or not,
// so entering/leaving a block doesn't shift the text below it.

function decorateFencedCode(node, view, decos) {
  const doc = view.state.doc;
  const fromLine = doc.lineAt(node.from);
  const toLineNo = doc.lineAt(Math.max(node.from, node.to - 1)).number;
  const marks = node.node.getChildren('CodeMark');
  const info = node.node.getChild('CodeInfo');
  const editingBlock = selectionTouchesRange(view.state, node.from, node.to);
  // fence-card/card-field.js replaces the whole block with a widget.
  if (!editingBlock && cardFence(doc, node.node)) return;
  // Unclosed fence extends to EOF — no card chrome until it's closed.
  if (marks.length < 2) return;
  for (let n = fromLine.number; n <= toLineNo; n++) {
    const classes = ['cm-lp-codeblock'];
    if (n === fromLine.number) classes.push('cm-lp-codeblock-first', 'cm-lp-codeblock-fence');
    if (n === toLineNo) classes.push('cm-lp-codeblock-last', 'cm-lp-codeblock-fence');
    decos.push(Decoration.line({ class: classes.join(' ') }).range(doc.line(n).from));
    // Unhighlighted code (no grammar, or not loaded yet) is plain text in a mono face.
    excludeFromTextSample(doc.line(n), decos);
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

// ── Wikilink / wikiimage — not atomic, so arrow keys can step inside and reveal the source.

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

function decorateWikiImage(node, view, decos, options) {
  const filename = wikiNodeInner(node.node, view.state.doc);
  if (!filename) return;
  if (selectionInsideRange(view.state, node.from, node.to) || isWidgetRevealed(view.state, node.from, node.to)) {
    styleRange(node.from, node.to, 'cm-lp-wikiimage-raw', decos);
    return;
  }
  decos.push(
    Decoration.replace({
      widget: new ImageWidget(options && options.resolveImageSrc ? options.resolveImageSrc(filename) : filename, filename, 3),
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
  Image: decorateImage,
  ListMark: decorateListMark,
  // HorizontalRule → horizontal-rule-field.js
  FencedCode: decorateFencedCode,
  WikiLink: decorateWikiLink,
  WikiImage: decorateWikiImage,
  TableHeader: decorateTableHeader,
  TableRow: decorateTableBodyRow,
  TableDelimiter: decorateTableDelimiterRow,
  Frontmatter: decorateFrontmatter,
};
