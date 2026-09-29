// Run with: node --test internal/server/view/assets/
//
// Regression test for the bug found alongside decorators.js's list-gap fix:
// this transform ran on every note load and silently merged two distinct,
// CommonMark-legal lists (different marker family) separated by one blank
// line, and the merge was re-saved to disk 800ms later by the normal
// autosave path. No CodeMirror/DOM needed — it's a pure string transform.
const test = require('node:test');
const assert = require('node:assert/strict');
const { __vaultrEditorTightenLists } = require('./list_tighten.js');

test('leaves a blank line between an ordered list and a following bullet list', () => {
  assert.equal(__vaultrEditorTightenLists('1. one\n\n- two\n'), '1. one\n\n- two\n');
});

test('leaves a blank line between a bullet list and a following ordered list', () => {
  assert.equal(__vaultrEditorTightenLists('- one\n\n1. two\n'), '- one\n\n1. two\n');
});

test('leaves a blank line between lists using different bullet characters', () => {
  assert.equal(__vaultrEditorTightenLists('- a\n\n* b\n'), '- a\n\n* b\n');
});

test('leaves a blank line between ordered lists using different delimiters', () => {
  assert.equal(__vaultrEditorTightenLists('1. a\n\n1) b\n'), '1. a\n\n1) b\n');
});

test('tightens a stray blank line within the same bullet list', () => {
  assert.equal(__vaultrEditorTightenLists('- a\n\n- b\n'), '- a\n- b\n');
});

test('tightens stray blank lines across a whole same-type ordered list', () => {
  assert.equal(__vaultrEditorTightenLists('1. a\n\n2. b\n\n3. c\n'), '1. a\n2. b\n3. c\n');
});

test('leaves non-list text untouched', () => {
  const md = 'Some prose.\n\nMore prose.\n';
  assert.equal(__vaultrEditorTightenLists(md), md);
});
