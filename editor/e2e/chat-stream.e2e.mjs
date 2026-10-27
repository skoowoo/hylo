// The chat pane while a reply streams, settles and syncs back from the DB: nothing
// already on screen may be rebuilt, shrink under the reader, or move once the run
// is over. The API is stubbed in the page; the pane is the real one.
import { test, before, after } from 'node:test';
import assert from 'node:assert/strict';
import { webkit } from 'playwright-core';
import { startServer } from './server.mjs';

let browser = null;
let server = null;
let launchError = null;

before(async () => {
  try {
    browser = await webkit.launch();
  } catch (e) {
    launchError = e;
    return;
  }
  server = await startServer();
});

after(async () => {
  if (browser) await browser.close();
  if (server) await server.close();
});

const REPLY = `根据你最近的笔记，我整理了几点关于**流式界面**的观察：

## 主要问题

1. 内容在显示之后又改变尺寸
2. 整行重建导致卡片重新挂载

| 来源 | 影响 |
| --- | --- |
| 整行重建 | 卡片重新挂载 |
| 正文重写 | 全段重排 |

\`\`\`js
function settle() {
  return pin();
}
\`\`\`

> 结构上解决，比事后补偿更可靠。

最后，我把这些整理进了 [[chat-redesign]]。`;

// Runs in the page before the pane loads. missing: the vault has none of the notes;
// steps: extra tool calls before the reply, for a trace taller than its cap.
function stub({ reply, missing, steps = 0 }) {
  const T = (window.__T = { db: [], frames: [], rebuilt: 0, sentAt: 0, endAt: 0 });
  const iso = (ms) => new Date(ms).toISOString();
  const json = (o, delay) => new Promise((r) => setTimeout(() => r(new Response(JSON.stringify(o))), delay || 0));
  const NOTES = ['/journal/october.md', '/projects/chat-redesign.md'];
  const t0 = Date.now() - 3600e3;
  T.db.push({ id: 'u0', role: 'user', content: '在吗', createdAt: iso(t0), updatedAt: iso(t0) });
  T.db.push({ id: 'a0', role: 'assistant', mateId: 'm1', agentId: 'claude', content: '在的。', status: 'succeeded', createdAt: iso(t0), updatedAt: iso(t0),
    noteAccess: [{ path: '/old/note.md', action: 'read' }] });
  const events = [
    [100, { type: 'thinking_start' }],
    [60, { type: 'thinking_delta', delta: '先查一下相关的笔记。' }],
    [100, { type: 'text_delta', delta: '我先看看你最近写的笔记，再整理一份回答给你。' }],
    [150, { type: 'tool_use', id: 't1', name: 'Read', input: { file_path: NOTES[0] } }],
    [60, { type: 'note_access', path: NOTES[0], action: 'read' }],
    [150, { type: 'tool_result', content: 'october' }],
    [100, { type: 'text_delta', delta: '接着把结论写进去。' }],
    [100, { type: 'tool_use', id: 't2', name: 'Edit', input: { file_path: NOTES[1] } }],
    [60, { type: 'note_access', path: NOTES[1], action: 'write' }],
    [150, { type: 'tool_result', content: 'ok' }],
  ].concat(Array.from({ length: steps }, (_, i) => [
    [60, { type: 'tool_use', id: 's' + i, name: 'Grep', input: { pattern: 'step ' + i } }],
    [60, { type: 'tool_result', content: 'hit ' + i }],
  ]).flat()).concat(reply.match(/[\s\S]{1,4}/g).map((d) => [12, { type: 'text_delta', delta: d }]));

  window.fetch = async (input, init) => {
    const u = new URL(typeof input === 'string' ? input : input.url, location.href);
    const method = (init && init.method) || 'GET';
    switch (u.pathname) {
      case '/api/mates': return json({ mates: [{ id: 'm1', name: 'Hylo', enabled: true, agentId: 'claude' }] });
      case '/api/mate-events': return json({ events: [] });
      case '/api/mates/m1': return json({ mate: { id: 'm1', triggers: [] } });
      case '/api/conversations': return json({ conversations: [{ id: 'c1' }] });
      case '/api/conversations/c1': {
        const since = +u.searchParams.get('since') || 0;
        return json({ messages: T.db.filter((m) => !since || new Date(m.updatedAt).getTime() > since), hasMore: false }, 20);
      }
      case '/api/notes/info': {
        const paths = missing ? [] : JSON.parse(init.body).paths;
        return json({ notes: paths.map((path) => ({ path, title: path.split('/').pop().replace(/\.md$/, ''), preview: '预览 ' + path, updatedAt: iso(Date.now()) })) }, 150);
      }
      case '/api/chat': {
        if (method !== 'POST') break;
        const req = JSON.parse(init.body);
        const start = Date.now(), trace = [], enc = new TextEncoder();
        T.sentAt = performance.now();
        return new Response(new ReadableStream({
          async start(ctrl) {
            const send = (e, d) => ctrl.enqueue(enc.encode('event: ' + e + '\ndata: ' + JSON.stringify(d) + '\n\n'));
            send('start', { runId: 'r1' });
            for (const [delay, d] of events) {
              await new Promise((r) => setTimeout(r, delay));
              d.at = Date.now();
              trace.push(d);
              send('agent', d);
            }
            const end = Date.now();
            T.db.push({ id: req.userMessageId, role: 'user', content: req.message, createdAt: iso(start), updatedAt: iso(start) });
            T.db.push({ id: req.assistantMessageId, role: 'assistant', mateId: 'm1', agentId: 'claude', content: reply, status: 'succeeded',
              createdAt: iso(start), updatedAt: iso(end), trace, noteAccess: [{ path: NOTES[0], action: 'read' }, { path: NOTES[1], action: 'write' }] });
            send('end', { status: 'succeeded' });
            T.endAt = performance.now();
            ctrl.close();
          },
        }));
      }
    }
    return new Response('', { status: 404 });
  };
}

