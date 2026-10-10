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

// Revealed line-start syntax ("## ", "> ") drawn in the left margin: a box of
// `width` pulled back by the same amount advances the text by zero, so the
// line's content doesn't shift when the caret arrives.
export function hangRange(from, to, width, className, decos) {
  if (from >= to) return;
  decos.push(
    Decoration.mark({
      class: className + ' cm-lp-hang',
      attributes: { style: 'width:' + width + ';margin-left:-' + width },
    }).range(from, to)
  );
}

// CM samples line height from the first short plain-text line; a styled line
// sampled there skews the height map and arrow keys jump far. A span opts it out.
export function excludeFromTextSample(line, decos) {
  styleRange(line.from, line.to, 'cm-lp-styled-line', decos);
}
