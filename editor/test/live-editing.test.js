// Headless checks for the editing commands and decorators: a fake view is
// enough since none of these touch the DOM.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { EditorState, EditorSelection } from '@codemirror/state';
import { syntaxTree, ensureSyntaxTree } from '@codemirror/language';
import { wikiMarkdownLanguage } from '../src/cm-live/index.js';
import { nodeDecorators, isListGapLine } from '../src/cm-live/decorators.js';
import { splitWikiLinkInner } from '../src/cm-live/wiki-syntax.js';
import { closeTypedFence, fenceEnter } from '../src/cm-live/fence-close.js';
import { frontmatterReadOnly } from '../src/cm-live/frontmatter-readonly.js';
import { skipListGaps } from '../src/cm-live/live-preview.js';
import { linkTarget } from '../src/cm-live/link-refs.js';
import { wordRangeAt } from '../src/cm-live/cjk-words.js';
import { toggleBold, toggleInlineCode, insertLink } from '../src/cm-live/inline-format.js';
import { nextCell, previousCell, tableEnter } from '../src/cm-live/table-keys.js';
import { applyTarget } from '../src/cm-live/wikilink-complete.js';
import { revealedWidgetField, revealWidgetBeforeCaret } from '../src/cm-live/widget-reveal.js';
import { jumpToHeading } from '../src/cm-live/link-click.js';
import { fenceBody } from '../src/cm-live/fence-card/registry.js';
import { Decoration } from '@codemirror/view';

const lang = wikiMarkdownLanguage();

function viewFor(doc, anchor, head = anchor) {
  const parse = (st) => (ensureSyntaxTree(st, st.doc.length, 5000), st);
  const view = {
    state: parse(EditorState.create({ doc, selection: EditorSelection.range(anchor, head), extensions: [lang, revealedWidgetField] })),
    plugin: () => ({ decorations: Decoration.set(decorate(view), true) }),
    focus() {},
    get visibleRanges() { return [{ from: 0, to: view.state.doc.length }]; },
    dispatch(...specs) { view.state = parse(view.state.update(...specs).state); },
  };
  return view;
}

const text = (view) => view.state.doc.toString();
const sel = (view) => [view.state.selection.main.anchor, view.state.selection.main.head];

function decorate(view) {
  const out = [];
  syntaxTree(view.state).iterate({
    enter(n) {
      const d = nodeDecorators[n.name];
      if (d) d(n, view, out, {}, new Map());
    },
  });
  return out;
}

function decoClasses(view) {
  return decorate(view).map((r) => r.value.spec.class || (r.value.spec.widget ? r.value.spec.widget.constructor.name : 'hidden'));
}

test('setext heading renders as a heading with its underline collapsed', () => {
  const classes = decoClasses(viewFor('Title\n===\n\nx', 12));
  assert.ok(classes.includes('cm-lp-heading cm-lp-h1'));
  assert.ok(classes.includes('cm-lp-hidden-line'));
});

test('setext underline under the caret stays raw', () => {
  assert.deepEqual(decoClasses(viewFor('Title\n-\n\nx', 7)), []);
});

test('wikilink renders as a widget until the caret steps inside', () => {
  assert.ok(decoClasses(viewFor('a [[note]] b', 10)).includes('WikiLinkWidget'));
  assert.ok(decoClasses(viewFor('a [[note]] b', 9)).includes('cm-lp-wikilink-raw'));
});

test('bold: empty toggle inserts a pair, second press removes it', () => {
  const view = viewFor('hello', 2);
  toggleBold(view);
  assert.equal(text(view), 'he****llo');
  toggleBold(view);
  assert.equal(text(view), 'hello');
  assert.deepEqual(sel(view), [2, 2]);
});

test('bold: caret inside a bold span unbolds it', () => {
  const view = viewFor('a **bold** b', 6);
  toggleBold(view);
  assert.equal(text(view), 'a bold b');
  assert.deepEqual(sel(view), [4, 4]);
});

test('inline code toggles off from inside the span', () => {
  const view = viewFor('a `code` b', 4);
  toggleInlineCode(view);
  assert.equal(text(view), 'a code b');
});

