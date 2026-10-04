import { Facet } from '@codemirror/state';

// Reading view has no caret worth revealing raw markdown for.
export const readingMode = Facet.define({ combine: (v) => v.some(Boolean) });

// Compartment reconfigure isn't a doc/selection change, so selection-driven
// decorations only know to rebuild if they check this.
export function readingModeToggled(startState, state) {
  return startState.facet(readingMode) !== state.facet(readingMode);
}

// Inclusive: boundary caret counts — reveal raw markdown at the edge.
export function selectionTouchesRange(state, from, to) {
  if (state.facet(readingMode)) return false;
  for (const range of state.selection.ranges) {
    if (range.from <= to && range.to >= from) return true;
  }
  return false;
}

export function selectionTouchesLine(state, line) {
  return selectionTouchesRange(state, line.from, line.to);
}

// Strict (boundary-exclusive) for atomic widgets — inclusive would un-atomize
// on the same caret atomicRanges protects, breaking one-keystroke delete/skip.
export function selectionInsideRange(state, from, to) {
  if (state.facet(readingMode)) return false;
  for (const range of state.selection.ranges) {
    if (range.from > from && range.to < to) return true;
  }
  return false;
}
