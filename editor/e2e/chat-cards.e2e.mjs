// Proposal, receipt and sources cards, hydrated as the chat pane does. Their buttons
// answer through `hylo:choice` only while the host has armed them.
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

const CARDS = {
  proposal: 'title: 每日 AI 早报\nsummary: 每天 08:00，挑 5 条和你的 RAG 项目相关的 AI 新闻，放进收件箱\ndetails: 主 bot · daily-ai-news（待添加）· 每天 08:00\n- 就这样\n- 先试跑一次\n- 改一下\n- 不用了',
  receipt: 'title: 每日 AI 早报已开启\n- 主 bot 每天 08:00 运行，结果进收件箱\n- 已添加 daily-ai-news\nundo: 撤销每日 AI 早报\nids: trigger m1/t9; skill daily-ai-news on m1 (added now)',
  sources: 'note: [RAG 踩坑](/notes?name=RAG%20%E8%B8%A9%E5%9D%91.md)\nnote: 向量库选型\nskill: writing-shape\nswap: 换个写法重做',
};

async function cardsPage(b, url, width = 720) {
  const page = await b.newPage({ viewport: { width, height: 700 } });
  await page.goto(url + '/?f=mixed.md');
  await page.waitForFunction(() => window.ready);
  await page.evaluate(async (cards) => {
    const box = document.createElement('div');
    box.id = 'chat';
    box.style.cssText = 'padding:16px;max-width:640px';
    for (const [lang, src] of Object.entries(cards)) {
      const pre = document.createElement('pre');
      const code = document.createElement('code');
      code.className = 'language-' + lang;
      code.textContent = src;
      pre.appendChild(code);
      box.appendChild(pre);
    }
    document.getElementById('area').style.display = 'none';
    document.body.prepend(box);
    window.picks = [];
    box.addEventListener('hylo:choices-mount', () => {
      box.querySelectorAll('.hy-choices-host').forEach((c) => c._hyChoices && c._hyChoices.setState({ mode: 'active' }));
      box.querySelectorAll('.hy-receipt-host').forEach((c) => c._hyReceipt.setState({ mode: 'active' }));
    });
    box.addEventListener('hylo:choice', (e) => window.picks.push(e.detail.text));
    const m = await import('/bundle/fence-card.js');
    m.hydrateFenceCards(box);
  }, CARDS);
  await page.waitForSelector('#chat .hy-proposal .hy-choices-active');
  await page.waitForSelector('#chat .hy-receipt-undo:not([disabled])');
  await page.waitForSelector('#chat .hy-sources-skill');
  return page;
}

test('every card answers with the text it was given', async (t) => {
  if (launchError) return t.skip('WebKit unavailable');
  const page = await cardsPage(browser, server.url);
  try {
    await page.click('#chat .hy-proposal .hy-choice >> text=先试跑一次');
    await page.click('#chat .hy-receipt-undo');
    await page.click('#chat .hy-sources-skill');
    assert.deepEqual(await page.evaluate(() => window.picks), ['先试跑一次', '撤销每日 AI 早报', '换个写法重做']);
  } finally {
    await page.close();
  }
});

test('details fold, ids stay hidden, notes link by name', async (t) => {
  if (launchError) return t.skip('WebKit unavailable');
  const page = await cardsPage(browser, server.url);
  try {
    assert.equal(await page.isVisible('#chat .hy-proposal-details'), false);
    await page.click('#chat .hy-proposal-more');
    assert.equal(await page.isVisible('#chat .hy-proposal-details'), true);
    assert.equal(await page.locator('#chat .hy-receipt').innerText().then((s) => s.includes('m1/t9')), false);
    const hrefs = await page.$$eval('#chat .hy-sources a', (as) => as.map((a) => decodeURIComponent(a.getAttribute('href'))));
    assert.deepEqual(hrefs, ['/notes?name=RAG 踩坑.md', '/notes?name=向量库选型.md']);
  } finally {
    await page.close();
  }
});

test('an undone receipt drops its button', async (t) => {
  if (launchError) return t.skip('WebKit unavailable');
  const page = await cardsPage(browser, server.url);
  try {
    await page.evaluate(() => document.querySelector('#chat .hy-receipt-host')._hyReceipt.setState({ mode: 'done' }));
    assert.equal(await page.locator('#chat .hy-receipt-undo').count(), 0);
    assert.equal(await page.locator('#chat .hy-receipt-done').innerText(), 'Undone');
  } finally {
    await page.close();
  }
});