test('link: selected text becomes the label, a selected URL the target', () => {
  const label = viewFor('see docs here', 4, 8);
  insertLink(label);
  assert.equal(text(label), 'see [docs]() here');
  assert.deepEqual(sel(label), [11, 11]);
  const url = viewFor('go https://x.com now', 3, 16);
  insertLink(url);
  assert.equal(text(url), 'go [](https://x.com) now');
  assert.deepEqual(sel(url), [4, 4]);
});

const TABLE = '| a | b |\n|---|---|\n| 1 | 2 |';

test('Tab walks cells across rows, skipping the delimiter row', () => {
  const view = viewFor(TABLE, 2);
  nextCell(view);
  assert.deepEqual(sel(view), [6, 7]);
  nextCell(view);
  assert.deepEqual(sel(view), [22, 23]);
  previousCell(view);
  assert.deepEqual(sel(view), [6, 7]);
});

test('Tab past the last cell appends a row', () => {
  const view = viewFor(TABLE, TABLE.length - 2);
  nextCell(view);
  assert.equal(text(view), TABLE + '\n|  |  |');
  assert.deepEqual(sel(view), [32, 32]);
});

test('Enter adds a row; Enter on an empty row leaves the table', () => {
  const view = viewFor(TABLE, TABLE.length - 2);
  tableEnter(view);
  assert.equal(text(view), TABLE + '\n|  |  |');
  tableEnter(view);
  assert.equal(text(view), TABLE + '\n');
});

test('Enter keeps a quoted table quoted', () => {
  const doc = '> | a | b |\n> |---|---|\n> | 1 | 2 |';
  const view = viewFor(doc, doc.length - 2);
  tableEnter(view);
  assert.equal(text(view), doc + '\n> |  |  |');
});

test('wikilink completion closes the link or replaces an existing target', () => {
  const fresh = viewFor('see [[no', 8);
  applyTarget('note')(fresh, null, 6, 8);
  assert.equal(text(fresh), 'see [[note]]');
  assert.deepEqual(sel(fresh), [12, 12]);

  const existing = viewFor('see [[no|alias]]', 8);
  applyTarget('note')(existing, null, 6, 8);
  assert.equal(text(existing), 'see [[note|alias]]');
});

function lineStyles(view) {
  const out = [];
  syntaxTree(view.state).iterate({
    enter(n) {
      const d = nodeDecorators[n.name];
      if (d) d(n, view, out, {}, new Map());
    },
  });
  return out
    .filter((r) => r.value.spec.attributes && r.value.spec.attributes.style)
    .map((r) => [view.state.doc.lineAt(r.from).number, r.value.spec.attributes.style]);
}

test('nested list text lines up with the parent item text', () => {
  for (const doc of ['- a\n  - b\n  - c\n', '1. a\n   1. b\n   2. c\n']) {
    const styles = lineStyles(viewFor(doc, 0));
    const indents = [2, 3].map((n) =>
      styles.filter(([line]) => line === n).flatMap(([, s]) => s.split(';').filter((d) => /^(padding-left|text-indent|margin-left)/.test(d)))
    );
    // The first nested item used to also pick up the parent's continuation margin.
    assert.deepEqual(indents[0], indents[1]);
    assert.ok(indents[0].every((s) => !s.includes('margin-left')));
  }
});

test('ordered lists widen their marker column with the digit count', () => {
  const doc = Array.from({ length: 10 }, (_, i) => `${i + 1}. x`).join('\n');
  const styles = lineStyles(viewFor(doc, 0));
  assert.match(styles[0][1], /padding-left:2\.05em;text-indent:-2\.05em/);
});

test('continuation lines of an item take the item text indent', () => {
  const styles = lineStyles(viewFor('- item\n  more\n\n  second para\n', 0));
  assert.deepEqual(styles.filter(([, s]) => s.startsWith('margin-left')).map(([n]) => n), [2, 4]);
});

test('task checkbox replaces "[ ] " including its space, marker hidden', () => {
  const classes = decoClasses(viewFor('- [ ] todo', 10));
  assert.ok(classes.includes('TaskCheckboxWidget'));
  assert.ok(!classes.includes('ListMarkerWidget'));
});

test('stepping into a list marker shows the raw marker', () => {
  assert.ok(decoClasses(viewFor('- item', 1)).includes('cm-lp-list-mark-raw'));
  assert.ok(decoClasses(viewFor('- item', 2)).includes('ListMarkerWidget'));
});

