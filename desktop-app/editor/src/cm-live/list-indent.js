import { indentMore, indentLess } from '@codemirror/commands';
import { insertNewlineContinueMarkup } from '@codemirror/lang-markdown';
import { syntaxTree } from '@codemirror/language';
import { keymap } from '@codemirror/view';
import { Prec } from '@codemirror/state';
import { lineInsideCodeBlock } from './format-guards.js';
import { resolveAncestor, nameIs } from './tree-utils.js';

// Tab/Shift-Tab nest/un-nest a list item. Two things the generic
// indentMore/indentLess (CM6's default "smart tab") get wrong for markdown
// lists specifically:
//   1. Their indent unit is a flat 2 spaces — enough to nest under a bullet
//      ("- " is 2 chars) but NOT under an ordered marker ("1. " is 3 chars,
//      "10. " is 4). CommonMark tolerates up to 3 stray leading spaces
//      before treating a marker as still top-level, so a 2-space Tab on an
//      ordered item is silently a no-op structurally — it *looks* indented
//      (this file hid the raw leading spaces from rendering) but doesn't
//      actually nest.
//   2. An ordered marker's number is literal source text, so nesting/
//      un-nesting leaves stale numbers behind ("3." dragged under "2." still
//      reads "3.").
// This file replaces indentMore/indentLess for list items with logic that
// measures the real target column from the syntax tree and renumbers
// affected ordered lists afterward, falling back to the generic commands
// for anything that isn't a list item (plain paragraphs, code blocks, etc).

function listMarkOf(itemNode) {
  const c = itemNode.firstChild;
  return c && c.name === 'ListMark' ? c : null;
}

// Column where this item's real content starts — past the marker and its
// one separator space. NOT past "[ ] "/"[x] " too for a task item: that
// checkbox text is inline paragraph content, not part of the structural
// marker CommonMark measures continuation-indent against. Using its width
// (6 for "- [ ] ") over-indents a nested task 4+ columns past the true
// reference column, which CommonMark reads as an indented code block
// instead of a nested list item — the checkbox silently stops rendering.
function contentColumn(state, itemNode) {
  const mark = listMarkOf(itemNode);
  if (!mark) return null;
  const doc = state.doc;
  let pos = mark.to;
  const line = doc.lineAt(mark.from);
  while (pos < line.to && doc.sliceString(pos, pos + 1) === ' ') pos++;
  return pos - line.from;
}

// Column where this item's own marker starts (its current nesting depth).
function markColumn(state, itemNode) {
  const mark = listMarkOf(itemNode);
  if (!mark) return null;
  return mark.from - state.doc.lineAt(mark.from).from;
}

function hasNestedList(itemNode) {
  for (let c = itemNode.firstChild; c; c = c.nextSibling) {
    if (c.name === 'BulletList' || c.name === 'OrderedList') return true;
  }
  return false;
}

function findListItemAt(state, pos) {
  return resolveAncestor(state, pos, nameIs(['ListItem']), -1);
}

function previousSiblingItem(itemNode) {
  // Lezer SyntaxNode objects are transient views, not stable singletons —
  // compare by position (.from), not reference.
  let prev = null;
  const parent = itemNode.parent;
  for (let n = parent && parent.firstChild; n; n = n.nextSibling) {
    if (n.from === itemNode.from) return prev;
    if (n.name === 'ListItem') prev = n;
  }
  return null;
}

function setLineIndent(state, itemNode, column, changes) {
  const mark = listMarkOf(itemNode) || itemNode;
  const line = state.doc.lineAt(mark.from);
  changes.push({ from: line.from, to: mark.from, insert: ' '.repeat(Math.max(0, column)) });
}

function collectOrderedLists(node, out) {
  if (node.name === 'OrderedList') out.push(node);
  for (let c = node.firstChild; c; c = c.nextSibling) collectOrderedLists(c, out);
}

