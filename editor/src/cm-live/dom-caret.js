// WebKit keeps painting the caret at its old x when a line shifts sideways
// but the DOM selection is unchanged; re-adding the range forces a relayout.
export function refreshDomCaret(view) {
  if (!view.hasFocus || view.composing) return;
  const sel = view.root.getSelection ? view.root.getSelection() : document.getSelection();
  if (!sel || !sel.rangeCount) return;
  const range = sel.getRangeAt(0).cloneRange();
  sel.removeAllRanges();
  sel.addRange(range);
}
