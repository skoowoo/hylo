const test = require('node:test');
const assert = require('node:assert/strict');
const { __hyloVisitStep } = require('./note_activity.js');

const fresh = () => ({ path: '', since: 0, last: 0, edited: false, sent: false });

test('a glance under 15s is never reported', () => {
  const v = fresh();
  assert.deepEqual(__hyloVisitStep(v, '/a.md', 0), []);
  assert.deepEqual(__hyloVisitStep(v, '/a.md', 14000), []);
  assert.deepEqual(__hyloVisitStep(v, '/b.md', 14500), []);
});

test('a read is reported once it counts, then again when it ends', () => {
  const v = fresh();
  __hyloVisitStep(v, '/a.md', 0);
  assert.deepEqual(__hyloVisitStep(v, '/a.md', 15000), [{ path: '/a.md', action: 'read', at: 15000 }]);
  assert.deepEqual(__hyloVisitStep(v, '/a.md', 17000), []);
  assert.deepEqual(__hyloVisitStep(v, '', 18000), [{ path: '/a.md', action: 'read', at: 17000 }]);
});

test('an edit is reported at once, even before 15s, and upgrades a counted read', () => {
  const v = fresh();
  __hyloVisitStep(v, '/a.md', 0);
  assert.deepEqual(__hyloVisitStep(v, '/a.md', 1000, true), [{ path: '/a.md', action: 'write', at: 1000 }]);
  assert.deepEqual(__hyloVisitStep(v, '/a.md', 2000, true), []);

  const r = fresh();
  __hyloVisitStep(r, '/b.md', 0);
  __hyloVisitStep(r, '/b.md', 15000);
  assert.deepEqual(__hyloVisitStep(r, '/b.md', 16000, true), [{ path: '/b.md', action: 'write', at: 16000 }]);
});

test('switching notes ends one visit and starts the next', () => {
  const v = fresh();
  __hyloVisitStep(v, '/a.md', 0);
  __hyloVisitStep(v, '/a.md', 15000);
  assert.deepEqual(__hyloVisitStep(v, '/b.md', 16000), [{ path: '/a.md', action: 'read', at: 15000 }]);
  assert.deepEqual(__hyloVisitStep(v, '/b.md', 31000), [{ path: '/b.md', action: 'read', at: 31000 }]);
});
