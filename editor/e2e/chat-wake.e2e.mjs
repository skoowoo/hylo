// A bot reaching out on its own: the wake is no row, and its reply joins the thread
// only once the turn is over, whole, with what woke it in the footer. Nothing on
// screen grows, shrinks or moves for a wake, a silent one included. The API and
// live feed are stubbed; the pane is real.
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

const REPLY = `## 本周回顾

这周你主要在想**流式界面**的稳定性，和上个月关于 [[chat-redesign]] 的判断一致。

- 内容显示后不该再改变尺寸
- 结构上解决，比事后补偿更可靠

> 最好的修复让问题无从发生。`;

function stub() {
  const T = (window.__T = { db: [], frames: [], removed: 0, live: {} });
  const iso = (ms) => new Date(ms).toISOString();
  const json = (o, delay) => new Promise((r) => setTimeout(() => r(new Response(JSON.stringify(o))), delay || 0));
  let t0 = Date.now() - 3600e3;
  for (let i = 0; i < 30; i++) {
    T.db.push({ id: 'u' + i, role: 'user', content: '第 ' + i + ' 个问题', createdAt: iso(t0), updatedAt: iso(t0) });
    T.db.push({ id: 'a' + i, role: 'assistant', mateId: 'm1', agentId: 'claude', status: 'succeeded', createdAt: iso(t0 + 1), updatedAt: iso(t0 + 1),
      content: '回答 ' + i + '：这是一段足够长的回复，好让整个对话超过一屏，可以向上滚动阅读。'.repeat(2) });
    t0 += 1000;
  }
  window.__hyloLive = { on: (topic, fn) => { T.live[topic] = fn; } };
  window.fetch = async (input, init) => {
    const u = new URL(typeof input === 'string' ? input : input.url, location.href);
    switch (u.pathname) {
      case '/api/mates': return json({ mates: [{ id: 'm1', name: 'Recall', enabled: true, agentId: 'claude' }] });
      case '/api/mate-events': return json({ events: [{ type: 'scheduled', label: 'Scheduled', vars: [] }] });
      case '/api/mates/m1/seen': return json({ ok: true });
      case '/api/conversations': return json({ conversations: [{ id: 'c1' }] });
      case '/api/conversations/c1': {
        const since = +u.searchParams.get('since') || 0;
        return json({ messages: T.db.filter((m) => !since || new Date(m.updatedAt).getTime() > since), hasMore: false }, 20);
      }
      case '/api/notes/info': return json({ notes: [] });
    }
    return new Response('', { status: 404 });
  };
}

// Per painted frame: the last history reply's top against the scroller's bottom
// (what a stuck reader sees move), and the wake reply's height once it exists.
function sample() {
  const T = window.__T;
  const sc = document.querySelector('#chat-scroll'), thread = document.querySelector('#chat-thread');
  new MutationObserver((list) => {
    for (const m of list) for (const n of m.removedNodes) if (n.nodeType === 1 && n.classList.contains('msg-row')) T.removed++;
  }).observe(thread, { childList: true, subtree: true });
  const tick = () => {
    const box = sc.getBoundingClientRect();
    const old = [...thread.children].find((n) => n._msgId === 'a29');
    const reply = [...thread.children].find((n) => n._msgId === 'r2' || n._msgId === 'r3');
    T.frames.push({
      t: performance.now(),
      stuck: sc.classList.contains('is-stuck'),
      old: old ? old.getBoundingClientRect().top - box.bottom : null,
      replyH: reply ? reply.getBoundingClientRect().height : null,
      last: thread.lastElementChild.previousElementSibling.getBoundingClientRect().bottom - box.bottom,
    });
    requestAnimationFrame(() => setTimeout(tick));
  };
  tick();
}

async function open(page) {
  await page.addInitScript(stub);
  await page.goto(server.url + '/chat');
  await page.evaluate(() => {
    const k = (window.__K = window.__hyloChatCreate({}));
    k.start();
    k.attach(document.getElementById('chat-root'));
    k.tryOpen('m1');
  });
  await page.waitForFunction(() => document.querySelectorAll('.msg-row-assistant').length === 30);
  await page.waitForTimeout(300);
  await page.evaluate(sample);
}

// The server's side of a wake: its rows go in when it starts and settle when it ends.
function wakeStarts(id) {
  const T = window.__T, at = new Date().toISOString();
  T.db.push({ id: 'w' + id, role: 'wake', content: 'TASK', triggerEvent: 'scheduled', triggerId: 't1', triggerName: 'Weekly review', triggerSchedule: 'weekly mon 09:00', createdAt: at, updatedAt: at });
  T.db.push({ id: 'r' + id, role: 'assistant', mateId: 'm1', agentId: 'claude', status: 'running', triggerEvent: 'scheduled', triggerId: 't1', content: '', createdAt: at, updatedAt: at });
}

const since = (T, t) => T.frames.filter((f) => f.t > t);
const spread = (xs) => Math.max(...xs) - Math.min(...xs);

test('a running wake stays off screen, and a silent one leaves without moving anything', async (t) => {
  if (launchError) return t.skip('WebKit unavailable');
  const page = await browser.newPage({ viewport: { width: 760, height: 720 } });
  try {
    await open(page);
    await page.evaluate(wakeStarts, 1);
    await page.evaluate(() => window.__K.reload());
    await page.waitForTimeout(400);
    assert.equal(await page.locator('.msg-wake-source[title]').count(), 0, 'a running wake is not shown');
    assert.equal(await page.locator('.msg-row').count(), 60, 'nor its reply');
    await page.evaluate(() => {
      const T = window.__T;
      T.db = T.db.filter((m) => m.id !== 'w1' && m.id !== 'r1');
      T.mark = performance.now();
      T.live.chat([{ topic: 'chat', op: 'wake', data: { mateId: 'm1', wakeId: 'w1', replyId: 'r1', status: 'succeeded', silent: true } }]);
    });
    await page.waitForTimeout(600);
    const T = await page.evaluate(() => window.__T);
    const ys = T.frames.map((f) => f.old);
    assert.ok(spread(ys) < 0.5, `the thread moved ${spread(ys).toFixed(1)}px for a wake the user never saw`);
    assert.equal(T.removed, 0, 'no row came and went');
  } finally {
    await page.close();
  }
});

