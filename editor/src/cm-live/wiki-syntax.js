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

// "note#Heading|alias" → { target: 'note', heading: 'Heading', alias: 'alias' }.
// An empty target ("[[#Heading]]") points into the current note.
export function splitWikiLinkInner(inner) {
  const bar = inner.indexOf('|');
  const ref = bar < 0 ? inner : inner.slice(0, bar);
  const alias = bar < 0 ? null : inner.slice(bar + 1);
  const hash = ref.indexOf('#');
  if (hash < 0) return { target: ref, heading: null, alias };
  return { target: ref.slice(0, hash).trim(), heading: ref.slice(hash + 1).trim() || null, alias };
}
