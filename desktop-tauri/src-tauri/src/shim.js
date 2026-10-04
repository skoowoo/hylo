// Injected at document start, before the Go page's own scripts.
// Go keeps calling window.hyloDesktop; this shim is the whole contract.

(function () {
  var platform = window.__HYLO_PLATFORM__ || "linux";

  function invoke(cmd, args) {
    var internals = window.__TAURI_INTERNALS__;
    if (internals && internals.invoke) return internals.invoke(cmd, args || {});
    var core = window.__TAURI__ && window.__TAURI__.core;
    if (core && core.invoke) return core.invoke(cmd, args || {});
    return Promise.reject(new Error("tauri ipc unavailable"));
  }

  window.hyloDesktop = {
    platform: platform,
    shell: "tauri",
    checkServer: function (url) {
      return invoke("check_server", { url: url });
    },
    startHyloServerDetached: function (opts) {
      return invoke("start_hylo_server_detached", { opts: opts || {} });
    },
    stopServer: function () {
      return invoke("stop_hylo_server");
    },
    getServerProcessStatus: function () {
      return invoke("get_server_process_status");
    },
    getShellDebugPaths: function () {
      return invoke("get_shell_debug_paths");
    },
    getServerUrl: function () {
      return invoke("get_server_url");
    },
    setServerUrl: function (url) {
      return invoke("set_server_url", { url: url });
    },
    restartServer: function () {
      return invoke("restart_hylo_server");
    },
    setViewBgColor: function (color, theme) {
      return invoke("set_view_bg_color", { color: color, theme: theme || "" });
    },
    setWindowButtonVisibility: function (visible) {
      return invoke("set_window_button_visibility", { visible: !!visible });
    },
    pickFolder: function (opts) {
      return invoke("pick_folder", { opts: opts || {} });
    },
    syncVaultDataAcrossSections: scheduleSync,
    inboxNotify: {
      getSettings: function () {
        return invoke("inbox_notify_get_settings");
      },
      setSettings: function (settings) {
        return invoke("inbox_notify_set_settings", { settings: settings || {} });
      },
      previewSound: function (sound) {
        return invoke("inbox_notify_preview_sound", { sound: sound || "" });
      },
    },
  };

  // One webview today (SECTIONS is only "home"). Debounce matches the Electron shell.
  var syncTimer = null;
  var syncWaiters = [];
  function scheduleSync() {
    return new Promise(function (resolve) {
      syncWaiters.push(resolve);
      clearTimeout(syncTimer);
      syncTimer = setTimeout(function () {
        var waiters = syncWaiters;
        syncWaiters = [];
        var result = { reloaded: [] };
        try {
          if (window.__hyloShellSafeForBackgroundReload && window.__hyloShellSafeForBackgroundReload()) {
            if (typeof window.__hyloBackgroundRefresh === "function") {
              window.__hyloBackgroundRefresh();
              result = { reloaded: ["home"] };
            } else {
              result = { reloaded: ["home"] };
              location.reload();
            }
          }
        } catch (e) {}
        waiters.forEach(function (fn) { fn(result); });
      }, 500);
    });
  }

  // -webkit-app-region is Chromium-only. Tag the same chrome the Go CSS marks as drag.
  function blocked(el) {
    return !!el.closest(
      "button, a, input, textarea, select, label, .content-pane-tabstrip, .content-pane-tabs-menu, .content-pane-bot-menu, .content-pane-more-menu, .settings-modal-close"
    );
  }
  function clearDrag(root) {
    root.removeAttribute("data-tauri-drag-region");
    var tagged = root.querySelectorAll("[data-tauri-drag-region]");
    for (var i = 0; i < tagged.length; i++) tagged[i].removeAttribute("data-tauri-drag-region");
  }
  function mark(root) {
    if (blocked(root)) root.removeAttribute("data-tauri-drag-region");
    else root.setAttribute("data-tauri-drag-region", "");
    var nodes = root.querySelectorAll("*");
    for (var i = 0; i < nodes.length; i++) {
      if (blocked(nodes[i])) nodes[i].removeAttribute("data-tauri-drag-region");
      else nodes[i].setAttribute("data-tauri-drag-region", "");
    }
  }
  function applyDragRegions() {
    if (platform !== "darwin") return;
    var roots = document.querySelectorAll(".home-side-top, .home-list-head, .content-pane-tool-bar");
    for (var i = 0; i < roots.length; i++) {
      var root = roots[i];
      // Settings reuses .home-list-head as the drag strip. The close button
      // is a sibling, already excluded by blocked().
      if (root.classList.contains("content-pane-tool-bar") && root.closest(".content-pane-is-opening")) {
        clearDrag(root);
        continue;
      }
      mark(root);
    }
  }

  // Tauri's drag.js maximizes through NSWindow zoom:, whose animation
  // WKWebView doesn't repaint during. Take the double-click before it does.
  function onDragRegion(e) {
    return e.target instanceof HTMLElement && e.target.hasAttribute("data-tauri-drag-region");
  }
  var dblX = 0;
  var dblY = 0;
  if (platform === "darwin") {
    window.addEventListener("mousedown", function (e) {
      if (e.button !== 0 || e.detail !== 2 || !onDragRegion(e)) return;
      dblX = e.clientX;
      dblY = e.clientY;
    }, true);
    window.addEventListener("mouseup", function (e) {
      if (e.button !== 0 || e.detail !== 2 || !onDragRegion(e)) return;
      e.stopImmediatePropagation();
      if (e.clientX === dblX && e.clientY === dblY) invoke("toggle_maximize");
    }, true);
  }

  var scheduled = false;
  function scheduleDrag() {
    if (scheduled) return;
    scheduled = true;
    requestAnimationFrame(function () {
      scheduled = false;
      applyDragRegions();
    });
  }

  function bootDrag() {
    applyDragRegions();
    if (!document.documentElement) return;
    new MutationObserver(scheduleDrag).observe(document.documentElement, {
      subtree: true,
      childList: true,
      attributes: true,
      attributeFilter: ["class"],
    });
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", bootDrag);
  } else {
    bootDrag();
  }
})();
