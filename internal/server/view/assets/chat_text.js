// Shared text helpers. Markdown is used by the inbox sheet; the composer
// height rule is used by Shorts. Neither belongs to the chat pane.
(function (root) {
  var mdCache = new Map();

  root.__vaultrRenderMarkdown = function (text, opts) {
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
        html = DOMPurify.sanitize(html, {
          ALLOWED_TAGS: ['p', 'br', 'strong', 'em', 's', 'del', 'code', 'pre', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6',
            'ul', 'ol', 'li', 'blockquote', 'a', 'hr', 'table', 'thead', 'tbody', 'tr', 'th', 'td', 'img'],
          ALLOWED_ATTR: ['href', 'title', 'src', 'alt'],
        });
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

  root.__vaultrAutoResize = function (el) {
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
