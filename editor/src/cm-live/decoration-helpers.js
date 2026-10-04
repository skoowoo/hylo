import { Decoration } from '@codemirror/view';

// Shared by decorators.js, frontmatter-decorate.js and tables-decorate.js —
// the two smallest, most-repeated primitives every node decorator builds on.

export function hideRange(from, to, decos) {
  if (from >= to) return;
  decos.push(Decoration.replace({}).range(from, to));
}

export function styleRange(from, to, className, decos) {
  if (from >= to) return;
  decos.push(Decoration.mark({ class: className }).range(from, to));
}
