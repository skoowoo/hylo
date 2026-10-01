var fmIconDown = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><path d="m7 6 5 5 5-5"/><path d="m7 13 5 5 5-5"/></svg>';
var fmIconUp   = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><path d="m7 11 5-5 5 5"/><path d="m7 18 5-5 5 5"/></svg>';

// Renders a key-combo hint (e.g. "⌘K") for the shared .kbd-combo component
// (base.css) — wraps ⌘ in .kbd-cmd so its fallback-font glyph can be
// rescaled independently of the surrounding mono text.
window.hyloKbdHTML = function(str) {
  return str.replace(/⌘/g, '<span class="kbd-cmd">⌘</span>');
};

window.fmToggleGrid = function(btn) {
  var details = btn.closest('details');
  var grid = details && details.querySelector('.fm-grid');
  if (!grid) return;
  var expanded = grid.classList.toggle('fm-expanded');
  btn.innerHTML = expanded ? fmIconUp : fmIconDown;
};

(function () {
  document.addEventListener('click', function (e) {
    var a = e.target.closest ? e.target.closest('a') : null;
    if (!a || !a.closest('.prose')) return;
    var href = a.getAttribute('href');
    if (!href) return;
    e.preventDefault();
    e.stopPropagation();
    if (/^https?:\/\//.test(href)) {
      window.open(href, '_blank', 'noopener,noreferrer');
    } else if (href.indexOf('/notes?') === 0) {
      try {
        var u = new URL(href, window.location.origin);
        var name = u.searchParams.get('name') || '';
        var path = u.searchParams.get('path') || '';
        if (path && window.__hyloContentPane) {
          var title = a.textContent.trim() || path.split('/').pop().replace(/\.md$/, '');
          void window.__hyloContentPane.openNoteInContentPane(path, title, false, false);
        } else if (name && typeof __hyloContentPaneOpenWikiLink === 'function') {
          void __hyloContentPaneOpenWikiLink(name.replace(/\.md$/, ''));
        }
      } catch(_) {}
    }
  }, true);
})();
