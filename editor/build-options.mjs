import { fileURLToPath } from 'url';
import { dirname, join } from 'path';

const __dirname = dirname(fileURLToPath(import.meta.url));

// Shared by build.mjs and the e2e tests, which bundle in memory so they always
// exercise the current src/ without touching internal/server/static.
export function bundleOptions(outdir) {
  return {
    entryPoints: { editor: join(__dirname, 'src/index.js'), 'fence-card': join(__dirname, 'src/fence-card.js') },
    bundle: true,
    format: 'esm',
    splitting: true,
    outdir,
    entryNames: '[name]',
    minify: true,
    target: ['chrome120'],
    treeShaking: true,
  };
}
