import esbuild from 'esbuild';
import { fileURLToPath } from 'url';
import { dirname, join } from 'path';
import { readdirSync, rmSync } from 'fs';
import { bundleOptions } from './build-options.mjs';

const __dirname = dirname(fileURLToPath(import.meta.url));
const outDir = join(__dirname, '../internal/server/static');

// Code splitting keeps each language grammar (cm-live/code-languages.js) in its
// own lazily fetched chunk. Chunk names are hashed, so stale ones are cleared
// first; vendor/ (third-party scripts), agents/ (agent logos) and the Go package files stay.
const keep = new Set(['vendor', 'agents', 'static.go', 'static_test.go']);
for (const name of readdirSync(outDir)) {
  if (!keep.has(name)) rmSync(join(outDir, name), { recursive: true, force: true });
}

await esbuild.build(bundleOptions(outDir));

console.log('built →', outDir);
