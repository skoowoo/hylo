import esbuild from 'esbuild';
import { fileURLToPath } from 'url';
import { dirname, join } from 'path';
import { readdirSync, rmSync } from 'fs';

const __dirname = dirname(fileURLToPath(import.meta.url));
const outDir = join(__dirname, '../internal/server/static');

// Code splitting keeps each language grammar (cm-live/code-languages.js) in its
// own lazily fetched chunk. Chunk names are hashed, so stale ones are cleared
// first; vendor/ holds hand-managed third-party scripts and stays.
for (const name of readdirSync(outDir)) {
  if (name !== 'vendor') rmSync(join(outDir, name), { recursive: true, force: true });
}

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
