// Reading view = live preview + non-editable + tighter vertical rhythm.
// Task checkboxes still toggle: readOnly only gates commands, and the
// checkbox widget dispatches its change directly.
import { EditorState } from '@codemirror/state';
import { EditorView } from '@codemirror/view';
import { readingMode } from './selection.js';

// Beats livePreviewTheme's `.cm-lp-hN` rules on specificity (extra .cm-reading
// class), not source order. Heading lines with a blank above carry inline
// padding from decorators.js, which nothing here can override.
const readingTheme = EditorView.theme({
  // Blank lines are real .cm-line rows at full line height; in reading view
  // they act as the paragraph gap, so 1em reads closer to a <p> margin.
  '&.cm-reading .cm-line:not([class*="cm-lp-"]):has(> br:only-child)': { lineHeight: '1' },
  '&.cm-reading .cm-lp-h1': { paddingTop: '1em !important', paddingBottom: '0.5em !important' },
  '&.cm-reading .cm-lp-h2': { paddingTop: '1.1em !important', paddingBottom: '0.5em !important' },
  '&.cm-reading .cm-lp-h3': { paddingTop: '1em !important', paddingBottom: '0.4em !important' },
  '&.cm-reading .cm-lp-h4, &.cm-reading .cm-lp-h5, &.cm-reading .cm-lp-h6': {
    paddingTop: '0.8em !important',
    paddingBottom: '0.4em !important',
  },
  '&.cm-reading .cm-content': { cursor: 'default' },
});

export function readingExtensions() {
  return [
    readingMode.of(true),
    EditorState.readOnly.of(true),
    EditorView.editable.of(false),
    EditorView.editorAttributes.of({ class: 'cm-reading' }),
    readingTheme,
  ];
}
