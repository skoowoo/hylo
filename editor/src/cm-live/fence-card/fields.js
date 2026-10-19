// Chat cards written by the model: `key: value` lines plus `- item` lines.
const FIELD = /^\s*([a-z][a-z-]*):\s*(.*?)\s*$/;
const ITEM = /^\s*[-*+]\s+(.*\S)\s*$/;

export function parseFields(source) {
  const fields = {};
  const items = [];
  for (const line of (source || '').split('\n')) {
    const item = ITEM.exec(line);
    if (item) {
      items.push(item[1]);
      continue;
    }
    const f = FIELD.exec(line);
    if (f && f[2]) (fields[f[1]] = fields[f[1]] || []).push(f[2]);
  }
  return { field: (k) => (fields[k] || [])[0] || '', all: (k) => fields[k] || [], items };
}

// Chat rewrites [[x]] to a markdown link before the fence reaches us; take the name back either way.
export function noteName(text) {
  const t = (text || '').trim();
  const link = /^\[([^\]]+)\]\(\/notes\?name=([^)]+)\)$/.exec(t);
  if (link) {
    try {
      return decodeURIComponent(link[2]).replace(/\.md$/, '');
    } catch {
      return link[1];
    }
  }
  return t.replace(/^\[\[/, '').replace(/\]\]$/, '').replace(/\|.*$/, '').trim();
}

export function injectStyle(id, css) {
  if (document.getElementById(id)) return;
  const el = document.createElement('style');
  el.id = id;
  el.textContent = css;
  document.head.appendChild(el);
}

const NOTE_LINK = /\[([^\]]+)\]\((\/notes\?name=[^)\s]+)\)/g;

// Plain text with note links kept clickable; the chat opens /notes? hrefs in the side pane.
export function appendText(el, text) {
  let last = 0;
  for (const m of (text || '').matchAll(NOTE_LINK)) {
    if (m.index > last) el.appendChild(document.createTextNode(text.slice(last, m.index)));
    el.appendChild(noteLink(m[1], m[2]));
    last = m.index + m[0].length;
  }
  if (last < (text || '').length) el.appendChild(document.createTextNode(text.slice(last)));
  return el;
}

export function noteLink(label, href) {
  const a = document.createElement('a');
  a.href = href || '/notes?name=' + encodeURIComponent(label + '.md');
  a.textContent = label;
  return a;
}
