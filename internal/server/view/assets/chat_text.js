// Shared text helpers. Markdown is used by the inbox sheet; the composer
// height rule is used by Shorts. Neither belongs to the chat pane.
(function (root) {
  var mdCache = new Map();

  // Fence hydration needs `class` on <code>; nothing else keeps it.
  function onlyFenceClass(node, data) {
    if (data.attrName === 'class' && !(node.nodeName === 'CODE' && /^language-[\w-]+$/.test(data.attrValue))) data.keepAttr = false;
  }

  // Mirrors the registry in editor/src/cm-live/fence-card/registry.js; only these load the chunk.
  var FENCE_CARD_SEL = ['mindmap', 'choices', 'proposal', 'receipt', 'sources', 'embed'].map(function (l) {
    return 'pre > code.language-' + l;
  }).join(', ');
  var fenceLoad = null;
  var fenceMod = null;
  function fenceModule() {
    fenceLoad = fenceLoad || import('/static/fence-card.js').then(function (m) { return (fenceMod = m); });
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

  root.__hyloRenderMarkdown = function (text, opts) {
    if (!text || typeof text !== 'string') return '';
    var useCache = !opts || opts.cache !== false;
    if (useCache) {
      var cached = mdCache.get(text);
      if (cached !== undefined) return cached;
    }
    var processed = text.replace(/\[\[([^\]\[|]+?)(?:\|([^\]\[]+?))?\]\]/g, function (_, target, display) {
      target = target.trim();
      display = (display || target).trim();
      var name = target.endsWith('.md') ? target : target + '.md';
      return '[' + display + '](/notes?name=' + encodeURIComponent(name) + ')';
    });
    processed = processed.replace(/(?<!~)~(?!~)/g, '\\~');
    var html;
    if (typeof marked === 'undefined') {
      html = processed.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
    } else {
      html = marked.parse(processed);
      if (typeof DOMPurify !== 'undefined') {
        DOMPurify.addHook('uponSanitizeAttribute', onlyFenceClass);
        try {
          html = DOMPurify.sanitize(html, {
            ALLOWED_TAGS: ['p', 'br', 'strong', 'em', 's', 'del', 'code', 'pre', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6',
              'ul', 'ol', 'li', 'blockquote', 'a', 'hr', 'table', 'thead', 'tbody', 'tr', 'th', 'td', 'img'],
            ALLOWED_ATTR: ['href', 'title', 'src', 'alt', 'class'],
          });
        } finally {
          DOMPurify.removeHook('uponSanitizeAttribute', onlyFenceClass);
        }
      }
    }
    if (useCache) {
      if (mdCache.size >= 300) {
        var iter = mdCache.keys();
        for (var ei = 0; ei < 50; ei++) {
          var nxt = iter.next();
          if (nxt.done) break;
          mdCache.delete(nxt.value);
        }
      }
      mdCache.set(text, html);
    }
    return html;
  };

  root.__hyloAutoResize = function (el) {
    if (!el) return;
    el.style.height = 'auto';
    el.style.height = Math.min(el.scrollHeight, 140) + 'px';
    var card = el.closest('.chat-input-card, .shorts-compose-card');
    if (!card) return;
    var cs = getComputedStyle(el);
    var singleLineH = (parseFloat(cs.lineHeight) || 20) + parseFloat(cs.paddingTop) + parseFloat(cs.paddingBottom);
    card.classList.toggle('is-multiline', el.scrollHeight > singleLineH + 1);
  };
})(typeof window !== 'undefined' ? window : global);
