// Thematic breaks need a StateField: CM6 rejects block:true decorations from
// a ViewPlugin (live-preview.js). HorizontalRuleWidget needs block:true for
// a full-width <hr>. Single-line only — multi-line block replace corrupts height-map.
import { StateField } from '@codemirror/state';
import { Decoration, EditorView } from '@codemirror/view';
import { syntaxTree } from '@codemirror/language';
import { selectionTouchesLine, readingModeToggled } from './selection.js';
import { HorizontalRuleWidget } from './widgets.js';

// Unlike Frontmatter, a HorizontalRule can appear anywhere a block can (top
// level, inside a Blockquote, inside a ListItem) — finding them needs a tree
// walk, not an O(1) lookup. But that walk only has to happen when the doc
// actually changes; a caret move (selectionSet, fired on every arrow key)
// just needs to recheck each already-found position's own line against the
// new selection, not re-walk the tree to rediscover them.
//
// NEVER_CONTAINS_HR prunes that walk to roughly the block-structure skeleton:
// a "---" line is only ever a direct child of a block *container*
// (Document/Blockquote/ListItem), never inside a leaf block's own content —
// so once the walk enters a Paragraph, a heading, a fenced/indented code
// block, a table cell, etc., there's no need to descend into its inline
// marks/text to rule out a nested HorizontalRule; it structurally can't be
// there. Verified against the actual parser (editor/probe_tree.mjs)
// against a HorizontalRule nested in a blockquote, a list item, past a table,
// and past a fenced code block containing a literal "---" comment line — same
// positions as the unpruned walk, ~55% fewer nodes visited on that fixture
// (a real prose-heavy note, being mostly inline text, prunes far more).
const NEVER_CONTAINS_HR = new Set([
  'Paragraph',
  'ATXHeading1', 'ATXHeading2', 'ATXHeading3', 'ATXHeading4', 'ATXHeading5', 'ATXHeading6',
  'SetextHeading1', 'SetextHeading2',
  'FencedCode', 'CodeBlock',
  'HTMLBlock',
  'CommentBlock', 'ProcessingInstruction',
  'Table', 'TableHeader', 'TableRow', 'TableCell', 'TableDelimiter',
  'Frontmatter',
  'LinkReference',
  'BlankLine',
]);

function findHorizontalRulePositions(state) {
  const positions = [];
  syntaxTree(state).iterate({
    enter(node) {
      if (node.name === 'HorizontalRule') { positions.push(node.from); return false; }
      if (NEVER_CONTAINS_HR.has(node.name)) return false;
    },
  });
  return positions;
}

function buildDecorations(state, positions) {
  const doc = state.doc;
  const ranges = [];
  for (const from of positions) {
    const line = doc.lineAt(from);
    if (selectionTouchesLine(state, line)) continue;
    const followedByBlank = line.number < doc.lines && doc.line(line.number + 1).text.trim() === '';
    // Include trailing newline or CM6 leaves a phantom empty .cm-line after <hr>.
    const to = line.to < doc.length ? line.to + 1 : line.to;
    ranges.push(
      Decoration.replace({
        widget: new HorizontalRuleWidget(followedByBlank),
        block: true,
        inclusiveEnd: false,
      }).range(from, to)
    );
  }
  return Decoration.set(ranges, true);
}

export function horizontalRuleField() {
  return StateField.define({
    create(state) {
      const positions = findHorizontalRulePositions(state);
      return { positions, decorations: buildDecorations(state, positions), tree: syntaxTree(state) };
    },
    update(value, tr) {
      // @codemirror/language only parses a fresh EditorState synchronously
      // up to ~3000 chars; the rest arrives later via a separate
      // Language.setState dispatch (background/idle-time parsing) that has
      // docChanged: false — so on a document longer than that, positions
      // found beyond ~3000 chars would otherwise never be discovered.
      // treeChanged (comparing this transaction's before/after tree, same
      // check live-preview.js's LivePreviewPlugin uses) catches that
      // follow-up dispatch and re-walks once the tree actually grew.
      const tree = syntaxTree(tr.state);
      const treeChanged = tree != value.tree;
      if (!tr.docChanged && !tr.selection && !treeChanged && !readingModeToggled(tr.startState, tr.state)) return value;
      const positions = (tr.docChanged || treeChanged) ? findHorizontalRulePositions(tr.state) : value.positions;
      return { positions, decorations: buildDecorations(tr.state, positions), tree };
    },
    provide: (f) => EditorView.decorations.from(f, (v) => v.decorations),
  });
}
