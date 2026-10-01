const test = require('node:test');
const assert = require('node:assert/strict');

global.window = global;
global.document = { visibilityState: 'visible', hidden: false };
global.sessionStorage = {
  getItem: function () { return null; },
  setItem: function () {},
};
global.requestAnimationFrame = function (fn) { return setTimeout(fn, 0); };
global.cancelAnimationFrame = function (id) { clearTimeout(id); };

require('./chat_reduce.js');
require('./chat_kernel.js');

function jsonResponse(status, body) {
  return {
    ok: status >= 200 && status < 300,
    status: status,
    json: async function () { return body; },
    text: async function () { return typeof body === 'string' ? body : JSON.stringify(body); },
  };
}

function sseResponse(chunks) {
  var i = 0;
  return {
    ok: true,
    status: 200,
    body: {
      getReader: function () {
        return {
          read: async function () {
            if (i >= chunks.length) return { done: true };
            var chunk = chunks[i++];
            if (chunk && chunk.then) return chunk;
            return { done: false, value: Buffer.from(chunk) };
          },
          releaseLock: function () {},
        };
      },
    },
  };
}

var realSetTimeout = setTimeout;
var realClearTimeout = clearTimeout;
var armed = [];
global.setTimeout = function (fn, ms) {
  var id = realSetTimeout(fn, ms);
  armed.push(id);
  return id;
};
global.clearTimeout = function (id) {
  var i = armed.indexOf(id);
  if (i >= 0) armed.splice(i, 1);
  return realClearTimeout(id);
};

function tick() {
  return new Promise(function (resolve) { realSetTimeout(resolve, 0); });
}

function disarm() {
  armed.splice(0).forEach(function (id) { realClearTimeout(id); });
}

test('phase, cancel, and cursor finisher', async function () {
  var toasts = [];
  var matesReady;
  var matesP = new Promise(function (resolve) { matesReady = resolve; });
  var api;
  var snaps = [];
  global.__hyloChatMount = function (el, facade) {
    api = facade;
    return {
      render: function () { snaps.push(facade.state()); },
      destroy: function () {},
      draftValue: function () { return facade.draft(); },
      clearComposer: function () {},
      focusComposer: function () {},
    };
  };

  var hang;
  var convFetches = 0;
  global.fetch = async function (url, opts) {
    var u = String(url);
    if (u === '/api/mates') {
      return jsonResponse(200, { mates: [{ id: 'm1', name: 'Ada', enabled: true, agentId: 'cursor-agent' }] });
    }
    if (u === '/api/mate-events') return jsonResponse(200, { events: [] });
    if (u.indexOf('/api/conversations?') === 0) {
      return jsonResponse(200, { conversations: [{ id: 'c1' }] });
    }
    if (u === '/api/conversations/c1' && (!opts || opts.method !== 'POST')) {
      convFetches++;
      var content = convFetches > 1 ? 'from-db' : '';
      return jsonResponse(200, { messages: content ? [{ id: 'db', role: 'assistant', content: content, status: 'succeeded' }] : [] });
    }
    if (u === '/api/chat') {
      return sseResponse([
        'event: start\ndata: {"runId":"r1"}\n\n',
        new Promise(function (resolve) { hang = resolve; }),
      ]);
    }
    if (u === '/api/runs/r1/cancel') return jsonResponse(200, {});
    throw new Error('unmocked ' + u);
  };

  var chat = global.__hyloChatCreate({
    onMates: function () { matesReady(); },
    onToast: function (text) { toasts.push(text); },
  });
  try {
    chat.start();
    await matesP;
    chat.attach({});
    chat.tryOpen('m1');
    await tick();
    assert.equal(chat.busy(), false);
    assert.equal(api.state().mateId, 'm1');

    api.setDraft('hello');
    var sent = api.send();
    await tick();
    var mid = api.state();
    assert.equal(mid.phase, 'streaming');
    assert.equal(mid.runId, 'r1');
    assert.equal(mid.busy, true);
    assert.equal(chat.tryOpen('m2'), false);

    await api.cancel();
    var stopping = api.state();
    assert.equal(stopping.phase, 'stopping');
    assert.equal(stopping.stoppingId, mid.messages[mid.messages.length - 1].id);

    hang({
      done: false,
      value: Buffer.from('event: agent\ndata: {"type":"text_delta","delta":"Hi"}\n\nevent: end\ndata: {"status":"succeeded"}\n\n'),
    });
    await sent;
    var done = api.state();
    assert.equal(done.phase, 'idle');
    assert.equal(done.busy, false);
    assert.equal(done.stoppingId, '');
    var assistant = done.messages[done.messages.length - 1];
    assert.equal(assistant.status, 'succeeded');
    assert.equal(assistant.segments[assistant.segments.length - 1].content, 'from-db');
    assert.equal(assistant.copied, undefined);
    assert.equal(assistant.stopping, undefined);
    assert.deepEqual(toasts, []);
  } finally {
    disarm();
  }
});

test('a failed mate list toasts once', async function () {
  var toasts = [];
  global.fetch = async function (url) {
    if (String(url) === '/api/mates' || String(url) === '/api/mate-events') {
      return jsonResponse(500, 'nope');
    }
    throw new Error('unmocked ' + url);
  };
  var chat = global.__hyloChatCreate({
    onToast: function (text) { toasts.push(text); },
  });
  try {
    chat.start();
    await tick();
    assert.deepEqual(toasts, ["Couldn't load agent bots", "Couldn't load triggers"]);
  } finally {
    disarm();
  }
});
