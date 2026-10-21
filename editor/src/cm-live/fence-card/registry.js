// Fenced-block renderers keyed by info string. A renderer's module is fetched on first use:
//   mount(el, source, { mode: 'card' | 'pip' | 'full', onError }) → { update(source), fit?(), destroy() }
// An inline card sizes to its content and gets no zoom or fullscreen. `editor: false`
// keeps a card out of notes: it only renders where marked HTML is hydrated (chat).
const entries = new Map();

export function registerFenceCard({ lang, alias = [], load, minHeight = 280, inline = false, editor = true }) {
  // `renderer` is set once loaded, so later cards can mount synchronously.
  const entry = { lang, minHeight, inline, editor, renderer: null };
  let pending = null;
  entry.load = () => (pending = pending || load().then((r) => (entry.renderer = r), (e) => { pending = null; throw e; }));
  for (const name of [lang, ...alias]) entries.set(name.toLowerCase(), entry);
}

export function cardEntry(info) {
  return entries.get((info || '').trim().split(/\s+/)[0].toLowerCase());
}

registerFenceCard({ lang: 'mindmap', load: () => import('./mindmap.js'), minHeight: 360 });
registerFenceCard({ lang: 'choices', load: () => import('./choices.js'), minHeight: 36, inline: true, editor: false });
registerFenceCard({ lang: 'proposal', load: () => import('./proposal.js'), minHeight: 120, inline: true, editor: false });
registerFenceCard({ lang: 'receipt', load: () => import('./receipt.js'), minHeight: 48, inline: true, editor: false });
registerFenceCard({ lang: 'sources', load: () => import('./sources.js'), minHeight: 20, inline: true, editor: false });

// Loads the chat's inline renderers ahead of the first reply that uses one.
export function preloadInlineCards() {
  return Promise.all([...new Set(entries.values())].filter((e) => e.inline).map((e) => e.load()));
}

// A closed fence → { info, body, from, to } (whole lines), else null.
// Fences nested in lists or quotes are supported when the opening line carries only
// indentation and `>` before the fence; that prefix is stripped from each body line.
export function fenceSource(doc, node) {
  const marks = node.getChildren('CodeMark');
  if (marks.length < 2) return null;
  const infoNode = node.getChild('CodeInfo');
  const info = infoNode ? doc.sliceString(infoNode.from, infoNode.to) : '';
  const open = doc.lineAt(node.from);
  if (!/^[ \t>]*$/.test(doc.sliceString(open.from, marks[0].from))) return null;
  const close = doc.lineAt(Math.max(node.from, node.to - 1));
  return { info, body: fenceBody(doc, node), from: open.from, to: close.to };
}

// Content lines with the fence's own indent/quote prefix removed; also works
// for a fence opened on a list-marker line.
export function fenceBody(doc, node) {
  const marks = node.getChildren('CodeMark');
  if (marks.length < 2) return '';
  const open = doc.lineAt(node.from);
  const width = marks[0].from - open.from;
  const close = doc.lineAt(Math.max(node.from, node.to - 1));
  const lines = [];
  for (let n = open.number + 1; n < close.number; n++) {
    const text = doc.line(n).text;
    let i = 0;
    while (i < width && i < text.length && (text[i] === '>' || text[i] === ' ' || text[i] === '\t')) i++;
    lines.push(text.slice(i));
  }
  return lines.join('\n');
}

// A closed fence whose info names a card the editor shows, else null.
export function cardFence(doc, node) {
  const f = fenceSource(doc, node);
  const entry = f && cardEntry(f.info);
  return entry && entry.editor ? f : null;
}