test('a finished wake reply lands whole, says what woke it, and the bottom stays put', async (t) => {
  if (launchError) return t.skip('WebKit unavailable');
  const page = await browser.newPage({ viewport: { width: 760, height: 720 } });
  try {
    await open(page);
    await page.evaluate(wakeStarts, 2);
    await page.evaluate((reply) => {
      const T = window.__T, at = new Date().toISOString();
      Object.assign(T.db.find((m) => m.id === 'r2'), { status: 'succeeded', content: reply, updatedAt: at, triggerName: 'Weekly review', triggerSchedule: 'weekly mon 09:00' });
      T.mark = performance.now();
      T.live.chat([{ topic: 'chat', op: 'wake', data: { mateId: 'm1', wakeId: 'w2', replyId: 'r2', status: 'succeeded', silent: false, text: reply } }]);
    }, REPLY);
    await page.waitForSelector('.msg-wake-source[title]');
    await page.waitForTimeout(800);
    const T = await page.evaluate(() => window.__T);
    const shown = since(T, T.mark).filter((f) => f.replyH != null);
    assert.ok(shown.length > 10, 'sampled the reply on screen');
    assert.ok(spread(shown.map((f) => f.replyH)) < 0.5, `the reply changed height by ${spread(shown.map((f) => f.replyH)).toFixed(1)}px`);
    const rest = T.frames.filter((f) => f.t < T.mark).pop().last;
    assert.ok(shown.every((f) => f.stuck && Math.abs(f.last - rest) < 0.5), 'the newest row sits where the last one did, from its first frame');
    assert.equal(T.removed, 0, 'no row is replaced');
    assert.equal(await page.locator('.msg-row').count(), 61, 'the wake itself is no row');
    const src = page.locator('.msg-wake-source[title]');
    assert.equal((await src.innerText()).trim(), 'Weekly review');
    assert.match(await src.getAttribute('title'), /Weekly mon 09:00/);
    const foot = await page.evaluate(() => {
      const f = document.querySelector('.msg-wake-source[title]').parentElement;
      const plain = document.querySelector('.msg-row-assistant .msg-footer');
      return [f.getBoundingClientRect().height, plain.getBoundingClientRect().height];
    });
    assert.ok(Math.abs(foot[0] - foot[1]) < 0.5, `the source made the footer ${foot[0]}px, not ${foot[1]}px`);
  } finally {
    await page.close();
  }
});

async function landWake(page) {
  await page.evaluate(wakeStarts, 3);
  await page.evaluate((reply) => {
    const T = window.__T;
    Object.assign(T.db.find((m) => m.id === 'r3'), { status: 'succeeded', content: reply, updatedAt: new Date().toISOString() });
    T.live.chat([{ topic: 'chat', op: 'wake', data: { mateId: 'm1', wakeId: 'w3', replyId: 'r3', status: 'succeeded', silent: false } }]);
  }, REPLY);
  await page.waitForSelector('.msg-wake-source[title]');
  await page.waitForTimeout(500);
}

test('a split change parks rows around a wake reply without moving what is in view', async (t) => {
  if (launchError) return t.skip('WebKit unavailable');
  const page = await browser.newPage({ viewport: { width: 760, height: 720 } });
  try {
    await open(page);
    await landWake(page);
    // Read from the top, so the wake reply sits among the rows that get parked.
    await page.mouse.move(380, 300);
    for (let i = 0; i < 40; i++) { await page.mouse.wheel(0, -600); await page.waitForTimeout(16); }
    await page.waitForTimeout(900);
    const before = await page.evaluate(() => [...document.querySelectorAll('#chat-thread > .msg-row')]
      .map((n) => n.getBoundingClientRect().top).filter((y) => y > 0 && y < innerHeight));
    const during = await page.evaluate(async () => {
      const w = document.querySelector('#chat-thread').getBoundingClientRect().width;
      window.dispatchEvent(new CustomEvent('hylo:split-moving', { detail: { listFrom: w, listTo: w } }));
      await new Promise((r) => requestAnimationFrame(() => setTimeout(r)));
      const parked = document.querySelectorAll('.is-split-parked').length;
      const ys = [...document.querySelectorAll('#chat-thread > .msg-row:not(.is-split-parked)')]
        .map((n) => n.getBoundingClientRect().top).filter((y) => y > 0 && y < innerHeight);
      window.dispatchEvent(new CustomEvent('hylo:split-settled'));
      await new Promise((r) => requestAnimationFrame(() => setTimeout(r)));
      return { parked, ys };
    });
    const after = await page.evaluate(() => [...document.querySelectorAll('#chat-thread > .msg-row')]
      .map((n) => n.getBoundingClientRect().top).filter((y) => y > 0 && y < innerHeight));
    assert.ok(during.parked > 0, 'rows out of view were parked');
    assert.deepEqual(during.ys.map(Math.round), before.map(Math.round), 'parking moved rows in view');
    assert.deepEqual(after.map(Math.round), before.map(Math.round), 'unparking moved rows in view');
  } finally {
    await page.close();
  }
});
