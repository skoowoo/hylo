import { syntaxTree } from '@codemirror/language';
import { touchedLines } from './selection-lines.js';
import { lineInsideCodeBlock } from './format-guards.js';

// Blockquote toggle for the right-click format menu. Unlike list markers
// (list-indent.js's ListItem, a container node you can resolve up to from a
// position inside it), a `>` is its own leaf token (QuoteMark) attached to
// the leaf block that starts on that line, not a child of Blockquote itself
// — see decorators.js's decorateBlockquote comment ("QuoteMark lives on
// leaf lines, not as Blockquote children"). So instead of walking up from a
// resolved position, this scans the line's own range directly for a
// QuoteMark token.
function quoteMarkOnLine(state, line) {
  let found = null;
  syntaxTree(state).iterate({
    from: line.from,
    to: line.to,
    enter(node) {
      if (node.name === 'QuoteMark') { found = { from: node.from, to: node.to }; return false; }
    },
  });
  return found;
}

// Past ">" and its one optional separator space — CommonMark allows
// ">text" with no space — mirroring decorateQuoteMark's own "fold space
// after > — not part of QuoteMark" rule so toggling off strips exactly what
// the live-preview widget already hides.
function quoteContentStart(state, mark, line) {
  let pos = mark.to;
  if (pos < line.to && state.doc.sliceString(pos, pos + 1) === ' ') pos++;
  return pos;
}

// All-or-nothing, same convention as list-format.js's applyListKind: only
// strips when EVERY touched line is already quoted; otherwise every
// unquoted line gets a `> ` added (an already-quoted line in a mixed
// selection is left alone — there's no second "kind" to convert it to the
// way bullet/ordered/task have). v1 only ever adds/removes one level — a
// selection already inside a nested `> > quote` isn't detected as "more
// quoted" and re-toggling it strips just the outermost QuoteMark on that
// line, same one-level-at-a-time behavior most editors give this action.
export function toggleBlockquote(view) {
  const state = view.state;
  const lines = touchedLines(state);
  // Same reasoning as list-format.js's applyListKind: a code block's lines
  // are verbatim, so prefixing one with "> " just inserts those literal
  // characters into the code instead of quoting anything.
  if (lines.some(line => lineInsideCodeBlock(state, line))) return true;
  const marks = lines.map(line => quoteMarkOnLine(state, line));
  const allQuoted = marks.every(m => m != null);

  const changes = [];
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    const mark = marks[i];
    if (allQuoted) {
      changes.push({ from: mark.from, to: quoteContentStart(state, mark, line), insert: '' });
    } else if (!mark) {
      changes.push({ from: line.from, to: line.from, insert: '> ' });
    }
  }
  if (!changes.length) return true;
  view.dispatch({ changes, userEvent: 'input' });
  return true;
}

export function quoteFormatActiveAt(state, range) {
  return !!quoteMarkOnLine(state, state.doc.lineAt(range.head));
}

// All QuoteMarks on the line, in source order — a nested "> > " line carries
// one per level, and emptyQuoteEnter needs the innermost plus its parent.
function lineQuoteMarks(state, line) {
  const marks = [];
  syntaxTree(state).iterate({
    from: line.from,
    to: line.to,
    enter(node) {
      if (node.name === 'QuoteMark') marks.push({ from: node.from, to: node.to });
    },
  });
  return marks;
}

function isEmptyQuotedLine(state, line, marks) {
  if (!marks.length) return false;
  const last = marks[marks.length - 1];
  return state.doc.sliceString(quoteContentStart(state, last, line), line.to).trim() === '';
}

// Range covering just the innermost QuoteMark (plus its separator space) on
// an already-empty quoted line — deleting it outdents one nesting level, or
// exits the blockquote entirely when there's no parent level left. Shared by
// emptyQuoteEnter and emptyQuoteBackspace so both keystrokes agree on
// exactly what "one level" means.
function innermostMarkRange(state, line, marks) {
  const last = marks[marks.length - 1];
  const prev = marks.length > 1 ? marks[marks.length - 2] : null;
  const from = prev ? quoteContentStart(state, prev, line) : line.from;
  const to = quoteContentStart(state, last, line);
  return { from, to };
}

// Enter on an empty quoted line ("> " with nothing typed after it) exits the
// quote on this keystroke — the same one-blank-line convention most markdown
// editors use, and the one list-indent.js's emptyItemEnter already gives
// lists. Left to lang-markdown's own insertNewlineContinueMarkup, a
// blockquote instead needs TWO consecutive empty quoted lines before it
// recognizes the exit (its "two aligned empty quoted lines in a row" check),
// so a plain Enter here silently inserts a second blank quoted line instead
// of leaving the quote — one extra, invisible keystroke. Nested quotes exit
// one level per Enter, same as the list outdent-on-empty-item convention.
export function emptyQuoteEnter(view) {
  const state = view.state;
  const sel = state.selection.main;
  if (!sel.empty) return false;
  const line = state.doc.lineAt(sel.head);
  const marks = lineQuoteMarks(state, line);
  if (!isEmptyQuotedLine(state, line, marks)) return false;
  const { from, to } = innermostMarkRange(state, line, marks);
  view.dispatch({ changes: { from, to, insert: '' }, userEvent: 'input' });
  return true;
}

// Backspace on an empty quoted line strips the innermost marker in one
// keystroke — same one-level-per-keystroke convention as emptyQuoteEnter
// and list-indent.js's emptyItemBackspace. Left unhandled, CM6's default
// deleteMarkupBackward erases the trailing space one character at a time,
// so clearing an empty quote line's marker takes as many keystrokes as it
// has characters instead of one. Unlike emptyItemBackspace's top-level
// case, this never needs to delete the whole line: stripping the outermost
// QuoteMark already leaves an ordinary blank line, and a second Backspace
// on that is plain, unremarkable default behavior — no extra case to write.
export function emptyQuoteBackspace(view) {
  const state = view.state;
  const sel = state.selection.main;
  if (!sel.empty) return false;
  const line = state.doc.lineAt(sel.head);
  const marks = lineQuoteMarks(state, line);
  if (!isEmptyQuotedLine(state, line, marks)) return false;
  const { from, to } = innermostMarkRange(state, line, marks);
  view.dispatch({ changes: { from, to, insert: '' }, userEvent: 'delete' });
  return true;
}
