import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { EditorState } from '@codemirror/state';
import { ensureSyntaxTree } from '@codemirror/language';
import { wikiMarkdownLanguage } from '../src/cm-live/index.js';
import { tagKey } from '../src/cm-live/tag-syntax.js';

const lang = wikiMarkdownLanguage();

function inlineTags(doc) {
  const state = EditorState.create({ doc, extensions: [lang] });
  const out = [];
  ensureSyntaxTree(state, doc.length, 5000).iterate({
    enter(node) {
      if (node.name !== 'HashTag') return;
      const key = tagKey(state.sliceDoc(node.from, node.to));
      if (!out.includes(key)) out.push(key);
    },
  });
  return out;
}

// Shared with the Go side (util.InlineTags), so server and editor agree.
const cases = JSON.parse(readFileSync(new URL('../../internal/util/testdata/tags.json', import.meta.url)));

for (const c of cases) {
  test(`HashTag: ${JSON.stringify(c.md)}`, () => {
    assert.deepEqual(inlineTags(c.md), c.tags);
  });
}

test('HashTag: front matter is not body text', () => {
  assert.deepEqual(inlineTags('---\ntitle: "#nope"\n---\n#yes'), ['yes']);
});
