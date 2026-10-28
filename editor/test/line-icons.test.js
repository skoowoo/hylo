// Icon names only resolve when a widget renders, so a typo would surface at
// runtime; check every literal name in the editor against the generated set.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, readdirSync } from 'node:fs';
import { join } from 'node:path';
import { lineIcon } from '../src/line-icons.js';

const SRC = new URL('../src/', import.meta.url).pathname;
const USE = /\b(?:lineIcon|lineIconMask|iconButton|group)\('([^']+)'/g;

function files(dir) {
  return readdirSync(dir, { withFileTypes: true }).flatMap((e) =>
    e.isDirectory() ? files(join(dir, e.name)) : e.name.endsWith('.js') && e.name !== 'line-icons.js' ? [join(dir, e.name)] : []);
}

test('every icon name the editor uses exists', () => {
  let seen = 0;
  for (const f of files(SRC)) {
    for (const m of readFileSync(f, 'utf8').matchAll(USE)) {
      seen++;
      assert.doesNotThrow(() => lineIcon(m[1]), `${f}: no line icon ${m[1]}`);
    }
  }
  assert.ok(seen >= 25, `found only ${seen} icon uses; the pattern is out of date`);
});
