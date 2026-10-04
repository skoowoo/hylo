import { syntaxTree } from '@codemirror/language';
import { resolveAncestor, nameIs } from './tree-utils.js';

// Shared sanity checks the right-click format menu's commands run before
// touching a selection — catches the ways "wrap/prefix whatever's selected"
// can produce markdown that doesn't mean what the menu click implied.

const CODE_NODE_NAMES = ['InlineCode', 'CodeBlock', 'FencedCode'];

// True when any part of [from, to] sits inside a code span or code block.
// Code content is verbatim — inserting markdown syntax into it (wrapping it
// in ** or prefixing a line with > ) doesn't format anything, it just adds
// literal characters to the code. Every format action treats this the same
// as "nothing sensible to do here."
export function rangeTouchesCode(state, from, to) {
  let hit = false;
  syntaxTree(state).iterate({
    from, to,
    enter(node) {
      if (CODE_NODE_NAMES.includes(node.name)) { hit = true; return false; }
    },
  });
  return hit;
}

// True when `line` is itself part of a fenced/indented code block's
// content (including its fence delimiter lines) — as opposed to
// rangeTouchesCode above, which also flags a line that merely *contains* an
// inline `code span` amid otherwise normal prose. list-format.js/
// quote-format.js want only the block case: a code block's lines are
// verbatim, so prefixing one with "- "/"> " just inserts those literal
// characters into the code, but a line with an inline code span in the
// middle of a sentence is still a perfectly normal line to listify/quote.
export function lineInsideCodeBlock(state, line) {
  return !!resolveAncestor(state, line.from, nameIs(['CodeBlock', 'FencedCode']), 1);
}

// True when a non-empty [from, to) spans more than one paragraph/block — a
// blank line sits strictly between its start and end line. Wrapping that in
// an inline mark (**foo\n\nbar**) doesn't round-trip as a single emphasis
// span in CommonMark (a blank line ends the enclosing paragraph, so the
// parser sees two separate, unterminated marks instead), so inline actions
// are scoped to a single block. List/quote toggles don't need this — they
// already act per line via selection-lines.js's touchedLines and treat each
// line independently, which is the correct behavior for those.
export function rangeCrossesBlock(state, from, to) {
  if (from >= to) return false;
  const startLine = state.doc.lineAt(from).number;
  const endLine = state.doc.lineAt(to).number;
  for (let n = startLine; n < endLine; n++) {
    if (state.doc.line(n).text.trim() === '') return true;
  }
  return false;
}

// The non-whitespace core of [from, to). CommonMark's emphasis/strikethrough
// delimiters must hug non-whitespace on their inner side — a closing
// delimiter can't be preceded by whitespace, an opening one can't be
// followed by it — so wrapping a selection that has leading/trailing spaces
// at its raw edges (e.g. a drag-select that caught a trailing space) needs
// to place the marks at this trimmed core instead, or the result doesn't
// parse as emphasis at all (renders as literal ** in the text).
export function trimWhitespace(state, from, to) {
  const text = state.doc.sliceString(from, to);
  let start = 0, end = text.length;
  while (start < end && /\s/.test(text[start])) start++;
  while (end > start && /\s/.test(text[end - 1])) end--;
  return { from: from + start, to: from + end };
}
