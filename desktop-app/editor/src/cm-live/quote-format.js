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
