const { app, BaseWindow, WebContentsView, shell, ipcMain, nativeImage, nativeTheme, Menu, dialog, Notification } = require("electron");

app.setName("Vaultr");

// Single-instance guard. Without this, anything that asks macOS to
// (re)launch this exact app — a stray `open -b <bundleId>` (this is what a
// dev-mode `electron .` run resolves to: the generic "com.github.Electron"
// bundle id, not something unique per project), the user double-clicking
// the Dock icon while already running, etc. — spins up a second, fully
// independent process that fights the first one over the same server port
// and userData directory instead of just switching to the window that's
// already open.
const gotSingleInstanceLock = app.requestSingleInstanceLock();
if (!gotSingleInstanceLock) {
  app.quit();
} else {
  app.on("second-instance", () => {
    if (win) {
      if (win.isMinimized()) win.restore();
      win.show();
      win.focus();
    }
  });
}

const path = require("node:path");
const fs = require("node:fs");
const http = require("node:http");
const https = require("node:https");
const { getAutoStart, setAutoStart, setServerUrl, getInboxNotifySettings, registerConfigIpcHandlers } = require("./config");
const { installCli } = require("./cli-installer");
const { registerQuickCapture, isCapturePanelOpen } = require("./quick-capture");

const serverManager = require("./server-manager");
serverManager.register({
  onRestartDone:  () => resetToStartScreen(),
  onServerStopped: () => resetToStartScreen(),
  onBeforeStop:   () => stopInboxNotifications(),
  getAutoStart:   () => getAutoStart(),
  setAutoStart:   (v) => setAutoStart(v),
});

const SECTIONS = ["home"];

/** @returns {import("electron").NativeImage | null} */
function getAppIconImage() {
  const root = path.join(__dirname, "..");
  const png = path.join(root, "assets", "icon.png");
  const icns = path.join(root, "assets", "icon.icns");
  // Prefer PNG: Chromium often fails to decode minimal .icns (e.g. only ic07) for dock/window icons.
  const candidates = [png, icns];
  for (const p of candidates) {
    if (!fs.existsSync(p)) continue;
    try {
      const img = nativeImage.createFromPath(p);
      if (!img.isEmpty()) return img;
    } catch {
      /* ignore */
    }
  }
  return null;
}

/**
 * Path for {@link app.dock.setIcon}. Uses PNG first — same .icns decode limits as {@link getAppIconImage}.
 * @returns {string | null}
 */
function getMacDockIconPath() {
  if (process.platform !== "darwin") return null;
  const root = path.resolve(__dirname, "..", "assets");
  const png = path.join(root, "icon.png");
  const icns = path.join(root, "icon.icns");
  if (fs.existsSync(png)) return png;
  if (fs.existsSync(icns)) return icns;
  return null;
}

let win;       // BaseWindow — no built-in webContents, avoids event interception
let startView; // WebContentsView for the connection screen
let serverUrl = null;
const views = {};
let activeSection = null;
// Track the most-recently applied theme background so detached views can be
// pre-synced before they are made visible (prevents flash-of-wrong-theme).
// On macOS this stays fully transparent instead of tracking the theme colour:
// the window itself gets a native `vibrancy` fill (see createWindow), and the
// page's own CSS (html.macos rules in home.css) paints every surface that
// should stay solid — the sidebar is the one deliberate gap that lets the
// vibrancy through. A view-level background colour here would just paint
// over that gap.
let currentViewBgColor = process.platform === 'darwin' ? '#00000000' : '#18191e';

// ── Inbox notifications (direct SSE from main process) ─────────────────────────
// Fires once per new /api/inbox message, regardless of producer (agent bot
// trigger runs today, other sources later) — see internal/inbox.Store.Notifications().

/** @type {import("node:http").ClientRequest | null} */
let inboxNotifReq = null;
let inboxNotifReconnectTimer = null;
let inboxNotifWatchdogTimer = null;
const INBOX_NOTIF_WATCHDOG_MS = 45_000;

function stopInboxNotifications() {
  if (inboxNotifReconnectTimer) {
    clearTimeout(inboxNotifReconnectTimer);
    inboxNotifReconnectTimer = null;
  }
  if (inboxNotifWatchdogTimer) {
    clearTimeout(inboxNotifWatchdogTimer);
    inboxNotifWatchdogTimer = null;
  }
  if (inboxNotifReq) {
    try { inboxNotifReq.destroy(); } catch (_) {}
    inboxNotifReq = null;
  }
}