test('CJK runs inside emphasis get emphasis dots instead of italic', () => {
  const classes = decoClasses(viewFor('a *重点 word* b', 0));
  assert.equal(classes.filter((c) => c === 'cm-lp-em-cjk').length, 1);
});

test('code block at rest shows a header widget; editing shows the raw fence', () => {
  const doc = 'x\n\n```js\nlet a\n```\n';
  assert.ok(decoClasses(viewFor(doc, 0)).includes('CodeHeaderWidget'));
  const editing = decoClasses(viewFor(doc, 10));
  assert.ok(!editing.includes('CodeHeaderWidget'));
  assert.equal(editing.filter((c) => c.includes('cm-lp-codeblock-fence')).length, 2);
});

test('typing the third backtick writes a closing fence', () => {
  const view = viewFor('``', 2);
  assert.equal(closeTypedFence(view, 2, 2, '`'), true);
  assert.equal(text(view), '```\n```');
  assert.deepEqual(sel(view), [3, 3]);
});

test('third backtick inside an open code block is just typed', () => {
  const view = viewFor('```\ncode\n``', 11);
  assert.equal(closeTypedFence(view, 11, 11, '`'), false);
});

test('Enter after an unclosed opening fence closes it', () => {
  const view = viewFor('> ```js', 7);
  assert.equal(fenceEnter(view), true);
  assert.equal(text(view), '> ```js\n> \n> ```');
  assert.deepEqual(sel(view), [10, 10]);
});

test('wikilink heading and alias split', () => {
  assert.deepEqual(splitWikiLinkInner('note#Heading|alias'), { target: 'note', heading: 'Heading', alias: 'alias' });
  assert.deepEqual(splitWikiLinkInner('#Only'), { target: '', heading: 'Only', alias: null });
});

test('a caret moved into frontmatter is pushed to the body start', () => {
  const doc = '---\ntitle: x\n---\nbody';
  let state = EditorState.create({ doc, extensions: [lang, frontmatterReadOnly()] });
  ensureSyntaxTree(state, state.doc.length, 5000);
  state = state.update({ selection: { anchor: 6 } }).state;
  assert.equal(state.selection.main.head, doc.indexOf('body'));
});

test('arrowing onto a collapsed list gap moves past it', () => {
  const doc = '- a\n\n- b';
  assert.equal(isListGapLine(viewFor(doc, 0).state, 2), true);
  let state = EditorState.create({ doc, selection: { anchor: 3 }, extensions: [lang, skipListGaps] });
  ensureSyntaxTree(state, state.doc.length, 5000);
  state = state.update({ selection: { anchor: 4 } }).state;
  assert.equal(state.selection.main.head, 8);
});

// CM samples line height from the first short plain-text line; a styled line
// passing that test skews the height map and arrow keys jump dozens of lines.
test('heading, quote and code lines are kept out of the text-size sample', () => {
  const doc = '# h111\n\n> quote\n\n```\naaaa\n```\n\nbody';
  const view = viewFor(doc, doc.length);
  const covered = new Set();
  syntaxTree(view.state).iterate({
    enter(n) {
      const d = nodeDecorators[n.name];
      if (!d) return;
      const out = [];
      d(n, view, out, {}, new Map());
      for (const r of out) if (r.value.spec.class === 'cm-lp-styled-line') covered.add(view.state.doc.lineAt(r.from).number);
    },
  });
  assert.ok([1, 3, 6].every((n) => covered.has(n)));
  assert.ok(!covered.has(9));
});

// ── P2 ──────────────────────────────────────────────────────────────────

function nodeAt(view, name, pos) {
  let found = null;
  syntaxTree(view.state).iterate({ from: pos, to: pos, enter(n) { if (n.name === name) found = n.node; } });
  return found;
}

test('reference links resolve full, collapsed and shortcut forms', () => {
  const doc = '[a][r] [b][] [c] [d][nope]\n\n[r]: https://r.example\n[b]: https://b.example\n[C]: https://c.example\n';
  const view = viewFor(doc, doc.length);
  const urls = [0, 7, 13, 17].map((pos) => {
    const t = linkTarget(view.state, nodeAt(view, 'Link', pos + 1));
    return t && t.url;
  });
  assert.deepEqual(urls, ['https://r.example', 'https://b.example', 'https://c.example', null]);
});

