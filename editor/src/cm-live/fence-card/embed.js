// One http(s) URL per line. One line is a single card; several share a grid, at most three
// across, which turns into a list of single cards where three would not fit.
import { injectStyle } from './fields.js';
import { icon } from './icons.js';

const CSS = `
.hy-embed{display:block;margin:0;border-radius:var(--r-lg,12px);background:var(--surface-soft,#f5f5f7);color:var(--fg,#18181b);font:var(--text-sm,.8125rem)/1.4 var(--font-ui,system-ui,sans-serif);overflow:hidden;text-decoration:none}
.hy-card-embed{container-type:inline-size}
.hy-embed-row{display:grid;grid-template-columns:minmax(0,1fr);gap:var(--space-xs,8px);width:100%;min-width:0}
.hy-embed-row>.hy-embed{min-width:0}
.hy-embed-text{min-width:0;display:flex;flex-direction:column;gap:2px}
.hy-embed-title{font-weight:var(--fw-semibold,600);display:-webkit-box;-webkit-box-orient:vertical;-webkit-line-clamp:2;overflow:hidden;overflow-wrap:anywhere}
.hy-embed-desc,.hy-embed-site,.hy-embed-time{color:var(--muted,#6d7080);font-size:var(--text-xs,.75rem);font-weight:400}
.hy-embed-desc{display:-webkit-box;-webkit-box-orient:vertical;-webkit-line-clamp:2;overflow:hidden}
.hy-embed-site{margin-top:auto}
.hy-embed-page{display:grid;grid-template-columns:clamp(96px,32%,160px) minmax(0,1fr);min-height:84px}
.hy-embed-page.hy-embed-no-image{grid-template-columns:minmax(0,1fr)}
.hy-embed-page .hy-embed-text{padding:10px 12px}
.prose .hy-embed-shot,.hy-embed-shot{width:100%;height:100%;object-fit:cover;display:block;border:none;border-radius:0;max-width:none}
.hy-embed-page>.hy-embed-shot{align-self:center;width:calc(100% - 8px);height:auto;aspect-ratio:1.91/1;margin:8px 0 8px 8px;border-radius:var(--r-md,8px)}
.hy-embed-picture .hy-embed-shot{height:auto;max-height:320px;object-fit:contain;background:var(--btn-bg,rgba(24,24,27,.06))}
.hy-embed-audio{display:flex;align-items:center;gap:10px;padding:10px 12px;cursor:pointer}
.hy-embed-audio:hover{background:var(--btn-bg-hov,rgba(24,24,27,.06))}
.hy-embed-audio .hy-embed-text{flex:1}
.hy-embed-audio .hy-embed-title{overflow-wrap:normal}
.hy-embed-meta{display:flex;align-items:baseline;gap:6px;min-width:0}
.hy-embed-meta .hy-embed-site{min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.hy-embed-site:not(:empty)+.hy-embed-time::before{content:"·";margin-right:6px}
.hy-embed-time{flex:none;white-space:nowrap;font-variant-numeric:tabular-nums}
.hy-embed-audio audio{display:none}
a.hy-embed-title,.prose a.hy-embed-title{color:inherit;text-decoration:none}
a.hy-embed-title:hover{text-decoration:underline}
.hy-embed-play{all:unset;box-sizing:border-box;cursor:pointer;display:flex;align-items:center;justify-content:center;width:32px;height:32px;border-radius:999px;background:var(--fg,#18181b);color:var(--bg,#fff);flex:none}
.hy-embed-play:focus-visible,.hy-embed-open:focus-visible,.hy-embed-retry:focus-visible,a.hy-embed-title:focus-visible{outline:2px solid rgba(var(--accent-rgb,94,106,210),.5);outline-offset:2px}
.hy-embed-video{position:relative;background:#000;cursor:pointer}
.hy-embed-video video{display:block;width:100%;aspect-ratio:16/9;max-height:280px;object-fit:contain;background:#000}
.hy-embed-video .hy-embed-play{position:absolute;left:50%;top:50%;transform:translate(-50%,-50%);width:48px;height:48px;background:rgba(255,255,255,.92);color:#111;box-shadow:0 2px 10px rgba(0,0,0,.35)}
.hy-embed-cap{position:absolute;left:0;right:0;bottom:0;display:flex;flex-direction:column;gap:2px;padding:28px 12px 10px;background:linear-gradient(transparent,rgba(0,0,0,.65));color:#fff;pointer-events:none}
.hy-embed-cap .hy-embed-site{color:rgba(255,255,255,.75)}
.hy-embed-cap .hy-embed-title{-webkit-line-clamp:1}
.hy-embed-video.is-started .hy-embed-play,.hy-embed-video.is-started .hy-embed-cap{display:none}
.hy-embed-video.is-started{cursor:default}
.hy-embed-open{all:unset;box-sizing:border-box;cursor:pointer;display:flex;align-items:center;justify-content:center;width:28px;height:28px;border-radius:var(--r-sm,6px)}
.hy-embed-video .hy-embed-open{position:absolute;top:8px;left:8px;background:rgba(0,0,0,.5);color:#fff;opacity:0;pointer-events:none;transition:opacity var(--motion-fast,100ms)}
.hy-embed-video:hover .hy-embed-open,.hy-embed-video:focus-within .hy-embed-open{opacity:1;pointer-events:auto}
.hy-embed-file{display:flex;align-items:center;justify-content:space-between;gap:12px;padding:10px 12px}
.hy-embed-kind{flex:none;font-size:var(--text-2xs,.6875rem);letter-spacing:.04em;color:var(--muted,#6d7080)}
.hy-embed-failed .hy-embed-title{font-weight:var(--fw-medium,500)}
.hy-embed-failed .hy-embed-meta{margin-top:auto}
.hy-embed-retry{all:unset;cursor:pointer;margin-left:auto;flex:none;color:var(--accent-text,#5e6ad2);font-size:var(--text-xs,.75rem);border-radius:var(--r-sm,6px)}
.hy-embed-retry:hover{text-decoration:underline}
.hy-embed-retry[disabled]{color:var(--muted,#6d7080);cursor:default;text-decoration:none}
.hy-embed-plain{padding:12px 14px;color:var(--muted,#6d7080);min-height:48px}
.hy-embed-pending{display:flex;align-items:flex-end;padding:10px 12px;color:var(--muted,#6d7080);font-size:var(--text-xs,.75rem);min-height:84px;animation:hy-embed-pulse 1.4s ease-in-out infinite}
.hy-embed-pending[data-kind="audio"]{min-height:56px;align-items:center}
.hy-embed-pending[data-kind="video"]{min-height:0;aspect-ratio:16/9;max-height:280px}
.hy-embed-pending[data-kind="image"]{min-height:200px}
@keyframes hy-embed-pulse{50%{opacity:.55}}
@media (prefers-reduced-motion:reduce){.hy-embed-pending{animation:none}}
@container (min-width:561px){
.hy-embed-row{grid-template-columns:repeat(3,minmax(0,1fr))}
.hy-embed-row[data-n="2"]{grid-template-columns:repeat(2,minmax(0,1fr))}
.hy-embed-row .hy-embed-page{grid-template-columns:minmax(0,1fr);grid-template-rows:108px minmax(0,1fr)}
.hy-embed-row .hy-embed-page.hy-embed-no-image{grid-template-rows:minmax(0,1fr)}
.hy-embed-row .hy-embed-page>.hy-embed-shot{width:100%;height:108px;aspect-ratio:auto;margin:0;border-radius:0}
.hy-embed-row .hy-embed-audio{flex-direction:column;align-items:stretch;gap:10px;padding:12px}
.hy-embed-row .hy-embed-file{align-items:flex-start}
.hy-embed-row>.hy-embed-pending{min-height:200px}
.hy-embed-row>.hy-embed-pending[data-kind="audio"]{min-height:104px}
}
.hy-embed svg{width:16px;height:16px;fill:none;stroke:currentColor;stroke-width:2;stroke-linecap:round;stroke-linejoin:round}
.hy-embed-play svg{fill:currentColor;stroke:none}
.prose a.hy-embed,.prose a.hy-embed:hover{color:inherit;text-decoration:none}
a.hy-embed-page:hover,.prose a.hy-embed-page:hover{background:var(--btn-bg-hov,rgba(24,24,27,.06))}
`;

