// The composer's command palette: "/" (or "、"), "@" and the + button open the
// same list; typing filters the current level: groups at the top, items inside.
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

function stub(skills) {
  const json = (o) => Promise.resolve(new Response(JSON.stringify(o)));
  window.__hyloLive = { on: () => {} };
  window.__hyloContentPane = { tabs: [{ path: 'work/Plan.md', title: 'Plan' }, { path: 'Today.md', title: 'Today' }], activeTab: 1 };
  window.fetch = async (input, init) => {
    const u = new URL(typeof input === 'string' ? input : input.url, location.href);
    switch (u.pathname) {
      case '/api/mates': return json({ mates: [{ id: 'm1', name: 'Recall', enabled: true, agentId: 'claude' }] });
      case '/api/mate-events': return json({ events: [] });
      case '/api/mates/m1/seen': return json({ ok: true });
      case '/api/conversations': return json({ conversations: [{ id: 'c1' }], activeId: 'c1' });
      case '/api/conversations/c1': return json({ messages: [], hasMore: false });
      case '/api/notes/info': return json({ notes: [] });
      case '/api/skills': window.__skillFetches = (window.__skillFetches || 0) + 1; return json({ skills });
      case '/api/search': return init && JSON.parse(init.body).q === '设' ? json({ results: [{ name: 'Hylo 设计.md' }] }) : json({ results: [] });
      case '/api/vault/list-dirs': return json({ dirs: { '/': ['journal', 'work'], '/journal/': ['2026'], '/journal/2026/': [] }[JSON.parse(init.body).path] || [] });
    }
    return new Response('', { status: 404 });
  };
}

const SKILLS = [
  { name: 'hylo-notes', default: true, description: 'Read and write notes', bots: [{ id: 'm1' }] },
  { name: 'news', default: false, description: 'Summarize headlines', bots: [{ id: 'm1' }] },
  { name: 'other-bot', default: false, description: 'Not on here', bots: [{ id: 'm2' }] },
];

async function open(skills) {
  const page = await browser.newPage();
  await page.addInitScript(stub, skills);
  await page.goto(server.url + '/chat');
  await page.evaluate(() => {
    const k = window.__hyloChatCreate({});
    k.start();
    k.attach(document.getElementById('chat-root'));
    k.tryOpen('m1');
  });
  await page.waitForFunction(() => !document.querySelector('#chat-textarea').disabled);
  await page.click('#chat-textarea');
  return page;
}

const rows = (page) => page.$$eval('#chat-cmd-list li[role="option"]', (lis) => lis.map((li) => li.querySelector('.ac-main').textContent));
const value = (page) => page.inputValue('#chat-textarea');
const shown = (page) => page.isVisible('#chat-cmd-panel');
const caret = (page) => page.evaluate(() => document.querySelector('#chat-textarea').selectionStart);
const settle = (page) => page.waitForTimeout(120);
const selected = (page) => page.$$eval('#chat-cmd-list li[aria-selected="true"]', (lis) => lis.map((li) => li.querySelector('.ac-main').textContent));
const head = (page) => page.evaluate(() => { const h = document.querySelector('#chat-cmd-panel .ac-head'); return h.hidden ? null : h.textContent; });
const TOP = ['Image', 'Notes', 'Folders', 'Skills'];

test('bare "/" lists the top level; a group opens in place and Backspace leaves it', async (t) => {
  if (launchError) return t.skip('WebKit unavailable: ' + launchError.message);
  const page = await open(SKILLS);
  try {
    await page.keyboard.type('use /');
    await page.waitForSelector('#chat-cmd-panel:not([hidden])');
    assert.deepEqual(await rows(page), TOP);
    assert.equal(await page.evaluate(() => window.__skillFetches || 0), 0, 'skills load once their group opens');

    await page.keyboard.press('ArrowUp');
    await page.keyboard.press('ArrowRight');
    await settle(page);
    assert.deepEqual(await rows(page), ['news', 'hylo-notes'], 'own skills, then built-in');
    await page.keyboard.press('Backspace');
    await settle(page);
    assert.deepEqual(await rows(page), TOP);
    assert.equal(await value(page), 'use /', 'leaving a group keeps the "/"');

    await page.keyboard.press('ArrowUp');
    await page.keyboard.press('Enter');
    await settle(page);
    await page.keyboard.press('ArrowDown');
    await page.keyboard.press('Enter');
    assert.equal(await value(page), 'use hylo-notes ');
    assert.equal(await shown(page), false);
  } finally {
    await page.close();
  }
});

