import { test, before, after } from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { webkit } from 'playwright-core';
import { startServer } from './server.mjs';

let browser, server;
before(async () => {
  browser = await webkit.launch();
  server = await startServer();
});
after(async () => {
  await browser?.close();
  await server?.close();
});

async function open(fixture) {
  const page = await browser.newPage({ viewport: { width: 1200, height: 800 } });
  await page.goto(server.url + '/?f=' + fixture);
  await page.waitForFunction(() => window.ready);
  for (const name of ['home.css', 'content_pane.css']) {
    await page.addStyleTag({ content: await readFile(new URL('../../internal/server/view/assets/' + name, import.meta.url), 'utf8') });
  }
  await page.addStyleTag({ content: ':root { --home-side-w:200px; --motion-view:320ms; --motion-slow:220ms; --ease-standard:ease; --ease-out:ease-out; --bg:white; --shell-bg:#aaa; --r-md:8px; } .home-shell { height:800px; flex:none; }' });
  await page.evaluate(() => {
    const shell = document.createElement('div');
    shell.className = 'home-shell content-pane-open';
    shell.style.cssText = '--split-ratio:0;--split-ratio-inv:1';
    shell.innerHTML = '<aside class="home-side"></aside><div class="home-main"><main class="home-list-pane"></main><div class="home-pane-resizer"></div><div class="content-pane open"><div class="content-pane-panel"></div></div></div>';
    document.body.append(shell);
    shell.querySelector('.content-pane-panel').append(document.getElementById('area'));
    window.originalView = view;
  });
  await page.waitForTimeout(500);
  return page;
}

async function animate(page, ratio, { open = true, drag = false, snap = false } = {}) {
  return page.evaluate(async ({ ratio, open, drag, snap }) => {
    const shell = document.querySelector('.home-shell');
    const pane = shell.querySelector('.content-pane');
    const panel = shell.querySelector('.content-pane-panel');
    const list = shell.querySelector('.home-list-pane');
    const frame = () => new Promise(resolve => requestAnimationFrame(resolve));
    const rows = [];
    const sample = () => {
      const a = pane.getBoundingClientRect(), b = panel.getBoundingClientRect(), c = list.getBoundingClientRect();
      rows.push({ paneWidth: a.width, cardWidth: b.width, rightError: Math.abs(a.right - b.right), gap: b.left - c.right,
        anchorY: view.lineBlockAt(window.anchorPos || 0).top - view.scrollDOM.scrollTop,
        sameView: view === originalView, connected: view.dom.isConnected });
    };
    sample();
    shell.classList.toggle('is-resizing', drag);
    shell.classList.toggle('is-split-snapping', snap);
    shell.classList.toggle('content-pane-open', open);
    shell.style.setProperty('--split-ratio', ratio);
    shell.style.setProperty('--split-ratio-inv', 1 - ratio);
    const start = performance.now();
    do { await frame(); sample(); } while (performance.now() - start < 450);
    return rows;
  }, { ratio, open, drag, snap });
}

for (const [fixture, middle] of [['handbook.md', false], ['handbook.md', true], ['mixed.md', false], ['mixed.md', true]]) {
  test(`focus transitions keep the card filled at ${middle ? 'middle' : 'top'} of ${fixture}`, async () => {
    const page = await open(fixture);
    try {
      await page.evaluate((middle) => {
        window.anchorPos = middle ? view.state.doc.line(Math.floor(view.state.doc.lines / 2)).from : 0;
        view.dispatch({ effects: cm.EditorView.scrollIntoView(anchorPos, { y: 'start' }) });
      }, middle);
      await page.waitForTimeout(600);
      for (const ratio of [0.5, 0, 0.65, 0]) {
        const rows = await animate(page, ratio);
        assert.ok(rows.every(r => r.rightError < 1 && Math.abs(r.gap - 4) < 1), JSON.stringify(rows));
        assert.ok(rows.every(r => r.sameView && r.connected));
        const growing = rows.at(-1).paneWidth > rows[0].paneWidth;
        for (let i = 1; i < rows.length; i++) {
          const delta = rows[i].paneWidth - rows[i - 1].paneWidth;
          assert.ok(growing ? delta >= -1 : delta <= 1, 'pane reversed direction');
          assert.ok(Math.abs(rows[i].anchorY - rows[i - 1].anchorY) < 120, 'reading position jumped: ' + JSON.stringify(rows));
        }
      }
    } finally { await page.close(); }
  });
}

test('opening, closing, dragging and snapping share the card boundary', async () => {
  const page = await open('mixed.md');
  try {
    for (const [ratio, options] of [[0.5, {}], [0.5, { open: false }], [0.5, {}], [0.7, { drag: true }], [0, { drag: true, snap: true }]]) {
      const rows = await animate(page, ratio, options);
      assert.ok(rows.filter(r => r.paneWidth >= 4).every(r => r.rightError < 1 && Math.abs(r.cardWidth - r.paneWidth + 4) < 1), JSON.stringify(rows));
      assert.ok(rows.every(r => r.sameView && r.connected));
    }
  } finally { await page.close(); }
});
