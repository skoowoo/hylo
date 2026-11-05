const test = require('node:test');
const assert = require('node:assert/strict');

global.window = global;
global.document = { visibilityState: 'visible', hidden: false, addEventListener: function () {} };
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
      return jsonResponse(200, { conversations: [{ id: 'c1' }], activeId: 'c1' });
    }
    if (u.split('?')[0] === '/api/conversations/c1' && (!opts || opts.method !== 'POST')) {
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

test('retry of a failed wake resends it as a wake, in the chat', async function () {
  var matesReady;
  var matesP = new Promise(function (resolve) { matesReady = resolve; });
  var api;
  global.__hyloChatMount = function (el, facade) {
    api = facade;
    return {
      render: function () {},
      destroy: function () {},
      draftValue: function () { return facade.draft(); },
      clearComposer: function () {},
      focusComposer: function () {},
    };
  };

  var chatCalls = [];
  global.fetch = async function (url, opts) {
    var u = String(url);
    if (u === '/api/mates') {
      return jsonResponse(200, { mates: [{ id: 'm1', name: 'Ada', enabled: true, agentId: 'claude' }] });
    }
    if (u === '/api/mate-events') return jsonResponse(200, { events: [] });
    if (u.indexOf('/api/conversations?') === 0) {
      return jsonResponse(200, { conversations: [{ id: 'c1' }], activeId: 'c1' });
    }
    if (u.split('?')[0] === '/api/conversations/c1') {
      return jsonResponse(200, {
        messages: [
          { id: 'w1', role: 'wake', content: 'Update my memory.', triggerEvent: 'scheduled', triggerId: 't1', triggerName: 'Memory', triggerSchedule: 'daily 09:00', createdAt: 1 },
          {
            id: 'a1', role: 'assistant', status: 'failed', triggerEvent: 'scheduled', triggerId: 't1',
            content: '', mateId: 'm1', agentId: 'claude', updatedAt: 2,
          },
        ],
      });
    }
    if (u === '/api/chat') {
      chatCalls.push(JSON.parse(opts.body));
      return sseResponse(['event: end\ndata: {"status":"succeeded"}\n\n']);
    }
    throw new Error('unmocked ' + u);
  };

  var chat = global.__hyloChatCreate({ onMates: function () { matesReady(); } });
  try {
    chat.start();
    await matesP;
    chat.attach({});
    chat.tryOpen('m1');
    await tick();

    var before = api.state();
    assert.deepEqual(before.messages.map(function (m) { return m.id; }), ['a1'], 'a wake is no row of its own');
    await api.retry('a1');

    assert.equal(chatCalls.length, 1);
    assert.equal(chatCalls[0].message, 'Update my memory.');
    assert.equal(chatCalls[0].retryOf, 'a1');
    assert.equal(chatCalls[0].triggerEvent, 'scheduled');
    assert.equal(chatCalls[0].triggerId, 't1');

    var after = api.state();
    assert.equal(after.messages.length, 1, 'the retry replaces the failed reply');
    assert.equal(after.messages[0].role, 'assistant');
    assert.equal(after.messages[0].id === 'a1', false);
  } finally {
    disarm();
  }
});

test('retry fetches the prompt when it sits on a page not loaded yet', async function () {
  var matesReady;
  var matesP = new Promise(function (resolve) { matesReady = resolve; });
  var api;
  global.__hyloChatMount = function (el, facade) {
    api = facade;
    return { render: function () {}, destroy: function () {}, draftValue: function () { return facade.draft(); }, clearComposer: function () {}, focusComposer: function () {} };
  };

  var retryCalls = [];
  var promptFetches = [];
  global.fetch = async function (url, opts) {
    var u = String(url);
    if (u === '/api/mates') return jsonResponse(200, { mates: [{ id: 'm1', name: 'Ada', enabled: true, agentId: 'claude' }] });
    if (u === '/api/mate-events') return jsonResponse(200, { events: [] });
    if (u.indexOf('/api/conversations?') === 0) return jsonResponse(200, { conversations: [{ id: 'c1' }], activeId: 'c1' });
    if (u.indexOf('/api/conversations/c1?limit=1&before=a1') === 0) {
      promptFetches.push(u);
      return jsonResponse(200, { messages: [{ id: 'u1', role: 'user', content: 'older prompt' }] });
    }
    if (u.split('?')[0] === '/api/conversations/c1') {
      return jsonResponse(200, {
        hasMore: true,
        messages: [{ id: 'a1', role: 'assistant', status: 'failed', content: '', mateId: 'm1' }],
      });
    }
    if (u === '/api/chat') {
      retryCalls.push(JSON.parse(opts.body));
      return sseResponse(['event: end\ndata: {"status":"succeeded"}\n\n']);
    }
    throw new Error('unmocked ' + u);
  };

  var chat = global.__hyloChatCreate({ onMates: function () { matesReady(); } });
  try {
    chat.start();
    await matesP;
    chat.attach({});
    chat.tryOpen('m1');
    await tick();

    await api.retry('a1');

    assert.equal(promptFetches.length, 1);
    assert.equal(retryCalls.length, 1);
    assert.equal(retryCalls[0].message, 'older prompt');
    assert.equal(retryCalls[0].retryOf, 'a1');
    assert.equal(retryCalls[0].triggerId, '');
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
    assert.deepEqual(toasts, ["Couldn't load bots", "Couldn't load triggers"]);
  } finally {
    disarm();
  }
});

test('a bot removed anywhere moves the chat to the next one', async function () {
  var listeners = {};
  global.__hyloLive = { on: function (topic, fn) { listeners[topic] = fn; } };
  var mates = [
    { id: 'm1', name: 'Hylo', enabled: true, agentId: 'pi' },
    { id: 'm2', name: 'Ada', enabled: true, agentId: 'pi' },
  ];
  global.fetch = async function (url) {
    var u = String(url);
    if (u === '/api/mates') return jsonResponse(200, { mates: mates.slice() });
    if (u === '/api/mate-events') return jsonResponse(200, { events: [] });
    if (u.indexOf('/api/conversations?') === 0) return jsonResponse(200, { conversations: [] });
    return jsonResponse(404, {});
  };
  var picked = [];
  var shown = [];
  var kernel = window.__hyloChatCreate({
    onMates: function (list) { shown = list.map(function (m) { return m.id; }); },
    onMate: function (id) { picked.push(id); },
  });
  try {
    kernel.start();
    await kernel.bootstrap();
    assert.equal(kernel.mateId(), 'm1');

    mates.shift();
    await listeners.bots([{ topic: 'bots', op: 'change' }]);
    assert.deepEqual(shown, ['m2']);
    assert.equal(kernel.mateId(), 'm2');

    mates.shift();
    await listeners.bots([{ topic: 'bots', op: 'change' }]);
    assert.deepEqual(shown, []);
    assert.equal(kernel.mateId(), '');

    mates.push({ id: 'm3', name: 'Hylo', enabled: true, agentId: 'pi' });
    await listeners.bots([{ topic: 'bots', op: 'change' }]);
    assert.equal(kernel.mateId(), 'm3');
  } finally {
    delete global.__hyloLive;
    disarm();
  }
});

test('a wake reply shows only once it is done, so a silent one never appears', async function () {
  var listeners = {};
  global.__hyloLive = { on: function (topic, fn) { listeners[topic] = fn; } };
  var api;
  global.__hyloChatMount = function (el, facade) {
    api = facade;
    return { render: function () {}, destroy: function () {}, draftValue: function () { return facade.draft(); }, clearComposer: function () {}, focusComposer: function () {} };
  };
  var seen = [];
  var unread = 1;
  global.fetch = async function (url, opts) {
    var u = String(url);
    if (u === '/api/mates') return jsonResponse(200, { mates: [{ id: 'm1', name: 'Ada', enabled: true, agentId: 'pi', unread: unread }] });
    if (u === '/api/mate-events') return jsonResponse(200, { events: [] });
    if (u === '/api/mates/m1/seen') { seen.push(opts.method); unread = 0; return jsonResponse(200, { ok: true }); }
    if (u.indexOf('/api/conversations?') === 0) return jsonResponse(200, { conversations: [{ id: 'c1' }], activeId: 'c1' });
    if (u.split('?')[0] === '/api/conversations/c1') {
      return jsonResponse(200, {
        messages: [
          { id: 'w0', role: 'wake', content: 'Recall', triggerEvent: 'short_note_created', triggerId: 't1', triggerName: 'Short notes', createdAt: 1 },
          { id: 'a0', role: 'assistant', status: 'succeeded', content: 'Two notes disagree.', triggerEvent: 'short_note_created', triggerId: 't1', triggerName: 'Short notes', mateId: 'm1', updatedAt: 2 },
          { id: 'w1', role: 'wake', content: 'Recall', triggerEvent: 'short_note_created', triggerId: 't1', createdAt: 3 },
          { id: 'a1', role: 'assistant', status: 'running', triggerEvent: 'short_note_created', triggerId: 't1', mateId: 'm1', updatedAt: 4 },
        ],
      });
    }
    throw new Error('unmocked ' + u);
  };
  var shown = [];
  var chat = global.__hyloChatCreate({ onMates: function (list) { shown = list; } });
  try {
    chat.start();
    await chat.bootstrap();
    chat.attach({});
    await tick();
    assert.deepEqual(seen, ['POST'], 'looking at the chat reads what the bot sent');
    assert.equal(shown[0].unread, 0);
    assert.deepEqual(api.state().messages.map(function (m) { return m.id; }), ['a0'], 'the running wake waits off screen');
    assert.equal(api.state().messages[0].triggerName, 'Short notes');

    listeners.chat([{ topic: 'chat', op: 'wake', data: { mateId: 'm1', wakeId: 'w1', replyId: 'a1', status: 'succeeded', silent: true } }]);
    assert.deepEqual(api.state().messages.map(function (m) { return m.id; }), ['a0']);
  } finally {
    delete global.__hyloLive;
    disarm();
  }
});

test('a chat change pulls the transcript, and a hidden tab waits to be shown', async function () {
  var listeners = {};
  var onShow = [];
  var prevListen = global.document.addEventListener;
  global.document.addEventListener = function (type, fn) {
    if (type === 'visibilitychange') onShow.push(fn);
  };
  global.__hyloLive = { on: function (topic, fn) { listeners[topic] = fn; } };
  var api;
  global.__hyloChatMount = function (el, facade) {
    api = facade;
    return { render: function () {}, destroy: function () {}, draftValue: function () { return facade.draft(); }, clearComposer: function () {}, focusComposer: function () {} };
  };
  var since = 0;
  var extra = false;
  global.fetch = async function (url) {
    var u = String(url);
    if (u === '/api/mates') return jsonResponse(200, { mates: [{ id: 'm1', name: 'Ada', enabled: true, agentId: 'pi' }] });
    if (u === '/api/mate-events') return jsonResponse(200, { events: [] });
    if (u.indexOf('/api/conversations?') === 0) return jsonResponse(200, { conversations: [{ id: 'c1' }], activeId: 'c1' });
    if (u.indexOf('since=') >= 0) {
      since++;
      var msgs = [{ id: 'u1', role: 'user', content: 'from the phone', createdAt: 1, updatedAt: 1 }];
      if (extra) msgs.push({ id: 'a2', role: 'assistant', content: 'On my way.', status: 'succeeded', createdAt: 2, updatedAt: 5 });
      return jsonResponse(200, { messages: msgs });
    }
    if (u.split('?')[0] === '/api/conversations/c1') {
      return jsonResponse(200, { messages: [{ id: 'u0', role: 'user', content: 'earlier', createdAt: 1, updatedAt: 1 }] });
    }
    throw new Error('unmocked ' + u);
  };
  var chat = global.__hyloChatCreate({});
  try {
    global.document.visibilityState = 'hidden';
    chat.start();
    await chat.bootstrap();
    chat.attach({});
    await tick();
    assert.equal(since, 0, 'opening a chat does not start a poll');

    listeners.chat([{ topic: 'chat', op: 'change' }]);
    await tick();
    await tick();
    assert.equal(since, 0, 'a hidden tab keeps the change until it is shown');
    assert.deepEqual(api.state().messages.map(function (m) { return m.id; }), ['u0']);

    global.document.visibilityState = 'visible';
    onShow.forEach(function (fn) { fn(); });
    await tick();
    await tick();
    assert.equal(since, 1);
    assert.deepEqual(api.state().messages.map(function (m) { return m.content; }), ['earlier', 'from the phone']);

    extra = true;
    listeners.chat([{ topic: 'chat', op: 'change' }]);
    await tick();
    await tick();
    assert.equal(since, 2);
    assert.deepEqual(api.state().messages.map(function (m) { return m.id; }), ['u0', 'u1', 'a2']);
  } finally {
    global.document.visibilityState = 'visible';
    global.document.addEventListener = prevListen;
    delete global.__hyloLive;
    disarm();
  }
});

test('the changes from a turn this page streamed to its end pull nothing; a dropped one pulls', async function () {
  var listeners = {};
  global.__hyloLive = { on: function (topic, fn) { listeners[topic] = fn; } };
  var api;
  global.__hyloChatMount = function (el, facade) {
    api = facade;
    return { render: function () {}, destroy: function () {}, draftValue: function () { return facade.draft(); }, clearComposer: function () {}, focusComposer: function () {} };
  };
  var since = 0;
  var drop = false;
  global.fetch = async function (url, opts) {
    var u = String(url);
    if (u === '/api/mates') return jsonResponse(200, { mates: [{ id: 'm1', name: 'Ada', enabled: true, agentId: 'pi' }] });
    if (u === '/api/mate-events') return jsonResponse(200, { events: [] });
    if (u.indexOf('/api/conversations?') === 0) return jsonResponse(200, { conversations: [{ id: 'c1' }], activeId: 'c1' });
    if (u.indexOf('since=') >= 0) { since++; return jsonResponse(200, { messages: [] }); }
    if (u.split('?')[0] === '/api/conversations/c1') return jsonResponse(200, { messages: [] });
    if (u === '/api/chat') {
      var body = JSON.parse(opts.body);
      var ids = [body.userMessageId, body.assistantMessageId];
      listeners.chat([{ topic: 'chat', op: 'change', data: ids.slice(0, 1) }, { topic: 'chat', op: 'change', data: ids.slice(1) }]);
      return sseResponse(drop ? ['event: start\ndata: {}\n\n'] : [
        'event: end\ndata: {"status":"succeeded"}\n\n',
        { then: function (resolve) { listeners.chat([{ topic: 'chat', op: 'change', data: ids.slice(1) }]); resolve({ done: true }); } },
      ]);
    }
    throw new Error('unmocked ' + u);
  };
  var chat = global.__hyloChatCreate({});
  try {
    chat.start();
    await chat.bootstrap();
    chat.attach({});
    await tick();

    api.setDraft('hello');
    await api.send();
    await tick();
    await tick();
    assert.equal(since, 0, 'its own rows are already here');

    listeners.chat([{ topic: 'chat', op: 'change', data: ['x9'] }]);
    await tick();
    await tick();
    assert.equal(since, 1, 'a row from elsewhere is pulled');

    drop = true;
    api.setDraft('again');
    await api.send();
    await tick();
    await tick();
    assert.equal(since, 2, 'a reply without its end comes from the store');
  } finally {
    delete global.__hyloLive;
    disarm();
  }
});

test('a new chat waits for its first message, and any chat can be opened again', async function () {
  global.__hyloLive = { on: function () {} };
  var api;
  global.__hyloChatMount = function (el, facade) {
    api = facade;
    return { render: function () {}, destroy: function () {}, draftValue: function () { return facade.draft(); }, clearComposer: function () {}, focusComposer: function () {} };
  };
  var active = 'c2';
  var puts = [];
  var pages = {
    c1: [{ id: 'u1', role: 'user', content: 'older ask', createdAt: 1 }, { id: 'a1', role: 'assistant', status: 'succeeded', content: 'older reply', updatedAt: 2 }],
    c2: [{ id: 'u2', role: 'user', content: 'latest ask', createdAt: 3 }, { id: 'a2', role: 'assistant', status: 'succeeded', content: 'latest reply', updatedAt: 4 }],
  };
  global.fetch = async function (url, opts) {
    var u = String(url);
    if (u === '/api/mates') return jsonResponse(200, { mates: [{ id: 'm1', name: 'Ada', enabled: true, agentId: 'pi' }] });
    if (u === '/api/mate-events') return jsonResponse(200, { events: [] });
    if (u === '/api/mates/m1/conversation') {
      active = JSON.parse(opts.body).conversationId;
      puts.push(active);
      return jsonResponse(200, { ok: true });
    }
    if (u.indexOf('/api/conversations?') === 0) {
      return jsonResponse(200, {
        activeId: active,
        conversations: [{ id: 'c2', title: 'latest ask', updatedAt: 40 }, { id: 'c1', title: 'older ask', updatedAt: 20 }],
      });
    }
    var id = u.split('?')[0].replace('/api/conversations/', '');
    if (pages[id]) return jsonResponse(200, { messages: pages[id], hasMore: false });
    throw new Error('unmocked ' + u);
  };
  var chat = global.__hyloChatCreate({});
  function ids(list) { return list.map(function (m) { return m.id; }); }
  try {
    chat.start();
    await chat.bootstrap();
    chat.attach({});
    await tick();
    var st = api.state();
    assert.equal(st.conversationId, 'c2');
    assert.deepEqual(ids(st.messages), ['u2', 'a2']);
    assert.deepEqual(st.chats.map(function (c) { return [c.id, c.title]; }), [['c2', 'latest ask'], ['c1', 'older ask']]);

    await api.openChat('');
    st = api.state();
    assert.deepEqual(puts, ['']);
    assert.equal(st.conversationId, '');
    assert.deepEqual(st.messages, []);

    await api.openChat('c1');
    st = api.state();
    assert.deepEqual(puts, ['', 'c1']);
    assert.equal(st.conversationId, 'c1');
    assert.deepEqual(ids(st.messages), ['u1', 'a1']);

    await api.openChat('c1');
    assert.deepEqual(puts, ['', 'c1'], 'the open chat is not reopened');
  } finally {
    delete global.__hyloLive;
    delete global.__hyloChatMount;
    disarm();
  }
});
