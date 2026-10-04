// Run with: node --test internal/server/view/assets/graph_layout.test.js
const test = require('node:test');
const assert = require('node:assert/strict');
const {
  __hyloLouvain, __hyloGraphPrepare, __hyloGraphLabeledHubs, __hyloGraphPickLabels,
} = require('./graph_layout.js');

function clique(offset, size) {
  const pairs = [];
  for (let i = 0; i < size; i++) for (let j = i + 1; j < size; j++) pairs.push([offset + i, offset + j]);
  return pairs;
}

test('louvain separates two cliques joined by a single bridge', () => {
  const pairs = clique(0, 5).concat(clique(5, 5), [[4, 5]]);
  const m = __hyloLouvain(10, pairs);
  for (let i = 1; i < 5; i++) assert.equal(m[i], m[0]);
  for (let i = 6; i < 10; i++) assert.equal(m[i], m[5]);
  assert.notEqual(m[0], m[5]);
});

test('louvain keeps isolated nodes in their own community', () => {
  const m = __hyloLouvain(4, [[0, 1]]);
  assert.equal(m[0], m[1]);
  assert.equal(new Set([m[1], m[2], m[3]]).size, 3);
});

test('prepare merges reciprocal links and counts distinct neighbours', () => {
  const nodes = [{ id: 'a' }, { id: 'b' }, { id: 'c' }];
  const edges = [
    { source: 'a', target: 'b' }, { source: 'b', target: 'a' },
    { source: 'a', target: 'c' }, { source: 'a', target: 'c' },
    { source: 'a', target: 'a' }, { source: 'a', target: 'missing' },
  ];
  const p = __hyloGraphPrepare(nodes, edges);
  assert.deepEqual(p.links, [
    { source: 'a', target: 'b', bidir: true },
    { source: 'a', target: 'c', bidir: false },
  ]);
  assert.deepEqual(p.degree, { a: 2, b: 1, c: 1 });
  for (const n of nodes) assert.ok(Number.isFinite(p.positions[n.id].x));
});

test('prepare is deterministic', () => {
  const nodes = Array.from({ length: 10 }, (_, i) => ({ id: 'n' + i }));
  const edges = clique(0, 5).concat(clique(5, 5), [[4, 5]])
    .map(([s, t]) => ({ source: 'n' + s, target: 'n' + t }));
  assert.deepEqual(__hyloGraphPrepare(nodes, edges), __hyloGraphPrepare(nodes, edges));
});

test('labeled hubs are ordered by degree and skip isolated nodes', () => {
  assert.deepEqual(__hyloGraphLabeledHubs({ a: 1, b: 5, c: 0, d: 3 }, 4), ['b', 'd', 'a']);
});

test('pick labels drops boxes overlapping an earlier one', () => {
  const kept = __hyloGraphPickLabels([
    { id: 'a', x1: 0, y1: 0, x2: 10, y2: 10 },
    { id: 'b', x1: 5, y1: 5, x2: 15, y2: 15 },
    { id: 'c', x1: 10, y1: 0, x2: 20, y2: 4 },
  ]);
  assert.deepEqual(kept, ['a', 'c']);
});
