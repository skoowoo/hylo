// Serves the editor bundle (built in memory from src/), the harness page,
// fixtures and a stand-in image, for the WebKit e2e tests; /chat is the real
// chat pane on the view's own assets, with the API left to the test to stub.
import http from 'node:http';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import esbuild from 'esbuild';
import { bundleOptions } from '../build-options.mjs';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const VIEW = path.join(HERE, '../../internal/server/view');
const VENDOR = path.join(HERE, '../../internal/server/static/vendor');

// The light theme's tokens, read from the Go strings the app injects them from.
function tokensCSS() {
  const go = ['shared_tokens.go', 'shared_content_tokens.go'].map((f) => fs.readFileSync(path.join(VIEW, f), 'utf8')).join('\n');
  const pick = (name) => (new RegExp('const ' + name + ' = `([^`]*)`').exec(go) || [])[1] || '';
  return ':root {' + ['appTokensShared', 'appTokensLight', 'contentTokensShared', 'contentTokensLight'].map(pick).join('') + '}';
}

function chatPage() {
  const css = ['base.css', 'home.css', 'note_shared_prose.css', 'agent_chat.css'];
  const js = ['shared_icons_line.js', 'keys.js', 'path_ac.js', 'chat_reduce.js', 'chat_text.js', 'chat_kernel.js', 'chat_rows.js', 'chat_view.js'];
  const section = fs.readFileSync(path.join(VIEW, 'assets/home_chat_section.html'), 'utf8').replace(/\{\{[^}]*\}\}/g, '');
  return `<!doctype html><html data-theme="light"><head><meta charset="utf-8">
<style>${tokensCSS()} html, body { margin: 0; height: 100%; } body { display: flex; font-family: -apple-system, sans-serif; } #chat-root { flex: 1; height: 100%; }</style>
${css.map((f) => `<link rel="stylesheet" href="/view/${f}">`).join('')}
<script src="/vendor/marked.min.js"></script><script src="/vendor/dompurify.min.js"></script>
<script>marked.setOptions({ gfm: true, breaks: true }); window.__hyloModules = { fenceCard: '/bundle/fence-card.js' };</script>
</head><body>${section}
${js.map((f) => `<script src="/view/${f}"></script>`).join('')}
</body></html>`;
}
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
    if (url.pathname === '/image') {
      // "…800x1200…" in the name sizes the stand-in; a short delay like a real fetch + decode.
      const dims = /(\d+)x(\d+)/.exec(url.search);
      const svg = dims ? IMAGE.replace(/1600/g, dims[1]).replace(/1000/g, dims[2]) : IMAGE;
      return setTimeout(() => send('image/svg+xml', svg), dims ? 80 : 0);
    }
    if (url.pathname === '/chat') return send('text/html', chatPage());
    const asset = /^\/(view|vendor)\/([\w.-]+)$/.exec(url.pathname);
    if (asset) {
      const file = path.join(asset[1] === 'view' ? path.join(VIEW, 'assets') : VENDOR, asset[2]);
      return send(asset[2].endsWith('.css') ? 'text/css' : 'text/javascript', fs.readFileSync(file));
    }
    if (files.has(url.pathname)) return send('text/javascript', files.get(url.pathname));
    res.statusCode = 404;
    res.end();
  });
  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
  return { url: `http://127.0.0.1:${server.address().port}`, close: () => new Promise((r) => server.close(r)) };
}
