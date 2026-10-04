import esbuild from 'esbuild';
import { fileURLToPath } from 'url';
import { dirname, join } from 'path';

const __dirname = dirname(fileURLToPath(import.meta.url));

await esbuild.build({
  entryPoints: [join(__dirname, 'demo.js')],
  bundle: true,
  format: 'iife', // not 'esm' — index.html is opened via file://, where
  // module-script fetches are blocked by same-origin rules even for a
  // single bundled file with no imports left; a plain <script> has none
  // of that restriction.
  outfile: join(__dirname, 'dist/bundle.js'),
  target: ['chrome120'],
  sourcemap: true,
});

console.log('built → livepreview-demo/dist/bundle.js — open livepreview-demo/index.html directly in a browser.');
