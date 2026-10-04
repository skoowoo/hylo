package view

// keysJS is the foundational keyboard runtime injected first on every page.
// It sets up two capture-phase listeners so both run before any element
// handler and fire regardless of which element currently has focus.
//
// Global hotkeys — window.__hyloHotkeys
//   Features register shortcuts via .register(id, defaultKey, fn) and remove
//   them with .unregister(id). The id is looked up in the user's custom
//   bindings (localStorage) before falling back to Mod+defaultKey.
//   Last-registered wins when two entries share a combo.
//   Handlers own their own open/close state; this registry just dispatches.
//
// ESC stack — window.__hyloEscPush / __hyloEscPop
//   Overlays push a named closer when they open and pop it when they close.
//   Escape dismisses only the topmost entry (strict LIFO order).
//
// Overlay base — window.hyloOverlay(escId, extra)
//   Shared Alpine x-data factory for every dialog/overlay (confirm, info,
//   short note, …): bundles an `open` flag with the ESC-stack bookkeeping so
//   a dialog can't leave a stale entry on the stack by forgetting to call
//   __hyloEscPush/Pop by hand. openOverlay()/closeOverlay() always move
//   both together; extra's own methods are merged on top and may still read/
//   write `open` directly for cases (e.g. an animated close) where hiding
//   needs to lag behind the ESC-stack pop.
const keysJS = `
  (function() {
    var _isMac = /Mac|iPhone|iPad/.test(navigator.platform || navigator.userAgent);
    var _reg = [];
    var _rawReg = [];
    var _cap = null;
    var _custom = {};
    try { _custom = JSON.parse(localStorage.getItem('hylo-custom-keys') || '{}') || {}; } catch (_) {}
    var _punct = {
      Backslash: '\\', Comma: ',', Period: '.', Slash: '/', Minus: '-', Equal: '=',
      Semicolon: ';', Quote: "'", BracketLeft: '[', BracketRight: ']', Backquote: '\x60',
    };
    // Combos are platform-neutral ("Mod+Shift+K"); derived from e.code so
    // Alt/Shift can't change the key name.
    function _combo(e) {
      var c = e.code || '', key = '';
      if (/^Key[A-Z]$/.test(c)) key = c.slice(3);
      else if (/^Digit\d$/.test(c)) key = c.slice(5);
      else if (_punct[c]) key = _punct[c];
      else if (c === 'Escape') key = 'Esc';
      else if (/^(F\d+|Arrow\w+|Enter|Space|Tab|Backspace)$/.test(c)) key = c;
      if (!key) return null;
      var parts = [];
      if (_isMac ? e.metaKey : e.ctrlKey) parts.push('Mod');
      if (_isMac ? e.ctrlKey : e.metaKey) parts.push(_isMac ? 'Ctrl' : 'Meta');
      if (e.altKey) parts.push('Alt');
      if (e.shiftKey) parts.push('Shift');
      parts.push(key);
      return parts.join('+');
    }
    window.__hyloHotkeys = {
      isMac: _isMac,
      custom: _custom,
      comboOf: _combo,
      setCustom: function(id, combo) {
        if (combo) _custom[id] = combo; else delete _custom[id];
        try { localStorage.setItem('hylo-custom-keys', JSON.stringify(_custom)); } catch (_) {}
      },
      // While set, every keydown goes to cb(event, combo|null) and nothing else
      // sees it. combo is null for bare modifier presses.
      capture: function(cb) { _cap = cb; },
      // fn may return false to decline the event (it then propagates untouched).
      register: function(id, key, fn) {
        _reg = _reg.filter(function(r) { return r.id !== id; });
        _reg.push({ id: id, def: 'Mod+' + key.toUpperCase(), fn: fn });
      },
      unregister: function(id) {
        _reg = _reg.filter(function(r) { return r.id !== id; });
      },
      // Raw handlers receive (event, mod) and fire before mod+key handlers.
      // Return true to stop further hotkey processing for that keydown.
      registerRaw: function(id, fn) {
        _rawReg = _rawReg.filter(function(r) { return r.id !== id; });
        _rawReg.push({ id: id, fn: fn });
      },
      unregisterRaw: function(id) {
        _rawReg = _rawReg.filter(function(r) { return r.id !== id; });
      },
    };
    document.addEventListener('keydown', function(e) {
      if (_cap) {
        e.preventDefault();
        e.stopImmediatePropagation();
        _cap(e, _combo(e));
        return;
      }
      var mod = _isMac ? e.metaKey : e.ctrlKey;
      for (var i = _rawReg.length - 1; i >= 0; i--) {
        if (_rawReg[i].fn(e, mod) === true) return;
      }
      if (!mod) return;
      var combo = _combo(e);
      if (!combo) return;
      for (var i = _reg.length - 1; i >= 0; i--) {
        if ((_custom[_reg[i].id] || _reg[i].def) === combo) {
          if (_reg[i].fn(e) === false) return;
          e.preventDefault();
          return;
        }
      }
    }, true);
  })();

  (function() {
    var _stk = [];
    window.__hyloEscPush = function(id, fn) {
      _stk = _stk.filter(function(e) { return e.id !== id; });
      _stk.push({ id: id, close: fn });
    };
    window.__hyloEscPop = function(id) {
      _stk = _stk.filter(function(e) { return e.id !== id; });
    };
    window.__hyloAnyModalOpen = function() { return _stk.length > 0; };
    document.addEventListener('keydown', function(e) {
      if (e.key !== 'Escape' || !_stk.length) return;
      e.preventDefault();
      e.stopPropagation();
      _stk[_stk.length - 1].close();
    }, true);
  })();

  window.hyloOverlay = function(escId, extra) {
    var base = {
      open: false,
      openOverlay: function(onEscClose) {
        this.open = true;
        var self = this;
        if (window.__hyloEscPush) {
          window.__hyloEscPush(escId, onEscClose || function() { self.closeOverlay(); });
        }
      },
      closeOverlay: function() {
        this.open = false;
        if (window.__hyloEscPop) window.__hyloEscPop(escId);
      },
    };
    for (var k in extra) base[k] = extra[k];
    return base;
  };
`
