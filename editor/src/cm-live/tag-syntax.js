// Obsidian #tag → HashTag syntax node. Same rules as util.InlineTags in Go;
// both are tested against internal/util/testdata/tags.json.

const CH_HASH = 35; // #
const TAG_RUN_RE = /^[\p{L}\p{N}\p{M}_\-/]+/u;
const DIGITS_RE = /^\p{Nd}+$/u;
const SPACE_RE = /\s/;

// Obsidian's limits on a run of tag characters: no leading '/', cut at "//",
// no trailing '/', not digits alone.
export function trimTagRun(run) {
  if (run.startsWith('/')) return '';
  const cut = run.indexOf('//');
  if (cut >= 0) run = run.slice(0, cut);
  run = run.replace(/\/+$/, '');
  return DIGITS_RE.test(run) ? '' : run;
}

function parseHashTag(cx, next, pos) {
  if (next !== CH_HASH) return -1;
  const prev = cx.char(pos - 1);
  if (prev >= 0 && !SPACE_RE.test(String.fromCharCode(prev))) return -1;
  const m = TAG_RUN_RE.exec(cx.slice(pos + 1, cx.end));
  const tag = m ? trimTagRun(m[0]) : '';
  if (!tag) return -1;
  return cx.addElement(cx.elt('HashTag', pos, pos + 1 + tag.length));
}

export const tagSyntax = {
  defineNodes: ['HashTag'],
  parseInline: [{ name: 'HashTag', parse: parseHashTag }],
};

// The canonical form tags are stored and looked up in (util.NormalizeTag).
export function tagKey(text) {
  return text.replace(/^#+/, '').normalize('NFC').toLowerCase();
}
