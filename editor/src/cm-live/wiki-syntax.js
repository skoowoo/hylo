// Obsidian [[wikilink]] / ![[wikiimage]] → WikiLink / WikiImage syntax nodes.

const CH_BANG = 33;   // !
const CH_OPEN = 91;   // [
const CH_CLOSE = 93;  // ]

// before Link so "[[" isn't swallowed by reference-link parsing.
function parseWikiLink(cx, next, pos) {
  if (next !== CH_OPEN || cx.char(pos + 1) !== CH_OPEN) return -1;
  let end = pos + 2;
  while (end < cx.end - 1 && !(cx.char(end) === CH_CLOSE && cx.char(end + 1) === CH_CLOSE)) {
    end++;
  }
  if (end >= cx.end - 1 || cx.char(end) !== CH_CLOSE || cx.char(end + 1) !== CH_CLOSE) return -1;
  if (end === pos + 2) return -1; // [[]]
  return cx.addElement(cx.elt('WikiLink', pos, end + 2));
}

// before Image so "![[" isn't swallowed by image parsing.
function parseWikiImage(cx, next, pos) {
  if (next !== CH_BANG || cx.char(pos + 1) !== CH_OPEN || cx.char(pos + 2) !== CH_OPEN) return -1;
  let end = pos + 3;
  while (end < cx.end - 1 && !(cx.char(end) === CH_CLOSE && cx.char(end + 1) === CH_CLOSE)) {
    end++;
  }
  if (end >= cx.end - 1 || cx.char(end) !== CH_CLOSE || cx.char(end + 1) !== CH_CLOSE) return -1;
  if (end === pos + 3) return -1;
  return cx.addElement(cx.elt('WikiImage', pos, end + 2));
}

export const wikiSyntax = {
  defineNodes: ['WikiLink', 'WikiImage'],
  parseInline: [
    { name: 'WikiLink', before: 'Link', parse: parseWikiLink },
    { name: 'WikiImage', before: 'Image', parse: parseWikiImage },
  ],
};

export function wikiNodeInner(node, doc) {
  const isImage = node.name === 'WikiImage';
  const openLen = isImage ? 3 : 2;
  return doc.sliceString(node.from + openLen, node.to - 2);
}

export function splitWikiLinkInner(inner) {
  const bar = inner.indexOf('|');
  if (bar < 0) return { target: inner, alias: null };
  return { target: inner.slice(0, bar), alias: inner.slice(bar + 1) };
}
