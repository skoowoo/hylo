// Overlay scrollbars for .overlay-scroll panes, the editor's bar
// (cm-live/scroll.js) in shape and timing: native bars are hidden page-wide
// (home.css), and a drawn one shows while a pane scrolls or the pointer is on
// its edge, then fades. One bar serves every pane, moving to the pane in use,
// so a pane that goes away can't leave a live strip behind; it takes the
// pointer only while the pointer is on its pane's edge, so it never covers
// what floats over the pane. Dragging or paging fires hylo:scrollbar-drag
// (detail true, then false) on the pane: the chat reads that as the reader's gesture.
(function () {
  var W = 10;
  var SHOWN = 900; // ms the bar stays after the last scroll
  var MIN_THUMB = 32;
  var bar = null, thumb = null, pane = null, timer = 0, drag = null;

  function pageStep(h) { return Math.max(h * 0.875, h - 40, 1); }
  // A thread stuck to its bottom scrolls column-reverse: 0 is the bottom, up is negative.
  function reversed(el) { return getComputedStyle(el).flexDirection === 'column-reverse'; }
  function range(el) { return el.scrollHeight - el.clientHeight; }

  function build() {
    bar = document.createElement('div');
    bar.className = 'scrollbar';
    bar.setAttribute('aria-hidden', 'true');
    thumb = bar.appendChild(document.createElement('div'));
    thumb.className = 'scrollbar-thumb';
    // A press on the bar must not move focus or start a text selection.
    bar.addEventListener('mousedown', function (e) { e.preventDefault(); });
    bar.addEventListener('pointerdown', press);
    bar.addEventListener('pointermove', move);
    bar.addEventListener('pointerup', release);
    bar.addEventListener('pointercancel', release);
    bar.addEventListener('pointerleave', function () { if (!drag) bar.classList.remove('is-hot'); });
  }

  // Beside the pane, so it shares the pane's containing block and stacking context.
  function attach(el) {
    if (!bar) build();
    if (pane !== el) {
      pane = el;
      bar.classList.remove('is-hot', 'is-shown');
    }
    if (el.nextSibling !== bar) el.after(bar);
    return draw();
  }

  function draw() {
    var el = pane;
    var r = range(el);
    bar.hidden = r < 1;
    if (r < 1) return false;
    var client = el.clientHeight;
    var len = Math.round(Math.max(MIN_THUMB, (client * client) / el.scrollHeight));
    var top = reversed(el) ? r + el.scrollTop : el.scrollTop;
    var at = Math.round((Math.min(Math.max(top, 0), r) / r) * (client - len));
    bar.style.top = (el.offsetTop + el.clientTop) + 'px';
    bar.style.left = (el.offsetLeft + el.clientLeft + el.clientWidth - W) + 'px';
    bar.style.height = client + 'px';
    thumb.style.height = len + 'px';
    thumb.style.transform = 'translateY(' + at + 'px)';
    return true;
  }

  function show() {
    bar.classList.add('is-shown');
    clearTimeout(timer);
    timer = setTimeout(function () {
      if (!drag) bar.classList.remove('is-shown');
    }, SHOWN);
  }

  function gesture(on) {
    pane.dispatchEvent(new CustomEvent('hylo:scrollbar-drag', { detail: on }));
  }

  function press(e) {
    if (e.button !== 0 || !pane) return;
    var t = thumb.getBoundingClientRect();
    if (e.target !== thumb) {
      // Pages toward the press, like the system's default track click.
      gesture(true);
      pane.scrollBy(0, e.clientY < t.top ? -pageStep(pane.clientHeight) : pageStep(pane.clientHeight));
      gesture(false);
      return;
    }
    bar.setPointerCapture(e.pointerId);
    bar.classList.add('is-dragging');
    drag = { y: e.clientY, from: t.top - bar.getBoundingClientRect().top, len: t.height };
    gesture(true);
    show();
  }

  function move(e) {
    if (!drag) return;
    var room = pane.clientHeight - drag.len;
    if (room <= 0) return;
    var frac = Math.min(Math.max((drag.from + e.clientY - drag.y) / room, 0), 1);
    var r = range(pane);
    pane.scrollTop = reversed(pane) ? frac * r - r : frac * r;
  }

  function release() {
    if (!drag) return;
    drag = null;
    bar.classList.remove('is-dragging');
    gesture(false);
    show();
  }

  // .edge-fade: each edge fades by how much content is hidden past it (capped at 24px).
  // Content changes size without a scroll (groups opening, results swapping), so watch that too.
  window.__hyloEdgeFade = function (el) {
    function sync() {
      var below = el.scrollHeight - el.clientHeight - el.scrollTop;
      el.style.setProperty('--fade-top', Math.min(el.scrollTop, 24) + 'px');
      el.style.setProperty('--fade-bottom', Math.min(Math.max(below, 0), 24) + 'px');
    }
    el.addEventListener('scroll', sync, { passive: true });
    sync();
    if (typeof ResizeObserver === 'undefined') return;
    var ro = new ResizeObserver(sync);
    ro.observe(el);
    Array.prototype.forEach.call(el.children, function (c) { ro.observe(c); });
    // Subtree too: display:contents children (.sr-item) have no box for ro to watch.
    new MutationObserver(function (records) {
      records.forEach(function (r) {
        Array.prototype.forEach.call(r.addedNodes, function (n) { if (n.nodeType === 1 && n.parentNode === el) ro.observe(n); });
        Array.prototype.forEach.call(r.removedNodes, function (n) { if (n.nodeType === 1) ro.unobserve(n); });
      });
      sync();
    }).observe(el, { childList: true, subtree: true });
  };

  document.addEventListener('scroll', function (e) {
    var el = e.target;
    if (el.nodeType !== 1 || !el.classList.contains('overlay-scroll')) return;
    if (attach(el)) show();
  }, { capture: true, passive: true });

  document.addEventListener('pointermove', function (e) {
    if (drag || e.pointerType === 'touch' || !e.target.closest) return;
    var el = e.target.closest('.overlay-scroll');
    if (!el) return;
    var right = el.getBoundingClientRect().left + el.clientLeft + el.clientWidth;
    if (e.clientX < right - W || e.clientX >= right) return;
    if (attach(el)) bar.classList.add('is-hot');
  }, { passive: true });
})();
