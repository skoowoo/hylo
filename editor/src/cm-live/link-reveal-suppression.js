// Hold off decorateLink reveal for a link being clicked open. mousedown can't
// preventDefault (breaks drag-select), so the caret lands in the link; the
// link stays rendered until the click turns out to be a drag, or the caret
// moves on by keyboard or the doc changes.
import { StateField, StateEffect } from '@codemirror/state';

export const setSuppressedLinkReveal = StateEffect.define();

export const suppressedLinkRevealField = StateField.define({
  create: () => null, // { from, to } | null
  update(value, tr) {
    for (const e of tr.effects) {
      if (e.is(setSuppressedLinkReveal)) return e.value;
    }
    if (tr.docChanged || (tr.selection && !tr.isUserEvent('select.pointer'))) return null;
    return value;
  },
});

export function isRevealSuppressed(state, from, to) {
  const range = state.field(suppressedLinkRevealField, false);
  return !!range && range.from === from && range.to === to;
}
