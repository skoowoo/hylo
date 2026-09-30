const test = require('node:test');
const assert = require('node:assert/strict');
const R = require('./chat_reduce.js');

test('text deltas append and then start a new segment after a tool', function () {
  var segs = [];
  R.applyAgentSegment(segs, { type: 'text_delta', delta: 'Hello' });
  R.applyAgentSegment(segs, { type: 'text_delta', delta: ' there' });
  assert.deepEqual(segs, [{ type: 'text', content: 'Hello there' }]);
  R.applyAgentSegment(segs, { type: 'tool_use', name: 'read' });
  R.applyAgentSegment(segs, { type: 'text_delta', delta: 'Done' });
  assert.equal(segs[2].content, 'Done');
  assert.equal(segs.length, 3);
});

test('text_replace keeps the last text segment and drops earlier ones', function () {
  var segs = [
    { type: 'text', content: 'a' },
    { type: 'thinking', content: 'hmm', open: false },
    { type: 'text', content: 'b' },
  ];
  R.applyAgentSegment(segs, { type: 'text_replace', text: 'final' });
  assert.deepEqual(segs.map(function (s) { return s.type + ':' + (s.content || ''); }), [
    'thinking:hmm',
    'text:final',
  ]);
});

test('tool_use merges only into the latest completed call of the same name', function () {
  var segs = [];
  R.applyAgentSegment(segs, { type: 'tool_use', name: 'read' });
  R.applyAgentSegment(segs, { type: 'tool_result', content: 'ok' });
  R.applyAgentSegment(segs, { type: 'tool_use', name: 'read' });
  assert.equal(segs.length, 1);
  assert.equal(segs[0].count, 2);
  assert.deepEqual(segs[0].results, ['ok']);
  R.applyAgentSegment(segs, { type: 'text_delta', delta: 'x' });
  R.applyAgentSegment(segs, { type: 'tool_use', name: 'read' });
  assert.equal(segs[segs.length - 1].type, 'tool_use');
  assert.equal(segs[segs.length - 1].count, 1);
});

test('tool_result attaches to the pending call and stops at text', function () {
  var segs = [{ type: 'tool_use', name: 'read', count: 1, results: [], open: false }];
  R.applyAgentSegment(segs, { type: 'tool_result', content: { a: 1 } });
  assert.deepEqual(segs[0].results, ['{"a":1}']);
  R.applyAgentSegment(segs, { type: 'text_delta', delta: 'hi' });
  R.applyAgentSegment(segs, { type: 'tool_result', content: 'late' });
  assert.equal(segs[segs.length - 1].type, 'tool_result');
  assert.equal(segs[segs.length - 1].content, 'late');
});

test('note_access merges into a single segment and upgrades read to write', function () {
  var segs = [];
  R.applyAgentSegment(segs, { type: 'note_access', path: '/a.md', action: 'read', tool: 'Read' });
  R.applyAgentSegment(segs, { type: 'text_delta', delta: 'thinking...' });
  R.applyAgentSegment(segs, { type: 'note_access', path: '/b.md', action: 'write', tool: 'Edit' });
  R.applyAgentSegment(segs, { type: 'note_access', path: '/a.md', action: 'write', tool: 'Edit' });

  var naSegs = segs.filter(function (s) { return s.type === 'note_access'; });
  assert.equal(naSegs.length, 1);
  assert.deepEqual(naSegs[0].items, [
    { path: '/a.md', action: 'write', tool: 'Edit' },
    { path: '/b.md', action: 'write', tool: 'Edit' },
  ]);
});

test('note_access drops glob-pattern paths', function () {
  var segs = [];
  R.applyAgentSegment(segs, { type: 'note_access', path: '/*.md', action: 'read', tool: 'Glob' });
  R.applyAgentSegment(segs, { type: 'note_access', path: '/daily-*.md', action: 'write', tool: 'Bash' });
  assert.deepEqual(segs, []);
});

