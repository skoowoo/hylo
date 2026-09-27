import { findListItemAt, listMarkOf, contentColumn, dispatchWithRenumber } from './list-indent.js';
import { touchedLines } from './selection-lines.js';
import { lineInsideCodeBlock } from './format-guards.js';

// Bullet/ordered/task list toggle commands for the right-click format menu
// (format-menu.js). Reuses list-indent.js's syntax-tree list-item lookup
// (findListItemAt/listMarkOf/contentColumn) and its renumber-in-one-
// transaction helper (dispatchWithRenumber) instead of re-deriving either —
// that file already solved the "keep Tab/Shift-Tab and ordered-list
// renumbering in a single undo step" problem this needs too.

// "Task" is a modifier layered on top of any ListItem regardless of its
// underlying marker (@lezer/markdown's TaskList extension parses `[ ] `
// right after a bullet OR an ordered marker) — so a task item's own
// bullet/ordered-ness is irrelevant to classification here. That collapses
// naturally into three flat, mutually exclusive kinds, matching the menu's
// three list actions.
function itemKind(state, item) {
  if (!item) return null;
  if (item.getChild('Task')) return 'task';
  const mark = listMarkOf(item);
  if (!mark) return null;
  return /^\d/.test(state.doc.sliceString(mark.from, mark.to)) ? 'ordered' : 'bullet';
}

// Absolute position where this item's real text content starts — past the
// structural marker AND, for a task item, past its "[ ] "/"[x] " checkbox.
// contentColumn (list-indent.js) only skips the marker: a Task node's own
// children (TaskMarker + inline content) are parsed as paragraph content
// starting exactly at that column, not part of the structural marker
// CommonMark measures indentation against — see list-indent.js's comment on
// contentColumn for why. Mirrors the checkbox-skipping loop in
// list-indent.js's emptyItemEnter.
function itemContentStart(state, item) {
  const mark = listMarkOf(item);
  if (!mark) return null;
  const line = state.doc.lineAt(mark.from);
  const cCol = contentColumn(state, item);
  if (cCol == null) return null;
  let pos = line.from + cCol;
  const task = item.getChild('Task');
  const taskMarker = task && task.getChild('TaskMarker');
  if (taskMarker) {
    pos = taskMarker.to;
    while (pos < line.to && state.doc.sliceString(pos, pos + 1) === ' ') pos++;
  }
  return pos;
}

function canonicalPrefix(kind) {
  if (kind === 'ordered') return '1. '; // placeholder number — dispatchWithRenumber fixes it up
  if (kind === 'task') return '- [ ] ';
  return '- ';
}

// All-or-nothing: only strips the list when EVERY touched line is already
// exactly `kind` — otherwise every touched line is force-set to `kind`
// (replacing whatever marker/checkbox it had), never a partial toggle. A
// mixed selection (one bullet line, one plain, one ordered) clicking
// "Ordered List" always turns all three into ordered items, not a confusing
// per-line toggle that leaves the already-ordered one alone.
function applyListKind(view, kind) {
  const state = view.state;
  const lines = touchedLines(state);
  // A fenced/indented code block's lines are verbatim — prefixing one with
  // "- "/"1. "/"- [ ] " doesn't make it a list item, it just inserts those
  // literal characters into the code, so treat it as nothing to do here.
  if (lines.some(line => lineInsideCodeBlock(state, line))) return true;
  const infos = lines.map(line => {
    const item = findListItemAt(state, line.to);
    return { line, item, kind: itemKind(state, item) };
  });
  const isTarget = k => (kind === 'task' ? k === 'task' : k === kind);
  const stripAll = infos.every(info => isTarget(info.kind));

  const changes = [];
  for (const info of infos) {
    if (stripAll) {
      if (!info.item) continue;
      const mark = listMarkOf(info.item);
      const contentStart = itemContentStart(state, info.item);
      if (mark && contentStart != null) changes.push({ from: mark.from, to: contentStart, insert: '' });
      continue;
    }
    if (info.item) {
      const mark = listMarkOf(info.item);
      const contentStart = itemContentStart(state, info.item);
      if (mark && contentStart != null) changes.push({ from: mark.from, to: contentStart, insert: canonicalPrefix(kind) });
    } else {
      const text = info.line.text;
      const indentLen = text.length - text.replace(/^[ \t]*/, '').length;
      const pos = info.line.from + indentLen;
      changes.push({ from: pos, to: pos, insert: canonicalPrefix(kind) });
    }
  }
  if (!changes.length) return true;
  dispatchWithRenumber(view, changes);
  return true;
}

export function toggleBulletList(view) { return applyListKind(view, 'bullet'); }
export function toggleOrderedList(view) { return applyListKind(view, 'ordered'); }
export function toggleTaskList(view) { return applyListKind(view, 'task'); }

// For the format menu's active-state highlighting.
export function listFormatActiveAt(state, range) {
  const line = state.doc.lineAt(range.head);
  const item = findListItemAt(state, line.to);
  return itemKind(state, item) || 'none';
}
