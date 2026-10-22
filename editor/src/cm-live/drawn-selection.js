import { drawSelection, EditorView, ViewPlugin } from '@codemirror/view';

// WebKit selects the IME's marked text while composing. That range is the
// composition, not a selection the user made, so it isn't painted. CM's own
// state, not composition events: WebKit can commit without a compositionend.
const composingSelectionHidden = [
  ViewPlugin.define(() => ({
    update(update) {
      update.view.scrollDOM.classList.toggle('cm-composing', update.view.compositionStarted);
    },
  })),
  EditorView.baseTheme({ '.cm-composing > .cm-selectionLayer': { display: 'none' } }),
];

export function drawnSelection() {
  return [drawSelection(), composingSelectionHidden];
}
