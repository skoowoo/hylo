// Headless checks for the editing commands and decorators: a fake view is
// enough since none of these touch the DOM.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { EditorState, EditorSelection } from '@codemirror/state';
import { syntaxTree, ensureSyntaxTree } from '@codemirror/language';
import { wikiMarkdownLanguage } from '../src/cm-live/index.js';
import { nodeDecorators, isListGapLine, isHairlineLine } from '../src/cm-live/decorators.js';
import { splitWikiLinkInner } from '../src/cm-live/wiki-syntax.js';
import { closeTypedFence, fenceEnter } from '../src/cm-live/fence-close.js';
import { findFrontmatterNode } from '../src/cm-live/frontmatter-syntax.js';
import { frontmatterCardField } from '../src/cm-live/frontmatter-card.js';
import { skipHairlines } from '../src/cm-live/live-preview.js';
import { linkTarget } from '../src/cm-live/link-refs.js';
import { wordRangeAt } from '../src/cm-live/cjk-words.js';
import { toggleBold, toggleInlineCode, insertLink } from '../src/cm-live/inline-format.js';
import { nextCell, previousCell, tableEnter } from '../src/cm-live/table-keys.js';
import { applyTarget } from '../src/cm-live/wikilink-complete.js';
import { revealedWidgetField, revealWidgetBeforeCaret, revealBlockBeforeCaret } from '../src/cm-live/widget-reveal.js';
import { imageNodeAt, imageSpan } from '../src/cm-live/image-nodes.js';
import { jumpToHeading } from '../src/cm-live/link-click.js';
import { fenceBody } from '../src/cm-live/fence-card/registry.js';
import { Decoration, EditorView } from '@codemirror/view';
import { imageField } from '../src/cm-live/image-field.js';

const lang = wikiMarkdownLanguage();

function viewFor(doc, anchor, head = anchor) {
  const parse = (st) => (ensureSyntaxTree(st, st.doc.length, 5000), st);
  const view = {
    state: parse(EditorState.create({ doc, selection: EditorSelection.range(anchor, head), extensions: [lang, revealedWidgetField, imageField()] })),
    plugin: () => ({ decorations: Decoration.set(decorate(view), true) }),
    focus() {},
    get visibleRanges() { return [{ from: 0, to: view.state.doc.length }]; },
    dispatch(...specs) { view.state = parse(view.state.update(...specs).state); },
  };
  return view;
}

const text = (view) => view.state.doc.toString();
const sel = (view) => [view.state.selection.main.anchor, view.state.selection.main.head];

// The live-preview plugin's decorations plus the state fields' (images).
function decorate(view) {
  const out = [];
  syntaxTree(view.state).iterate({
    enter(n) {
      const d = nodeDecorators[n.name];
      if (d) d(n, view, out, {}, new Map());
    },
  });
  for (const set of view.state.facet(EditorView.decorations)) {
    if (typeof set === 'function') continue;
    for (const c = set.iter(); c.value; c.next()) out.push(c.value.range(c.from, c.to));
  }
  return out;
}

const isImage = (r) => r.value.spec.widget && r.value.spec.widget.constructor.name === 'ImageWidget';

function decoClasses(view) {
  return decorate(view).map((r) => r.value.spec.class || (r.value.spec.widget ? r.value.spec.widget.constructor.name : 'hidden'));
}

test('setext heading renders as a heading with its underline collapsed', () => {
  const classes = decoClasses(viewFor('Title\n===\n\nx', 12));
  assert.ok(classes.includes('cm-lp-heading cm-lp-h1'));
  assert.ok(classes.includes('cm-lp-hidden-line'));
});

test('bare block markers stay raw until the space is typed', () => {
  for (const doc of ['#', '###', '-', '*', '1.', '> #', '> -']) {
    const block = decoClasses(viewFor(doc, doc.length)).filter((c) => /heading|list|ListMarker/.test(c));
    assert.deepEqual(block, [], doc);
  }
  assert.ok(decoClasses(viewFor('# ', 2)).includes('cm-lp-heading cm-lp-h1'));
  assert.ok(decoClasses(viewFor('- ', 2)).includes('ListMarkerWidget'));
});

