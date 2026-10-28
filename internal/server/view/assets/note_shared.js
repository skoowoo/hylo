var fmIconDown = hyloIcon('chevrons-down');
var fmIconUp   = hyloIcon('chevrons-up');

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
