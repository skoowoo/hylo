import esbuild from 'esbuild';
import { fileURLToPath } from 'url';
import { dirname, join } from 'path';

const __dirname = dirname(fileURLToPath(import.meta.url));
const outDir = join(__dirname, '../internal/server/static');

// outdir + splitting (was a single outfile) — cm-live/index.js's
// codeLanguages: languages (@codemirror/language-data) pulls in ~140
// language grammars, each behind its own `load: () => import(...)`. Without
// splitting, esbuild has no way to know which of those will actually be
// used at runtime (that's the whole point of lazy per-fence loading), so it
// inlines all of them into the one file unconditionally — every note pays
// for every language on every load. With splitting, each language package
// becomes its own chunk that's only fetched the first time a fence
// actually needs it. entryNames keeps the entry file at the same
// static/editor.js path drawer.js already `import()`s; chunkNames/
// assetNames are left at esbuild's defaults (hashed, collision-safe) since
// nothing references those by a fixed name — internal/server/static_files.go
// serves the whole directory generically (http.FileServer over the go:embed
// FS), so new chunk files need no server-side wiring.
await esbuild.build({
  entryPoints: [join(__dirname, 'src/index.js')],
  bundle: true,
  format: 'esm',
  splitting: true,
  outdir: outDir,
  entryNames: 'editor',
  minify: true,
  target: ['chrome120'],
  treeShaking: true,
});

console.log('built →', outDir);