test('status ignores running and requesting and updates the tail label', function () {
  var segs = [];
  R.applyAgentSegment(segs, { type: 'status', label: 'running' });
  R.applyAgentSegment(segs, { type: 'status', label: 'requesting' });
  R.applyAgentSegment(segs, { type: 'status', label: 'thinking' });
  R.applyAgentSegment(segs, { type: 'status', label: 'streaming' });
  assert.deepEqual(segs, [{ type: 'status', label: 'streaming' }]);
});

test('merge keeps rich segments after the message has finished', function () {
  var messages = [{
    id: 'a', role: 'assistant', status: 'succeeded', content: 'old',
    segments: [{ type: 'thinking', content: 'hmm', open: false }, { type: 'text', content: 'old' }],
    completedAt: 1, rev: 1,
  }];
  var changed = R.mergeSynced(messages, [{
    id: 'a', role: 'assistant', content: 'new', status: 'succeeded', mateId: 'm',
    updatedAt: '2026-09-29T00:00:00.000Z',
  }]);
  assert.equal(changed, true);
  assert.equal(messages[0].segments[0].type, 'thinking');
  assert.equal(messages[0].segments[1].content, 'new');
  assert.equal(messages[0].status, 'succeeded');
});

test('merge replaces segments while the message is still running', function () {
  var messages = [{
    id: 'a', role: 'assistant', status: 'running',
    segments: [{ type: 'thinking', content: 'hmm', open: false }, { type: 'text', content: 'partial' }],
    rev: 1,
  }];
  R.mergeSynced(messages, [{
    id: 'a', role: 'assistant', content: 'from-db', status: 'running', mateId: 'm',
    updatedAt: '2026-09-29T00:00:00.000Z',
  }]);
  assert.deepEqual(messages[0].segments, [{ type: 'text', content: 'from-db' }]);
});

test('merge appends a row the live list has not seen', function () {
  var messages = [];
  R.mergeSynced(messages, [{
    id: 'u', role: 'user', content: 'hi', createdAt: '2026-09-29T00:00:00.000Z',
  }]);
  assert.equal(messages.length, 1);
  assert.equal(messages[0].role, 'user');
  assert.equal(messages[0].content, 'hi');
});

test('feedSSE reports end even when the payload is not valid JSON', function () {
  var seen = [];
  var fed = R.feedSSE('event: end\ndata: {bad\n\n', function (event, raw) {
    seen.push(event);
    try { JSON.parse(raw); } catch (e) { return; }
  });
  assert.equal(fed.gotEnd, true);
  assert.deepEqual(seen, ['end']);
  assert.equal(fed.buf, '');
});

test('feedSSE keeps a trailing partial block', function () {
  var fed = R.feedSSE('event: heartbeat\ndata: {}\n\nevent: agent\ndata: {"type":"text_delta"}', function () {});
  assert.equal(fed.gotEnd, false);
  assert.ok(fed.buf.indexOf('text_delta') !== -1);
});

test('new segments do not carry view flags', function () {
  var segs = [];
  R.applyAgentSegment(segs, { type: 'thinking_delta', delta: 'hmm' });
  R.applyAgentSegment(segs, { type: 'tool_use', name: 'read' });
  R.applyAgentSegment(segs, { type: 'tool_result', content: 'ok' });
  assert.equal(segs[0].open, undefined);
  assert.equal(segs[1].open, undefined);
  var stored = R.formatStoredMessages([{ id: 'a', role: 'assistant', content: 'hi', status: 'succeeded' }]);
  assert.equal(stored[0].copied, undefined);
  assert.equal(stored[0].stopping, undefined);
});

test('formatTime buckets', function () {
  var now = Date.parse('2026-09-29T15:00:00');
  assert.equal(R.formatTime(0, now), '');
  assert.equal(R.formatTime(now - 10 * 1000, now), 'now');
  assert.equal(R.formatTime(now - 5 * 60 * 1000, now), '5m ago');
  assert.equal(R.formatTime(now - 3 * 60 * 60 * 1000, now), '3h ago');
  assert.ok(R.formatTime(now - 26 * 60 * 60 * 1000, now).indexOf('Yesterday') === 0);
});