test('link text carries its URL as a tooltip until the caret reaches it', () => {
  const doc = 'x [docs](https://d.example) y';
  const titleOf = (view) => {
    let title = null;
    syntaxTree(view.state).iterate({
      enter(n) {
        const d = nodeDecorators[n.name];
        if (!d) return;
        const out = [];
        d(n, view, out, {}, new Map());
        for (const r of out) if (r.value.spec.attributes && r.value.spec.attributes.title) title = r.value.spec.attributes.title;
      },
    });
    return title;
  };
  assert.equal(titleOf(viewFor(doc, 0)), 'https://d.example');
  assert.equal(titleOf(viewFor(doc, 5)), null);
});

test('escape backslash is hidden until the caret touches it', () => {
  const hidden = (pos) => decoClasses(viewFor('a \\*b\\* c', pos)).filter((c) => c === 'hidden').length;
  assert.equal(hidden(0), 2);
  // Caret on the first escape reveals only that backslash.
  assert.equal(hidden(3), 1);
});

test('indented code renders as a code block with its indent hidden', () => {
  const classes = decoClasses(viewFor('para\n\n    code\n    more\n', 0));
  assert.equal(classes.filter((c) => c.includes('cm-lp-codeblock-indented')).length, 2);
  assert.equal(classes.filter((c) => c === 'hidden').length, 2);
});

test('CJK word selection uses segmented words, not the whole run', () => {
  const state = EditorState.create({ doc: '右键会用自动选词，可能一下' });
  const w = wordRangeAt(state, 5); // inside 自动
  assert.equal(state.sliceDoc(w.from, w.to), '自动');
  const latin = EditorState.create({ doc: 'hello world' });
  const l = wordRangeAt(latin, 7);
  assert.equal(latin.sliceDoc(l.from, l.to), 'world');
});

test('bold: a caret on either outer edge of a span unbolds it instead of fusing marks', () => {
  const before = viewFor('see **bold** x', 4);
  toggleBold(before);
  assert.equal(text(before), 'see bold x');
  const after = viewFor('see **bold** x', 12);
  toggleBold(after);
  assert.equal(text(after), 'see bold x');
  assert.deepEqual(sel(after), [8, 8]);
});

test('a blank line between nested items collapses and is skipped alike', () => {
  const view = viewFor('- top\n  - a\n\n  - b\n- next', 0);
  assert.equal(isListGapLine(view.state, 3), true);
  assert.ok(decoClasses(view).includes('cm-lp-list-gap'));
});

test('Backspace after a wikilink widget reveals the source first', () => {
  const view = viewFor('see [[note]]', 12);
  assert.equal(revealWidgetBeforeCaret(view), true);
  assert.equal(text(view), 'see [[note]]');
  assert.ok(decoClasses(view).includes('cm-lp-wikilink-raw'));
  assert.equal(revealWidgetBeforeCaret(view), false);
  view.dispatch({ selection: { anchor: 0 } });
  assert.ok(decoClasses(view).includes('WikiLinkWidget'));
});

test('Backspace after a markdown image reveals it; plain text is left to the default', () => {
  const view = viewFor('![a](u.png)', 11);
  assert.equal(revealWidgetBeforeCaret(view), true);
  assert.equal(revealWidgetBeforeCaret(viewFor('plain', 5)), false);
});

test('wikilink completion keeps a #heading after the target', () => {
  const view = viewFor('[[foo#Intro]]', 4);
  applyTarget('foobar')(view, null, 2, 4);
  assert.equal(text(view), '[[foobar#Intro]]');
  assert.deepEqual(sel(view), [9, 9]);
});

test('heading jump finds setext headings', () => {
  const view = viewFor('x\n\nIntro\n=====\n\ny', 0);
  assert.equal(jumpToHeading(view, 'Intro'), true);
  assert.deepEqual(sel(view), [3, 3]);
});

test('code copy text strips the item indent of a fence opened on a list line', () => {
  const view = viewFor('- ```js\n  const a = 1\n  ```', 0);
  let fence = null;
  syntaxTree(view.state).iterate({ enter: (n) => { if (n.name === 'FencedCode') fence = n.node; } });
  assert.equal(fenceBody(view.state.doc, fence), 'const a = 1');
});
