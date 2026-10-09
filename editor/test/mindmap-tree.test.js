import test from 'node:test';
import assert from 'node:assert/strict';
import { parseOutline } from '../src/cm-live/fence-card/mindmap-tree.js';

test('headings and nested lists build one tree', () => {
  const root = parseOutline('# R\n## A\n- a1\n  - a1x\n- a2\n## B\n');
  assert.equal(root.content, 'R');
  assert.deepEqual(root.children.map((c) => c.content), ['A', 'B']);
  assert.deepEqual(root.children[0].children.map((c) => c.content), ['a1', 'a2']);
  assert.equal(root.children[0].children[0].children[0].content, 'a1x');
});

test('multiple top-level nodes get a synthetic root; html is escaped', () => {
  const root = parseOutline('- <b>x</b>\n- y');
  assert.equal(root.content, '');
  assert.equal(root.children[0].content, '&lt;b&gt;x&lt;/b&gt;');
});

test('quotes in a link target cannot break out of href', () => {
  const root = parseOutline(`- [x](https://a"onmouseover="location='javascript:alert%281%29')`);
  assert.doesNotMatch(root.content, /"onmouseover=/);
  assert.match(root.content, /href="https:\/\/a&quot;onmouseover=&quot;location=&#39;/);
});
