import { EditorSelection } from '@codemirror/state';
import { rangeTouchesCode, rangeCrossesBlock, trimWhitespace } from './format-guards.js';
import { resolveAncestor } from './tree-utils.js';

// Bold/italic/strikethrough toggle commands for the right-click format menu
// (format-menu.js). CM6 ships no markdown-formatting commands at all (only
// generic editing ones in @codemirror/commands) — this is the hand-rolled
// equivalent of "wrap/unwrap the selection", using the syntax tree (already
// parsed by GFM, no custom syntax needed for these three) to detect whether
// a selection is already inside the target mark, the same way decorators.js
// finds these nodes to color them.

// Nearest ancestor of `range` whose node type is one of `nodeNames` and
// which fully contains the range — not just touches it, so a selection that
// only partially overlaps an existing mark is treated as "not yet applied"
// (see the toggleWrap comment below for why that's the deliberate choice).
function enclosingNode(state, range, nodeNames) {
  return resolveAncestor(
    state,
    range.from,
    (n) => nodeNames.includes(n.name) && n.from <= range.from && n.to >= range.to,
    1
  );
}

function markChild(node, markName, fromEnd) {
  const c = fromEnd ? node.lastChild : node.firstChild;
  return c && c.name === markName ? c : null;
}

function toggleWrap(view, { nodeNames, markName, markStart, markEnd }) {
  const state = view.state;
  const tr = state.changeByRange(range => {
    // Code content is verbatim — inserting mark characters into it doesn't
    // format anything, so leave it untouched regardless of selection shape.
    if (rangeTouchesCode(state, range.from, range.to)) return { range };

    if (range.empty) {
      // Bare cursor — drop a fresh pair of marks and land the cursor between
      // them so typing continues inside, the same behavior most markdown
      // editors give an empty-selection toggle.
      const changes = [
        { from: range.from, insert: markStart },
        { from: range.to, insert: markEnd },
      ];
      return { changes, range: EditorSelection.cursor(range.from + markStart.length) };
    }

    // A mark spanning a blank line doesn't round-trip as one emphasis span
    // (the blank line ends the paragraph) — leave a block-crossing selection
    // alone rather than emit syntax that won't parse back the way it looks.
    if (rangeCrossesBlock(state, range.from, range.to)) return { range };

    // CommonMark's delimiters must hug non-whitespace on their inner side, so
    // a selection with leading/trailing spaces at its raw edges (e.g. a
    // drag-select that caught a trailing space) needs the marks placed at
    // the trimmed core, not the raw bounds, or the result doesn't parse as
    // emphasis at all — it renders as literal ** in the text.
    const { from, to } = trimWhitespace(state, range.from, range.to);
    if (from >= to) return { range }; // selection was pure whitespace

    const trimmedRange = { from, to };
    const node = enclosingNode(state, trimmedRange, nodeNames);
    const open = node && markChild(node, markName, false);
    const close = node && markChild(node, markName, true);
    if (open && close && close.from >= open.to) {
      // Already wrapped — strip both marks, keep the unwrapped text selected.
      const changes = [
        { from: open.from, to: open.to, insert: '' },
        { from: close.from, to: close.to, insert: '' },
      ];
      const newFrom = open.from;
      const newTo = newFrom + (close.from - open.to);
      return { changes, range: EditorSelection.range(newFrom, newTo) };
    }
    // Not wrapped — wrap the trimmed core, leaving any outer whitespace
    // untouched outside the marks.
    const changes = [
      { from, insert: markStart },
      { from: to, insert: markEnd },
    ];
    const newFrom = from + markStart.length;
    const newTo = to + markStart.length;
    return { changes, range: EditorSelection.range(newFrom, newTo) };
  });
  view.dispatch(state.update(tr, { scrollIntoView: true, userEvent: 'input' }));
  return true;
}

export function toggleBold(view) {
  return toggleWrap(view, { nodeNames: ['StrongEmphasis'], markName: 'EmphasisMark', markStart: '**', markEnd: '**' });
}

// Inserts `*..*`, not `_..._`: underscores can't open/close emphasis
// intraword per CommonMark (`foo_bar_baz` doesn't italicize "bar" — verified
// against this project's own parser), which breaks the common case of a
// selection that starts/ends mid-word with no space boundary. `*` has no
// such restriction and was verified (same parser) to still resolve
// correctly even directly adjacent to a closed **bold** run
// (`**bold***rest*` parses as bold + italic, not something ambiguous) — so
// there's no real tradeoff here, `*` is simply the more correct default. The
// two markers are equivalent to the parser either way (both resolve to the
// same Emphasis node), so toggle-off recognizes existing `_text_` too.
export function toggleItalic(view) {
  return toggleWrap(view, { nodeNames: ['Emphasis'], markName: 'EmphasisMark', markStart: '*', markEnd: '*' });
}

export function toggleStrikethrough(view) {
  return toggleWrap(view, { nodeNames: ['Strikethrough'], markName: 'StrikethroughMark', markStart: '~~', markEnd: '~~' });
}

// For the format menu's active-state highlighting (e.g. showing "Bold" as
// pressed when the selection is already bold). Mirrors toggleWrap's own
// guards/trimming so the indicator never disagrees with what a click would
// actually do: code content and pure whitespace never show as "active",
// and a selection with padding whitespace is checked against its trimmed
// core, matching what toggleWrap would toggle off.
// Known v1 limitation shared with toggleWrap: a selection straddling a
// mark's edge (part plain, part bold) reads as "not bold" here too, same as
// toggleWrap would re-wrap it — CM6 has no primitive for splitting a range
// at existing-format boundaries.
export function inlineFormatActiveAt(state, range) {
  const none = { bold: false, italic: false, strike: false };
  if (rangeTouchesCode(state, range.from, range.to)) return none;
  if (range.empty) return none;
  const { from, to } = trimWhitespace(state, range.from, range.to);
  if (from >= to) return none;
  const r = { from, to };
  return {
    bold: !!enclosingNode(state, r, ['StrongEmphasis']),
    italic: !!enclosingNode(state, r, ['Emphasis']),
    strike: !!enclosingNode(state, r, ['Strikethrough']),
  };
}