const cache = new Map();
const waiting = new Map();
let queue = [];
let scheduled = false;
const playing = new Set();
const clientTTL = 10 * 60 * 1000;

function lines(source) {
  return (source || '').split('\n').map((s) => s.trim()).filter(Boolean);
}

function lineURL(line) {
  return /^https?:\/\/\S+$/i.test(line) ? httpURL(line) : '';
}

export function embedURLs(source) {
  const urls = lines(source).map(lineURL);
  return urls.every(Boolean) ? urls : [];
}

function cached(url) {
  const hit = cache.get(url);
  return hit && Date.now() - hit.at < clientTTL ? hit.embed : null;
}

// retry asks again for a URL whose last answer was a failure.
function requestEmbed(url, retry = false) {
  if (!retry) {
    const hit = cached(url);
    if (hit) return Promise.resolve(hit);
  }
  if (waiting.has(url)) return waiting.get(url);
  if (retry) cache.delete(url);
  const p = new Promise((resolve) => {
    queue.push({ url, resolve, retry });
    if (!scheduled) {
      scheduled = true;
      queueMicrotask(flushEmbeds);
    }
  });
  waiting.set(url, p);
  return p;
}

function flushEmbeds() {
  scheduled = false;
  const batch = queue.splice(0, 20);
  if (queue.length) {
    scheduled = true;
    queueMicrotask(flushEmbeds);
  }
  for (const retry of [false, true]) {
    const part = batch.filter((b) => b.retry === retry);
    if (part.length) streamEmbeds(part, retry);
  }
}