function renumberList(state, listNode, changes) {
  let n = 0;
  for (let item = listNode.firstChild; item; item = item.nextSibling) {
    if (item.name !== 'ListItem') continue;
    const mark = listMarkOf(item);
    if (!mark) continue;
    const text = state.doc.sliceString(mark.from, mark.to);
    const m = /^(\d+)([.)])$/.exec(text);
    if (!m) continue;
    n = n === 0 ? parseInt(m[1], 10) : n + 1;
    const next = n + m[2];
    if (next !== text) changes.push({ from: mark.from, to: mark.to, insert: next });
  }
}

// Renumbers every OrderedList in the whole top-level list block containing
// `pos` — covers both the list an item joined and the one it left (an
// outdented item's abandoned sibling list isn't an ancestor of its new
// position, but it's still a descendant of the same top-level list root).
function renumberChangesAt(state, pos, seenTops, changes) {
  let top = null;
  for (let p = syntaxTree(state).resolveInner(pos, -1); p; p = p.parent) {
    if (p.name === 'BulletList' || p.name === 'OrderedList') top = p;
  }
  if (!top || seenTops.has(top.from)) return;
  seenTops.add(top.from);
  const lists = [];
  collectOrderedLists(top, lists);
  for (const list of lists) renumberList(state, list, changes);
}

// One transaction, not two: renumbering needs the list structure *after*
// the indent/outdent lands (nesting only changes once that edit is in), but
// dispatching the indent first and the renumber second — as this used to —
// split one Tab press into two undo-history entries, so a single Ctrl+Z only
// unwound the renumber and left the indent behind. `state.update` builds the
// post-indent state without dispatching it, and ChangeSet.compose chains the
// two edits (each already expressed in its own doc's coordinate space —
// composing is the CM6-correct way to combine them; concatenating them into
// one `changes:` array would instead treat both as relative to the same
// original document, corrupting the renumber positions) into a single
// changeset for one dispatch.
function dispatchWithRenumber(view, changes) {
  const indentChanges = view.state.changes(changes);
  const afterIndent = view.state.update({ changes: indentChanges }).state;
  const renumber = [];
  const seenTops = new Set();
  for (const range of afterIndent.selection.ranges) renumberChangesAt(afterIndent, range.head, seenTops, renumber);
  if (!renumber.length) {
    view.dispatch({ changes: indentChanges });
    return;
  }
  view.dispatch({ changes: indentChanges.compose(afterIndent.changes(renumber)) });
}

function smartIndentMore(view) {
  const state = view.state;
  const sel = state.selection.main;
  if (!sel.empty) return indentMore(view);
  const item = findListItemAt(state, sel.head);
  if (!item) return indentMore(view);
  const prev = previousSiblingItem(item);
  if (!prev) return true; // first item in its list — nothing to nest under, mainstream editors no-op this
  const target = contentColumn(state, prev);
  if (target == null) return indentMore(view);
  const changes = [];
  setLineIndent(state, item, target, changes);
  // Joining a brand-new nested list (the preceding sibling has no sublist of
  // its own yet) starts that list at 1, same as typing "1." fresh — not
  // whatever digit this item happened to carry at the outer level. Joining
  // an *existing* nested list is left alone; renumberList below continues
  // that list's own sequence correctly on its own.
  if (!hasNestedList(prev)) {
    const mark = listMarkOf(item);
    if (mark) {
      const text = state.doc.sliceString(mark.from, mark.to);
      const m = /^(\d+)([.)])$/.exec(text);
      if (m && m[1] !== '1') changes.push({ from: mark.from, to: mark.to, insert: '1' + m[2] });
    }
  }
  dispatchWithRenumber(view, changes);
  return true;
}

function parentItemOf(itemNode) {
  const list = itemNode.parent;
  return list && list.parent && list.parent.name === 'ListItem' ? list.parent : null;
}

function smartIndentLess(view) {
  const state = view.state;
  const sel = state.selection.main;
  if (!sel.empty) return indentLess(view);
  const item = findListItemAt(state, sel.head);
  if (!item) return indentLess(view);
  const parentItem = parentItemOf(item);
  if (!parentItem) return true; // already top-level — nothing to outdent to
  const target = markColumn(state, parentItem);
  if (target == null) return indentLess(view);
  const changes = [];
  setLineIndent(state, item, target, changes);
  dispatchWithRenumber(view, changes);
  return true;
}