function scheduleInboxNotifReconnect(url) {
  inboxNotifReconnectTimer = setTimeout(() => {
    inboxNotifReconnectTimer = null;
    if (serverUrl === url) startInboxNotifications(url);
  }, 5000);
}

function resetInboxNotifWatchdog(url) {
  if (inboxNotifWatchdogTimer) clearTimeout(inboxNotifWatchdogTimer);
  inboxNotifWatchdogTimer = setTimeout(() => {
    inboxNotifWatchdogTimer = null;
    if (serverUrl === url) startInboxNotifications(url);
  }, INBOX_NOTIF_WATCHDOG_MS);
}

/**
 * Play a sound by name or path.
 * Accepts: "beep" (system alert), "none"/falsy (silence),
 *          macOS system sound name (e.g. "Glass", "Ping"),
 *          or an absolute file path to a .aiff/.wav/.mp3.
 */
function playSound(sound) {
  if (!sound || sound === "none") return;
  if (sound === "beep") { shell.beep(); return; }
  if (process.platform === "darwin") {
    const { spawn } = require("node:child_process");
    const soundPath = sound.startsWith("/")
      ? sound
      : `/System/Library/Sounds/${sound}.aiff`;
    const child = spawn("/usr/bin/afplay", [soundPath], { stdio: "ignore" });
    child.on("error", () => shell.beep()); // fall back if afplay is unavailable
    child.unref();
  } else {
    shell.beep();
  }
}

function showInboxNotification(message) {
  const settings = getInboxNotifySettings();

  if (settings.textEnabled && Notification.isSupported()) {
    const title = message.title || message.source || "Inbox";
    const body = (message.body || "").trim().slice(0, 150);
    new Notification({ title, body, silent: true }).show();
  }

  if (settings.soundEnabled) {
    playSound(settings.sound);
  }
}

function startInboxNotifications(url) {
  stopInboxNotifications();
  if (!url) return;

  let parsed;
  try { parsed = new URL("/api/inbox/notifications", url); } catch { return; }
  const mod = parsed.protocol === "https:" ? https : http;

  let buffer = "";
  const req = mod.request(parsed, (res) => {
    if (res.statusCode !== 200) {
      res.resume();
      scheduleInboxNotifReconnect(url);
      return;
    }
    resetInboxNotifWatchdog(url);
    res.on("data", (chunk) => {
      resetInboxNotifWatchdog(url);
      buffer += chunk.toString();
      const blocks = buffer.split("\n\n");
      buffer = blocks.pop(); // keep the incomplete trailing fragment
      for (const block of blocks) {
        if (!block.trim()) continue;
        const dataMatch = block.match(/^data: (.+)$/m);
        if (!dataMatch) continue;
        try {
          showInboxNotification(JSON.parse(dataMatch[1].trim()));
        } catch { /* ignore malformed events */ }
      }
    });
    res.on("end",   () => scheduleInboxNotifReconnect(url));
    res.on("error", () => scheduleInboxNotifReconnect(url));
  });
  req.on("error", () => scheduleInboxNotifReconnect(url));
  req.end();
  inboxNotifReq = req;
}

// ─────────────────────────────────────────────────────────────────────────────

function getContentBounds() {
  const [w, h] = win.getContentSize();
  return { x: 0, y: 0, width: w, height: h };
}

function resizeViews() {
  const bounds = getContentBounds();
  if (activeSection && views[activeSection]) {
    views[activeSection].setBounds(bounds);
  } else if (startView) {
    startView.setBounds(bounds);
  }
}

function makeWebPrefs() {
  return {
    preload: path.join(__dirname, "preload.js"),
    contextIsolation: true,
    nodeIntegration: false,
    sandbox: true,
  };
}