// The server answers one JSON line per URL as each resolves, so a slow site holds back only its own card.
async function streamEmbeds(batch, retry) {
  const left = new Map(batch.map((b) => [b.url, b]));
  try {
    const r = await fetch('/api/embeds', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ urls: batch.map((b) => b.url), retry }),
    });
    if (!r.ok || !r.body) throw new Error(String(r.status));
    const reader = r.body.pipeThrough(new TextDecoderStream()).getReader();
    let buf = '';
    for (;;) {
      const { value, done } = await reader.read();
      if (done) break;
      buf += value;
      let nl;
      while ((nl = buf.indexOf('\n')) >= 0) {
        const line = buf.slice(0, nl).trim();
        buf = buf.slice(nl + 1);
        if (!line) continue;
        const embed = JSON.parse(line);
        const b = left.get(embed.url);
        if (!b) continue;
        left.delete(embed.url);
        settle(b, embed);
      }
    }
  } catch {
    /* whatever did not arrive fails below, uncached */
  }
  for (const b of left.values()) {
    waiting.delete(b.url);
    b.resolve({ url: b.url, status: 'error', error: 'fetch_failed' });
  }
}

function settle(b, embed) {
  cache.set(b.url, { embed, at: Date.now() });
  waiting.delete(b.url);
  b.resolve(embed);
}

function httpURL(raw) {
  try {
    const u = new URL(raw);
    if (u.protocol === 'http:' || u.protocol === 'https:') return u.href;
  } catch {
    /* ignore */
  }
  return '';
}

function hostName(raw) {
  try {
    return new URL(raw).hostname.replace(/^www\./, '');
  } catch {
    return '';
  }
}

function fileName(raw) {
  try {
    return decodeURIComponent(new URL(raw).pathname.split('/').filter(Boolean).pop() || '');
  } catch {
    return '';
  }
}

// host/path, for a link whose page could not be read.
function shortURL(raw) {
  try {
    const u = new URL(raw);
    let path = u.pathname.replace(/\/+$/, '');
    try { path = decodeURI(path); } catch { /* keep encoded */ }
    return u.hostname.replace(/^www\./, '') + path;
  } catch {
    return raw;
  }
}

// The kind a URL will most likely resolve to, so its placeholder takes about the final size.
function guessKind(url) {
  const ext = (/\.([a-z0-9]+)$/i.exec(safePath(url)) || [])[1];
  switch ((ext || '').toLowerCase()) {
    case 'mp3': case 'm4a': case 'aac': case 'wav': case 'ogg': case 'flac': case 'opus': return 'audio';
    case 'mp4': case 'webm': case 'mov': case 'm4v': return 'video';
    case 'png': case 'jpg': case 'jpeg': case 'gif': case 'webp': case 'avif': case 'svg': return 'image';
    default: return 'page';
  }
}

function safePath(raw) {
  try {
    return new URL(raw).pathname;
  } catch {
    return '';
  }
}

