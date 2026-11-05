// CM hit-tests one mousedown twice, and focuses the editor between the two.
// WKWebView then scrolls the old caret into view, so the same screen point
// lands lines apart and the press is stored as a selection covering the gap.
// Until the pointer moves the press keeps its first hit. The caret is placed
// there before focus, so that scroll follows the click. Otherwise this is
// CM's basicMouseSelection, which it doesn't export.
import { EditorSelection, findClusterBreak } from '@codemirror/state';
import { EditorView } from '@codemirror/view';

// CM's groupAt: a double-click selects the run of one character category,
// so spaces and punctuation select too, not just words.
export function groupAt(state, pos, bias = 1) {
  const categorize = state.charCategorizer(pos);
  const line = state.doc.lineAt(pos);
  const at = pos - line.from;
  if (line.length === 0) return EditorSelection.cursor(pos);
  if (at === 0) bias = 1;
  else if (at === line.length) bias = -1;
  let from = at;
  let to = at;
  if (bias < 0) from = findClusterBreak(line.text, at, false);
  else to = findClusterBreak(line.text, at);
  const cat = categorize(line.text.slice(from, to));
  while (from > 0) {
    const prev = findClusterBreak(line.text, from, false);
    if (categorize(line.text.slice(prev, from)) !== cat) break;
    from = prev;
  }
  while (to < line.length) {
    const next = findClusterBreak(line.text, to);
    if (categorize(line.text.slice(to, next)) !== cat) break;
    to = next;
  }
  return EditorSelection.range(from + line.from, to + line.from);
}

export function rangeAtClick(state, pos, assoc, type) {
  if (type === 1) return EditorSelection.cursor(pos, assoc);
  if (type === 2) return groupAt(state, pos, assoc);
  const line = state.doc.lineAt(pos);
  const to = line.to < state.doc.length ? line.to + 1 : line.to;
  return EditorSelection.range(line.from, to);
}

function removeRangeAround(selection, pos) {
  const i = selection.ranges.findIndex((r) => r.from <= pos && r.to >= pos);
  if (i < 0) return null;
  const main = selection.mainIndex === i ? 0 : selection.mainIndex - (selection.mainIndex > i ? 1 : 0);
  return EditorSelection.create(selection.ranges.filter((_, j) => j !== i), main);
}

export function pointerSelection(state, selection, start, cur, { type, extend, moved, multiple }) {
  const at = moved ? cur : start;
  let range = rangeAtClick(state, at.pos, at.assoc, type);
  if (moved && start.pos !== cur.pos && !extend) {
    const there = rangeAtClick(state, start.pos, start.assoc, type);
    const from = Math.min(there.from, range.from);
    const to = Math.max(there.to, range.to);
    range = from < range.from ? EditorSelection.range(from, to, range.assoc) : EditorSelection.range(to, from, range.assoc);
  }
  if (extend) return selection.replaceRange(selection.main.extend(range.from, range.to, range.assoc));
  if (multiple && type === 1 && selection.ranges.length > 1) {
    const removed = removeRangeAround(selection, at.pos);
    if (removed) return removed;
  }
  if (multiple) return selection.addRange(range);
  return EditorSelection.create([range]);
}

export function clickSelection() {
  return EditorView.mouseSelectionStyle.of((view, event) => {
    if (event.button !== 0) return null;
    const start = view.posAndSideAtCoords({ x: event.clientX, y: event.clientY }, false);
    if (!start) return null;
    const type = event.detail;
    let selection = view.state.selection;
    if (!view.hasFocus && !event.shiftKey && type === 1) {
      view.dispatch({ selection: EditorSelection.cursor(start.pos, start.assoc), userEvent: 'select.pointer' });
      selection = view.state.selection;
    }
    const x = event.clientX;
    const y = event.clientY;
    // Exact point, not a threshold: WebKit's mousemove after a scroll repeats
    // the press's point, while any real drag, however short, still selects.
    let moved = false;
    return {
      update(update) {
        if (!update.docChanged) return;
        start.pos = update.changes.mapPos(start.pos);
        selection = selection.map(update.changes);
      },
      get(event, extend, multiple) {
        if (event.clientX !== x || event.clientY !== y) moved = true;
        const cur = moved ? view.posAndSideAtCoords({ x: event.clientX, y: event.clientY }, false) : start;
        if (!cur) return selection;
        return pointerSelection(view.state, selection, start, cur, { type, extend, moved, multiple });
      },
    };
  });
}
