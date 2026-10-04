// Leading "---\n...\n---" as one Frontmatter block node.
// Without this: opening --- → HorizontalRule; closing --- → Setext H2.
import { syntaxTree } from '@codemirror/language';

const FRONTMATTER_NODE = 'Frontmatter';

function isDelimLine(line) {
  return line.text === '---';
}

export const frontmatterSyntax = {
  defineNodes: [{ name: FRONTMATTER_NODE, block: true }],
  parseBlock: [
    {
      name: FRONTMATTER_NODE,
      before: 'HorizontalRule', // must claim opening --- first
      parse(cx, line) {
        if (cx.lineStart !== 0 || !isDelimLine(line)) return false;
        const from = cx.lineStart;
        while (cx.nextLine()) {
          if (isDelimLine(line)) {
            cx.nextLine(); // consume closing ---
            break;
          }
        }
        // Unclosed → EOF, same as FencedCode.
        cx.addElement(cx.elt(FRONTMATTER_NODE, from, cx.prevLineEnd()));
        return true;
      },
    },
  ],
};

// The parse rule above only fires at cx.lineStart === 0, so Frontmatter (if
// present) is always the document's first top-level node — callers that
// just need to locate it (frontmatter-collapse.js, frontmatter-readonly.js)
// can check that one child directly instead of walking the whole tree.
export function findFrontmatterNode(state) {
  const first = syntaxTree(state).topNode.firstChild;
  return first && first.name === FRONTMATTER_NODE ? first : null;
}