// True when there's nothing but the marker (and, for a task, the checkbox)
// on this item's one and only line — the state mainstream editors treat
// Enter specially for (outdent, or exit the list on the last level).
function isItemEmpty(state, itemNode) {
  const mark = listMarkOf(itemNode);
  if (!mark) return false;
  const line = state.doc.lineAt(mark.from);
  if (itemNode.to > line.to + 1) return false; // spans more than this one line
  const task = itemNode.getChild('Task');
  const taskMarker = task && task.getChild('TaskMarker');
  const afterMarker = taskMarker ? taskMarker.to : mark.to;
  return state.doc.sliceString(afterMarker, line.to).trim() === '';
}

// Enter on an empty list item: CM6's own "continue the list" logic
// (defaultKeymap's insertNewlineAndIndent, via @codemirror/lang-markdown's
// indent service) is supposed to strip the marker and outdent/exit here
// instead of inserting another copy of it — but that only reliably fires
// when the empty item was never actually a fresh Enter-created line, e.g.
// dispatched in directly. Reached by pressing Enter a second time right
// after the first Enter *created* the empty item, it instead leaves the
// marker in place and just inserts a blank line above it — reported as
// "second Enter doesn't return to the parent level, it adds a blank line
// instead". This handles the empty-item case explicitly so it doesn't
// depend on that fragile built-in path; everything else (continuing a
// non-empty item) is left to the default binding.
function emptyItemEnter(view) {
  const state = view.state;
  const sel = state.selection.main;
  if (!sel.empty) return false;
  const item = findListItemAt(state, sel.head);
  if (!item || !isItemEmpty(state, item)) return false;

  const mark = listMarkOf(item);
  const line = state.doc.lineAt(mark.from);
  const parentItem = parentItemOf(item);

  if (parentItem) {
    // Outdent one level, same as Shift-Tab — keeps the marker.
    const target = markColumn(state, parentItem);
    const changes = [];
    setLineIndent(state, item, target, changes);
    dispatchWithRenumber(view, changes);
  } else if (neighborIsListItem(state.doc, line, 1)) {
    // Next line is another item. Leaving a blank here splits the list, and
    // live preview hides that blank, so the source grows a gap the user
    // never sees. Drop the empty item instead.
    deleteLine(view, line);
  } else {
    // End of the list (or prose follows) — exit to an empty paragraph.
    const task = item.getChild('Task');
    const taskMarker = task && task.getChild('TaskMarker');
    const doc = state.doc;
    let end = taskMarker ? taskMarker.to : mark.to;
    while (end < line.to && doc.sliceString(end, end + 1) === ' ') end++;
    dispatchWithRenumber(view, [{ from: line.from, to: end, insert: '' }]);
  }
  return true;
}

const LIST_LINE = /^[ \t]*(?:[-+*]|\d+[.)]) /;

