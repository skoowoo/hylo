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

// A closed fence whose info names a card → { info, body, from, to } (whole lines), else null.
// Fences nested in lists or quotes are supported when the opening line carries only
// indentation and `>` before the fence; that prefix is stripped from each body line.
export function cardFence(doc, node) {
  const marks = node.getChildren('CodeMark');
  const infoNode = node.getChild('CodeInfo');
  if (marks.length < 2 || !infoNode) return null;
  const info = doc.sliceString(infoNode.from, infoNode.to);
  if (!cardEntry(info)) return null;
  const open = doc.lineAt(node.from);
  const prefix = doc.sliceString(open.from, marks[0].from);
  if (!/^[ \t>]*$/.test(prefix)) return null;
  const close = doc.lineAt(Math.max(node.from, node.to - 1));
  const lines = [];
  for (let n = open.number + 1; n < close.number; n++) {
    const text = doc.line(n).text;
    let i = 0;
    while (i < prefix.length && i < text.length && (text[i] === '>' || text[i] === ' ' || text[i] === '\t')) i++;
    lines.push(text.slice(i));
  }
  return { info, body: lines.join('\n'), from: open.from, to: close.to };
}
