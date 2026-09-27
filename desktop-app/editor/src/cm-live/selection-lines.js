// Every line touched by the selection, deduped across multi-cursor ranges.
// Shared by list-format.js and quote-format.js — both act "per selected
// line" rather than per selected character range.
//
// A range ending exactly at a line's start doesn't pull that trailing line
// in (same rule @codemirror/commands' own line-comment toggling uses for
// drag-to-next-line-start selections), so triple-clicking one line doesn't
// also touch the next. Blank lines are dropped from a selection that has at
// least one non-blank line (a stray trailing blank shouldn't get
// listified/quoted alongside real content); if every touched line is blank,
// they're kept — otherwise right-clicking an empty paragraph and picking a
// block action would silently do nothing.
export function touchedLines(state) {
  const byNumber = new Map();
  for (const range of state.selection.ranges) {
    const startLine = state.doc.lineAt(range.from);
    let endLine = state.doc.lineAt(range.to);
    if (!range.empty && range.to === endLine.from && endLine.number > startLine.number) {
      endLine = state.doc.lineAt(range.to - 1);
    }
    for (let n = startLine.number; n <= endLine.number; n++) {
      if (!byNumber.has(n)) byNumber.set(n, state.doc.line(n));
    }
  }
  const lines = [...byNumber.values()].sort((a, b) => a.from - b.from);
  const nonBlank = lines.filter(l => l.text.trim() !== '');
  return nonBlank.length ? nonBlank : lines;
}
