// Embed fences in the editor: a page card links out, an audio card plays in place.
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

function silentWav() {
  const dataLen = 2;
  const buf = Buffer.alloc(44 + dataLen);
  buf.write('RIFF', 0);
  buf.writeUInt32LE(36 + dataLen, 4);
  buf.write('WAVE', 8);
  buf.write('fmt ', 12);
  buf.writeUInt32LE(16, 16);
  buf.writeUInt16LE(1, 20);
  buf.writeUInt16LE(1, 22);
  buf.writeUInt32LE(8000, 24);
  buf.writeUInt32LE(16000, 28);
  buf.writeUInt16LE(2, 32);
  buf.writeUInt16LE(16, 34);
  buf.write('data', 36);
  buf.writeUInt32LE(dataLen, 40);
  return buf;
}

test('page card links out and audio plays without leaving the note', async (t) => {
  if (launchError) return t.skip('WebKit unavailable');
  const page = await browser.newPage({ viewport: { width: 800, height: 700 } });
  const wav = server.url + '/media.wav';
  await page.route('**/media.wav', (route) => route.fulfill({
    status: 200,
    contentType: 'audio/wav',
    body: silentWav(),
  }));
  await page.route('**/api/embeds', async (route) => {
    const body = JSON.parse(route.request().postData() || '{"urls":[]}');
    const embeds = (body.urls || []).map((url) => {
      if (url.endsWith('.mp3')) {
        return { url, final_url: wav, kind: 'audio', status: 'ok', title: 'Episode', site_name: 'Radio' };
      }
      return {
        url, final_url: url, kind: 'page', status: 'ok',
        title: 'Story title', description: 'A short line', site_name: 'Example',
      };
    });
    await route.fulfill({
      status: 200,
      contentType: 'application/x-ndjson',
      body: embeds.map((e) => JSON.stringify(e) + '\n').join(''),
    });
  });
  try {
    await page.goto(server.url + '/?f=embed.md');
    await page.waitForFunction(() => window.ready);
    await page.waitForSelector('.hy-embed-page');
    await page.waitForSelector('.hy-embed-audio');
    const href = await page.getAttribute('.hy-embed-page', 'href');
    assert.equal(href, 'https://example.com/story');
    assert.equal(await page.textContent('.hy-embed-page .hy-embed-title'), 'Story title');
    assert.equal(await page.getAttribute('.hy-embed-page', 'target'), '_blank');
    await page.click('.hy-embed-audio');
    await page.waitForFunction(() => {
      const a = document.querySelector('.hy-embed-audio audio');
      return a && !a.paused;
    });
    await page.waitForFunction(() =>
      [...document.querySelectorAll('.hy-embed-row')].some((row) => row.querySelectorAll('.hy-embed-page').length === 4));
    await page.waitForFunction(() =>
      [...document.querySelectorAll('.hy-embed-row')].some((row) => row.querySelectorAll('.hy-embed-audio').length === 3));
    assert.equal(await page.locator('.hy-embed').count(), 11);
    const audio = await page.evaluate(() => {
      const row = [...document.querySelectorAll('.hy-embed-row')].find((r) => r.querySelectorAll('.hy-embed-audio').length === 3);
      const card = row.querySelector('.hy-embed-audio');
      const play = card.querySelector('.hy-embed-play').getBoundingClientRect();
      const title = card.querySelector('.hy-embed-title').getBoundingClientRect();
      const box = card.getBoundingClientRect();
      return { titleBelow: title.y >= play.y + play.height - 2, titleWide: title.width > box.width * 0.7 };
    });
    assert.equal(audio.titleBelow, true);
    assert.equal(audio.titleWide, true);
    assert.equal(await page.getAttribute('.hy-embed-audio a.hy-embed-title', 'href'), wav);
    // Edit lives in the card's bar, clear of anything the embed puts in its corner.
    await page.mouse.move(0, 0);
    const solo = '.hy-card-embed:has(> .hy-card-body > .hy-embed-audio)';
    const bar = solo + ' > .hy-card-bar';
    await page.waitForFunction((sel) => getComputedStyle(document.querySelector(sel)).opacity === '0', bar);
    await page.hover(solo);
    await page.waitForFunction((sel) => getComputedStyle(document.querySelector(sel)).opacity === '1', bar);
    const clear = await page.evaluate((sel) => {
      const card = document.querySelector(sel);
      const b = card.querySelector('.hy-card-bar').getBoundingClientRect();
      return [...card.querySelectorAll('.hy-embed-play, .hy-embed-time')].every((n) => {
        const r = n.getBoundingClientRect();
        return r.right <= b.left || r.left >= b.right || r.bottom <= b.top || r.top >= b.bottom;
      });
    }, solo);
    assert.equal(clear, true);
    const rows = await page.locator('.hy-embed-row').evaluateAll((list) =>
      list.map((row) => [...row.querySelectorAll('.hy-embed-page')].map((el) => {
        const r = el.getBoundingClientRect();
        return { x: r.x, y: r.y, w: r.width };
      })));
    const pair = rows.find((r) => r.length === 2);
    const grid = rows.find((r) => r.length === 4);
    assert.ok(pair[1].x >= pair[0].x + pair[0].w - 1);
    assert.ok(Math.abs(pair[0].y - pair[1].y) < 2);
    assert.ok(grid[1].x >= grid[0].x + grid[0].w - 1);
    assert.ok(grid[2].x >= grid[1].x + grid[1].w - 1);
    assert.ok(Math.abs(grid[0].y - grid[2].y) < 2);
    assert.ok(grid[3].y > grid[0].y + 40);
    assert.ok(Math.abs(grid[3].x - grid[0].x) < 2);
    assert.ok(Math.abs(grid[3].w - grid[0].w) < 2);
  } finally {
    await page.close();
  }
});

