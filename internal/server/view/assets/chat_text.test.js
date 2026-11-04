const test = require('node:test');
const assert = require('node:assert');

global.marked = require('../../static/vendor/marked.min.js');
require('./chat_text.js');

test('image embeds render as vault images, wikilinks as note links', () => {
  const html = global.__hyloRenderMarkdown('see ![[grug-2026-11-03.jpg]] and [[Hylo 设计]]', { cache: false });
  assert.match(html, /<img src="\/api\/images\/serve\?name=grug-2026-11-03\.jpg" alt="grug-2026-11-03\.jpg">/);
  assert.match(html, /<a href="\/notes\?name=Hylo%20%E8%AE%BE%E8%AE%A1\.md">Hylo 设计<\/a>/);
  assert.doesNotMatch(html, /\.jpg\.md/);
});