test('typing filters the top level, then the group it opens', async (t) => {
  if (launchError) return t.skip('WebKit unavailable: ' + launchError.message);
  const page = await open(SKILLS);
  try {
    await page.keyboard.type('帮我用/news');
    await settle(page);
    assert.equal(await shown(page), false, 'group items are not searched from the top');
    for (let i = 0; i < 4; i++) await page.keyboard.press('Backspace');
    await page.keyboard.type('sk');
    await page.waitForSelector('#chat-cmd-panel:not([hidden])');
    assert.deepEqual(await rows(page), ['Skills']);
    await page.keyboard.press('Enter');
    assert.equal(await value(page), '帮我用/', 'entering a group clears the query');
    await page.keyboard.type('ne');
    await settle(page);
    assert.deepEqual(await rows(page), ['news']);
    await page.keyboard.press('Enter');
    assert.equal(await value(page), '帮我用news ');

    await page.keyboard.type('/tmpx');
    await settle(page);
    assert.equal(await shown(page), false, 'no match closes');
    await page.keyboard.press('Enter');
    assert.equal(await value(page), '', 'with nothing open Enter sends');

    await page.evaluate(() => { document.querySelector('#chat-file-input').click = () => { window.__picked = true; }; });
    await page.keyboard.type('/图片');
    await page.waitForSelector('#chat-cmd-panel:not([hidden])');
    await page.keyboard.press('Enter');
    assert.equal(await page.evaluate(() => window.__picked), true, 'Image opens the file picker');
    assert.equal(await value(page), '');
  } finally {
    await page.close();
  }
});

test('Esc keeps the typed token and stays shut until it ends', async (t) => {
  if (launchError) return t.skip('WebKit unavailable: ' + launchError.message);
  const page = await open(SKILLS);
  try {
    await page.keyboard.type('/n');
    await page.waitForSelector('#chat-cmd-panel:not([hidden])');
    await page.keyboard.press('Escape');
    await page.keyboard.type('e');
    await settle(page);
    assert.equal(await shown(page), false);
    assert.equal(await value(page), '/ne');
    await page.keyboard.type(' /');
    await page.waitForSelector('#chat-cmd-panel:not([hidden])');
  } finally {
    await page.close();
  }
});

test('the + button types the same "/" and takes it back when nothing is picked', async (t) => {
  if (launchError) return t.skip('WebKit unavailable: ' + launchError.message);
  const page = await open([SKILLS[2]]);
  try {
    await page.keyboard.type('abc');
    await page.click('#chat-add');
    await page.waitForSelector('#chat-cmd-panel:not([hidden])');
    assert.equal(await value(page), 'abc /');
    assert.equal(await page.evaluate(() => document.activeElement.id), 'chat-textarea');
    assert.equal(await page.getAttribute('#chat-add', 'aria-expanded'), 'true');

    await page.click('#chat-cmd-list li[role="option"]:nth-child(4)');
    await settle(page);
    assert.match(await page.textContent('#chat-cmd-list'), /No skills on for this bot/);
    await page.keyboard.press('Escape');
    assert.equal(await value(page), 'abc', 'the + button\'s "/" goes away');
    assert.equal(await caret(page), 3);

    await page.click('#chat-add');
    await page.waitForSelector('#chat-cmd-panel:not([hidden])');
    await page.click('#chat-add');
    assert.equal(await shown(page), false);
    assert.equal(await value(page), 'abc');
  } finally {
    await page.close();
  }
});