function fmt(seconds) {
  if (!Number.isFinite(seconds) || seconds < 0) seconds = 0;
  const s = Math.floor(seconds);
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const pad = (n) => String(n).padStart(2, '0');
  return h ? `${h}:${pad(m)}:${pad(s % 60)}` : `${m}:${pad(s % 60)}`;
}

function openLink(href) {
  const a = document.createElement('a');
  a.className = 'hy-embed-open';
  a.href = href;
  a.target = '_blank';
  a.rel = 'noopener noreferrer';
  a.title = 'Open';
  a.setAttribute('aria-label', 'Open');
  a.innerHTML = icon('external-link');
  a.addEventListener('click', (e) => e.stopPropagation());
  return a;
}

function plain(text) {
  const div = document.createElement('div');
  div.className = 'hy-embed hy-embed-plain';
  div.textContent = text || 'Empty embed';
  return div;
}

function failedCard(embed, onRetry) {
  const href = httpURL(embed.final_url) || httpURL(embed.url) || '';
  const card = document.createElement('div');
  card.className = 'hy-embed hy-embed-page hy-embed-no-image hy-embed-failed';
  const text = document.createElement('span');
  text.className = 'hy-embed-text';
  const title = document.createElement('a');
  title.className = 'hy-embed-title';
  title.href = href;
  title.target = '_blank';
  title.rel = 'noopener noreferrer';
  title.textContent = shortURL(href || embed.url || '');
  const desc = document.createElement('span');
  desc.className = 'hy-embed-desc';
  desc.textContent = 'Preview unavailable';
  const meta = document.createElement('span');
  meta.className = 'hy-embed-meta';
  const site = document.createElement('span');
  site.className = 'hy-embed-site';
  site.textContent = hostName(href);
  meta.appendChild(site);
  if (onRetry && embed.error !== 'blocked' && embed.error !== 'invalid_url') {
    const retry = document.createElement('button');
    retry.type = 'button';
    retry.className = 'hy-embed-retry';
    retry.textContent = 'Retry';
    retry.addEventListener('click', () => {
      retry.disabled = true;
      retry.textContent = 'Retrying…';
      onRetry();
    });
    meta.appendChild(retry);
  }
  text.append(title, desc, meta);
  card.appendChild(text);
  return card;
}

function pageCard(embed) {
  const href = httpURL(embed.final_url) || httpURL(embed.url);
  const a = document.createElement('a');
  a.className = 'hy-embed hy-embed-page hy-embed-no-image';
  a.href = href;
  a.target = '_blank';
  a.rel = 'noopener noreferrer';
  const shot = httpURL(embed.image_url);
  if (shot) {
    const img = document.createElement('img');
    img.className = 'hy-embed-shot';
    img.alt = '';
    img.src = shot;
    img.addEventListener('error', () => {
      img.remove();
      a.classList.add('hy-embed-no-image');
    });
    a.classList.remove('hy-embed-no-image');
    a.appendChild(img);
  }
  const text = document.createElement('span');
  text.className = 'hy-embed-text';
  const title = document.createElement('span');
  title.className = 'hy-embed-title';
  title.textContent = embed.title || shortURL(href);
  text.appendChild(title);
  if (embed.description) {
    const desc = document.createElement('span');
    desc.className = 'hy-embed-desc';
    desc.textContent = embed.description;
    text.appendChild(desc);
  }
  const site = document.createElement('span');
  site.className = 'hy-embed-site';
  site.textContent = embed.site_name || hostName(href);
  text.appendChild(site);
  a.appendChild(text);
  return a;
}

function bindMedia(card, media, button, embed) {
  const sync = () => {
    const on = !media.paused && !media.ended;
    button.setAttribute('aria-label', on ? 'Pause' : 'Play');
    button.innerHTML = icon(on ? 'pause' : 'play');
  };
  media.addEventListener('play', () => {
    for (const other of playing) if (other !== media) other.pause();
    playing.add(media);
    sync();
    // Once a video starts, its own controls take over: seeking, volume, fullscreen.
    if (media.tagName === 'VIDEO' && !media.controls) {
      media.controls = true;
      card.classList.add('is-started');
    }
  });
  media.addEventListener('pause', () => {
    playing.delete(media);
    sync();
    if (media._flush) media._flush();
  });
  media.addEventListener('ended', () => {
    sync();
    if (media._flush) media._flush();
  });
  card.addEventListener('click', (e) => {
    if (e.target.closest('a') || media.controls) return;
    e.preventDefault();
    if (media.paused || media.ended) media.play().catch(() => {});
    else media.pause();
  });
  sync();
  rememberProgress(media, embed);
  return media;
}

