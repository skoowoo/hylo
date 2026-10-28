import { Facet } from '@codemirror/state';

// Reading view has no caret worth revealing raw markdown for.
export const readingMode = Facet.define({ combine: (v) => v.some(Boolean) });

// Nor does the agent's caret: what it writes is watched rendered, and showing raw
// syntax where it types would reshape each block as the caret passed through.
export const agentCaret = Facet.define({ combine: (v) => v.some(Boolean) });

export const revealOff = (state) => state.facet(readingMode) || state.facet(agentCaret);

// Neither switch is a doc/selection change, so selection-driven decorations only
// know to rebuild if they check this.
export function revealToggled(startState, state) {
  return revealOff(startState) !== revealOff(state);
}

// A selection change only matters to what reveals while reveal is on; the agent's
// caret steps through a write once per reveal.
export function selectionMatters(tr) {
  return !!tr.selection && !(revealOff(tr.startState) && revealOff(tr.state));
}

// What shows raw follows the caret. Inline syntax and line-level rendering
// (marks, links, heading/quote markers, table rows, rules) reveal only around
// an empty selection, boundary included; a range (a drag, select-all) leaves
// them rendered, so text never reflows under a selection being made.
export function caretTouchesRange(state, from, to) {
  if (revealOff(state)) return false;
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
  if (revealOff(state)) return false;
  for (const range of state.selection.ranges) {
    if (range.head >= from && range.head <= to) return true;
  }
  return false;
}

// Strict (boundary-exclusive) for widgets: a caret just before/after one keeps
// the rendered form; stepping inside reveals the source.
export function selectionInsideRange(state, from, to) {
  if (revealOff(state)) return false;
  for (const range of state.selection.ranges) {
    if (range.from > from && range.to < to) return true;
  }
  return false;
}

// selected outline while staying rendered — a caret inside still reveals.
export function selectionCoversRange(state, from, to) {
  if (revealOff(state)) return false;
  for (const range of state.selection.ranges) {
    if (!range.empty && range.from <= from && range.to >= to) return true;
  }
  return false;
}
