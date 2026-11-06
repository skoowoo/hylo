// The list inside a composer autocomplete panel (@ and /): an optional fixed
// head above a scrolling list, and one active row that keys and the pointer
// both move, so there is never a second highlight.
//
// opts: head (an empty element above the list), list (the <ul>), onPick() for the active row, signal
(function (root) {
  root.__hyloAcList = function (opts) {
    var list = opts.list;
    var head = opts.head;
    var items = [];
    var active = -1;
    var nav = null;
    var px = -1, py = -1;

    function rows() { return list.querySelectorAll('li[role="option"]'); }

    function setActive(i, reveal) {
      var lis = rows();
      if (!lis.length) { active = -1; return; }
      active = (i + lis.length) % lis.length;
      lis.forEach(function (li, j) { li.setAttribute('aria-selected', j === active ? 'true' : 'false'); });
      if (!reveal) return;
      var li = lis[active];
      var pad = parseFloat(getComputedStyle(list).paddingTop) || 0;
      if (active === 0) list.scrollTop = 0;
      else if (li.offsetTop - pad < list.scrollTop) list.scrollTop = li.offsetTop - pad;
      else if (li.offsetTop + li.offsetHeight + pad > list.scrollTop + list.clientHeight) list.scrollTop = li.offsetTop + li.offsetHeight + pad - list.clientHeight;
    }

    // Only a real move picks the row under the pointer: WebKit fires a move at
    // the old spot when the list scrolls or redraws under a still mouse.
    document.addEventListener('pointermove', function (e) {
      var moved = e.clientX !== px || e.clientY !== py;
      px = e.clientX; py = e.clientY;
      if (!moved || !list.contains(e.target)) return;
      var li = e.target.closest('li[role="option"]');
      if (li) setActive(Array.prototype.indexOf.call(rows(), li), false);
    }, { capture: true, signal: opts.signal });

    // mousedown, not click: the composer keeps focus.
    list.addEventListener('mousedown', function (e) {
      e.preventDefault();
      var li = e.target.closest('li[role="option"]');
      if (!li) return;
      setActive(Array.prototype.indexOf.call(rows(), li), false);
      opts.onPick();
    }, { signal: opts.signal });
    // An earlier crumb jumps to its level; the chevron or the current one goes back one.
    head.addEventListener('mousedown', function (e) {
      e.preventDefault();
      var b = nav && e.target.closest('button');
      if (!b) return;
      var i = b.dataset.crumb == null ? -1 : +b.dataset.crumb;
      if (i >= 0 && i < nav.crumbs.length - 1) nav.onCrumb(i);
      else nav.onBack();
    }, { signal: opts.signal });

    function crumb(label, i) {
      var b = document.createElement('button');
      b.type = 'button';
      b.tabIndex = -1;
      b.className = 'ac-crumb';
      if (i >= 0) b.dataset.crumb = i;
      if (i < 0) b.innerHTML = hyloIcon('chevron-left');
      else b.textContent = label;
      return b;
    }

    // spec: { items, row(item, li), note, head: {crumbs, onBack, onCrumb} }
    function show(spec) {
      items = spec.items || [];
      nav = spec.head || null;
      head.hidden = !nav;
      head.innerHTML = '';
      if (nav) {
        head.appendChild(crumb('', -1));
        nav.crumbs.forEach(function (label, i) {
          if (i) {
            var sep = document.createElement('span');
            sep.className = 'ac-sep';
            sep.textContent = '/';
            head.appendChild(sep);
          }
          head.appendChild(crumb(label, i));
        });
      }
      list.innerHTML = '';
      if (spec.note) {
        var n = document.createElement('li');
        n.className = 'ac-note';
        n.setAttribute('role', 'presentation');
        n.textContent = spec.note;
        list.appendChild(n);
      }
      items.forEach(function (item) {
        var li = document.createElement('li');
        li.setAttribute('role', 'option');
        spec.row(item, li);
        list.appendChild(li);
      });
      list.scrollTop = 0;
      setActive(0, false);
    }

    function clear() {
      items = [];
      active = -1;
      nav = null;
      head.hidden = true;
      head.innerHTML = '';
      list.innerHTML = '';
    }

    return {
      show: show,
      clear: clear,
      move: function (d) { if (active >= 0) setActive(active + d, true); },
      index: function () { return active; },
      item: function () { return active >= 0 ? items[active] : null; },
      count: function () { return items.length; },
    };
  };
})(typeof window !== 'undefined' ? window : globalThis);
