import { resolveAncestor } from './tree-utils.js';

const IMAGE_NODES = new Set(['Image', 'WikiImage']);

function imageCovering(state, pos) {
  const starts = resolveAncestor(state, pos, (n) => IMAGE_NODES.has(n.name), 1);
  if (starts && pos >= starts.from && pos < starts.to) return starts;
  const ends = resolveAncestor(state, pos, (n) => IMAGE_NODES.has(n.name), -1);
  if (ends && pos > ends.from && pos <= ends.to) return ends;
  return null;
}

// [![alt](src)](href): the picture widget covers the whole link.
export function linkWrappingImage(image) {
  const link = image.parent;
  if (!link || link.name !== 'Link') return null;
  let open = null;
  let close = null;
  for (let c = link.firstChild; c; c = c.nextSibling) {
    if (c.name !== 'LinkMark') continue;
    if (!open) open = c;
    else { close = c; break; }
  }
  if (!open || !close || open.to !== image.from || close.from !== image.to) return null;
  return link;
}

export function imageFillsLink(link) {
  for (let c = link.firstChild; c; c = c.nextSibling) {
    if (IMAGE_NODES.has(c.name) && linkWrappingImage(c)) return c;
  }
  return null;
}

export function imageNodeAt(state, pos) {
  const direct = imageCovering(state, pos);
  if (direct) return direct;
  const link = resolveAncestor(state, pos, (n) => n.name === 'Link', 1)
    || resolveAncestor(state, pos, (n) => n.name === 'Link', -1);
  if (!link || pos < link.from || pos > link.to) return null;
  return imageFillsLink(link);
}

// What the picture replaces: the image, or the link it fills.
export function imageSpan(node) {
  const link = linkWrappingImage(node);
  return link ? { from: link.from, to: link.to } : { from: node.from, to: node.to };
}
