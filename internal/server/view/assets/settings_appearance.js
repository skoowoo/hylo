  function settingsAppearanceTab() {
    return {
      // Read live by the editor's remark pipeline on every parse (breaks.js),
      // not threaded through Editor.make() config — so this takes effect on
      // the next parse without needing the editor to be recreated.
      lineBreaksPref: localStorage.getItem('hylo-line-breaks') || 'loose',
      setLineBreaks(key) { this.lineBreaksPref = key; localStorage.setItem('hylo-line-breaks', key); },

      // themePref mirrors what the theme script in head.html already resolved at
      // first paint (light/dark/auto) — switching here just persists the
      // new choice and re-runs that same resolution logic immediately via
      // window.__hyloApplyTheme (defined once in <head>, shared by both).
      themePref: localStorage.getItem('hylo-theme') || 'auto',
      setTheme(key) {
        this.themePref = key;
        localStorage.setItem('hylo-theme', key);
        if (window.__hyloApplyTheme) window.__hyloApplyTheme(key);
      },

      // Presets come from the accent script in head.html, which also
      // owns applying them; this just persists the pick and re-runs it.
      accentPresets: window.__hyloAccentPresets || [],
      accentPref: (function() {
        var id = localStorage.getItem('hylo-accent');
        return (window.__hyloAccentPresets || []).some(function(p) { return p.id === id; }) ? id : 'indigo';
      })(),
      setAccent(id) {
        this.accentPref = id;
        localStorage.setItem('hylo-accent', id);
        if (window.__hyloApplyAccent) window.__hyloApplyAccent(id);
      },

    };
  }
