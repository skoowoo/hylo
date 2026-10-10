// Where a Link node points, for all three markdown forms:
//   [text](url)   inline — URL child
//   [text][label] full reference, [text][] collapsed — LinkLabel child
//   [text]        shortcut — neither; only a link if the label is defined
import { syntaxTree } from '@codemirror/language';
// Definitions only start a block, so the walk can skip leaf blocks.
import { NEVER_CONTAINS_HR as LEAF_BLOCKS } from './horizontal-rule-field.js';

// CommonMark label matching: case-insensitive, inner whitespace collapsed.
function normalizeLabel(label) {
  return label.trim().replace(/\s+/g, ' ').toLowerCase();
}

function collect(state) {
  const refs = new Map();
  const doc = state.doc;
  syntaxTree(state).iterate({
    enter(n) {
      if (n.name === 'LinkReference') {
        const label = n.node.getChild('LinkLabel');
        const url = n.node.getChild('URL');
        if (label && url) {
          const key = normalizeLabel(doc.sliceString(label.from + 1, label.to - 1));
          // First definition wins.
          if (!refs.has(key)) refs.set(key, doc.sliceString(url.from, url.to));
        }
        return false;
      }
      if (LEAF_BLOCKS.has(n.name)) return false;
    },
  });
  return refs;
}

// `cache` is the live-preview layout cache (cleared on doc/tree change).
export function linkReferences(state, cache) {
  if (!cache) return collect(state);
  let refs = cache.get('linkRefs');
  if (!refs) cache.set('linkRefs', (refs = collect(state)));
  return refs;
}

// → { url, textFrom, textTo } or null (unclosed, or an undefined reference).
export function linkTarget(state, link, cache) {
  const marks = link.getChildren('LinkMark');
  if (marks.length < 2) return null;
  const textFrom = marks[0].to;
  const textTo = marks[1].from;
  const url = link.getChild('URL');
  if (url) return { url: state.doc.sliceString(url.from, url.to), textFrom, textTo };
  const label = link.getChild('LinkLabel');
  const raw = label && label.to - label.from > 2
    ? state.doc.sliceString(label.from + 1, label.to - 1)
    : state.doc.sliceString(textFrom, textTo);
  const found = linkReferences(state, cache).get(normalizeLabel(raw));
  return found ? { url: found, textFrom, textTo } : null;
}