// opts: { fullUrl?: string, reload?: boolean }
function showSection(name, opts = {}) {
  const { fullUrl = null, reload = false } = opts;
  if (!views[name]) return;

  const view = views[name];

  // Reveals the view and gives it focus. Called either immediately (no navigation
  // needed) or after a pending loadURL/reload finishes, so the previous view stays
  // on screen until the new content is actually ready — no flash of the target
  // view's stale previous page (e.g. Inbox's nav highlight before Agent Chat repaints).
  function reveal() {
    if (activeSection && activeSection !== name && views[activeSection]) {
      win.contentView.removeChildView(views[activeSection]);
    }
    if (activeSection !== name) {
      activeSection = name;
      // Pre-sync the native background colour before the view is composited so
      // the OS-level layer never flashes the wrong colour on first paint.
      try { view.setBackgroundColor(currentViewBgColor); } catch (_) {}
      win.contentView.addChildView(view);
      view.setBounds(getContentBounds());
    }
    view.webContents.focus();
  }

  if (reload) {
    const onDone = () => {
      view.webContents.removeListener("did-finish-load", onDone);
      view.webContents.removeListener("did-fail-load", onDone);
      reveal();
    };
    view.webContents.once("did-finish-load", onDone);
    view.webContents.once("did-fail-load", onDone);
    view.webContents.reload();
    return;
  }

  if (fullUrl) {
    // Compare against the view's actual current URL, not an assumed section base —
    // a section's view can be sitting on a sub-page (home has its own client-side
    // sidebar navigation, so its URL rarely changes, but the check still matters
    // generically) rather than its nominal root, so the base URL alone can't tell
    // us whether a reload is needed.
    const currentUrl = view.webContents.getURL();
    if (fullUrl !== currentUrl) {
      const onDone = () => {
        view.webContents.removeListener("did-finish-load", onDone);
        view.webContents.removeListener("did-fail-load", onDone);
        reveal();
      };
      view.webContents.once("did-finish-load", onDone);
      view.webContents.once("did-fail-load", onDone);
      view.webContents.loadURL(fullUrl);
      return;
    }
  }

  reveal();
}

// Show the connection screen, discarding any active section views.
function resetToStartScreen() {
  if (!serverUrl) return; // already reset, guard against concurrent did-fail-load calls
  serverUrl = null;
  agentCache = null;
  stopInboxNotifications();

  if (activeSection && views[activeSection]) {
    try { win.contentView.removeChildView(views[activeSection]); } catch { }
  }
  activeSection = null;
  for (const section of SECTIONS) {
    if (views[section]) {
      try { views[section].webContents.destroy(); } catch { }
      delete views[section];
    }
  }

  showStartScreen();
}

function showStartScreen() {
  startView = new WebContentsView({ webPreferences: makeWebPrefs(), backgroundColor: currentViewBgColor });

  startView.webContents.setWindowOpenHandler(({ url }) => {
    shell.openExternal(url);
    return { action: "deny" };
  });

  // Reload start.html if the file itself somehow fails (defensive)
  startView.webContents.on("did-fail-load", (_event, errorCode, _desc, validatedURL) => {
    if (validatedURL && !validatedURL.startsWith("file://") && errorCode < 0) {
      startView.webContents.loadFile(path.join(__dirname, "start.html"));
    }
  });

  win.contentView.addChildView(startView);
  startView.setBounds(getContentBounds());
  startView.webContents.loadFile(path.join(__dirname, "start.html"));
}

function setupSectionView(view, sectionName) {
  view.webContents.setWindowOpenHandler(({ url }) => {
    // Cmd+Click on an internal link — navigate within the app instead of opening the browser.
    if (serverUrl && url.startsWith(serverUrl)) {
      try {
        const parsed = new URL(url);
        const firstSeg = parsed.pathname.split("/").filter(Boolean)[0];
        if (SECTIONS.includes(firstSeg)) {
          setImmediate(() => showSection(firstSeg, { fullUrl: url }));
          return { action: "deny" };
        }
      } catch { /* ignore malformed URLs */ }
      setImmediate(() => view.webContents.loadURL(url));
      return { action: "deny" };
    }
    shell.openExternal(url);
    return { action: "deny" };
  });

  // When the server stops, the active view will fail to load on any navigation.
  // ERR_ABORTED (-3) is a normal cancellation (e.g. redirect); ignore it.
  view.webContents.on("did-fail-load", (_event, errorCode, _desc, validatedURL) => {
    if (validatedURL && !validatedURL.startsWith("file://") && errorCode < 0 && errorCode !== -3) {
      resetToStartScreen();
    }
  });

  view.webContents.on("will-navigate", (event, url) => {
    try {
      const parsed = new URL(url);
      const firstSeg = parsed.pathname.split("/").filter(Boolean)[0];
      if (SECTIONS.includes(firstSeg) && firstSeg !== sectionName) {
        event.preventDefault();
        showSection(firstSeg, { fullUrl: url });
      }
    } catch { }
  });
}

