// Where the cursor should land the moment a note's EditorState is built,
// instead of always doc offset 0 (EditorState.create's default with no
// selection given). Runs on raw text, before any EditorState/syntax tree
// exists — frontmatterSyntax's own Lezer node isn't available yet, so this
// re-detects the same leading `---\n...\n---` block by regex instead.
export function initialCursorOffset(content) {
  if (!content) return 0;

  var offset = 0;
  var fm = /^---\r?\n[\s\S]*?\r?\n---[ \t]*(?:\r?\n|$)/.exec(content);
  if (fm) offset = fm[0].length;

  // Walk to the first non-blank line after frontmatter, then skip past a
  // leading list/task/heading marker on it so the cursor starts on the
  // actual text instead of before the marker's own characters.
  while (offset < content.length) {
    var nl = content.indexOf('\n', offset);
    var end = nl === -1 ? content.length : nl;
    var line = content.slice(offset, end);
    if (line.trim() !== '') {
      var marker = /^\s*(?:[-*+]\s+\[[ xX]\]\s+|[-*+]\s+|\d+[.)]\s+|#{1,6}\s+)/.exec(line);
      return offset + (marker ? marker[0].length : 0);
    }
    if (nl === -1) return content.length;
    offset = nl + 1;
  }
  return content.length;
}