test('the + button keeps a selection, and only backing out takes its "/" back', async (t) => {
  if (launchError) return t.skip('WebKit unavailable: ' + launchError.message);
  const page = await open(SKILLS);
  try {
    await page.keyboard.type('hello world');
    await page.keyboard.press('Meta+A');
    await page.click('#chat-add');
    await page.waitForSelector('#chat-cmd-panel:not([hidden])');
    assert.equal(await value(page), 'hello world /', 'the selection stays; "/" goes after it');
    await page.keyboard.press('Escape');
    assert.equal(await value(page), 'hello world');

    await page.click('#chat-add');
    await page.waitForSelector('#chat-cmd-panel:not([hidden])');
    await page.evaluate(() => { const b = document.createElement('button'); b.id = 'away'; document.body.appendChild(b); b.focus(); });
    await page.waitForTimeout(300);
    assert.equal(await shown(page), false);
    assert.equal(await page.evaluate(() => document.activeElement.id), 'away', 'closing on blur leaves focus where it went');
    assert.equal(await value(page), 'hello world /');
  } finally {
    await page.close();
  }
});

test('a pick from the + button keeps the space before it', async (t) => {
  if (launchError) return t.skip('WebKit unavailable: ' + launchError.message);
  const page = await open(SKILLS);
  try {
    await page.keyboard.type('abc');
    await page.click('#chat-add');
    await page.waitForSelector('#chat-cmd-panel:not([hidden])');
    await page.keyboard.type('ski');
    await settle(page);
    await page.keyboard.press('Enter');
    await page.keyboard.type('ne');
    await settle(page);
    await page.keyboard.press('Enter');
    assert.equal(await value(page), 'abc news ');
  } finally {
    await page.close();
  }
});

const MANY = Array.from({ length: 12 }, (_, i) => ({ name: 'skill-' + i, default: false, description: 'Does thing ' + i, bots: [{ id: 'm1' }] }));

test('a group\'s head stays above its list through a full round of keys', async (t) => {
  if (launchError) return t.skip('WebKit unavailable: ' + launchError.message);
  const page = await open(MANY);
  try {
    await page.keyboard.type('/');
    await page.waitForSelector('#chat-cmd-panel:not([hidden])');
    await page.keyboard.press('ArrowUp');
    await page.keyboard.press('Enter');
    await settle(page);
    const headIn = () => page.evaluate(() => {
      const h = document.querySelector('#chat-cmd-panel .ac-head'), p = document.querySelector('#chat-cmd-panel');
      const a = h.getBoundingClientRect(), b = p.getBoundingClientRect();
      return !h.hidden && a.height > 0 && a.top >= b.top && a.bottom <= b.bottom && h.textContent;
    });
    assert.equal(await headIn(), 'Skills');
    for (let i = 0; i < 13; i++) await page.keyboard.press('ArrowDown');
    await page.keyboard.press('ArrowUp');
    await page.keyboard.press('ArrowDown');
    assert.deepEqual(await selected(page), ['skill-1']);
    assert.equal(await headIn(), 'Skills');
    await page.click('#chat-cmd-panel .ac-crumb');
    await settle(page);
    assert.deepEqual(await rows(page), TOP);
  } finally {
    await page.close();
  }
});

test('the pointer and the keys share one highlight', async (t) => {
  if (launchError) return t.skip('WebKit unavailable: ' + launchError.message);
  const page = await open(MANY);
  try {
    await page.keyboard.type('/skills');
    await page.waitForSelector('#chat-cmd-panel:not([hidden])');
    await page.keyboard.press('Enter');
    await page.keyboard.type('skill-');
    await settle(page);
    const box = await page.locator('#chat-cmd-list li[role="option"]').nth(2).boundingBox();
    await page.mouse.move(box.x + 20, box.y + box.height / 2);
    assert.deepEqual(await selected(page), ['skill-2']);
    await page.keyboard.press('ArrowDown');
    assert.deepEqual(await selected(page), ['skill-3'], 'keys move off the row under a still pointer');
    const bg = await page.$$eval('#chat-cmd-list li[role="option"]', (lis) => lis.map((li) => getComputedStyle(li).backgroundColor));
    assert.equal(new Set(bg).size, 2, 'only the active row is tinted');
    await page.mouse.down();
    await page.mouse.up();
    assert.equal(await value(page), 'skill-2 ', 'a click picks the row under the pointer');
  } finally {
    await page.close();
  }
});

