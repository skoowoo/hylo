import { LanguageDescription, LanguageSupport, StreamLanguage } from '@codemirror/language';

// Explicit list instead of @codemirror/language-data, whose registry bundles ~140 grammars.
// Fences in other languages render as plain text. `load` must resolve to a LanguageSupport;
// a bare Language makes the markdown parser throw and drops highlighting for the whole note.
const lezer = (name, alias, load) => LanguageDescription.of({ name, alias, load });
const legacy = (name, alias, load) =>
  LanguageDescription.of({ name, alias, load: async () => new LanguageSupport(StreamLanguage.define(await load())) });

// The stock shell mode leaves command names as plain variables, which no highlight style colors.
const shellMode = async () => {
  const { shell } = await import('@codemirror/legacy-modes/mode/shell');
  return { ...shell, token: (stream, state) => {
    const t = shell.token(stream, state);
    return t === 'variable' ? 'builtin' : t;
  } };
};

export const codeLanguages = [
  lezer('JavaScript', ['js', 'jsx', 'mjs', 'cjs', 'node'], async () => (await import('@codemirror/lang-javascript')).javascript({ jsx: true })),
  lezer('TypeScript', ['ts', 'tsx'], async () => (await import('@codemirror/lang-javascript')).javascript({ jsx: true, typescript: true })),
  lezer('JSON', ['jsonc', 'json5'], async () => (await import('@codemirror/lang-json')).json()),
  lezer('Python', ['py'], async () => (await import('@codemirror/lang-python')).python()),
  lezer('Go', ['golang'], async () => (await import('@codemirror/lang-go')).go()),
  lezer('Rust', ['rs'], async () => (await import('@codemirror/lang-rust')).rust()),
  lezer('Java', [], async () => (await import('@codemirror/lang-java')).java()),
  lezer('C++', ['c', 'cpp', 'cc', 'h', 'hpp'], async () => (await import('@codemirror/lang-cpp')).cpp()),
  lezer('HTML', ['htm', 'svg'], async () => (await import('@codemirror/lang-html')).html()),
  lezer('CSS', ['scss', 'less'], async () => (await import('@codemirror/lang-css')).css()),
  lezer('XML', [], async () => (await import('@codemirror/lang-xml')).xml()),
  lezer('YAML', ['yml'], async () => (await import('@codemirror/lang-yaml')).yaml()),
  lezer('SQL', ['postgres', 'postgresql', 'pgsql', 'plsql', 'mysql', 'mariadb', 'mssql', 'tsql', 'sqlite'], async () => (await import('@codemirror/lang-sql')).sql()),
  lezer('PHP', [], async () => (await import('@codemirror/lang-php')).php()),
  legacy('Shell', ['sh', 'bash', 'zsh', 'console'], shellMode),
  legacy('TOML', [], async () => (await import('@codemirror/legacy-modes/mode/toml')).toml),
  legacy('Diff', ['patch'], async () => (await import('@codemirror/legacy-modes/mode/diff')).diff),
  legacy('Dockerfile', ['docker'], async () => (await import('@codemirror/legacy-modes/mode/dockerfile')).dockerFile),
  legacy('Ruby', ['rb'], async () => (await import('@codemirror/legacy-modes/mode/ruby')).ruby),
  legacy('Swift', [], async () => (await import('@codemirror/legacy-modes/mode/swift')).swift),
  legacy('Kotlin', ['kt'], async () => (await import('@codemirror/legacy-modes/mode/clike')).kotlin),
  legacy('C#', ['cs', 'csharp'], async () => (await import('@codemirror/legacy-modes/mode/clike')).csharp),
];

const FENCE_INFO_RE = /^ {0,3}(?:`{3,}|~{3,})[ \t]*([^\s`]+)/gm;

// Grammars are lazy chunks; one that isn't loaded yet when the note is parsed
// leaves its fence unhighlighted until the chunk lands and a reparse runs.
// Resolves once every language this text fences is loaded; null if none pending.
export function preloadCodeLanguages(text) {
  const pending = new Set();
  for (const m of text.matchAll(FENCE_INFO_RE)) {
    const desc = LanguageDescription.matchLanguageName(codeLanguages, m[1], true);
    if (desc && !desc.support) pending.add(desc);
  }
  if (!pending.size) return null;
  return Promise.all([...pending].map((d) => d.load().catch(() => null)));
}
