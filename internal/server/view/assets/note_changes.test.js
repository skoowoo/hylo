const test = require('node:test');
const assert = require('node:assert/strict');
const { affect } = require('./note_changes.js');

const keys = (o) => Object.keys(o).sort();

test('a new note anywhere refreshes the sidebar and the lists it shows in', () => {
  const a = affect([{ op: 'create', path: '/Projects/plan.md' }]);
  assert.equal(a.sidebar, true);
  assert.deepEqual(keys(a.refetch), ['folder', 'pinned', 'recent', 'tags']);
  const k = affect([{ op: 'create', path: '/_knowledge/ai/agents.md' }]);
  assert.deepEqual(keys(k.refetch), ['knowledge']);
});

test('the open note saving only drops cached derived lists', () => {
  const a = affect([{ op: 'write', path: '/a.md' }], { editing: '/a.md' });
  assert.equal(a.sidebar, false);
  assert.deepEqual(keys(a.refetch), []);
  assert.deepEqual(keys(a.park), ['knowledge', 'recent', 'tags']);
});

test('someone else writing reloads where that note shows', () => {
  const m = affect([{ op: 'write', path: '/_memory/_goals.md' }], { editing: '/a.md' });
  assert.deepEqual(keys(m.refetch), ['memory']);
  assert.equal(m.sidebar, false);
  const i = affect([{ op: 'write', path: '/_knowledge/_index/ai.md' }]);
  assert.equal(i.sidebar, true);
});

test('a rename touches both places, and a batch merges', () => {
  const a = affect([
    { op: 'rename', path: '/_knowledge/b.md', oldPath: '/a.md' },
    { op: 'write', path: '/x.md' },
    { op: 'image', path: '/_assets/202610/p.png' },
  ], { editing: '/x.md' });
  assert.deepEqual(keys(a.refetch), ['folder', 'images', 'knowledge', 'pinned', 'recent', 'tags']);
  assert.deepEqual(keys(a.park), []);
});

test('pins, shorts and resync', () => {
  assert.deepEqual(keys(affect([{ op: 'pin', path: '/a.md' }]).refetch), ['folder', 'pinned', 'recent', 'tags']);
  assert.deepEqual(keys(affect([{ op: 'pin', path: '/_knowledge/k.md' }]).refetch), ['knowledge', 'pinned']);
  assert.deepEqual(keys(affect([{ op: 'write', path: '/_shorts/2026-10-13.md' }]).refetch), ['recent', 'shorts']);
  const r = affect([{ op: 'resync' }]);
  assert.equal(r.all && r.sidebar, true);
});

test('a write naming its short entry updates that card, not the feed', () => {
  const a = affect([
    { op: 'write', path: '/_shorts/2026-10-29.md', data: { id: '2026-10-29-090000', hash: 'h' } },
    { op: 'delete', path: '/_shorts/2026-10-28.md', data: { id: '2026-10-28-100000' } },
  ]);
  assert.deepEqual(keys(a.refetch), ['recent']);
  assert.deepEqual(keys(a.park), ['shorts']);
  assert.deepEqual(a.shorts.map((s) => s.id), ['2026-10-29-090000', '2026-10-28-100000']);
  assert.equal(a.sidebar, true);
});
