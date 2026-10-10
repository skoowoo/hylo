// Auto-close code fences. An unclosed fence runs to the end of the document,
// so until the closing ``` exists everything below renders as code.
import { EditorView, keymap } from '@codemirror/view';
import { Prec } from '@codemirror/state';
import { resolveAncestor, nameIs } from './tree-utils.js';

const inCode = (state, pos) => !!resolveAncestor(state, pos, nameIs(['FencedCode', 'CodeBlock']), -1);

// Third backtick/tilde at the start of a line → write the closing fence too.
export function closeTypedFence(view, from, to, text) {
  if ((text !== '`' && text !== '~') || view.state.readOnly) return false;
  const state = view.state;
  const line = state.doc.lineAt(from);
  if (state.sliceDoc(to, line.to) !== '') return false;
  const m = /^([ \t]*(?:>[ \t]?)*[ \t]*)(``|~~)$/.exec(state.sliceDoc(line.from, from));
  if (!m || m[2][0] !== text || inCode(state, line.from)) return false;
  const fence = m[2] + text;
  view.dispatch({
    changes: { from, to, insert: text + '\n' + m[1] + fence },
    selection: { anchor: from + 1 },
    userEvent: 'input.type',
  });
  return true;
}

// Enter at the end of an opening fence that has no closing one yet.
export function fenceEnter(view) {
  const state = view.state;
  if (state.readOnly) return false;
  const sel = state.selection.main;
  if (!sel.empty) return false;
  const line = state.doc.lineAt(sel.head);
  if (sel.head !== line.to) return false;
  const m = /^([ \t]*(?:>[ \t]?)*[ \t]*)(`{3,}|~{3,})[^`]*$/.exec(line.text);
  if (!m) return false;
  // Must open here (not close an earlier block) and still be unclosed.
  const block = resolveAncestor(state, line.from + m[1].length, nameIs(['FencedCode']), 1);
  if (!block || state.doc.lineAt(block.from).number !== line.number) return false;
  if (block.getChildren('CodeMark').length >= 2) return false;
  const insert = '\n' + m[1] + '\n' + m[1] + m[2];
  view.dispatch({
    changes: { from: sel.head, insert },
    selection: { anchor: sel.head + 1 + m[1].length },
    scrollIntoView: true,
    userEvent: 'input',
  });
  return true;
}

export const fenceAutoClose = [EditorView.inputHandler.of(closeTypedFence), Prec.highest(keymap.of([{ key: 'Enter', run: fenceEnter }]))];