// Per painted frame: where the last user bubble sits above the scroller's bottom,
// and the reply's first finished block, so a rebuilt one shows up as a new node.
function sample() {
  const T = window.__T;
  const sc = document.querySelector('#chat-scroll'), thread = document.querySelector('#chat-thread');
  new MutationObserver((list) => {
    for (const m of list) for (const n of m.removedNodes) if (n.nodeType === 1 && n.classList.contains('msg-row')) T.rebuilt++;
  }).observe(thread, { childList: true, subtree: true });
  const blocks = new Map();
  let nextId = 1;
  // The live trace's box, and whether its newest step sits inside it.
  const traceFrame = (row) => {
    const clip = row && row.querySelector('.seg-trace-shell > .chat-clip');
    const last = clip && clip.querySelector('.seg-trace-rail').lastElementChild;
    if (!last) return null;
    const c = clip.getBoundingClientRect(), r = last.getBoundingClientRect();
    return { h: c.height, rail: clip.firstElementChild.offsetHeight, newest: r.bottom <= c.bottom + 0.5 && r.top >= c.top - 0.5 };
  };
  const tick = () => {
    const users = thread.querySelectorAll('.msg-row-user');
    const rows = thread.querySelectorAll('.msg-row-assistant');
    // Only once a later block exists: the tail is redrawn as it grows, by design.
    const prose = rows.length > 1 && rows[rows.length - 1].querySelector('.msg-body > .prose');
    const first = prose && prose.children.length > 1 && prose.firstElementChild;
    const live = rows.length > 1 && rows[rows.length - 1];
    if (first && !blocks.has(first)) blocks.set(first, nextId++);
    T.frames.push({
      t: performance.now(),
      y: users.length ? users[users.length - 1].getBoundingClientRect().top - sc.getBoundingClientRect().bottom : null,
      stuck: sc.classList.contains('is-stuck'),
      parts: live ? [...live.querySelectorAll('.msg-body > *, .seg-trace-rail > *')].map((n) => n.className.split(' ')[0] + '=' + n.getBoundingClientRect().height.toFixed(1)).join(' ') : '',
      first: first ? blocks.get(first) : 0,
      cards: rows.length > 1 ? rows[rows.length - 1].querySelectorAll('.seg-note-access-card').length : 0,
      trace: traceFrame(live),
    });
    requestAnimationFrame(() => setTimeout(tick));
  };
  tick();
}

async function streamTurn(page, opts) {
  await page.addInitScript(stub, { reply: REPLY, ...opts });
  await page.goto(server.url + '/chat');
  await page.evaluate(() => {
    const k = window.__hyloChatCreate({});
    k.start();
    k.attach(document.getElementById('chat-root'));
    k.tryOpen('m1');
  });
  await page.waitForSelector('.msg-row-assistant');
  await page.evaluate(sample);
  await page.fill('#chat-textarea', '整理一下流式界面的问题');
  await page.press('#chat-textarea', 'Enter');
  await page.waitForFunction(() => window.__T.endAt > 0, null, { timeout: 20000 });
  // Past the 5s DB sync that brings the stored reply and its note_access back.
  await page.waitForTimeout(6500);
  return page.evaluate(() => window.__T);
}

function assertSettledStill(T) {
  const ys = T.frames.filter((f) => f.t > T.endAt + 800).map((f) => f.y);
  assert.ok(Math.max(...ys) - Math.min(...ys) < 0.5, `the settled thread moved ${(Math.max(...ys) - Math.min(...ys)).toFixed(1)}px`);
}

test('a streamed reply never rebuilds, shrinks under the reader, or moves once settled', async (t) => {
  if (launchError) return t.skip('WebKit unavailable');
  const page = await browser.newPage({ viewport: { width: 760, height: 720 } });
  try {
    const T = await streamTurn(page, {});
    const live = T.frames.filter((f) => f.t > T.sentAt + 100 && f.t < T.endAt && f.stuck && f.y != null);
    assert.equal(T.rebuilt, 0, 'no row is ever replaced');
    const firsts = new Set(T.frames.map((f) => f.first).filter(Boolean));
    assert.equal(firsts.size, 1, 'a finished block is never rebuilt');
    for (let i = 1; i < live.length; i++) {
      assert.ok(live[i].y <= live[i - 1].y + 0.5, `streaming moved earlier content down by ${(live[i].y - live[i - 1].y).toFixed(1)}px\n${live[i - 1].parts}\n${live[i].parts}`);
    }
    assert.ok(live.some((f) => f.cards > 0), 'touched notes show while the run is live');
    assertSettledStill(T);
  } finally {
    await page.close();
  }
});

