// Just enough YAML for the properties card: top-level keys with scalars, flow or
// block lists, block scalars and one level of nested maps. Anything else falls
// back to its raw text; the source is one click away.
//
// value: { type: 'empty' } | { type: 'scalar', text, quoted } | { type: 'text', text }
//      | { type: 'list', items: string[] } | { type: 'map', entries: { key, text }[] }

const KEY_RE = /^("(?:[^"\\]|\\.)*"|'(?:[^']|'')*'|[^\s#'"\-[{][^:#]*?|-[^\s:#][^:#]*?)\s*:(?:\s+(.*))?$/;
const ITEM_RE = /^(\s*)-(?:\s+(.*))?$/;
const BLOCK_SCALAR_RE = /^([|>])[+-]?\d*\s*(?:#.*)?$/;
const NULL_RE = /^(?:~|null)$/i;

export function unquote(raw) {
  const s = raw.trim();
  if (s.length >= 2 && s[0] === '"' && s[s.length - 1] === '"') {
    try {
      return JSON.parse(s);
    } catch {
      return s.slice(1, -1);
    }
  }
  if (s.length >= 2 && s[0] === "'" && s[s.length - 1] === "'") return s.slice(1, -1).replace(/''/g, "'");
  return s;
}

function isQuoted(raw) {
  const s = raw.trim();
  return s.length >= 2 && (s[0] === '"' || s[0] === "'") && s[s.length - 1] === s[0];
}

// " #" starts a comment only outside quotes.
function stripComment(raw) {
  let quote = null;
  for (let i = 0; i < raw.length; i++) {
    const c = raw[i];
    if (quote) {
      if (c === quote) quote = null;
    } else if (c === '"' || c === "'") {
      if (i === 0 || /\s|[,[]/.test(raw[i - 1])) quote = c;
    } else if (c === '#' && (i === 0 || /\s/.test(raw[i - 1]))) {
      return raw.slice(0, i).trimEnd();
    }
  }
  return raw.trimEnd();
}

function splitFlow(inner) {
  const items = [];
  let quote = null;
  let depth = 0;
  let cur = '';
  for (const c of inner) {
    if (quote) {
      if (c === quote) quote = null;
    } else if (c === '"' || c === "'") {
      quote = c;
    } else if (c === '[' || c === '{') {
      depth++;
    } else if (c === ']' || c === '}') {
      depth--;
    } else if (c === ',' && depth === 0) {
      items.push(cur);
      cur = '';
      continue;
    }
    cur += c;
  }
  items.push(cur);
  return items.map(unquote).filter((s) => s.length > 0);
}

function dedent(lines) {
  const indents = lines.filter((l) => l.trim()).map((l) => l.length - l.trimStart().length);
  const min = indents.length ? Math.min(...indents) : 0;
  return lines.map((l) => l.slice(min));
}

function blockScalar(style, lines) {
  const body = dedent(lines);
  while (body.length && !body[body.length - 1].trim()) body.pop();
  if (style === '|') return body.join('\n');
  // Folded: single newlines become spaces, blank lines stay as breaks.
  return body.join('\n').replace(/([^\n])\n(?=[^\n])/g, '$1 ');
}

function listItems(lines) {
  const items = [];
  for (const line of dedent(lines)) {
    const m = ITEM_RE.exec(line);
    if (m && m[1] === '') items.push(m[2] ? stripComment(m[2]) : '');
    else if (items.length && line.trim()) items[items.length - 1] += ' ' + line.trim();
  }
  return items.map(unquote).filter((s) => s.length > 0);
}

function mapEntries(lines) {
  const entries = [];
  for (const line of dedent(lines)) {
    if (!line.trim() || /^#/.test(line)) continue;
    const m = /^[^\s]/.test(line) && KEY_RE.exec(line);
    if (m) entries.push({ key: unquote(m[1]), text: m[2] ? unquote(stripComment(m[2])) : '' });
    else if (entries.length) entries[entries.length - 1].text += (entries[entries.length - 1].text ? ' ' : '') + line.trim();
  }
  return entries;
}

function childValue(children) {
  const content = children.filter((l) => l.trim() && !/^\s*#/.test(l));
  if (!content.length) return { type: 'empty' };
  const lead = dedent(content);
  if (lead.every((l) => /^\s/.test(l) || ITEM_RE.test(l))) return { type: 'list', items: listItems(content) };
  if (KEY_RE.test(lead[0])) return { type: 'map', entries: mapEntries(content) };
  return { type: 'text', text: content.map((l) => l.trim()).join(' ') };
}

function fieldValue(inline, children) {
  if (inline == null || inline === '') return childValue(children);
  const block = BLOCK_SCALAR_RE.exec(inline);
  if (block) return { type: 'text', text: blockScalar(block[1], children) };
  const raw = stripComment([inline, ...children.map((l) => l.trim())].filter(Boolean).join(' '));
  if (raw[0] === '[' && raw[raw.length - 1] === ']') return { type: 'list', items: splitFlow(raw.slice(1, -1)) };
  if (!raw || (!isQuoted(raw) && NULL_RE.test(raw))) return { type: 'empty' };
  return { type: 'scalar', text: unquote(raw), quoted: isQuoted(raw) };
}

// A line belongs to the field above when it's indented, blank, or a "- item"
// at column 0 under a key with no inline value (YAML allows that).
function isChild(line, openValue) {
  return !line.trim() || /^\s/.test(line) || (openValue && ITEM_RE.test(line));
}

export function parseFrontmatter(text) {
  const lines = text.split('\n');
  const fields = [];
  let i = 0;
  while (i < lines.length) {
    const line = lines[i++];
    if (!line.trim() || /^#/.test(line)) continue;
    const m = KEY_RE.exec(line);
    const inline = m && m[2] != null ? m[2] : '';
    const children = [];
    while (i < lines.length && isChild(lines[i], m && !inline.trim())) children.push(lines[i++]);
    while (children.length && !children[children.length - 1].trim()) children.pop();
    if (m) fields.push({ key: unquote(m[1]), value: fieldValue(inline.trim(), children) });
    else fields.push({ key: '', value: { type: 'text', text: [line, ...children].map((l) => l.trim()).join(' ') } });
  }
  return fields;
}