test('"@" opens Notes on the open tabs and searches as you type', async (t) => {
  if (launchError) return t.skip('WebKit unavailable: ' + launchError.message);
  const page = await open(SKILLS);
  try {
    await page.keyboard.type('see @');
    await page.waitForSelector('#chat-cmd-panel:not([hidden])');
    assert.equal(await head(page), 'Notes');
    assert.deepEqual(await rows(page), ['Today', 'Plan'], 'active tab first');
    assert.equal(await page.locator('#chat-cmd-list .ac-sub').count(), 0, 'single-line rows');
    await page.keyboard.type('设');
    await page.waitForTimeout(400);
    assert.deepEqual(await rows(page), ['Hylo 设计']);
    await page.keyboard.press('Enter');
    assert.equal(await value(page), 'see [[Hylo 设计]] ');

    await page.keyboard.type('@');
    await page.waitForSelector('#chat-cmd-panel:not([hidden])');
    await page.keyboard.press('Backspace');
    assert.equal(await value(page), 'see [[Hylo 设计]] ', 'Backspace deletes a bare "@"');
    assert.equal(await shown(page), false);

    await page.keyboard.type('@');
    await page.waitForSelector('#chat-cmd-panel:not([hidden])');
    await page.keyboard.press('ArrowLeft');
    await settle(page);
    assert.equal(await head(page), null, '← steps out to the top level');
    assert.deepEqual(await rows(page), TOP);
  } finally {
    await page.close();
  }
});

test('Folders steps in with → and Tab, back with ← and the crumbs, and attaches a level', async (t) => {
  if (launchError) return t.skip('WebKit unavailable: ' + launchError.message);
  const page = await open(SKILLS);
  try {
    await page.keyboard.type('/fol');
    await page.waitForSelector('#chat-cmd-panel:not([hidden])');
    await page.keyboard.press('Enter');
    await settle(page);
    assert.equal(await head(page), 'Folders');
    assert.deepEqual(await rows(page), ['journal', 'work']);
    assert.equal(await value(page), '/', 'entering a group clears the query');

    await page.keyboard.press('ArrowRight');
    await settle(page);
    assert.equal(await head(page), 'Folders/journal');
    assert.deepEqual(await rows(page), ['Attach /journal/', '2026']);
    await page.keyboard.press('ArrowDown');
    await page.keyboard.press('Tab');
    await settle(page);
    assert.equal(await head(page), 'Folders/journal/2026');
    assert.match(await page.textContent('#chat-cmd-list'), /Attach \/journal\/2026\//);

    await page.click('#chat-cmd-panel .ac-crumb[data-crumb="1"]');
    await settle(page);
    assert.equal(await head(page), 'Folders/journal');
    await page.keyboard.press('Backspace');
    await settle(page);
    assert.equal(await head(page), 'Folders');

    await page.keyboard.type('wo');
    await settle(page);
    assert.deepEqual(await rows(page), ['work']);
    await page.keyboard.press('Enter');
    assert.equal(await value(page), '/work/ ');
  } finally {
    await page.close();
  }
});

test('pinyin being composed holds the palette where it is', async (t) => {
  if (launchError) return t.skip('WebKit unavailable: ' + launchError.message);
  const page = await open(SKILLS);
  try {
    await page.keyboard.type('/');
    await page.waitForSelector('#chat-cmd-panel:not([hidden])');
    await page.keyboard.press('ArrowDown');
    await page.keyboard.press('Enter');
    await settle(page);
    assert.equal(await head(page), 'Notes');
    // What WebKit does with marked text: the whole of it selected, not yet the query.
    await page.evaluate(() => {
      const t = document.querySelector('#chat-textarea');
      t.dispatchEvent(new CompositionEvent('compositionstart'));
      t.value = "/she'ji";
      t.setSelectionRange(1, 7);
      t.dispatchEvent(new InputEvent('input', { bubbles: true, isComposing: true, inputType: 'insertCompositionText' }));
    });
    await settle(page);
    assert.equal(await head(page), 'Notes');
    await page.evaluate(() => {
      const t = document.querySelector('#chat-textarea');
      t.value = '/设';
      t.setSelectionRange(2, 2);
      t.dispatchEvent(new CompositionEvent('compositionend'));
      t.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertFromComposition' }));
    });
    await page.waitForTimeout(400);
    assert.equal(await head(page), 'Notes');
    assert.deepEqual(await rows(page), ['Hylo 设计']);
  } finally {
    await page.close();
  }
});
