// ── Tags wall: word-cloud layout via d3-cloud (vendor/d3-cloud.min.js) ────
// Packed tightly like a classic word cloud — plain text sized by tag
// frequency, not the pill/count-badge treatment used elsewhere in this app,
// since d3-cloud's interlocking placement is computed from each word's own
// glyph shape (rendered to a hidden canvas to measure it) and stops being
// tight the moment you wrap that measured shape in a padded, bordered pill.
// d3-cloud silently drops words that don't fit inside the box it's given
// (no wrapping/scrolling within the layout itself), so the box height is
// estimated from total glyph area rather than a fixed guess.
//
// Re-laid-out on panel resize (split-pane drag, reading-pane toggle, window
// resize) via a ResizeObserver on #home-list-body — never on #tag-cloud
// itself, since this function sets #tag-cloud's own height as part of every
// run, which would just retrigger the observer on itself. __hyloTagCloudGen
// guards against a slow, still-running layout (d3-cloud places words in
// timed async batches) finishing after a newer resize already started a
// fresh one and clearing the container first.
var __hyloTagCloudObserver = null;
var __hyloTagCloudResizeTimer = null;
var __hyloTagCloudGen = 0;

function __hyloRenderTagCloud() {
  var el = document.getElementById('tag-cloud');
  if (!el || typeof d3 === 'undefined' || !d3.layout || !d3.layout.cloud) return;
  var tags;
  try { tags = JSON.parse(el.dataset.tags || '[]'); } catch (e) { return; }
  if (!tags.length) return;

  var maxCount = 0;
  for (var i = 0; i < tags.length; i++) { if (tags[i].count > maxCount) maxCount = tags[i].count; }
  var MIN_SIZE = 14, MAX_SIZE = 56;
  var words = tags.map(function (t) {
    var ratio = maxCount > 0 ? t.count / maxCount : 0;
    var size = MIN_SIZE + (MAX_SIZE - MIN_SIZE) * Math.sqrt(ratio);
    return {
      text: t.name,
      count: t.count,
      size: size,
      weight: size > 34 ? '600' : (size > 20 ? '500' : '400')
    };
  });
  var fontFamily = (getComputedStyle(document.documentElement).getPropertyValue('--font-ui') || 'sans-serif').trim();

  function layoutAndRender() {
    var gen = ++__hyloTagCloudGen;
    var width = el.clientWidth || 800;
    el.dataset.layoutWidth = String(width);
    // Rough glyph-area estimate (avg glyph ~0.6x its font-size square) to
    // size the box. The multiplier controls the cloud's silhouette, not
    // just how much empty margin there is: d3-cloud's spiral starts at the
    // box center and grows outward until it finds room for each word, so
    // slack this close to the true minimum forces later words out toward
    // the box's corners — the filled shape reads as roughly rectangular.
    // More headroom lets everything settle near the center before that
    // happens, so the outline stays a rounded/elliptical blob instead.
    var totalArea = 0;
    for (var j = 0; j < words.length; j++) {
      totalArea += words[j].size * words[j].size * words[j].text.length * 0.6;
    }
    var height = Math.max(420, Math.ceil((totalArea * 2.9) / width));
    el.style.height = height + 'px';
    el.innerHTML = '';

    d3.layout.cloud()
      .size([width, height])
      .words(words)
      .padding(5)
      .rotate(0)
      .font(fontFamily)
      .fontWeight(function (d) { return d.weight; })
      .fontSize(function (d) { return d.size; })
      .on('end', function (placed) {
        if (gen !== __hyloTagCloudGen) return; // a newer resize already re-ran this
        placed.forEach(function (d) {
          var btn = document.createElement('button');
          btn.type = 'button';
          btn.className = 'tag-cloud-word';
          btn.textContent = d.text;
          btn.title = d.text + ' · ' + d.count;
          btn.style.fontSize = d.size + 'px';
          btn.style.fontWeight = d.weight;
          btn.style.left = (width / 2 + d.x) + 'px';
          btn.style.top = (height / 2 + d.y) + 'px';
          btn.addEventListener('click', function () {
            if (window._homeData) window._homeData.selectTag(d.text);
          });
          el.appendChild(btn);
        });
      })
      .start();
  }

  // d3-cloud measures each word's collision box by rendering it to a hidden
  // canvas at the resolved font — but fonts.css loads Inter with
  // font-display: swap, so a layout run before it's actually downloaded
  // measures against whatever fallback font the browser is showing that
  // instant. The real DOM text swaps to Inter once it lands, wider or
  // narrower than the fallback, and the collision boxes computed earlier no
  // longer match — small words showing that mismatch first since a couple
  // px of error is proportionally much bigger at 14px than at 56px.
  // document.fonts.ready resolves once every requested font has settled, so
  // waiting on it here keeps what's measured and what's rendered in sync.
  function watch() {
    if (__hyloTagCloudObserver) { __hyloTagCloudObserver.disconnect(); __hyloTagCloudObserver = null; }
    var body = el.parentElement;
    if (!body || typeof ResizeObserver === 'undefined') return;
    var lastWidth = body.clientWidth;
    __hyloTagCloudObserver = new ResizeObserver(function () {
      var w = body.clientWidth;
      if (Math.abs(w - lastWidth) < 4) return; // sub-pixel/height-only noise
      lastWidth = w;
      clearTimeout(__hyloTagCloudResizeTimer);
      __hyloTagCloudResizeTimer = setTimeout(layoutAndRender, 150);
    });
    __hyloTagCloudObserver.observe(body);
  }

  // Restoring a parked tag wall already has its words. Re-hook resize only,
  // so coming back doesn't wipe the cloud and jump the scroll position —
  // unless the width changed while it was parked.
  var laidOut = Number(el.dataset.layoutWidth);
  if (el.querySelector('.tag-cloud-word') && Math.abs((el.clientWidth || 800) - laidOut) < 4) { watch(); return; }

  if (document.fonts && document.fonts.ready) {
    document.fonts.ready.then(layoutAndRender).catch(layoutAndRender);
  } else {
    layoutAndRender();
  }
  watch();
}