test('notes the vault does not have take no room, live or from history', async (t) => {
  if (launchError) return t.skip('WebKit unavailable');
  const page = await browser.newPage({ viewport: { width: 760, height: 720 } });
  try {
    const T = await streamTurn(page, { missing: true });
    assert.ok(T.frames.every((f) => f.cards === 0), 'a live run never shows a card for a missing note');
    const strips = await page.evaluate(() => [...document.querySelectorAll('.chat-reveal')]
      .filter((r) => r.querySelector('.seg-note-access'))
      // Its own height, the negative margin that takes back its gap, and that gap.
      .map((r) => r.getBoundingClientRect().height + parseFloat(getComputedStyle(r).marginTop) + parseFloat(getComputedStyle(r.parentElement).rowGap)));
    strips.forEach((h) => assert.ok(Math.abs(h) < 0.5, `an empty notes strip still takes ${h.toFixed(1)}px`));
    assertSettledStill(T);
  } finally {
    await page.close();
  }
});

test('a small scroll up mid-stream leaves the bottom and stays where the reader put it', async (t) => {
  if (launchError) return t.skip('WebKit unavailable');
  const page = await browser.newPage({ viewport: { width: 760, height: 720 } });
  try {
    await page.addInitScript(stub, { reply: REPLY + '\n\n' + REPLY });
    await page.goto(server.url + '/chat');
    await page.evaluate(() => {
      const k = window.__hyloChatCreate({});
      k.start();
      k.attach(document.getElementById('chat-root'));
      k.tryOpen('m1');
    });
    await page.waitForSelector('.msg-row-assistant');
    await page.fill('#chat-textarea', '整理一下流式界面的问题');
    await page.press('#chat-textarea', 'Enter');
    await page.waitForFunction(() => document.querySelectorAll('.msg-body > .prose > *').length > 4);
    // Well inside the bottom's 64px stretch: the reader's direction alone has to count.
    await page.mouse.move(380, 300);
    for (let i = 0; i < 3; i++) { await page.mouse.wheel(0, -12); await page.waitForTimeout(16); }
    const place = () => page.evaluate(() => {
      const sc = document.querySelector('#chat-scroll'), rows = document.querySelectorAll('.msg-row-user');
      return { stuck: sc.classList.contains('is-stuck'), y: rows[rows.length - 1].getBoundingClientRect().top };
    });
    // The origin moves only once the gesture is over.
    await page.waitForTimeout(800);
    const left = await place();
    await page.waitForFunction(() => window.__T.endAt > 0, null, { timeout: 20000 });
    await page.waitForTimeout(1200);
    const after = await place();
    assert.equal(left.stuck, false, 'scrolling up left the bottom');
    assert.equal(after.stuck, false, 'and nothing pulled it back');
    assert.ok(Math.abs(after.y - left.y) < 1, `the reader's place moved ${(after.y - left.y).toFixed(1)}px while the reply finished`);
  } finally {
    await page.close();
  }
});

test('a long trace stays capped with its newest step in view, and opens in full', async (t) => {
  if (launchError) return t.skip('WebKit unavailable');
  const page = await browser.newPage({ viewport: { width: 760, height: 720 } });
  try {
    const T = await streamTurn(page, { steps: 10 });
    const cap = await page.evaluate(() => parseFloat(getComputedStyle(document.documentElement).fontSize) * 9.5);
    const live = T.frames.filter((f) => f.t > T.sentAt && f.t < T.endAt && f.trace);
    assert.ok(live.some((f) => f.trace.rail > cap + 20), 'the trace outgrew its cap');
    live.forEach((f) => assert.ok(f.trace.h <= cap + 0.5, `the live trace grew to ${f.trace.h.toFixed(1)}px`));
    live.forEach((f) => assert.ok(f.trace.newest, 'the newest step is in view'));
    // A stuck thread pins itself back to the bottom, so the head is clicked in place.
    const open = await page.evaluate(async () => {
      const row = [...document.querySelectorAll('.msg-row-assistant')].pop();
      row.querySelector('.seg-trace-head').click();
      await new Promise((r) => setTimeout(r, 600));
      const clip = row.querySelector('.seg-trace-shell > .chat-clip');
      return { h: clip.getBoundingClientRect().height, rail: clip.firstElementChild.offsetHeight };
    });
    assert.ok(Math.abs(open.h - open.rail) < 0.5, `opened, it shows ${open.h.toFixed(1)} of ${open.rail}px`);
  } finally {
    await page.close();
  }
});
