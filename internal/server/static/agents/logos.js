// Agent logos, shared by the web's bot setup and the desktop onboarding
// (sync-ui copies this folder into the shell). Each page passes where the
// folder is served and its own glyph and light/dark class names.
(function () {
  // Monochrome marks are masks, so they take the text color on either theme.
  var LOGOS = {
    pi: { mask: 'pi.svg' },
    claude: { img: 'claude.svg' },
    codex: { full: 'codex.png' },
    opencode: { full: 'opencode.svg' },
    'cursor-agent': { dark: 'cursor-dark.svg', light: 'cursor-light.svg' },
    copilot: { mask: 'copilot.svg' },
  };
  window.__hyloAgentLogo = function (id, base, cls) {
    var l = LOGOS[id];
    if (!l) return '';
    if (l.mask) return '<span class="' + cls.glyph + '" style="--src: url(' + base + l.mask + ')"></span>';
    if (l.img) return '<img src="' + base + l.img + '" alt="">';
    if (l.full) return '<img class="is-full" src="' + base + l.full + '" alt="">';
    return '<img class="is-full ' + cls.dark + '" src="' + base + l.dark + '" alt="">' +
      '<img class="is-full ' + cls.light + '" src="' + base + l.light + '" alt="">';
  };
})();