// Playback position lives on the url_embeds row. A metadata refresh leaves it.
function rememberProgress(media, embed) {
  const url = embed && embed.url;
  if (!url) return;
  let saved = embed.position_ms || 0;
  let timer = 0;
  const send = (ms) => {
    if (ms === saved) return;
    saved = ms;
    const hit = cache.get(url);
    if (hit && hit.embed) hit.embed.position_ms = ms;
    fetch('/api/embeds/progress', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ url, position_ms: ms }),
    }).catch(() => {});
  };
  const read = () => {
    const sec = media.currentTime;
    if (!Number.isFinite(sec)) return saved;
    if (media.ended) return 0;
    return Math.round(sec * 1000);
  };
  const flush = () => {
    clearTimeout(timer);
    timer = 0;
    if (media._ignore) return;
    send(read());
  };
  // timeupdate fires several times a second; keep the first timer so a play writes about every 2s.
  media.addEventListener('timeupdate', () => {
    if (timer || media._ignore) return;
    timer = setTimeout(flush, 2000);
  });
  media._flush = flush;
  if (saved <= 0) return;
  const resume = saved;
  const seek = () => {
    if (Number.isFinite(media.duration) && resume / 1000 >= media.duration) return;
    try { media.currentTime = resume / 1000; } catch { /* metadata not in yet */ }
  };
  if (media.readyState >= 1) seek();
  else media.addEventListener('loadedmetadata', seek, { once: true });
}

// The title links out, so nothing sits in the top-right corner the card's Edit button uses.
function audioCard(embed) {
  const href = httpURL(embed.final_url) || httpURL(embed.url);
  const card = document.createElement('div');
  card.className = 'hy-embed hy-embed-audio';
  const button = document.createElement('button');
  button.type = 'button';
  button.className = 'hy-embed-play';
  const text = document.createElement('span');
  text.className = 'hy-embed-text';
  const title = document.createElement('a');
  title.className = 'hy-embed-title';
  title.href = href;
  title.target = '_blank';
  title.rel = 'noopener noreferrer';
  title.textContent = embed.title || fileName(href) || hostName(href) || href;
  const meta = document.createElement('span');
  meta.className = 'hy-embed-meta';
  const site = document.createElement('span');
  site.className = 'hy-embed-site';
  site.textContent = embed.site_name || hostName(href);
  const time = document.createElement('span');
  time.className = 'hy-embed-time';
  time.textContent = embed.position_ms > 0 ? fmt(embed.position_ms / 1000) : '0:00';
  meta.append(site, time);
  text.append(title, meta);
  const audio = document.createElement('audio');
  audio.preload = embed.position_ms > 0 ? 'metadata' : 'none';
  audio.src = href;
  const showTime = () => {
    const dur = Number.isFinite(audio.duration) ? ` / ${fmt(audio.duration)}` : '';
    time.textContent = fmt(audio.currentTime) + dur;
  };
  audio.addEventListener('timeupdate', showTime);
  audio.addEventListener('loadedmetadata', showTime);
  card.append(button, text, audio);
  return { node: card, media: bindMedia(card, audio, button, embed) };
}

function videoCard(embed) {
  const href = httpURL(embed.final_url) || httpURL(embed.url);
  const card = document.createElement('div');
  card.className = 'hy-embed hy-embed-video';
  const video = document.createElement('video');
  video.preload = 'metadata';
  video.playsInline = true;
  // A time fragment makes WebKit paint the first frame instead of black.
  video.src = href.includes('#') ? href : href + '#t=0.001';
  const button = document.createElement('button');
  button.type = 'button';
  button.className = 'hy-embed-play';
  const cap = document.createElement('span');
  cap.className = 'hy-embed-cap';
  const title = document.createElement('span');
  title.className = 'hy-embed-title';
  title.textContent = embed.title || fileName(href) || hostName(href);
  const site = document.createElement('span');
  site.className = 'hy-embed-site';
  site.textContent = embed.site_name || hostName(href);
  cap.append(title, site);
  card.append(video, cap, button, openLink(href));
  return { node: card, media: bindMedia(card, video, button, embed) };
}

