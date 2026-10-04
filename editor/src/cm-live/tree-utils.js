import { syntaxTree } from '@codemirror/language';

// Shared "walk the parent chain looking for a match" — format-guards.js,
// list-indent.js, inline-format.js and decorators.js each hand-rolled their
// own version of this independently, with no shared "is this the *same*
// node" check. That's exactly the gap that let decorators.js's list-gap
// collapse match *any* ListItem in the tree instead of one belonging to the
// same List — same class of bug, one shared place to get it right.

export function nearestAncestor(node, predicate) {
  for (let n = node; n; n = n.parent) {
    if (predicate(n)) return n;
  }
  return null;
}

export function nameIs(names) {
  return (n) => names.includes(n.name);
}

export function resolveAncestor(state, pos, predicate, side = 1) {
  return nearestAncestor(syntaxTree(state).resolveInner(pos, side), predicate);
}

// Lezer SyntaxNode objects are transient views, not stable singletons —
// compare by position + name, not reference (same rule list-indent.js's
// previousSiblingItem already followed for the same reason).
export function isSameNode(a, b) {
  return !!a && !!b && a.from === b.from && a.to === b.to && a.name === b.name;
}
