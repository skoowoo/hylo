import test from 'node:test';
import assert from 'node:assert/strict';
import { embedURLs } from '../src/cm-live/fence-card/embed.js';

test('an embed body is one http(s) URL per line', () => {
  assert.deepEqual(embedURLs('https://Example.com/a'), ['https://example.com/a']);
  assert.deepEqual(embedURLs('\nhttps://example.com/a?b=1\n'), ['https://example.com/a?b=1']);
  assert.deepEqual(embedURLs('https://example.com/a\n\nhttps://example.com/b'), [
    'https://example.com/a',
    'https://example.com/b',
  ]);
  assert.deepEqual(embedURLs('not a url'), []);
  assert.deepEqual(embedURLs('https://example.com/a\nsee note'), []);
  assert.deepEqual(embedURLs('ftp://example.com/a'), []);
  assert.deepEqual(embedURLs(''), []);
});
