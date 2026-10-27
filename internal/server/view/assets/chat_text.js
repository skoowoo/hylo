// Shared text helpers: markdown (chat, inbox), fence cards, and the composer
// height rule (chat, Shorts).
(function (root) {
  var mdCache = new Map();

  // Fence hydration needs `class` on <code>; nothing else keeps it.
  function onlyFenceClass(node, data) {
    if (data.attrName === 'class' && !(node.nodeName === 'CODE' && /^language-[\w-]+$/.test(data.attrValue))) data.keepAttr = false;
  }

  // Mirrors the registry in editor/src/cm-live/fence-card/registry.js; only these load the chunk.
  var CARD_LANGS = ['mindmap', 'choices', 'proposal', 'receipt', 'sources', 'embed'];
  var FENCE_CARD_SEL = CARD_LANGS.map(function (l) {
    return 'pre > code.language-' + l;
  }).join(', ');
  var fenceLoad = null;
  var fenceMod = null;
  function fenceModule() {
    fenceLoad = fenceLoad || import(window.__hyloModules.fenceCard).then(function (m) { return (fenceMod = m); });
    return fenceLoad;
  }

  // The chat loads its card renderers up front, so the first card in a reply mounts
  // at its final size instead of growing in a few frames after the chunk arrives.
  root.__hyloPreloadFenceCards = function () {
    fenceModule().then(function (m) { return m.preloadInlineCards(); }).catch(function () {});
  };

  // Bumping _fenceGen drops a hydrate still waiting on the chunk for content that has moved on.
  root.__hyloReleaseFenceCards = function (node) {
    if (!node) return;
    node._fenceGen = (node._fenceGen || 0) + 1;
    if (fenceMod) fenceMod.releaseFenceCards(node);
    else if (fenceLoad) fenceLoad.then(function (m) { m.releaseFenceCards(node); }).catch(function () {});
  };

  // Once the chunk is in, a node already in the page hydrates in the same call: the
  // chat pins its scroll right after rendering, so a card put back a microtask later
  // would shift the content on every streamed token. A node still being built waits
  // the microtask until its host has inserted it, so cards measure on mount.
  // Hydrating also releases the previous render. Card fences stay hidden while the
  // chunk loads, so they never paint as code.
  root.__hyloHydrateFenceCards = function (node, opts) {
    if (!node) return;
    var gen = node._fenceGen = (node._fenceGen || 0) + 1;
    if (fenceMod && node.isConnected) { fenceMod.hydrateFenceCards(node, opts); return; }
    var pending = node.querySelectorAll(FENCE_CARD_SEL);
    if (!fenceLoad && !pending.length) return;
    var shown = function (on) {
      pending.forEach(function (code) { code.parentElement.style.display = on ? '' : 'none'; });
    };
    shown(false);
    fenceModule().then(function (m) { if (node._fenceGen === gen) m.hydrateFenceCards(node, opts); })
      .catch(function () { shown(true); });
  };

  function preprocess(text) {
    var processed = text.replace(/\[\[([^\]\[|]+?)(?:\|([^\]\[]+?))?\]\]/g, function (_, target, display) {
      target = target.trim();
      display = (display || target).trim();
      var name = target.endsWith('.md') ? target : target + '.md';
      return '[' + display + '](/notes?name=' + encodeURIComponent(name) + ')';
    });
    return processed.replace(/(?<!~)~(?!~)/g, '\\~');
  }

  function escapeHTML(s) {
    return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
  }

  function sanitize(html) {
    if (typeof DOMPurify === 'undefined') return html;
    DOMPurify.addHook('uponSanitizeAttribute', onlyFenceClass);
    try {
      return DOMPurify.sanitize(html, {
        ALLOWED_TAGS: ['p', 'br', 'strong', 'em', 's', 'del', 'code', 'pre', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6',
          'ul', 'ol', 'li', 'blockquote', 'a', 'hr', 'table', 'thead', 'tbody', 'tr', 'th', 'td', 'img'],
        ALLOWED_ATTR: ['href', 'title', 'src', 'alt', 'class'],
      });
    } finally {
      DOMPurify.removeHook('uponSanitizeAttribute', onlyFenceClass);
    }
  }

  function cachePut(cache, key, val) {
    if (cache.size >= 300) {
      var iter = cache.keys();
      for (var ei = 0; ei < 50; ei++) {
        var nxt = iter.next();
        if (nxt.done) break;
        cache.delete(nxt.value);
      }
    }
    cache.set(key, val);
  }

  root.__hyloRenderMarkdown = function (text, opts) {
    if (!text || typeof text !== 'string') return '';
    var useCache = !opts || opts.cache !== false;
    if (useCache) {
      var cached = mdCache.get(text);
      if (cached !== undefined) return cached;
    }
    var processed = preprocess(text);
    var html = typeof marked === 'undefined' ? escapeHTML(processed) : sanitize(marked.parse(processed));
    if (useCache) cachePut(mdCache, text, html);
    return html;
  };

  var blockCache = new Map();

  // The top-level blocks of a reply, each rendered on its own, so a host can keep
  // every block whose HTML didn't change and only redo the one still growing.
  // A block: { html, card, source }; card is the fence language when it has one.
  root.__hyloMarkdownBlocks = function (text) {
    if (!text || typeof text !== 'string') return [];
    var processed = preprocess(text);
    if (typeof marked === 'undefined') return [{ html: '<p>' + escapeHTML(processed) + '</p>', card: '' }];
    var tokens = marked.lexer(processed);
    var out = [];
    for (var i = 0; i < tokens.length; i++) {
      var tok = tokens[i];
      if (tok.type === 'space') continue;
      var html = blockCache.get(tok.raw);
      if (html === undefined) {
        var one = [tok];
        one.links = tokens.links;
        html = sanitize(marked.parser(one));
        cachePut(blockCache, tok.raw, html);
      }
      if (!html.trim()) continue;
      var card = tok.type === 'code' ? cardLang(tok.lang) : '';
      out.push({ html: html, card: card, source: card ? tok.text : '' });
    }
    return out;
  };

  function cardLang(info) {
    var lang = String(info || '').trim().split(/\s/)[0];
    return CARD_LANGS.indexOf(lang) !== -1 ? lang : '';
  }

  // One card fence a host renders as its own block: the card replaces `pre` and
  // the handle's update/destroy are the host's. Until the chunk is in, the code
  // block stays hidden so it never paints as code.
  root.__hyloFenceBlock = function (pre, partial) {
    if (fenceMod) return fenceMod.mountFenceBlock(pre, { partial: partial });
    var real = null, dead = false, next = null, nextPartial = partial;
    pre.style.display = 'none';
    fenceModule().then(function (m) {
      // A parked row is out of the page but still owns this block: mount it anyway.
      if (dead) return;
      real = m.mountFenceBlock(pre, { partial: nextPartial });
      if (real && next !== null) real.update(next, { partial: nextPartial });
    }).catch(function () { pre.style.display = ''; });
    return {
      get el() { return real ? real.el : pre; },
      update: function (src, o) {
        nextPartial = !!(o && o.partial);
        if (real) real.update(src, o);
        else next = src;
      },
      destroy: function () { dead = true; if (real) real.destroy(); },
    };
  };

  // Through the editing path, not a value write, so undo survives and WebKit
  // reveals the caret. Fires `input` either way.
  root.__hyloTextareaInsert = function (el, start, end, text) {
    if (!el.disabled) {
      el.focus();
      el.setSelectionRange(start, end);
      if (document.execCommand('insertText', false, text)) return;
    }
    el.setRangeText(text, start, end, 'end');
    el.dispatchEvent(new Event('input', { bubbles: true }));
  };

  function fitHeight(el) {
    if (getComputedStyle(el).fieldSizing === 'content') return;
    el.style.height = 'auto';
    el.style.height = el.scrollHeight + 'px';
  }

  // ev is the input event when the user edited, absent for a programmatic
  // value. Typing only grows the composer: a wrapped shape stays until the
  // text is emptied, and the height until the user deletes, since IME marked
  // text (pinyin) is wider than what it commits and would bounce both.
  root.__hyloAutoResize = function (el, ev) {
    if (!el) return;
    var type = ev ? ev.inputType || '' : '';
    var deleted = /^delete/.test(type) && type !== 'deleteCompositionText';
    if (!ev || deleted || !el.value) el.style.removeProperty('--grow-floor');
    var card = el.closest('.chat-input-card');
    if (card && !(ev && el.value && card.classList.contains('is-multiline'))) {
      // Judge against the single-row shape: multiline widens the textarea, so
      // measuring in it would unwrap the text and flip back.
      card.classList.remove('is-multiline');
      fitHeight(el);
      card.classList.toggle('is-multiline', el.offsetHeight > (parseFloat(getComputedStyle(el).lineHeight) || 20) + 1);
    }
    fitHeight(el);
    if (ev && !deleted && el.value) el.style.setProperty('--grow-floor', el.offsetHeight + 'px');
  };
})(typeof window !== 'undefined' ? window : global);
