// Leading "---\n...\n---" as one Frontmatter block node, its body parsed as YAML.
// Without this: opening --- → HorizontalRule; closing --- → Setext H2.
import { syntaxTree, LanguageDescription, ParseContext } from '@codemirror/language';
import { parseMixed } from '@lezer/common';
import { codeLanguages } from './code-languages.js';

const FRONTMATTER_NODE = 'Frontmatter';
const DELIM = '---';
const SCAN_WINDOW = 8192;

// Unclosed isn't frontmatter (same rule as util.ParseFrontmatter and
// initialCursorOffset), so typing the opening "---" can't swallow the note.
// cx.input is untyped API, but a block parser can't un-consume lines to look ahead.
function hasClosingDelim(input) {
  const len = input.length;
  for (let pos = DELIM.length; pos < len; pos += SCAN_WINDOW) {
    const text = input.read(pos - 1, Math.min(len, pos + SCAN_WINDOW + DELIM.length + 1));
    if (text.includes('\n' + DELIM + '\n')) return true;
  }
  return len > DELIM.length && input.read(len - DELIM.length - 1, len) === '\n' + DELIM;
}

export const frontmatterSyntax = {
  defineNodes: [{ name: FRONTMATTER_NODE, block: true }],
  parseBlock: [
    {
      name: FRONTMATTER_NODE,
      before: 'HorizontalRule', // must claim opening --- first
      parse(cx, line) {
        if (cx.lineStart !== 0 || line.text !== DELIM || !hasClosingDelim(cx.input)) return false;
        while (cx.nextLine() && line.text !== DELIM);
        cx.nextLine(); // consume closing ---
        cx.addElement(cx.elt(FRONTMATTER_NODE, 0, cx.prevLineEnd()));
        return true;
      },
    },
  ],
  // Same lazy grammar fenced ```yaml uses; highlighting fills in once it loads.
  wrap: parseMixed((node) => {
    if (node.type.name !== FRONTMATTER_NODE) return null;
    const from = node.from + DELIM.length + 1;
    const to = node.to - DELIM.length - 1;
    if (to <= from) return null;
    const yaml = LanguageDescription.matchLanguageName(codeLanguages, 'yaml');
    const parser = yaml.support ? yaml.support.language.parser : ParseContext.getSkippingParser(yaml.load());
    return { parser, overlay: [{ from, to }] };
  }),
};

// The parse rule above only fires at cx.lineStart === 0, so Frontmatter (if
// present) is always the document's first top-level node.
export function findFrontmatterNode(state) {
  const first = syntaxTree(state).topNode.firstChild;
  return first && first.name === FRONTMATTER_NODE ? first : null;
}