function imageCard(embed) {
  const href = httpURL(embed.final_url) || httpURL(embed.url);
  const a = document.createElement('a');
  a.className = 'hy-embed hy-embed-picture';
  a.href = href;
  a.target = '_blank';
  a.rel = 'noopener noreferrer';
  const img = document.createElement('img');
  img.className = 'hy-embed-shot';
  img.alt = embed.title || fileName(href) || '';
  img.src = href;
  a.appendChild(img);
  return a;
}

function fileCard(embed) {
  const href = httpURL(embed.final_url) || httpURL(embed.url);
  const a = document.createElement('a');
  a.className = 'hy-embed hy-embed-file';
  a.href = href;
  a.target = '_blank';
  a.rel = 'noopener noreferrer';
  const text = document.createElement('span');
  text.className = 'hy-embed-text';
  const title = document.createElement('span');
  title.className = 'hy-embed-title';
  title.textContent = embed.title || fileName(href) || hostName(href) || href;
  const site = document.createElement('span');
  site.className = 'hy-embed-site';
  site.textContent = hostName(href);
  text.append(title, site);
  const kind = document.createElement('span');
  kind.className = 'hy-embed-kind';
  const sub = (embed.content_type || '').split('/')[1] || '';
  kind.textContent = sub.replace(/^x-/, '').replace(/^vnd\./, '').toUpperCase();
  a.append(text, kind);
  return a;
}

function paint(embed, onRetry) {
  if (!embed || embed.status !== 'ok') return { node: failedCard(embed || {}, onRetry) };
  switch (embed.kind) {
    case 'page': return { node: pageCard(embed) };
    case 'audio': return audioCard(embed);
    case 'video': return videoCard(embed);
    case 'image': return { node: imageCard(embed) };
    default: return { node: fileCard(embed) };
  }
}

function hold(url) {
  const div = document.createElement('div');
  div.className = 'hy-embed hy-embed-pending';
  div.dataset.kind = guessKind(url);
  div.textContent = hostName(url);
  return div;
}

export function mount(el, source, opts = {}) {
  injectStyle('hy-embed-style', CSS);
  let gen = 0;
  let medias = [];

  function dropMedia(media) {
    media._ignore = true;
    playing.delete(media);
    media.pause();
    media.removeAttribute('src');
    try { media.load(); } catch { /* ignore */ }
  }

  function stop() {
    for (const media of medias) {
      // currentTime stays 0 until a saved position can seek; flushing that would wipe the row.
      if (media._flush && media.played && media.played.length) media._flush();
      dropMedia(media);
    }
    medias = [];
  }

  // A partial source is still being typed or streamed: fetching it would send every
  // prefix of a URL out, so it shows only what the cache already has.
  function render(next, partial) {
    const my = ++gen;
    stop();
    let urls = partial ? lines(next).map(lineURL) : embedURLs(next);
    if (!urls.length) {
      if (!partial) {
        el.replaceChildren(plain((next || '').trim()));
        return;
      }
      urls = [''];
    }
    const slots = urls.map(hold);
    let view = slots[0];
    if (slots.length > 1) {
      view = document.createElement('div');
      view.className = 'hy-embed-row';
      view.dataset.n = String(slots.length);
      view.append(...slots);
    }
    el.replaceChildren(view);
    const fill = (i, embed) => {
      if (my !== gen || !slots[i].isConnected) return;
      const retry = () => requestEmbed(urls[i], true).then((again) => fill(i, again));
      const built = paint(embed, retry);
      slots[i].replaceWith(built.node);
      slots[i] = built.node;
      if (built.media) medias.push(built.media);
    };
    urls.forEach((url, i) => {
      if (!url) return;
      if (partial) {
        const hit = cached(url);
        if (hit) fill(i, hit);
      } else {
        requestEmbed(url).then((embed) => fill(i, embed));
      }
    });
  }

  render(source, !!opts.partial);
  return {
    update(next, o) {
      render(next, !!(o && o.partial));
    },
    destroy() {
      gen++;
      stop();
    },
  };
}
