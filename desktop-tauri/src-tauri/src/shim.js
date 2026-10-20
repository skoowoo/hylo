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
    stopServer: function (opts) {
      return invoke("stop_hylo_server", { opts: opts || {} });
    },
    getServerProcessStatus: function () {
      return invoke("get_server_process_status");
    },
    getShellDebugPaths: function () {
      return invoke("get_shell_debug_paths");
    },
    // Server registry (see servers.rs). Keys stay in the shell's config file; the page
    // only ever sees whether one is stored. Remote pages may list, probe, switch and open the
    // manager; add/update/remove are only granted to the shell's own pages (capabilities).
    servers: {
      list: function () {
        return invoke("list_servers");
      },
      add: function (opts) {
        return invoke("add_server", { opts: opts });
      },
      update: function (id, opts) {
        return invoke("update_server", { id: id, opts: opts || {} });
      },
      remove: function (id) {
        return invoke("remove_server", { id: id });
      },
      // Resolves once the server can be entered (the local one is started and
      // waited for); rejects with { code, message }.
      check: function (id, opts) {
        return invoke("check_server", { id: id, opts: opts || {} });
      },
      // From a server's page: the shell page takes over and enters it.
      switchTo: function (id) {
        return invoke("switch_server", { id: id });
      },
      // The shell page's own last step in, after check().
      enter: function (id) {
        return invoke("enter_server", { id: id });
      },
      probe: function () {
        return invoke("probe_servers");
      },
      openManager: function () {
        return invoke("open_manager");
      },
    },
    onboarding: {
      get: function () {
        return invoke("onboarding_get");
      },
      setStep: function (step) {
        return invoke("onboarding_set_step", { step: step || "" });
      },
      finish: function (landing) {
        return invoke("onboarding_finish", { landing: landing || null });
      },
      vaults: function () {
        return invoke("onboarding_vaults");
      },
      setVault: function (path, pauseGitSync) {
        return invoke("onboarding_set_vault", { path: path, pauseGitSync: !!pauseGitSync });
      },
      pathNeeded: function () {
        return invoke("onboarding_path_needed");
      },
      addToPath: function () {
        return invoke("onboarding_add_to_path");
      },
      ready: function () {
        return invoke("onboarding_ready");
      },
      agents: function () {
        return invoke("onboarding_agents");
      },
      hello: function (agentId, token, context) {
        return invoke("onboarding_hello", { agentId: agentId, token: token, context: context || null });
      },
      createBot: function (agentId, greeting) {
        return invoke("onboarding_create_bot", { agentId: agentId, greeting: greeting || null });
      },
    },
    restartServer: function (opts) {
      return invoke("restart_hylo_server", { opts: opts || {} });
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

  // -webkit-app-region is Chromium-only. Tag the same chrome the Go CSS marks as drag.
  function blocked(el) {
    return !!el.closest(
      "button, a, input, textarea, select, label, .content-pane-tab, .content-pane-tabs-menu, .content-pane-bot-menu, .content-pane-more-menu, .settings-modal-close"
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
