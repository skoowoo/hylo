import test from 'node:test';
import assert from 'node:assert/strict';
import { parseFields, noteName } from '../src/cm-live/fence-card/fields.js';

test('fields and items come apart; repeated keys keep every value', () => {
  const p = parseFields('title: 每日 AI 早报\nsummary: 每天 08:00: 放进收件箱\nnote: A\nnote: B\nempty:\n- 就这样\n- 不用了\nloose text');
  assert.equal(p.field('title'), '每日 AI 早报');
  assert.equal(p.field('summary'), '每天 08:00: 放进收件箱');
  assert.deepEqual(p.all('note'), ['A', 'B']);
  assert.equal(p.field('empty'), '');
  assert.deepEqual(p.items, ['就这样', '不用了']);
});

test('a note name survives the chat turning [[x]] into a link', () => {
  assert.equal(noteName('[[RAG 踩坑]]'), 'RAG 踩坑');
  assert.equal(noteName('[[RAG 踩坑|坑]]'), 'RAG 踩坑');
  assert.equal(noteName('[RAG 踩坑](/notes?name=' + encodeURIComponent('RAG 踩坑.md') + ')'), 'RAG 踩坑');
  assert.equal(noteName(' plain '), 'plain');
});
