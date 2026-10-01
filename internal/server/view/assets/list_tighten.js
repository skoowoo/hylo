  // In its own file (unlike the rest of editor_session.js) because it's a
  // pure, DOM-free string transform — Node's test runner can exercise it
  // directly (list_tighten.test.js) without the browser globals the other
  // concatenated assets in this directory depend on.
  //
  // Same list only: a marker-type change (bullet -> ordered, or a different
  // bullet/delimiter char) starts a brand-new list per CommonMark even across
  // a single blank line, so that blank line is a deliberate separator, not
  // stray whitespace to tighten. Collapsing it regardless of marker family
  // silently merged two distinct lists on every note load and re-saved that
  // merge 800ms later via the normal autosave path.
  function __hyloEditorSameListMarker(a, b) {
    var aOrdered = /^\d+[.)]$/.test(a), bOrdered = /^\d+[.)]$/.test(b);
    if (aOrdered !== bOrdered) return false;
    return aOrdered ? a[a.length - 1] === b[b.length - 1] : a === b;
  }
  function __hyloEditorTightenLists(md) {
    return md.replace(
      /^([ \t]*)([-*+]|\d+[.)])( [^\n]*)\n\n(?=([ \t]*)([-*+]|\d+[.)]) )/gm,
      function(match, indent, marker, rest, nextIndent, nextMarker) {
        if (!__hyloEditorSameListMarker(marker, nextMarker)) return match;
        return indent + marker + rest + '\n';
      }
    );
  }
  // Browser: concatenated into the shared inline <script> scope (home.go),
  // same as every other file here — this export is a no-op there since
  // `module` doesn't exist. Node: list_tighten.test.js requires this file
  // directly.
  if (typeof module !== 'undefined') {
    module.exports = { __hyloEditorTightenLists, __hyloEditorSameListMarker };
  }
