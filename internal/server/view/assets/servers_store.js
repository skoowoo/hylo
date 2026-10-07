// Server registry for the desktop app. The list, keys and switching live in the
// shell (window.hyloDesktop.servers); this store only mirrors them for the UI.
function hyloServersStore() {
  // The shell rejects with { code, message }; the code drives the wording.
  function parseError(e) {
    if (e && typeof e === 'object' && e.code) return { code: e.code, message: e.message || '' };
    return { code: 'error', message: String((e && e.message) || e || '') };
  }
  function api() { return window.hyloDesktop && window.hyloDesktop.servers; }

  return {
    available: !!api(),
    items: [],
    activeId: '',
    health: {},

    active() { return this.items.find(function (s) { return s.id === this.activeId; }, this) || null; },
    host(url) { try { return new URL(url).host; } catch (_) { return url; } },
    // The page itself is served by the active server, so it is online until probed otherwise.
    state(id) { return this.health[id] || (id === this.activeId ? 'online' : 'checking'); },

    async load() {
      if (!api()) return false;
      try {
        var r = await api().list();
        this.items = r.servers;
        this.activeId = r.activeId;
        return true;
      } catch (_) { return false; }
    },

    // Reload the list and re-check every server's reachability.
    async refresh() {
      if (await this.load()) this.probe();
    },

    async probe() {
      try {
        var out = await api().probe();
        var next = {};
        out.forEach(function (p) { next[p.id] = p.health; });
        this.health = next;
      } catch (_) { /* leave the previous states */ }
    },

    // The shell reloads the window on the target server, so resolve only on failure.
    async switchTo(id) {
      if (id === this.activeId) return null;
      try { await api().switchTo(id); return null; } catch (e) { return parseError(e); }
    },

    // The registry is edited in the shell's own manager page, not from a server's page.
    openManager() { return api().openManager(); },
  };
}
