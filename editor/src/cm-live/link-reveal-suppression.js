// Hold off decorateLink reveal while a click-to-open is pending.
// mousedown can't preventDefault (breaks drag-select), so the caret briefly
// touches the link — without this, raw "[text](url)" flashes before mouseup.
// StateField (not a module var) so it stays in CM6's transaction pipeline.
import { StateField, StateEffect } from '@codemirror/state';

export const setSuppressedLinkReveal = StateEffect.define();

export const suppressedLinkRevealField = StateField.define({
  create: () => null, // { from, to } | null
  update(value, tr) {
    for (const e of tr.effects) {
      if (e.is(setSuppressedLinkReveal)) value = e.value;
    }
    // Doc edit invalidates the range; also heals a missed mouseup clear.
    return tr.docChanged ? null : value;
  },
});

export function isRevealSuppressed(state, from, to) {
  const range = state.field(suppressedLinkRevealField, false);
  return !!range && range.from === from && range.to === to;
}