test('a streaming fence asks for nothing until it closes', async (t) => {
  if (launchError) return t.skip('WebKit unavailable');
  const page = await browser.newPage({ viewport: { width: 800, height: 700 } });
  const asked = [];
  await page.route('**/api/embeds', async (route) => {
    const { urls } = JSON.parse(route.request().postData());
    asked.push(...urls);
    await route.fulfill({
      status: 200,
      contentType: 'application/x-ndjson',
      body: urls.map((url) => JSON.stringify({ url, final_url: url, kind: 'page', status: 'ok', title: 'Done' }) + '\n').join(''),
    });
  });
  try {
    await page.goto(server.url + '/?f=mixed.md');
    await page.waitForFunction(() => window.ready);
    await page.evaluate(async () => {
      const m = await import('/bundle/fence-card.js');
      const box = document.createElement('div');
      box.id = 'chat';
      document.body.prepend(box);
      const full = 'https://news.example.com/2026/10/23/story';
      const put = (src, openTail) => {
        box.innerHTML = '<pre><code class="language-embed"></code></pre>';
        box.querySelector('code').textContent = src;
        m.hydrateFenceCards(box, { openTail });
      };
      for (let n = 'https://n'.length; n <= full.length; n += 4) put(full.slice(0, n), true);
      put(full, true);
      await new Promise((r) => setTimeout(r, 50));
      window.beforeClose = document.querySelectorAll('#chat .hy-embed-pending').length;
      put(full, false);
    });
    await page.waitForSelector('#chat .hy-embed-page');
    assert.equal(await page.evaluate(() => window.beforeClose), 1);
    assert.deepEqual(asked, ['https://news.example.com/2026/10/23/story']);
  } finally {
    await page.close();
  }
});

async function embedPage(width, onRequest) {
  const page = await browser.newPage({ viewport: { width, height: 700 } });
  await page.route('**/api/embeds', async (route) => {
    const body = JSON.parse(route.request().postData());
    const embeds = body.urls.map((url) => onRequest(url, body));
    await route.fulfill({
      status: 200,
      contentType: 'application/x-ndjson',
      body: embeds.map((e) => JSON.stringify(e) + '\n').join(''),
    });
  });
  await page.goto(server.url + '/?f=embed.md');
  await page.waitForFunction(() => window.ready);
  return page;
}

test('a failed card says so and retries on demand', async (t) => {
  if (launchError) return t.skip('WebKit unavailable');
  const retries = [];
  const page = await embedPage(800, (url, body) => {
    if (body.retry) retries.push(url);
    if (url !== 'https://example.com/story') return { url, final_url: url, kind: 'page', status: 'ok', title: 'Other' };
    return body.retry
      ? { url, final_url: url, kind: 'page', status: 'ok', title: 'Back again' }
      : { url, final_url: url, status: 'error', error: 'fetch_failed' };
  });
  try {
    const failed = page.locator('.hy-embed-failed').first();
    await failed.waitFor();
    assert.equal(await failed.locator('.hy-embed-title').textContent(), 'example.com/story');
    assert.equal(await failed.locator('.hy-embed-desc').textContent(), 'Preview unavailable');
    await failed.locator('.hy-embed-retry').click();
    await page.waitForSelector('.hy-embed-page >> text=Back again');
    assert.deepEqual(retries, ['https://example.com/story']);
    assert.equal(await page.locator('.hy-embed-failed').count(), 0);
  } finally {
    await page.close();
  }
});

test('a grid too narrow for three across is a list', async (t) => {
  if (launchError) return t.skip('WebKit unavailable');
  const page = await embedPage(520, (url) => ({ url, final_url: url, kind: 'page', status: 'ok', title: 'T', image_url: server.url + '/image' }));
  try {
    await page.waitForFunction(() =>
      [...document.querySelectorAll('.hy-embed-row')].some((row) => row.querySelectorAll('.hy-embed-page').length === 4));
    const xs = await page.evaluate(() => {
      const row = [...document.querySelectorAll('.hy-embed-row')].find((r) => r.querySelectorAll('.hy-embed-page').length === 4);
      return [...row.children].map((c) => Math.round(c.getBoundingClientRect().x));
    });
    assert.equal(new Set(xs).size, 1);
    // Listed cards keep the single card's image-left layout.
    const side = await page.evaluate(() => {
      const card = document.querySelector('.hy-embed-row .hy-embed-page');
      const img = card.querySelector('.hy-embed-shot').getBoundingClientRect();
      const text = card.querySelector('.hy-embed-text').getBoundingClientRect();
      return text.left >= img.right - 1;
    });
    assert.equal(side, true);
  } finally {
    await page.close();
  }
});

test('Edit source puts the caret at the end of the first URL, with no floating preview', async (t) => {
  if (launchError) return t.skip('WebKit unavailable');
  const page = await embedPage(800, (url) => ({ url, final_url: url, kind: 'page', status: 'ok', title: 'T' }));
  try {
    const card = page.locator('.hy-card-embed').first();
    await card.waitFor();
    await card.hover();
    await card.locator('.hy-card-bar button').click();
    const caret = await page.evaluate(() => {
      const sel = getSelection();
      const line = sel.focusNode && sel.focusNode.parentElement.closest('.cm-line');
      const r = document.createRange();
      r.setStart(line, 0);
      r.setEnd(sel.focusNode, sel.focusOffset);
      return { line: line.textContent, before: r.toString() };
    });
    assert.equal(caret.line, 'https://example.com/story');
    assert.equal(caret.before, caret.line);
    assert.equal(await page.locator('.hy-pip').count(), 0);
  } finally {
    await page.close();
  }
});