function createSectionViews(url) {
  serverUrl = url;
  for (const section of SECTIONS) {
    const view = new WebContentsView({ webPreferences: makeWebPrefs(), backgroundColor: currentViewBgColor });
    view.webContents.loadURL(url + "/" + section);
    setupSectionView(view, section);
    views[section] = view;
  }
  // Remove start screen, show home
  if (startView) {
    win.contentView.removeChildView(startView);
    try { startView.webContents.destroy(); } catch { }
    startView = null;
  }
  showSection("home");
}

function createWindow() {
  const icon = getAppIconImage();
  const isMac = process.platform === "darwin";
  win = new BaseWindow({
    width: 1440,
    height: 960,
    minWidth: 960,
    minHeight: 640,
    title: "Vaultr",
    // On macOS the window itself supplies a native vibrant (blurred,
    // see-through-to-other-windows) fill instead of a flat colour; home.css's
    // html.macos rules leave the sidebar transparent so this actually shows
    // through there, while every other surface keeps painting its own solid
    // background on top. Windows/Linux keep today's flat colour.
    backgroundColor: isMac ? undefined : "#18191e",
    titleBarStyle: isMac ? "hiddenInset" : "default",
    ...(isMac ? { vibrancy: "sidebar", visualEffectState: "active" } : {}),
    ...(icon ? { icon } : {}),
  });

  win.on("resize", resizeViews);

  // Reset module-level state when the window is closed so that re-opening
  // (via macOS dock activate) starts with a clean slate.
  win.on("closed", () => {
    activeSection = null;
    serverUrl = null;
    if (startView) {
      try { startView.webContents.destroy(); } catch { }
      startView = null;
    }
    for (const section of SECTIONS) {
      if (views[section]) {
        try { views[section].webContents.destroy(); } catch { }
        delete views[section];
      }
    }
    win = null;
  });

  showStartScreen();
}

// ── IPC: server connection ────────────────────────────────────────────────────

registerConfigIpcHandlers(ipcMain);

ipcMain.handle("set-server-url", (_event, url) => {
  agentCache = null;
  setServerUrl(url);
  startInboxNotifications(url);
  if (win) {
    if (Object.keys(views).length === 0) {
      // First successful connection — create all section views
      createSectionViews(url);
    } else {
      // Server URL changed — reload all views with the new base URL
      serverUrl = url;
      for (const section of SECTIONS) {
        views[section].webContents.loadURL(url + "/" + section);
      }
      showSection("home");
    }
  }
});

/** @returns {Promise<string[]>} section names reloaded */
async function syncVaultDataAcrossSectionViews() {
  const reloaded = [];
  if (!win || win.isDestroyed() || !serverUrl) return reloaded;
  for (const section of SECTIONS) {
    const view = views[section];
    if (!view) continue;
    const wc = view.webContents;
    if (wc.isDestroyed()) continue;
    try {
      // Returns 'reload' | 'custom' | 'skip'.
      // If the page defines __vaultrBackgroundRefresh, call it instead of wc.reload().
      const action = await wc.executeJavaScript(`(function(){
        if(!window.__vaultrShellSafeForBackgroundReload||!window.__vaultrShellSafeForBackgroundReload())return'skip';
        if(typeof window.__vaultrBackgroundRefresh==='function'){window.__vaultrBackgroundRefresh();return'custom';}
        return'reload';
      })()`, true);
      if (action === 'reload') { wc.reload(); reloaded.push(section); }
      else if (action === 'custom') { reloaded.push(section); }
    } catch {
      /* blank, unloading, or JS error */
    }
  }
  return reloaded;
}

const SYNC_VAULT_DEBOUNCE_MS = 500;
let syncVaultDebounceTimer = null;
/** @type {Promise<{ reloaded: string[] }> | null} */
let syncVaultDebouncePromise = null;
/** @type {((value: { reloaded: string[] }) => void) | null} */
let syncVaultDebounceResolve = null;

/**
 * Global debounce for vault-driven section reloads: IPC, app foreground, content
 * pane close, etc. share one window — only the last trigger within SYNC_VAULT_DEBOUNCE_MS runs.
 * Concurrent ipcRenderer.invoke callers share the same Promise until flush.
 */