// lang-markdown repeats a loose list's blank line on every Enter, and its
// ordered-list continuation regex drops a task checkbox ("1. [ ] a" becomes
// "2. "). Both apply at every nesting depth. Fold the repairs into the
// continuation transaction so one undo removes the whole keystroke.
function listEnter(view) {
  if (emptyItemEnter(view)) return true;
  const origin = view.state.doc.lineAt(view.state.selection.main.head).text;
  const orderedTask = /^\s*\d+[.)][ \t]+\[[ xX]\]/.test(origin);
  let tr = null;
  const ok = insertNewlineContinueMarkup({
    state: view.state,
    dispatch(next) { tr = next; },
  });
  if (!ok || !tr) return false;

  let state = view.state.update(tr).state;
  let changes = tr.changes;
  const line = state.doc.lineAt(state.selection.main.head);
  if (line.number > 1 && LIST_LINE.test(line.text)) {
    const prev = state.doc.line(line.number - 1);
    if (prev.text.trim() === '') {
      const from = prev.from > 0 ? prev.from - 1 : prev.from;
      const to = prev.from > 0 ? prev.to : Math.min(state.doc.length, prev.to + 1);
      const removal = state.changes({ from, to, insert: '' });
      changes = changes.compose(removal);
      state = state.update({ changes: removal }).state;
    }
  }
  if (orderedTask) {
    const cur = state.doc.lineAt(state.selection.main.head);
    if (/^\s*\d+[.)]\s*$/.test(cur.text) && !cur.text.includes('[')) {
      const insert = /\s$/.test(cur.text) ? '[ ] ' : ' [ ] ';
      const add = state.changes({ from: cur.to, to: cur.to, insert });
      changes = changes.compose(add);
      state = state.update({ changes: add, selection: { anchor: cur.to + insert.length } }).state;
    }
  }
  view.dispatch({ changes, selection: state.selection, scrollIntoView: true, userEvent: 'input' });
  return true;
}

// Drop a whole source line, including the newline that separates it from
// its sibling, so removing an empty item doesn't leave a blank line behind.
function deleteLine(view, line) {
  const doc = view.state.doc;
  const from = line.number > 1 ? line.from - 1 : line.from;
  const to = line.number > 1 ? line.to : (line.number < doc.lines ? line.to + 1 : line.to);
  dispatchWithRenumber(view, [{ from, to, insert: '' }]);
  return true;
}

function neighborIsListItem(doc, line, dir) {
  const n = line.number + dir;
  if (n < 1 || n > doc.lines) return false;
  return /^[ \t]*(?:[-+*]|\d+[.)]) /.test(doc.line(n).text);
}

// Backspace on an empty item. lang-markdown's deleteMarkupBackward replaces
// the marker with the same number of spaces ("keep the indent") — right for
// an item that still has text, but an empty todo has nothing to keep, so
// each such keystroke leaves a whitespace line. Live preview then collapses
// that line, and the blanks pile up in the source. Obsidian / Logseq /
// Typora remove one nesting level instead: nested items outdent, a top-level
// empty item disappears. A leftover whitespace-only line between items is
// the same junk and goes away in one keystroke.
function emptyItemBackspace(view) {
  const state = view.state;
  const sel = state.selection.main;
  if (!sel.empty) return false;
  const line = state.doc.lineAt(sel.head);
  if (lineInsideCodeBlock(state, line)) return false;

  const item = findListItemAt(state, sel.head);
  if (item && isItemEmpty(state, item) && state.doc.lineAt(listMarkOf(item).from).number === line.number) {
    const parentItem = parentItemOf(item);
    if (parentItem) {
      const target = markColumn(state, parentItem);
      const changes = [];
      setLineIndent(state, item, target, changes);
      dispatchWithRenumber(view, changes);
      return true;
    }
    return deleteLine(view, line);
  }

  if (line.text.length > 0 && line.text.trim() === '' &&
      (neighborIsListItem(state.doc, line, -1) || neighborIsListItem(state.doc, line, 1))) {
    return deleteLine(view, line);
  }
  return false;
}

// Prec.highest: @codemirror/lang-markdown's own language support registers
// a high-precedence Enter/Backspace binding for list markup — the exact
// thing emptyItemEnter/emptyItemBackspace are replacing for the empty-item
// case — so a normal-precedence keymap.of(...) here would lose to it
// regardless of where it sits in the extensions array.
export const listIndentExtension = Prec.highest(
  keymap.of([
    { key: 'Enter', run: listEnter },
    { key: 'Backspace', run: emptyItemBackspace },
    { key: 'Tab', run: smartIndentMore },
    { key: 'Shift-Tab', run: smartIndentLess },
  ])
);

// Reused by cm-live/list-format.js (right-click "list" toggles) so it shares
// this file's syntax-tree list-item lookup and renumber-in-one-transaction
// logic instead of re-deriving it.
export { findListItemAt, listMarkOf, markColumn, contentColumn, dispatchWithRenumber };
