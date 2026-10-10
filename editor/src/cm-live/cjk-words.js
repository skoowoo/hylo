// CM treats a whole CJK run as one word; Intl.Segmenter finds real word boundaries.
import { EditorView } from '@codemirror/view';
import { EditorSelection } from '@codemirror/state';

const CJK_RE = /[\u3040-\u30ff\u3400-\u4dbf\u4e00-\u9fff\uf900-\ufaff\uac00-\ud7af]/;
const segmenter = typeof Intl !== 'undefined' && Intl.Segmenter ? new Intl.Segmenter(undefined, { granularity: 'word' }) : null;

// Position of the character under the pointer. posAtCoords snaps to the
// nearest boundary, which over a character's right half is the next one.
export function charPosAtCoords(view, x, y) {
  const pos = view.posAtCoords({ x, y });
  if (pos == null || pos === 0) return pos;
  const c = view.coordsAtPos(pos, 1);
  return c && x < c.left && view.state.doc.lineAt(pos).from < pos ? pos - 1 : pos;
}

// Word range containing the character at `pos`.
export function wordRangeAt(state, pos) {
  const line = state.doc.lineAt(pos);
  const at = pos - line.from;
  if (!CJK_RE.test(line.text[at] || '') || !segmenter) return state.wordAt(pos);
  for (const seg of segmenter.segment(line.text)) {
    if (at >= seg.index && at < seg.index + seg.segment.length) {
      return seg.isWordLike ? EditorSelection.range(line.from + seg.index, line.from + seg.index + seg.segment.length) : null;
    }
  }
  return null;
}

// Double-click on CJK text selects the segmented word. Drag-extending by
// words after a double-click is CM's and stays categorizer-based.
export const cjkWordSelection = EditorView.domEventHandlers({
  mousedown(e, view) {
    if (e.detail !== 2 || e.button !== 0 || e.shiftKey || e.altKey || e.metaKey || e.ctrlKey) return false;
    const pos = charPosAtCoords(view, e.clientX, e.clientY);
    if (pos == null || !CJK_RE.test(view.state.sliceDoc(pos, pos + 1))) return false;
    const word = wordRangeAt(view.state, pos);
    if (!word) return false;
    e.preventDefault();
    view.dispatch({ selection: word, userEvent: 'select.pointer' });
    view.focus();
    return true;
  },
});
