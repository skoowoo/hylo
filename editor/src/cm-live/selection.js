import { Facet } from '@codemirror/state';

// Reading view has no caret worth revealing raw markdown for.
export const readingMode = Facet.define({ combine: (v) => v.some(Boolean) });

// Compartment reconfigure isn't a doc/selection change, so selection-driven
// decorations only know to rebuild if they check this.
export function readingModeToggled(startState, state) {
  return startState.facet(readingMode) !== state.facet(readingMode);
}

// What shows raw follows the caret. Inline syntax and line-level rendering
// (marks, links, heading/quote markers, table rows, rules) reveal only around
// an empty selection, boundary included; a range (a drag, select-all) leaves
// them rendered, so text never reflows under a selection being made.
export function caretTouchesRange(state, from, to) {
  if (state.facet(readingMode)) return false;
  for (const range of state.selection.ranges) {
    if (range.empty && range.head >= from && range.head <= to) return true;
  }
  return false;
}

export function caretOnLine(state, line) {
  return caretTouchesRange(state, line.from, line.to);
}

// A block drawn as a widget (fence card, code fence chrome, frontmatter) opens
// while a selection's head is inside it, so extending a selection through its
// source keeps it open.
export function headInBlock(state, from, to) {
  if (state.facet(readingMode)) return false;
  for (const range of state.selection.ranges) {
    if (range.head >= from && range.head <= to) return true;
  }
  return false;
}

// Strict (boundary-exclusive) for widgets: a caret just before/after one keeps
// the rendered form; stepping inside reveals the source.
export function selectionInsideRange(state, from, to) {
  if (state.facet(readingMode)) return false;
  for (const range of state.selection.ranges) {
    if (range.from > from && range.to < to) return true;
  }
  return false;
}

// selected outline while staying rendered — a caret inside still reveals.
export function selectionCoversRange(state, from, to) {
  if (state.facet(readingMode)) return false;
  for (const range of state.selection.ranges) {
    if (!range.empty && range.from <= from && range.to >= to) return true;
  }
  return false;
}