function scheduleSyncVaultDataAcrossSectionViews() {
  if (!win || win.isDestroyed() || !serverUrl) {
    return Promise.resolve({ reloaded: [] });
  }
  if (!syncVaultDebouncePromise) {
    syncVaultDebouncePromise = new Promise((resolve) => {
      syncVaultDebounceResolve = resolve;
    });
  }
  if (syncVaultDebounceTimer) clearTimeout(syncVaultDebounceTimer);
  syncVaultDebounceTimer = setTimeout(async () => {
    syncVaultDebounceTimer = null;
    const resolve = syncVaultDebounceResolve;
    syncVaultDebouncePromise = null;
    syncVaultDebounceResolve = null;
    try {
      const reloaded = await syncVaultDataAcrossSectionViews();
      resolve({ reloaded });
    } catch {
      resolve({ reloaded: [] });
    }
  }, SYNC_VAULT_DEBOUNCE_MS);
  return syncVaultDebouncePromise;
}

// Reload each section WebContentsView whose page reports it is safe to full-reload
// (see window.__vaultrShellSafeForBackgroundReload in server HTML).
ipcMain.handle("sync-vault-data-across-sections", () => {
  return scheduleSyncVaultDataAcrossSectionViews();
});

// Update the calling WebContentsView's background color when the renderer theme changes.
// This prevents a flash of the wrong background color during wc.reload() from main process.
ipcMain.on("set-window-button-visibility", (_event, visible) => {
  try { if (win) win.setWindowButtonVisibility(!!visible); } catch (_) {}
});

ipcMain.on("set-view-bg-color", (event, color, theme) => {
  // Always track the latest theme bg so showSection() can pre-sync detached views.
  // macOS keeps every view fully transparent regardless of theme — see the
  // currentViewBgColor declaration above for why.
  currentViewBgColor = process.platform === 'darwin' ? '#00000000' : color;

  // The sidebar's native vibrancy (createWindow) is an NSVisualEffectView,
  // which tints itself from the OS-level appearance — not from this app's
  // own light/dark toggle. Left alone, a user who picks "light" in-app but
  // runs macOS in Dark Mode gets a dark, blown-out sidebar next to a light
  // content pane. Forcing nativeTheme.themeSource to match keeps them in
  // sync; 'auto' (theme === '') falls back to whatever the OS is doing.
  if (process.platform === 'darwin') {
    nativeTheme.themeSource = theme === 'light' ? 'light' : theme === 'dark' ? 'dark' : 'system';
  }

  // Update every view's native background colour immediately — including detached
  // ones — so the OS layer never shows a stale colour when a section is revealed.
  const allViews = [...Object.values(views), startView].filter(Boolean);
  for (const v of allViews) {
    if (!v.webContents.isDestroyed()) {
      try { v.setBackgroundColor(currentViewBgColor); } catch (_) {}
    }
  }

  // Detached WebContentsViews may not receive localStorage storage events or
  // matchMedia change events from Chromium while they are off-screen.  Push a
  // lightweight data-theme sync into every view that didn't originate this call
  // so their HTML is already correct when they are next made visible.
  //
  // `theme` is exactly the data-theme value shared_theme.go's
  // window.__vaultrApplyTheme just set on the originating view: 'light',
  // 'dark', or '' for auto (attribute removed, bare :root default +
  // prefers-color-scheme fallback takes over). Passed through as-is rather
  // than re-derived from `color`, since color alone can't distinguish an
  // explicit "dark" choice from "auto" resolving to dark.
  const senderWc = event.sender;
  for (const v of Object.values(views)) {
    if (v.webContents.isDestroyed() || v.webContents === senderWc) continue;
    v.webContents.executeJavaScript(`(function(){
      try{
        if (${JSON.stringify(theme || "")}) { document.documentElement.setAttribute('data-theme', ${JSON.stringify(theme || "")}); }
        else { document.documentElement.removeAttribute('data-theme'); }
      }catch(_){}
    })()`).catch(() => {});
  }
});

ipcMain.handle("inbox-notify:preview-sound", (_e, sound) => {
  playSound(sound);
});

ipcMain.handle("pick-folder", async (_event, opts = {}) => {
  const result = await dialog.showOpenDialog(win, {
    title: opts.title || "Select Folder",
    properties: ["openDirectory", "createDirectory"],
    defaultPath: opts.defaultPath || app.getPath("home"),
  });
  if (result.canceled || !result.filePaths.length) return { canceled: true };
  return { canceled: false, path: result.filePaths[0] };
});

// ── App lifecycle ─────────────────────────────────────────────────────────────

