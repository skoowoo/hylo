  // How many leading chips fit in `avail`. Pure so a resize can commit the
  // final count in one write — probing the live strip one chip at a time
  // paints every intermediate count.
  //
  // overflowW is a number, or a function of how many chips stay hidden
  // (the "+N" control is wider for "+9" than "+1"). It is reserved only
  // while something remains hidden. Always keeps one chip when there is
  // any width at all; the strip clips that chip rather than showing zero.
  function __vaultrFitTabCount(widths, avail, gap, overflowW) {
    var n = widths.length;
    if (!n || !(avail > 0)) return 0;
    var overflowAt = typeof overflowW === 'function' ? overflowW : function() { return overflowW; };
    var used = 0, fit = 0;
    for (var k = 0; k < n; k++) {
      var next = used + (k > 0 ? gap : 0) + widths[k];
      var hidden = n - k - 1;
      var reserve = hidden > 0 ? gap + overflowAt(hidden) : 0;
      if (next + reserve > avail + 0.5) break;
      used = next;
      fit = k + 1;
    }
    return fit > 0 ? fit : 1;
  }
  if (typeof module !== 'undefined') {
    module.exports = { __vaultrFitTabCount: __vaultrFitTabCount };
  }
