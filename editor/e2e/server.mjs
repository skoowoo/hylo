// Serves the editor bundle (built in memory from src/), the harness page,
// fixtures and a stand-in image, for the WebKit e2e tests.
import http from 'node:http';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import esbuild from 'esbuild';
import { bundleOptions } from '../build-options.mjs';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const OUT = '/bundle';
// Wide and tall like a typical screenshot, so the 360px max-height applies.
const IMAGE = '<svg xmlns="http://www.w3.org/2000/svg" width="1600" height="1000"><rect width="1600" height="1000" fill="#9ab"/></svg>';

export async function startServer() {
  const result = await esbuild.build({ ...bundleOptions(OUT), write: false });
  const files = new Map(result.outputFiles.map((f) => ['/bundle/' + path.basename(f.path), f.contents]));
  const server = http.createServer((req, res) => {
    const url = new URL(req.url, 'http://localhost');
    const send = (type, body) => {
      res.setHeader('Content-Type', type);
      res.end(body);
    };
    if (url.pathname === '/') return send('text/html', fs.readFileSync(path.join(HERE, 'harness.html')));
    if (url.pathname === '/fixture') {
      const name = path.basename(url.searchParams.get('f') || '');
      return send('text/plain; charset=utf-8', fs.readFileSync(path.join(HERE, 'fixtures', name)));
    }
    if (url.pathname === '/image') return send('image/svg+xml', IMAGE);
    if (files.has(url.pathname)) return send('text/javascript', files.get(url.pathname));
    res.statusCode = 404;
    res.end();
  });
  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
  return { url: `http://127.0.0.1:${server.address().port}`, close: () => new Promise((r) => server.close(r)) };
}
