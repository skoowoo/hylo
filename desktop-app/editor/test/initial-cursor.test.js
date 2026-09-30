import { test } from 'node:test';
import assert from 'node:assert/strict';
import { initialCursorOffset } from '../src/cm-live/initial-cursor.js';

test('empty doc → offset 0', () => {
  assert.equal(initialCursorOffset(''), 0);
});

test('plain paragraph, no frontmatter or marker → offset 0', () => {
  assert.equal(initialCursorOffset('hello world'), 0);
});

test('skips frontmatter, lands at start of body', () => {
  const doc = '---\ntitle: foo\n---\nhello world';
  assert.equal(initialCursorOffset(doc), doc.indexOf('hello'));
});

test('frontmatter followed by a blank line, then a body line', () => {
  const doc = '---\ntitle: foo\n---\n\nhello world';
  assert.equal(initialCursorOffset(doc), doc.indexOf('hello'));
});

test('skips a bullet marker after frontmatter', () => {
  const doc = '---\ntitle: foo\n---\n- first item';
  assert.equal(initialCursorOffset(doc), doc.indexOf('first item'));
});

test('skips an ordered list marker', () => {
  const doc = '1. first item';
  assert.equal(initialCursorOffset(doc), doc.indexOf('first item'));
});

test('skips a task marker', () => {
  const doc = '- [ ] do the thing';
  assert.equal(initialCursorOffset(doc), doc.indexOf('do the thing'));
});

test('skips a checked task marker', () => {
  const doc = '- [x] done thing';
  assert.equal(initialCursorOffset(doc), doc.indexOf('done thing'));
});

test('skips a heading marker', () => {
  const doc = '## Section title';
  assert.equal(initialCursorOffset(doc), doc.indexOf('Section title'));
});

test('no frontmatter, no marker → offset 0', () => {
  const doc = 'just a normal first line\nmore text';
  assert.equal(initialCursorOffset(doc), 0);
});

test('frontmatter with no body at all → end of doc', () => {
  const doc = '---\ntitle: foo\n---\n';
  assert.equal(initialCursorOffset(doc), doc.length);
});

test('frontmatter with trailing blank lines only → end of doc', () => {
  const doc = '---\ntitle: foo\n---\n\n\n';
  assert.equal(initialCursorOffset(doc), doc.length);
});

test('indented list marker inside a nested item', () => {
  const doc = '  - nested item';
  assert.equal(initialCursorOffset(doc), doc.indexOf('nested item'));
});
