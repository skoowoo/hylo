import test from 'node:test';
import assert from 'node:assert/strict';
import { parseFrontmatter } from '../src/cm-live/frontmatter-yaml.js';

const byKey = (text) => Object.fromEntries(parseFrontmatter(text).map((f) => [f.key, f.value]));

test('scalars: quotes stripped, colons and urls kept, comments dropped', () => {
  const v = byKey('title: "Quoted: with colon"\nsource: https://a.b/c#x\nrating: 4.5 # stars\nname: \'it\'\'s\'\nnone: ~');
  assert.deepEqual(v.title, { type: 'scalar', text: 'Quoted: with colon', quoted: true });
  assert.equal(v.source.text, 'https://a.b/c#x');
  assert.deepEqual(v.rating, { type: 'scalar', text: '4.5', quoted: false });
  assert.equal(v.name.text, "it's");
  assert.deepEqual(v.none, { type: 'empty' });
});

test('keys may be CJK, contain spaces, or be quoted; comment lines are skipped', () => {
  const f = parseFrontmatter('创建日期: 2026-10-01\n# note\nCreated date: x\n"a: b": y');
  assert.deepEqual(f.map((x) => x.key), ['创建日期', 'Created date', 'a: b']);
});

test('flow and block lists, including items at the key column and blank gaps', () => {
  const v = byKey('tags: [ml, "a, b", \'c\']\naliases:\n  - One\n\n  - "[[Two]]"\nflat:\n- x\n- y\nempty: []');
  assert.deepEqual(v.tags.items, ['ml', 'a, b', 'c']);
  assert.deepEqual(v.aliases.items, ['One', '[[Two]]']);
  assert.deepEqual(v.flat.items, ['x', 'y']);
  assert.deepEqual(v.empty, { type: 'list', items: [] });
});

test('block scalars keep or fold lines; nested maps become entries', () => {
  const v = byKey('lit: |\n  one\n  two\nfold: >-\n  one\n  two\n\n  three\nmeta:\n  author: skoo\n  version: 2\nblank:');
  assert.equal(v.lit.text, 'one\ntwo');
  assert.equal(v.fold.text, 'one two\n\nthree');
  assert.deepEqual(v.meta.entries, [{ key: 'author', text: 'skoo' }, { key: 'version', text: '2' }]);
  assert.deepEqual(v.blank, { type: 'empty' });
});

test('a line that is not a key keeps its raw text', () => {
  assert.deepEqual(parseFrontmatter('just text'), [{ key: '', value: { type: 'text', text: 'just text' } }]);
});
