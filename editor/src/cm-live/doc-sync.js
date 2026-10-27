// Brings the open doc up to the text on disk with one minimal change, so the
// caret, scroll position, undo history and every widget outside the change
// survive. A whole new state would remount every card and reset the caret.
import { Annotation, Transaction } from '@codemirror/state';

// Marks the change as already saved: the host's autosave skips it.
export const docSynced = Annotation.define();

export function syncDoc(view, text) {
  const cur = view.state.doc.toString();
  if (cur === text) return false;
  const max = Math.min(cur.length, text.length);
  let start = 0;
  while (start < max && cur.charCodeAt(start) === text.charCodeAt(start)) start++;
  let end = 0;
  while (end < max - start && cur.charCodeAt(cur.length - 1 - end) === text.charCodeAt(text.length - 1 - end)) end++;
  view.dispatch({
    changes: { from: start, to: cur.length - end, insert: text.slice(start, text.length - end) },
    annotations: [docSynced.of(true), Transaction.addToHistory.of(false)],
  });
  return true;
}
