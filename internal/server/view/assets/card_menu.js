// One ⋯ menu for every card. It lives in the top layer (popover), so no card's
// overflow, isolation or neighbours can clip or cover it; Esc closes it through
// the page's Esc stack.
//   __hyloCardMenu.toggle(trigger, items) — items: {icon, label, run, danger} or 'divider'
var __hyloCardMenu = (function () {
  var menu = null, trigger = null, gone = null;

  function build() {
    menu = document.createElement('div');
    menu.className = 'cselect-dropdown card-menu';
    menu.setAttribute('popover', 'manual');
    menu.setAttribute('role', 'menu');
    menu.tabIndex = -1;
    menu.addEventListener('keydown', onKey);
    document.body.appendChild(menu);
    // A menu outlives neither its trigger moving nor its card being replaced.
    document.addEventListener('scroll', function (e) {
      if (trigger && e.target.contains && e.target.contains(trigger)) close();
    }, true);
    gone = new MutationObserver(function () { if (trigger && !trigger.isConnected) close(); });
    window.addEventListener('resize', function () { close(); });
    window.addEventListener('blur', function () { close(); });
    document.addEventListener('pointerdown', function (e) {
      if (trigger && !menu.contains(e.target) && !trigger.contains(e.target)) close();
    }, true);
  }

  function items(list) {
    menu.textContent = '';
    list.forEach(function (it) {
      if (it === 'divider') {
        var hr = document.createElement('div');
        hr.className = 'cselect-divider hairline';
        menu.appendChild(hr);
        return;
      }
      var b = document.createElement('button');
      b.type = 'button';
      b.className = 'cselect-option' + (it.danger ? ' card-menu-danger' : '');
      b.setAttribute('role', 'menuitem');
      b.innerHTML = hyloIcon(it.icon) + '<span></span>';
      b.lastChild.textContent = it.label;
      b.addEventListener('click', function () { close(); it.run(); });
      menu.appendChild(b);
    });
  }

  // Below the trigger, right edges aligned; above it when the window has no room.
  function place() {
    var r = trigger.getBoundingClientRect(), gap = 4, edge = 8;
    var w = menu.offsetWidth, h = menu.offsetHeight;
    var top = r.bottom + gap + h <= innerHeight - edge ? r.bottom + gap : r.top - gap - h;
    var left = Math.min(Math.max(r.right - w, edge), innerWidth - edge - w);
    // Whole pixels: WebKit draws a box at a fractional offset a pixel off when it composites.
    menu.style.top = Math.round(Math.max(top, edge)) + 'px';
    menu.style.left = Math.round(left) + 'px';
  }

  function open(t, list, byKey) {
    if (!menu) build();
    if (trigger) close();
    trigger = t;
    items(list);
    menu.showPopover();
    place();
    trigger.setAttribute('aria-expanded', 'true');
    var card = trigger.closest('.list-card');
    if (card) card.classList.add('is-menu-open');
    gone.observe(document.body, { childList: true, subtree: true });
    if (window.__hyloEscPush) window.__hyloEscPush('card-menu', close, menu);
    var first = menu.querySelector('.cselect-option');
    (byKey && first ? first : menu).focus({ preventScroll: true });
  }

  function close() {
    if (!trigger) return;
    var t = trigger;
    trigger = null;
    gone.disconnect();
    t.setAttribute('aria-expanded', 'false');
    var card = t.closest('.list-card');
    if (card) card.classList.remove('is-menu-open');
    menu.hidePopover();
    if (window.__hyloEscPop) window.__hyloEscPop('card-menu');
  }

  function onKey(e) {
    if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp' && e.key !== 'Home' && e.key !== 'End') return;
    e.preventDefault();
    var opts = Array.prototype.slice.call(menu.querySelectorAll('.cselect-option'));
    var i = opts.indexOf(document.activeElement);
    if (e.key === 'Home') i = 0;
    else if (e.key === 'End') i = opts.length - 1;
    else if (e.key === 'ArrowDown') i = i < 0 ? 0 : (i + 1) % opts.length;
    else i = i < 0 ? opts.length - 1 : (i - 1 + opts.length) % opts.length;
    if (opts[i]) opts[i].focus({ preventScroll: true });
  }

  return {
    // A keyboard press arrives as a click with detail 0; it focuses the first item.
    toggle: function (t, list, e) {
      if (trigger === t) { close(); return; }
      open(t, list, !!(e && e.detail === 0));
    },
  };
})();
