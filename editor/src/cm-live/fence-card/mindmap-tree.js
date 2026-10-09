// Markdown outline → mindmap INode tree. Headings set depth; list items nest by indent.
// Quotes too: link targets land inside an href="…" attribute.
const esc = (s) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;').replace(/'/g, '&#39;');

function inline(s) {
  return esc(s)
    .replace(/`([^`]+)`/g, '<code>$1</code>')
    .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
    .replace(/(?<![*\w])\*([^*]+)\*(?![*\w])/g, '<em>$1</em>')
    .replace(/~~([^~]+)~~/g, '<del>$1</del>')
    .replace(/\[([^\]]+)\]\((https?:[^)\s]+)\)/g, '<a href="$2" target="_blank" rel="noopener noreferrer">$1</a>');
}

const node = (text) => ({ content: inline(text), children: [], payload: {} });

export function parseOutline(source) {
  const root = node('');
  const stack = [{ level: 0, indent: -1, node: root, heading: true }];
  let inFence = false;

  for (const raw of source.split('\n')) {
    if (/^\s*(```|~~~)/.test(raw)) { inFence = !inFence; continue; }
    if (inFence || !raw.trim()) continue;

    const h = /^(#{1,6})\s+(.*?)\s*#*\s*$/.exec(raw);
    if (h) {
      while (stack.length > 1 && stack[stack.length - 1].level >= h[1].length) stack.pop();
      const n = node(h[2]);
      stack[stack.length - 1].node.children.push(n);
      stack.push({ level: h[1].length, indent: -1, node: n, heading: true });
      continue;
    }

    const li = /^(\s*)(?:[-*+]|\d+[.)])\s+(?:\[[ xX]\]\s+)?(.*)$/.exec(raw);
    if (li) {
      const indent = li[1].replace(/\t/g, '    ').length;
      while (stack.length > 1 && !stack[stack.length - 1].heading && stack[stack.length - 1].indent >= indent) stack.pop();
      const n = node(li[2]);
      stack[stack.length - 1].node.children.push(n);
      stack.push({ level: 7, indent, node: n, heading: false });
    }
  }

  // A single top-level node is the map's root; several get a synthetic one.
  if (root.children.length === 1) return root.children[0];
  root.content = '';
  return root;
}
