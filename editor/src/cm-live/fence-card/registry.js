// Fenced-block renderers keyed by info string. A renderer's module is fetched on first use:
//   mount(el, source, { mode: 'card' | 'pip' | 'full', onError }) → { update(source), fit?(), destroy() }
const entries = new Map();

export function registerFenceCard({ lang, alias = [], load, minHeight = 280 }) {
  const entry = { lang, load, minHeight };
  for (const name of [lang, ...alias]) entries.set(name.toLowerCase(), entry);
}

export function cardEntry(info) {
  return entries.get((info || '').trim().split(/\s+/)[0].toLowerCase());
}

registerFenceCard({ lang: 'mindmap', load: () => import('./mindmap.js'), minHeight: 360 });

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

// A closed fence whose info names a card, else null.
export function cardFence(doc, node) {
  const f = fenceSource(doc, node);
  return f && cardEntry(f.info) ? f : null;
}
