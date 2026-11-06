const test = require('node:test');
const assert = require('node:assert');

require('./chat_slash.js');
const parse = global.__hyloSlashParse;
const search = global.__hyloSlashSearch;

const at = (s) => parse(s, s.length);

test('"/" opens at a word start, after CJK, and never on paths or links', () => {
  assert.deepEqual(at('/'), { start: 0, query: '', trigger: '/' });
  assert.deepEqual(at('use /ne'), { start: 4, query: 'ne', trigger: '/' });
  assert.deepEqual(at('帮我用/ne'), { start: 3, query: 'ne', trigger: '/' });
  assert.deepEqual(at('/图片'), { start: 0, query: '图片', trigger: '/' });
  assert.equal(at('/journal/'), null);
  assert.equal(at('/@'), null);
  assert.equal(at('https://x'), null);
  assert.equal(at('1/2'), null);
  assert.equal(at('/news '), null);
});

test('"、" opens only at a word start, so Chinese lists stay quiet', () => {
  assert.deepEqual(at('、'), { start: 0, query: '', trigger: '/' });
  assert.deepEqual(at('帮我 、ne'), { start: 3, query: 'ne', trigger: '/' });
  assert.equal(at('苹果、'), null);
  assert.equal(at('苹果、香蕉'), null);
});

test('"@" opens on a note name, never inside a word or an email', () => {
  assert.deepEqual(at('@'), { start: 0, query: '', trigger: '@' });
  assert.deepEqual(at('看 @Hylo.设计'), { start: 2, query: 'Hylo.设计', trigger: '@' });
  assert.deepEqual(at('看看@设'), { start: 2, query: '设', trigger: '@' });
  assert.equal(at('a@b.com'), null);
  assert.equal(at('@a/b'), null);
  assert.equal(at('@a b'), null);
});

test('search ranks name prefix, name, alias, description, and stays on its level', () => {
  const skills = { label: 'Skills', group: true, children: [{ label: 'news', sub: 'Read the headlines' }] };
  const cmds = [
    { label: 'Image', aliases: ['图片'], sub: 'Attach images from disk' },
    { label: 'Notes', sub: 'Attach a note' },
    skills,
  ];
  const names = (q) => search(cmds, q).map((c) => c.label);
  assert.deepEqual(names('i'), ['Image', 'Skills']);
  assert.deepEqual(names('note'), ['Notes']);
  assert.deepEqual(names('图'), ['Image']);
  assert.deepEqual(names('news'), []);
  assert.deepEqual(search(skills.children, 'ne').map((c) => c.label), ['news']);
});
