// Run with: node --test internal/server/view/assets/tab_fit.test.js
const test = require('node:test');
const assert = require('node:assert/strict');
const { __vaultrFitTabCount } = require('./tab_fit.js');

test('returns 0 when there is nothing to show or no width yet', () => {
  assert.equal(__vaultrFitTabCount([], 400, 4, 44), 0);
  assert.equal(__vaultrFitTabCount([100, 100], 0, 4, 44), 0);
});

test('fits every chip when the row is wide enough, with no overflow reserve', () => {
  // 100+4+100+4+100 = 308
  assert.equal(__vaultrFitTabCount([100, 100, 100], 308, 4, 44), 3);
});

test('reserves the overflow control only while a chip stays hidden', () => {
  // two chips + gap + overflow: 100+4+100+4+44 = 252
  // three chips and no overflow: 308, which does not fit
  assert.equal(__vaultrFitTabCount([100, 100, 100], 252, 4, 44), 2);
  assert.equal(__vaultrFitTabCount([100, 100, 100], 251, 4, 44), 1);
});

test('uses the hidden-count when overflow width is a function', () => {
  var overflow = function(hidden) { return hidden >= 2 ? 60 : 44; };
  // one chip + wider "+2": 100+4+60 = 164, two chips + "+1": 100+4+100+4+44 = 252
  assert.equal(__vaultrFitTabCount([100, 100, 100], 200, 4, overflow), 1);
  assert.equal(__vaultrFitTabCount([100, 100, 100], 252, 4, overflow), 2);
});

test('keeps a single chip when even the first one is wider than the row', () => {
  assert.equal(__vaultrFitTabCount([300, 300], 40, 4, 44), 1);
});
