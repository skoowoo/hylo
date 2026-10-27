// Focus handed back to the editor: an overlay closing, Esc, a mode switch, a note opening.
// WKWebView scrolls a contenteditable's caret into view (centred) whenever it gains focus —
// preventScroll or not, through CM's view.focus() or natively — so with the caret scrolled
// away the note jumped back to it. The editor only takes focus back where that can't move
// the view, with its caret on screen; otherwise focus stays out until a click places the
// caret. Returns whether the editor has focus. For a caret just placed, view.focus() is right.
export function focusInPlace(view) {
  if (view.hasFocus) return true;
  if (!caretOnScreen(view)) return false;
  view.focus();
  return true;
}

// CM keeps the caret's line drawn even when scrolled away, so its coords are real.
function caretOnScreen(view) {
  const caret = view.coordsAtPos(view.state.selection.main.head);
  if (!caret) return false;
  const box = view.scrollDOM.getBoundingClientRect();
  return caret.bottom > box.top && caret.top < box.bottom;
}
