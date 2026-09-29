// No DOM needed — @codemirror/state + @lezer/markdown build and resolve a
// real syntax tree headlessly. These pin down the exact distinction whose
// absence let decorators.js's list-gap collapse match *any* ListItem in the
// document instead of one belonging to the same List (see tree-utils.js).
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { EditorState } from '@codemirror/state';
import { markdown } from '@codemirror/lang-markdown';
import { syntaxTree } from '@codemirror/language';
import { nearestAncestor, nameIs, resolveAncestor, isSameNode } from '../src/cm-live/tree-utils.js';

function stateFor(doc) {
  return EditorState.create({ doc, extensions: [markdown()] });
}

test('resolveAncestor finds the nearest ListItem containing a position', () => {
  const state = stateFor('- a\n- b\n');
  const item = resolveAncestor(state, 2, nameIs(['ListItem']), -1);
  assert.equal(item.name, 'ListItem');
});

test('resolveAncestor returns null when no ancestor matches', () => {
  const state = stateFor('plain text\n');
  const item = resolveAncestor(state, 2, nameIs(['ListItem']), -1);
  assert.equal(item, null);
});

test('an ordered list and a following bullet list are distinct List nodes, even with no blank line', () => {
  const state = stateFor('1. one\n- two\n');
  const lists = [];
  syntaxTree(state).iterate({
    enter(n) { if (n.name === 'OrderedList' || n.name === 'BulletList') lists.push(n.node); },
  });
  assert.equal(lists.length, 2);
  assert.equal(isSameNode(lists[0], lists[1]), false);
});

test('isSameNode: a node is the same as itself, not as a same-named sibling', () => {
  const state = stateFor('- a\n\n- b\n');
  const items = [];
  syntaxTree(state).iterate({ enter(n) { if (n.name === 'ListItem') items.push(n.node); } });
  assert.equal(items.length, 2);
  assert.equal(isSameNode(items[0], items[0]), true);
  assert.equal(isSameNode(items[0], items[1]), false);
});

test('nearestAncestor walks a plain node parent chain by predicate', () => {
  const state = stateFor('- a\n  - b\n');
  let innerList = null;
  syntaxTree(state).iterate({
    enter(n) { if (n.name === 'BulletList' && n.from > 0) innerList = n.node; },
  });
  const outerList = nearestAncestor(innerList.parent, nameIs(['BulletList']));
  assert.equal(outerList.name, 'BulletList');
  assert.equal(isSameNode(outerList, innerList), false);
});
