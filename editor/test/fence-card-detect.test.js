import test from 'node:test';
import assert from 'node:assert/strict';
import { Text } from '@codemirror/state';
import { parser } from '@lezer/markdown';
import { cardFence } from '../src/cm-live/fence-card/registry.js';

function fences(src) {
  const doc = Text.of(src.split('\n'));
  const out = [];
  parser.parse(src).iterate({
    enter(n) {
      if (n.name !== 'FencedCode') return;
      out.push(cardFence(doc, n.node));
      return false;
    },
  });
  return out;
}

test('top-level fence keeps its body as written', () => {
  const [f] = fences('```mindmap\n# R\n- a\n```');
  assert.equal(f.info, 'mindmap');
  assert.equal(f.body, '# R\n- a');
});

test('indentation and quote markers are stripped from nested fences', () => {
  assert.equal(fences('- item\n\n  ```mindmap\n  # R\n    - a\n  ```')[0].body, '# R\n  - a');
  assert.equal(fences('> ```mindmap\n> # R\n> - a\n> ```')[0].body, '# R\n- a');
});

test('unregistered, unclosed and bullet-line fences are not cards', () => {
  assert.deepEqual(fences('```js\nx\n```'), [null]);
  assert.deepEqual(fences('```mindmap\n# R'), [null]);
  assert.deepEqual(fences('- ```mindmap\n  # R\n  ```'), [null]);
});
