package view

// keysJS is the foundational keyboard runtime injected first on every page.
// It sets up two capture-phase listeners so both run before any element
// handler and fire regardless of which element currently has focus.
//
// Global hotkeys — window.__hyloHotkeys
//   Features register shortcuts via .register(id, key, fn) and remove them
//   with .unregister(id). Last-registered wins when two entries share a key.
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
    window.__hyloHotkeys = {
      isMac: _isMac,
      register: function(id, key, fn) {
        _reg = _reg.filter(function(r) { return r.id !== id; });
        _reg.push({ id: id, key: key, fn: fn });
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
      var mod = _isMac ? e.metaKey : e.ctrlKey;
      for (var i = _rawReg.length - 1; i >= 0; i--) {
        if (_rawReg[i].fn(e, mod) === true) return;
      }
      if (!mod || e.shiftKey || e.altKey) return;
      for (var i = _reg.length - 1; i >= 0; i--) {
        if (e.key === _reg[i].key) {
          e.preventDefault();
          _reg[i].fn(e);
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