// macOS: BaseWindow has no default application menu, so Cmd+Z/X/C/V/A don't
// reach contenteditable elements unless an Edit menu is explicitly registered.
// role:"undo" click works because Electron calls webContents.undo() explicitly,
// but the keyboard accelerator goes through the native macOS undo: responder chain
// which doesn't reach WebContentsView in a BaseWindow. So we use an explicit
// click handler for undo/redo so both click AND keyboard call webContents.undo().
function getActiveWebContents() {
  if (activeSection && views[activeSection] && !views[activeSection].webContents.isDestroyed()) {
    return views[activeSection].webContents;
  }
  return null;
}

function buildAppMenu() {
  const template = [
    ...(process.platform === "darwin" ? [{ role: "appMenu" }] : []),
    {
      label: "Edit",
      submenu: [
        {
          label: "Undo",
          accelerator: "CmdOrCtrl+Z",
          // registerAccelerator:false → Cmd+Z is NOT intercepted by the OS menu;
          // it reaches the renderer as a keydown event so ProseMirror / CodeMirror
          // can handle it via their own Mod-z keymap bindings.
          // The click handler covers the mouse-click path via executeJavaScript.
          registerAccelerator: false,
          click: () => {
            const wc = getActiveWebContents();
            if (wc) wc.executeJavaScript("window.__vaultrUndo && window.__vaultrUndo()").catch(() => { });
          },
        },
        {
          label: "Redo",
          accelerator: "Shift+CmdOrCtrl+Z",
          registerAccelerator: false,
          click: () => {
            const wc = getActiveWebContents();
            if (wc) wc.executeJavaScript("window.__vaultrRedo && window.__vaultrRedo()").catch(() => { });
          },
        },
        { type: "separator" },
        { role: "cut" },
        { role: "copy" },
        { role: "paste" },
        { role: "pasteAndMatchStyle" },
        { role: "delete" },
        { role: "selectAll" },
      ],
    },
    { label: "Window", role: "windowMenu" },
  ];
  Menu.setApplicationMenu(Menu.buildFromTemplate(template));
}

// Must be called before app is ready
app.commandLine.appendSwitch("enable-gpu-rasterization");
app.commandLine.appendSwitch("enable-zero-copy");

app.whenReady().then(() => {
  buildAppMenu();
  const dockIconPath = getMacDockIconPath();
  if (app.dock && dockIconPath) {
    try {
      app.dock.setIcon(dockIconPath);
    } catch (e) {
      console.error("[vaultr-shell] app.dock.setIcon failed:", e);
    }
  }

  createWindow();
  registerQuickCapture(ipcMain, { getMainWindow: () => win, getServerUrl: () => serverUrl });

  // Install bundled CLI to system PATH in the background — does not block window startup.
  // On a real upgrade (not skipped), kill the old running server so the start screen's
  // auto-restart loop picks up the new binary.
  installCli((msg) => console.error("[vaultr-shell]", msg))
    .then(async (r) => {
      if (!r.ok) { console.error("[vaultr-shell] cli auto-install failed:", r.error); return; }
      if (!r.skipped) await serverManager.restartServerAfterCliUpdate();
    })
    .catch((e) => console.error("[vaultr-shell] cli auto-install threw:", e));

  app.on("activate", () => {
    if (!win || win.isDestroyed()) {
      createWindow();
    } else if (!win.isVisible()) {
      // win.hide() (quick-capture.js re-hides the main window after a
      // screenshot taken while ⌘H-hidden) is a per-window orderOut:, not
      // the same thing as ⌘H's app-level hide — the OS only auto-restores
      // the latter on a Dock-icon click, so a plain per-window hide needs
      // this explicit show() or there's no way back in at all.
      win.show();
    }
  });

  // 'activate' only fires for a Dock-icon click / relaunch. Switching to
  // Vaultr via ⌘-Tab (or any other way of activating the app) doesn't go
  // through that at all — it needs this separate, broader event instead.
  // Guarded on !isCapturePanelOpen(): the capture panel intentionally hides
  // this window while it's open (see quick-capture.js), and an activation
  // blip caused by the panel itself is still a possibility even after the
  // webContents.focus()/mousedown fixes there — this stops that from
  // undoing the hide mid-capture.
  app.on("did-become-active", () => {
    if (win && !win.isDestroyed() && !win.isVisible() && !isCapturePanelOpen()) {
      win.show();
    }
  });
});

app.on("window-all-closed", () => {
  if (process.platform !== "darwin") {
    app.quit();
  }
});
