// Frontmatter is read-only in the live-preview view — decorateFrontmatter
// (decorators.js) always renders the decorated/collapsed form now, never
// raw syntax on caret entry, and typing into it should be a no-op rather
// than silently corrupting a YAML block the decorations assume is
// well-formed. The only way to edit it is the "Metadata" header's edit
// button (frontmatter-collapse.js), which hands the raw YAML to an
// app-level dialog (content_pane.js) and writes the result back through a
// transaction annotated with allowFrontmatterEdit — the one escape hatch
// this filter grants.
import { EditorState, EditorSelection, Annotation } from '@codemirror/state';
import { findFrontmatterNode } from './frontmatter-syntax.js';

export const allowFrontmatterEdit = Annotation.define();
// For the one caret move that should land inside: "+N more" expanding a list.
export const allowFrontmatterCaret = Annotation.define();

// A caret inside the block is invisible (most rows are hairlines) and can't
// type, so arrowing up from the body would just lose it. Push it back out.
const keepCaretOut = EditorState.transactionFilter.of((tr) => {
  if (!tr.selection || tr.docChanged || tr.annotation(allowFrontmatterCaret)) return tr;
  const node = findFrontmatterNode(tr.startState);
  if (!node || node.to >= tr.newDoc.length) return tr;
  const bodyStart = node.to + 1;
  const sel = tr.newSelection;
  if (!sel.ranges.some((r) => r.empty && r.head < bodyStart)) return tr;
  const ranges = sel.ranges.map((r) => (r.empty && r.head < bodyStart ? EditorSelection.cursor(bodyStart) : r));
  return [tr, { selection: EditorSelection.create(ranges, sel.mainIndex), sequential: true }];
});

export function frontmatterReadOnly() {
  return [keepCaretOut, EditorState.changeFilter.of((tr) => {
    if (tr.annotation(allowFrontmatterEdit)) return true;
    // changeFilter runs on every edit transaction — an O(1) lookup here
    // (Frontmatter is always the tree's first top-level node, if present)
    // instead of a full-tree walk on every keystroke.
    const node = findFrontmatterNode(tr.startState);
    if (!node) return true;
    // changeFilter's array return is the SUPPRESSED range(s), not the
    // allowed ones (easy to get backwards — flip this and frontmatter
    // becomes the only *editable* part of the document).
    return [0, node.to];
  })];
}