test('revealed heading/quote markers hang as a widget until the caret steps in', () => {
  assert.ok(decoClasses(viewFor('## a', 3)).includes('HangMarkWidget'));
  assert.ok(decoClasses(viewFor('## ', 3)).includes('HangMarkWidget'));
  assert.ok(decoClasses(viewFor('> a', 2)).includes('HangMarkWidget'));
  assert.ok(!decoClasses(viewFor('> - a', 5)).includes('HangMarkWidget'));
  const inside = decoClasses(viewFor('## a', 2));
  assert.ok(inside.includes('cm-lp-heading-mark cm-lp-hang'));
  assert.ok(!inside.includes('HangMarkWidget'));
  assert.ok(decoClasses(viewFor('## a', 0)).includes('cm-lp-heading-mark cm-lp-hang'));
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

test('frontmatter needs its closing ---, so a lone opener stays a rule', () => {
  const fm = (doc) => {
    const state = EditorState.create({ doc, extensions: [lang] });
    ensureSyntaxTree(state, doc.length, 5000);
    const node = findFrontmatterNode(state);
    return node && [node.from, node.to];
  };
  assert.deepEqual(fm('---\ntitle: x\n---\nbody'), [0, 16]);
  assert.deepEqual(fm('---\ntitle: x\n---'), [0, 16]);
  assert.deepEqual(fm('---\n---\nbody'), [0, 7]);
  assert.equal(fm('---\ntitle: x\nbody'), null);
  assert.equal(fm('---\ntitle: x\n----\nbody'), null);
});

function fmState(doc, anchor) {
  const state = EditorState.create({ doc, selection: { anchor }, extensions: [lang, frontmatterCardField()] });
  ensureSyntaxTree(state, doc.length, 5000);
  return state;
}

test('a caret moved under the frontmatter card lands at the body start, or the card edge with no body', () => {
  assert.equal(fmState('---\nt: x\n---\nbody', 17).update({ selection: { anchor: 6 } }).state.selection.main.head, 13);
  assert.equal(fmState('---\nt: x\n---\nbody', 17).update({ selection: { anchor: 12 } }).state.selection.main.head, 13);
  assert.equal(fmState('---\nt: x\n---', 12).update({ selection: { anchor: 0 } }).state.selection.main.head, 12);
  // A selection may still span it.
  assert.equal(fmState('---\nt: x\n---\nbody', 17).update({ selection: { anchor: 17, head: 0 } }).state.selection.main.head, 0);
});

test('typing at the card edge starts a new line; deleting into the card does nothing', () => {
  const doc = '---\nt: x\n---';
  const typed = fmState(doc, 12).update({ changes: { from: 12, insert: 'a' }, selection: { anchor: 13 }, userEvent: 'input.type' }).state;
  assert.equal(typed.doc.toString(), doc + '\na');
  assert.equal(typed.selection.main.head, 14);
  const deleted = fmState(doc, 12).update({ changes: { from: 11, to: 12 }, userEvent: 'delete.backward' }).state;
  assert.equal(deleted.doc.toString(), doc);
  // Programmatic and history changes pass through.
  assert.equal(fmState(doc, 12).update({ changes: { from: 11, to: 12 } }).state.doc.toString(), '---\nt: x\n--');
});

test('a state built with the caret inside the frontmatter opens its source', () => {
  const doc = '---\nt: x\n---\nbody';
  assert.equal(fmState(doc, 6).update({ selection: { anchor: 7 } }).state.selection.main.head, 7);
  assert.equal(fmState(doc, 17).update({ selection: { anchor: 7 } }).state.selection.main.head, 13);
  assert.equal(fmState('---\nt: x\n---', 12).update({ selection: { anchor: 7 } }).state.selection.main.head, 12);
});

test('a range selection leaves inline syntax and rows rendered; a caret reveals them', () => {
  const doc = 'a **b** [l](u) c\n\n| x |\n|---|\n| 1 |\n\n---\n\nend';
  const hidden = (view) => decorate(view).filter((r) => !r.value.spec.widget && !r.value.spec.class && !r.value.spec.attributes).length;
  const all = viewFor(doc, 0, doc.length);
  const caret = viewFor(doc, 4);
  assert.ok(hidden(all) > hidden(caret));
  assert.ok(!decoClasses(all).some((c) => /table-raw/.test(c)));
  assert.ok(decoClasses(all).includes('HorizontalRuleWidget'));
  assert.ok(decoClasses(viewFor(doc, doc.indexOf('| 1'))).some((c) => /table-raw/.test(c)));
  assert.ok(!decoClasses(viewFor(doc, doc.indexOf('---\n\nend'))).includes('HorizontalRuleWidget'));
});

test('arrowing onto a collapsed list gap moves past it', () => {
  const doc = '- a\n\n- b';
  assert.equal(isListGapLine(viewFor(doc, 0).state, 2), true);
  let state = EditorState.create({ doc, selection: { anchor: 3 }, extensions: [lang, skipHairlines] });
  ensureSyntaxTree(state, state.doc.length, 5000);
  state = state.update({ selection: { anchor: 4 } }).state;
  assert.equal(state.selection.main.head, 5);
});

test('arrowing across a table delimiter row or a setext underline steps over it', () => {
  const step = (doc, from, to) => {
    let state = EditorState.create({ doc, selection: { anchor: from }, extensions: [lang, skipHairlines] });
    ensureSyntaxTree(state, state.doc.length, 5000);
    return state.update({ selection: { anchor: to } }).state.selection.main.head;
  };
  const table = '| a | b |\n|---|---|\n| 1 | 2 |';
  assert.equal(step(table, 9, 10), 20);
  assert.equal(step(table, 20, 19), 9);
  assert.equal(isHairlineLine(viewFor(table, 0).state, 2), true);
  assert.equal(isHairlineLine(viewFor(table, 0).state, 3), false);
  const setext = 'Title\n---\n\nnext';
  assert.equal(step(setext, 5, 6), 10);
  assert.equal(isHairlineLine(viewFor(setext, 0).state, 2), true);
  assert.equal(isHairlineLine(viewFor('- a\n---', 0).state, 2), false);
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

test('Backspace below a rule steps onto it instead of joining the line to it', () => {
  const doc = 'a\n\n---\nText';
  const view = viewFor(doc, doc.indexOf('Text'));
  view.lineBlockAt = (pos) => ({ type: 'text', from: view.state.doc.lineAt(pos).from, to: view.state.doc.lineAt(pos).to });
  assert.equal(revealBlockBeforeCaret(view), true);
  assert.equal(text(view), doc);
  assert.equal(view.state.selection.main.head, doc.indexOf('---') + 3);
  const plain = viewFor('a\nText', 2);
  plain.lineBlockAt = view.lineBlockAt;
  assert.equal(revealBlockBeforeCaret(plain), false);
});

test('Backspace after an image is left to delete the picture; plain text stays default', () => {
  const doc = '![a](u.png)';
  const view = viewFor(doc, doc.length);
  assert.equal(revealWidgetBeforeCaret(view), false);
  assert.ok(decoClasses(view).includes('ImageWidget'));
  assert.equal(revealWidgetBeforeCaret(viewFor('plain', 5)), false);
});

test('a selection covering images keeps them rendered and atomic', () => {
  const doc = '![a](one.png)\n\n![b](two.png)\n';
  const view = viewFor(doc, 0, doc.length);
  const decos = decorate(view);
  const images = decos.filter(isImage);
  assert.equal(images.length, 2);
  assert.equal(images[0].value.spec.widget.selected, true);
  assert.equal(images[1].value.spec.widget.selected, true);
  assert.ok(decoClasses(viewFor(doc, 2)).includes('cm-lp-wikiimage-raw'));
});

test('imageNodeAt finds markdown and wiki images', () => {
  const doc = 'before\n\n![a](u.png)\n\n![[pic.png]]';
  const state = viewFor(doc, 0).state;
  const md = imageNodeAt(state, doc.indexOf('![a]') + 2);
  assert.equal(state.sliceDoc(md.from, md.to), '![a](u.png)');
  const wiki = imageNodeAt(state, doc.indexOf('![[') + 3);
  assert.equal(state.sliceDoc(wiki.from, wiki.to), '![[pic.png]]');
  assert.equal(imageNodeAt(state, doc.indexOf('before')), null);
});

test('a linked image stays rendered when selected and spans the whole link', () => {
  const doc = '[![Image](https://img.example/a.png)](https://mp.weixin.qq.com/s?x=1 "title")\n';
  const resting = viewFor(doc, 0);
  const decos = decorate(resting);
  Decoration.set(decos, true);
  const images = decos.filter(isImage);
  assert.equal(images.length, 1);
  assert.equal(resting.state.sliceDoc(images[0].from, images[0].to), doc.slice(0, -1));
  assert.equal(images[0].value.spec.widget.selected, false);
  const node = imageNodeAt(resting.state, 0);
  assert.equal(resting.state.sliceDoc(node.from, node.to), '![Image](https://img.example/a.png)');
  const block = imageSpan(node);
  assert.equal(resting.state.sliceDoc(block.from, block.to), doc.slice(0, -1));
  const selected = viewFor(doc, block.from, block.to);
  const selectedDecos = decorate(selected);
  Decoration.set(selectedDecos, true);
  const widget = selectedDecos.find(isImage);
  assert.equal(widget.value.spec.widget.selected, true);
  assert.ok(!decoClasses(selected).includes('cm-lp-wikiimage-raw'));
  const inside = viewFor(doc, node.from + 2);
  assert.ok(decoClasses(inside).includes('cm-lp-wikiimage-raw'));
  assert.ok(!decoClasses(inside).includes('ImageWidget'));
  const url = viewFor(doc, doc.indexOf('https://mp.weixin'));
  assert.ok(decoClasses(url).includes('cm-lp-wikiimage-raw'));
  assert.ok(!decoClasses(url).includes('ImageWidget'));
  assert.ok(decoClasses(url).includes('cm-lp-link cm-lp-link-editing'));
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
