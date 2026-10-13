// Chat choices, hydrated from marked HTML as the chat pane does: one row when every
// option fits, else one per line; picks bubble up as `hylo:choice` once a host arms them.
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

const SHORT = '- Yes\n- No';
const LONG = [
  '- [ ] daily-ai-news — daily AI news from news sites and web search',
  '- [ ] hackernews — top Hacker News stories and discussions',
  '- [ ] the-news — front-page headlines from newspapers in 20 countries',
].join('\n');

async function chatPage(width) {
  const page = await browser.newPage({ viewport: { width, height: 700 } });
  await page.goto(server.url + '/?f=mixed.md');
  await page.waitForFunction(() => window.ready);
  await page.evaluate(async ({ short, long }) => {
    // A scrolled thread like the chat's: content above, the cards at the bottom.
    const box = document.createElement('div');
    box.id = 'chat';
    box.style.cssText = 'height:400px;overflow-y:auto';
    const above = document.createElement('div');
    above.style.height = '1200px';
    box.appendChild(above);
    for (const src of [short, long]) {
      const pre = document.createElement('pre');
      const code = document.createElement('code');
      code.className = 'language-choices';
      code.textContent = src;
      pre.appendChild(code);
      box.appendChild(pre);
    }
    document.getElementById('area').style.display = 'none';
    document.body.prepend(box);
    window.picks = [];
    box.addEventListener('hylo:choices-mount', (e) => e.target._hyChoices.setState({ mode: 'active' }));
    box.addEventListener('hylo:choice', (e) => window.picks.push(e.detail.text));
    const m = await import('/bundle/fence-card.js');
    m.hydrateFenceCards(box);
  }, { short: SHORT, long: LONG });
  await page.waitForFunction(() => document.querySelectorAll('#chat .hy-choices-active:not(.hy-choices-measuring)').length === 2);
  return page;
}

const layouts = (page) => page.$$eval('#chat .hy-choices', (els) => els.map((e) => (e.classList.contains('hy-choices-row') ? 'row' : e.classList.contains('hy-choices-col') ? 'col' : 'none')));

test('short options share a row, long ones stack, and a narrow view stacks both', async (t) => {
  if (launchError) return t.skip('WebKit unavailable');
  const page = await chatPage(900);
  try {
    assert.deepEqual(await layouts(page), ['row', 'col']);
    await page.setViewportSize({ width: 80, height: 700 });
    await page.waitForFunction(() => document.querySelector('#chat .hy-choices').classList.contains('hy-choices-col'));
    assert.deepEqual(await layouts(page), ['col', 'col']);
  } finally {
    await page.close();
  }
});

test('a pick is sent as the option text; several are joined', async (t) => {
  if (launchError) return t.skip('WebKit unavailable');
  const page = await chatPage(900);
  try {
    await page.click('#chat .hy-choices >> nth=0 >> text=No');
    const multi = page.locator('#chat .hy-choices').nth(1);
    await multi.locator('.hy-choice', { hasText: 'hackernews' }).click();
    await multi.locator('.hy-choice', { hasText: 'daily-ai-news' }).click();
    await multi.locator('.hy-choices-send').click();
    assert.deepEqual(await page.evaluate(() => window.picks), [
      'No',
      'daily-ai-news — daily AI news from news sites and web search, hackernews — top Hacker News stories and discussions',
    ]);
  } finally {
    await page.close();
  }
});

test('picking several keeps the thread where it is', async (t) => {
  if (launchError) return t.skip('WebKit unavailable');
  const page = await chatPage(900);
  try {
    const top = await page.evaluate(() => {
      const box = document.getElementById('chat');
      box.scrollTop = box.scrollHeight;
      return box.scrollTop;
    });
    const multi = page.locator('#chat .hy-choices').nth(1);
    for (const name of ['hackernews', 'the-news', 'hackernews']) {
      await multi.locator('.hy-choice', { hasText: name }).click();
      assert.equal(await page.evaluate(() => document.getElementById('chat').scrollTop), top, `scroll moved after ${name}`);
    }
    assert.equal(await multi.locator('.hy-choice-on').count(), 1);
  } finally {
    await page.close();
  }
});
