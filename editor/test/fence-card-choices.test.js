import test from 'node:test';
import assert from 'node:assert/strict';
import { choiceLabel, parseChoices, pickedOptions } from '../src/cm-live/fence-card/choices.js';

test('plain items pick one, checkbox items pick several', () => {
  assert.deepEqual(parseChoices('- 中文\n- English\n'), { multi: false, options: ['中文', 'English'] });
  assert.deepEqual(parseChoices('- [ ] a\n- [x] b\n\nnot an item\n1. c'), { multi: true, options: ['a', 'b', 'c'] });
});

test('a reply picks the options it names', () => {
  const opts = ['daily-ai-news', 'hackernews', 'the-news'];
  assert.deepEqual([...pickedOptions(opts, 'hackernews')], ['hackernews']);
  assert.deepEqual([...pickedOptions(opts, 'daily-ai-news, the-news')], ['daily-ai-news', 'the-news']);
  assert.equal(pickedOptions(opts, 'something else entirely').size, 0);
});

test('an option containing the separator still matches whole', () => {
  assert.deepEqual([...pickedOptions(['Yes, compile them', 'Not now'], 'Yes, compile them')], ['Yes, compile them']);
});

test('a label shows inline markdown as plain text', () => {
  assert.equal(choiceLabel('[小而美](/notes?name=%E5%B0%8F.md)'), '小而美');
  assert.equal(choiceLabel('[[note|别名]] and [[other]]'), '别名 and other');
  assert.equal(choiceLabel('**粗** `code` *斜*'), '粗 code 斜');
  assert.equal(choiceLabel('2 * 3 = 6'), '2 * 3 = 6');
  assert.equal(choiceLabel('call __init__'), 'call __init__');
});
