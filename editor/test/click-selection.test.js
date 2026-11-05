import { test } from 'node:test';
import assert from 'node:assert/strict';
import { EditorState, EditorSelection } from '@codemirror/state';
import { pointerSelection } from '../src/cm-live/click-selection.js';

function state(text, anchor = 0, head = anchor) {
  return EditorState.create({ doc: text, selection: EditorSelection.range(anchor, head) });
}

const hit = (pos) => ({ pos, assoc: 1 });

test('a press whose two hit-tests disagree stays a caret at the first', () => {
  const s = state('x'.repeat(2000), 148);
  const sel = pointerSelection(s, s.selection, hit(1312), hit(172), { type: 1, extend: false, moved: false, multiple: false });
  assert.equal(sel.main.empty, true);
  assert.equal(sel.main.head, 1312);
});

test('a drag selects from the first hit to the pointer', () => {
  const s = state('x'.repeat(200), 0);
  const sel = pointerSelection(s, s.selection, hit(10), hit(40), { type: 1, extend: false, moved: true, multiple: false });
  assert.equal(sel.main.anchor, 10);
  assert.equal(sel.main.head, 40);
});

test('a drag upward keeps the anchor at the press', () => {
  const s = state('x'.repeat(200), 0);
  const sel = pointerSelection(s, s.selection, hit(40), hit(10), { type: 1, extend: false, moved: true, multiple: false });
  assert.equal(sel.main.anchor, 40);
  assert.equal(sel.main.head, 10);
});

test('an unmoved double-click selects the word under the press, not the second hit', () => {
  const s = state('hello world\nnext line', 0);
  const sel = pointerSelection(s, s.selection, hit(1), hit(14), { type: 2, extend: false, moved: false, multiple: false });
  assert.equal(s.sliceDoc(sel.main.from, sel.main.to), 'hello');
});

test('shift-click extends to the press when the pointer has not moved', () => {
  const s = state('x'.repeat(200), 20);
  const sel = pointerSelection(s, s.selection, hit(80), hit(5), { type: 1, extend: true, moved: false, multiple: false });
  assert.equal(sel.main.anchor, 20);
  assert.equal(sel.main.head, 80);
});

test('a one-character drag selects that character', () => {
  const s = state('hello world', 0);
  const sel = pointerSelection(s, s.selection, hit(4), hit(5), { type: 1, extend: false, moved: true, multiple: false });
  assert.equal(s.sliceDoc(sel.main.from, sel.main.to), 'o');
});

test('a double-click on punctuation or spaces selects the run, as CM does', () => {
  const s = state('a，，b   c', 0);
  const punct = pointerSelection(s, s.selection, hit(1), hit(1), { type: 2, extend: false, moved: false, multiple: false });
  assert.equal(s.sliceDoc(punct.main.from, punct.main.to), '，，');
  const space = pointerSelection(s, s.selection, hit(5), hit(5), { type: 2, extend: false, moved: false, multiple: false });
  assert.equal(s.sliceDoc(space.main.from, space.main.to), '   ');
});

test('a multi-cursor click on an existing cursor removes it', () => {
  const s = EditorState.create({
    doc: 'x'.repeat(50),
    selection: EditorSelection.create([EditorSelection.cursor(5), EditorSelection.cursor(20)], 1),
    extensions: EditorState.allowMultipleSelections.of(true),
  });
  const sel = pointerSelection(s, s.selection, hit(20), hit(20), { type: 1, extend: false, moved: false, multiple: true });
  assert.equal(sel.ranges.length, 1);
  assert.equal(sel.main.head, 5);
});
