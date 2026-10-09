// Node-name → decorator. live-preview.js walks the tree and dispatches here.
// hideRange collapses markers when caret leaves; styleRange is always-on.
// GFM tables and frontmatter live in their own files (tables-decorate.js,
// frontmatter-decorate.js) — this file keeps the smaller inline/list/quote/
// code decorators that don't carry enough independent state to justify one.
import { Decoration } from '@codemirror/view';
import { selectionTouchesRange, selectionTouchesLine, selectionInsideRange } from './selection.js';
import { isRevealSuppressed } from './link-reveal-suppression.js';
import { WikiLinkWidget, WikiImageWidget, TaskCheckboxWidget, MarkdownImageWidget, BulletWidget } from './widgets.js';
import { wikiNodeInner, splitWikiLinkInner } from './wiki-syntax.js';
import { hideRange, styleRange } from './decoration-helpers.js';
import { nearestAncestor, nameIs, isSameNode } from './tree-utils.js';
import { decorateTableHeader, decorateTableBodyRow, decorateTableDelimiterRow } from './tables-decorate.js';
import { decorateFrontmatter } from './frontmatter-decorate.js';
import { cardFence } from './fence-card/registry.js';

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

// Is `lineNumber` the start of a ListItem belonging to `list` itself (not
// just any list in the tree) — a marker-type change starts a new List node
// even across a single blank line, and that gap must not collapse as if it
// were a loose-list item gap within the same list.
function lineIsListItemLine(node, doc, lineNumber, list) {
  if (lineNumber < 1 || lineNumber > doc.lines) return false;
  // Walk to the tree root via .parent instead of importing syntaxTree() —
  // node is already a live SyntaxNodeRef in that same tree.
  let root = node.node;
  while (root.parent) root = root.parent;
  const item = nearestAncestor(root.resolve(doc.line(lineNumber).from, 1), nameIs(['ListItem']));
  return isSameNode(item && item.parent, list);
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
  if (isBlankLine(doc, line.number + 1) && lineIsListItemLine(node, doc, line.number + 2, list)) {
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
  // fence-card/card-field.js replaces the whole block with a widget.
  if (!editingBlock && cardFence(view.state.doc, node.node)) return;
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
  // The corner badge is render-only; while editing, the name stays inline after the fence.
  if (info) styleRange(info.from, info.to, editingBlock ? 'cm-lp-code-lang-edit' : 'cm-lp-code-lang', decos);
}

// ── GFM task marker

function decorateTaskMarker(node, view, decos) {
  const raw = view.state.doc.sliceString(node.from, node.to);
  const checked = /\[[xX]\]/.test(raw);
  decos.push(Decoration.replace({ widget: new TaskCheckboxWidget(checked) }).range(node.from, node.to));
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
