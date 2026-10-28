// /dev/icons: theme switch (this page only, not saved) and per-icon metrics.
(function () {
  var root = document.documentElement;
  function syncSeg() {
    var dark = root.getAttribute('data-theme') === 'dark' ||
      (!root.hasAttribute('data-theme') && !matchMedia('(prefers-color-scheme: light)').matches);
    document.querySelectorAll('[data-theme-set]').forEach(function (b) {
      b.classList.toggle('active', (b.dataset.themeSet === 'dark') === dark);
    });
  }
  document.querySelectorAll('[data-theme-set]').forEach(function (b) {
    b.addEventListener('click', function () { root.setAttribute('data-theme', b.dataset.themeSet); syncSeg(); });
  });
  syncSeg();

  var OPTICAL = [17.5, 20.5], OFFSET = 0.6, N = 240;
  // Fill each ink row between its outermost pixels; the area of that
  // silhouette tracks how big an icon reads better than its bounding box.
  async function measure(svg) {
    var s = svg.cloneNode(true);
    s.setAttribute('xmlns', 'http://www.w3.org/2000/svg');
    s.setAttribute('width', N); s.setAttribute('height', N); s.setAttribute('stroke', '#000');
    var img = new Image();
    img.src = 'data:image/svg+xml,' + encodeURIComponent(s.outerHTML);
    await img.decode();
    var c = document.createElement('canvas'); c.width = c.height = N;
    var g = c.getContext('2d'); g.drawImage(img, 0, 0);
    var d = g.getImageData(0, 0, N, N).data, area = 0, x0 = N, x1 = 0, y0 = N, y1 = 0;
    for (var y = 0; y < N; y++) {
      var l = -1, r = -1;
      for (var x = 0; x < N; x++) if (d[(y * N + x) * 4 + 3] > 60) { if (l < 0) l = x; r = x; }
      if (l < 0) continue;
      area += r - l + 1; x0 = Math.min(x0, l); x1 = Math.max(x1, r); y0 = Math.min(y0, y); y1 = y;
    }
    var u = 24 / N;
    return { optical: Math.sqrt(area) * u, offset: Math.hypot((x0 + x1) / 2 * u - 12, (y0 + y1) / 2 * u - 12),
             clipped: x0 === 0 || y0 === 0 || x1 === N - 1 || y1 === N - 1 };
  }
  document.querySelectorAll('tr[data-icon]').forEach(async function (row) {
    var m = await measure(row.querySelector('.spec-glyph-24 svg'));
    var opt = row.querySelector('[data-metric="optical"]'), off = row.querySelector('[data-metric="offset"]');
    opt.textContent = m.optical.toFixed(1);
    off.textContent = m.offset.toFixed(2);
    // Ink on the box edge means the zoom pushed part of the icon out of view.
    if (m.clipped) { off.textContent += ' clipped'; off.classList.add('is-off'); return; }
    if (!row.hasAttribute('data-check')) return;
    opt.classList.toggle('is-off', m.optical < OPTICAL[0] || m.optical > OPTICAL[1]);
    off.classList.toggle('is-off', m.offset > OFFSET);
  });
})();
