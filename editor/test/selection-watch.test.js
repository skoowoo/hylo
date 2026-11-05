import { test } from 'node:test';
import assert from 'node:assert/strict';
import { EditorState, EditorSelection } from '@codemirror/state';
import { selectionJumped } from '../src/cm-live/selection-watch.js';

function doc(text) {
  return EditorState.create({ doc: text }).doc;
}

const cursor = (n) => EditorSelection.cursor(n);
const range = (from, to) => EditorSelection.range(from, to);

test('a caret moving within a line is not a jump', () => {
  const d = doc('hello world');
  assert.equal(selectionJumped(d, cursor(0), d, cursor(5)), false);
});

test('a short same-line selection is not a jump', () => {
  const d = doc('hello world, this is a sentence');
  assert.equal(selectionJumped(d, cursor(0), d, range(0, 20)), false);
});

test('a long same-line selection is a jump', () => {
  const d = doc('x'.repeat(200));
  assert.equal(selectionJumped(d, cursor(0), d, range(0, 119)), false);
  assert.equal(selectionJumped(d, cursor(0), d, range(0, 120)), true);
});

test('a selection that crosses a line break is a jump', () => {
  const d = doc('a\nb');
  assert.equal(selectionJumped(d, cursor(0), d, range(0, 3)), true);
});

test('a single line including its trailing newline is not a jump', () => {
  const d = doc('hello\nworld');
  assert.equal(selectionJumped(d, cursor(0), d, range(0, 6)), false);
});

test('extending a selection that is already large is not a new jump', () => {
  const d = doc('a\nb\nc');
  assert.equal(selectionJumped(d, range(0, 3), d, range(0, 5)), false);
});

test('collapsing a large selection is not a jump', () => {
  const d = doc('a\nb');
  assert.equal(selectionJumped(d, range(0, 3), d, cursor(3)), false);
});
