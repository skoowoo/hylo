import { keymap } from '@codemirror/view';
import { Prec } from '@codemirror/state';
import { toggleBold, toggleItalic, toggleStrikethrough, toggleInlineCode, insertLink } from './inline-format.js';

// Shared with format-menu.js so the hints it shows match what's bound.
export const FORMAT_KEYS = {
  bold: 'Mod-b',
  italic: 'Mod-i',
  strike: 'Mod-Shift-x',
  code: 'Mod-e',
  link: 'Mod-k',
};

// Prec.high: defaultKeymap binds Mod-i to selectParentSyntax.
export const formatKeymap = Prec.high(
  keymap.of([
    { key: FORMAT_KEYS.bold, run: toggleBold },
    { key: FORMAT_KEYS.italic, run: toggleItalic },
    { key: FORMAT_KEYS.strike, run: toggleStrikethrough },
    { key: FORMAT_KEYS.code, run: toggleInlineCode },
    { key: FORMAT_KEYS.link, run: insertLink },
  ])
);
