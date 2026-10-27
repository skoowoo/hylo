import { Decoration } from '@codemirror/view';

// The primitives the decoration builders share.

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

// What CM draws: the visible viewport plus the lines holding the main
// selection's ends, which stay in the DOM after scrolling away. Decorating
// less leaves those lines measured raw, and their height changes on return.
export function drawnRanges(view) {
  const ranges = view.visibleRanges.map(({ from, to }) => ({ from, to }));
  const { main } = view.state.selection;
  for (const pos of [main.anchor, main.head]) {
    if (ranges.some((r) => pos >= r.from && pos <= r.to)) continue;
    const line = view.state.doc.lineAt(pos);
    ranges.push({ from: line.from, to: line.to });
  }
  return ranges.sort((a, b) => a.from - b.from);
}
