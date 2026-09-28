// Immersive screenshot -> quick-view -> agent bot flow, macOS-only for now.
//
// Deliberately its own top-level BaseWindow (a native, non-activating NSPanel
// via type:"panel") rather than a WebContentsView bolted onto the main
// window: that's what lets it float over whatever the user is doing —
// including a fullscreen browser — without ever bringing the main Vaultr
// window (or even the app itself) to the foreground. Capture -> message ->
// send should feel like Spotlight, not like switching apps.
//
// Region selection reuses macOS's own `screencapture -i`, not a hand-built
// overlay: it already handles multi-monitor, Retina scaling, space-to-toggle
// window capture, and Esc-to-cancel, for free.

const { BaseWindow, WebContentsView, screen, globalShortcut, nativeImage, app } = require("electron");
const { spawn, execFile } = require("node:child_process");
const path = require("node:path");
const os = require("node:os");
const fs = require("node:fs");
const http = require("node:http");
const https = require("node:https");
const activeWin = require("active-win");

const QUICK_CAPTURE_ACCELERATOR = "Alt+Shift+C";

const PANEL_PADDING = 20;
const HEADER_H = 34;    // label + close button row, incl. margin-bottom (index.html #header)
const IMG_GAP = 16;      // image box -> composer (index.html #img-wrap margin-bottom)
const COMPOSER_H = 76;   // composer's own fixed height (index.html #composer)
const ACTIONS_GAP = 12;  // composer -> Save/Send row (index.html #actions margin-top)
const ACTIONS_H = 32;    // Save/Send button row height (index.html .btn, == --btn-h)
const IMG_INSET = 10;    // breathing room between the image and its own box's edge

// Bounds for the image preview box — kept independent of the screenshot's own
// size/aspect ratio so a tiny icon-sized grab and a multi-monitor-wide one
// both land in a panel that still looks intentional, never a box shaped by
// whatever the user happened to select.
const MIN_BOX_W = 260, MAX_BOX_W = 640;
const MIN_BOX_H = 90, MAX_BOX_H = 420;

/**
 * Fits the screenshot into a bounded preview box without ever upscaling it.
 * `imgW`/`imgH` are what the <img> itself renders at (crisp, never blown up
 * past 1x); `boxW`/`boxH` are the (possibly larger, for the MIN_BOX_W /
 * MIN_BOX_H / IMG_INSET floor) letterboxing container around it — sized
 * IMG_INSET bigger than the image on each side so index.js's flex-centering
 * leaves visible breathing room instead of the image touching the box's edge.
 */
function computeLayout(iw, ih, workArea) {
  const maxBoxW = Math.min(MAX_BOX_W, Math.round(workArea.width * 0.6));
  const maxBoxH = Math.min(MAX_BOX_H, Math.round(workArea.height * 0.5));
  const scale = Math.min(1, (maxBoxW - IMG_INSET * 2) / iw, (maxBoxH - IMG_INSET * 2) / ih);
  const imgW = Math.max(1, Math.round(iw * scale));
  const imgH = Math.max(1, Math.round(ih * scale));
  const boxW = Math.min(maxBoxW, Math.max(MIN_BOX_W, imgW + IMG_INSET * 2));
  const boxH = Math.min(maxBoxH, Math.max(MIN_BOX_H, imgH + IMG_INSET * 2));
  return { imgW, imgH, boxW, boxH };
}

let captureInFlight = false;
/** @type {import("electron").BaseWindow | null} */
let panelWin = null;
let panelImagePath = null;
// macOS's NSWindowStyleMaskNonactivatingPanel (Electron's type:"panel") is
// meant to let a window become key — so the textarea can type — without the
// owning app itself activating. In practice a WebContentsView's own click/
// focus handling still ends up activating Vaultr (a known rough edge, not
// something toggleable from here) — so instead of fighting that, we record
// whichever app was frontmost right when the shortcut fired and explicitly
// reactivate it once the panel closes, regardless of what activated in
// between. Without this, closing the panel leaves Vaultr's main window
// showing instead of returning to the app being screenshotted.
let frontAppBeforeCapture = null;
// Whether Vaultr was ⌘H-hidden (app.isHidden()) right when the shortcut
// fired. Showing the panel unavoidably un-hides the whole app — ⌘H hides
// at the NSApplication level, not per-window, so revealing any one window
// of the app un-hides all of them, main window included. If it was hidden,
// openPanelForImage() re-hides just the main window right after, so ⌘H
// stays respected instead of getting silently undone by taking a screenshot.
let appWasHiddenBeforeCapture = false;
/** Set by registerQuickCapture() — lets this module reach the main window without importing main.js. */
let getMainWindow = () => null;
/** Set by registerQuickCapture() — same idea, for the local server's base URL (main.js's `serverUrl`). */
let getServerUrl = () => null;

function makePanelWebPrefs() {
  return {
    preload: path.join(__dirname, "quick-capture", "preload.js"),
    contextIsolation: true,
    nodeIntegration: false,
    sandbox: true,
  };
}

// active-win ships a prebuilt native helper on macOS (no AppleScript/OSA
// runtime to spin up), so it's a lower-latency read than shelling out to
// `osascript -l JavaScript` ourselves — same tool hylo (../hylo) uses for
// this exact "who's frontmost" query.
const FRONTMOST_APP_TIMEOUT_MS = 1500;

// Bundle ids that mean "that's us" — never a legitimate reactivation target.
// "dev.hardhacker.vaultr" is the packaged app id (package.json's
// build.appId); "com.github.Electron" is what an *unpackaged* `electron .`
// dev run gets instead, and it's Electron's own generic dev-mode id, not
// something unique to this project. Confirmed live on this machine: the
// running dev Vaultr process actually reports bundle id "com.github.Electron",
// and LaunchServices currently resolves that id to this exact app's own
// Electron.app. If the shortcut ever fires while Vaultr itself happens to be
// frontmost, activateApp() would later run `open -b com.github.Electron` on
// ourselves — and since there's no app.requestSingleInstanceLock() guarding
// against it, that can kick off a second instance that fights the running
// one over the same server port / userData dir, which is what made the main
// window (and with it, the Dock icon) disappear. Filtering both ids out
// here means getFrontmostAppId() can never hand activateApp() a target that
// resolves back to us.
const SELF_BUNDLE_IDS = new Set(["dev.hardhacker.vaultr", "com.github.Electron"]);

/**
 * Bundle identifier of whatever app is frontmost right now (e.g. the browser
 * the user was screenshotting), so the panel can hand focus back to it
 * explicitly on close — see the note on `frontAppBeforeCapture` below for why.
 * Returns null if that's Vaultr itself (see SELF_BUNDLE_IDS) — nothing to
 * restore focus to in that case, and reactivating "ourselves" is exactly
 * the bug this guards against.
 */
async function getFrontmostAppId() {
  try {
    const timeout = new Promise((_, reject) => {
      setTimeout(() => reject(new Error("active-win timeout")), FRONTMOST_APP_TIMEOUT_MS);
    });
    const win = await Promise.race([
      activeWin({ screenRecordingPermission: false, accessibilityPermission: false }),
      timeout,
    ]);
    const bundleId = win?.owner?.bundleId || null;
    return bundleId && !SELF_BUNDLE_IDS.has(bundleId) ? bundleId : null;
  } catch {
    return null;
  }
}

/**
 * `open -b <bundleId>` is a thin LaunchServices call — for an app that's
 * already running (always true here, it was frontmost seconds ago) it just
 * switches to it, the same as double-clicking it in Finder, with none of
 * osascript's AppleScript/OSA runtime startup cost. `osascript ... activate`
 * is only the fallback for whatever `open` can't handle. Same fast-path/
 * fallback split as hylo's AppLauncher.buildFastActivateCommand.
 */
function activateApp(bundleId) {
  // Defense in depth: getFrontmostAppId() already filters these out, but
  // never let this function itself be the one thing standing between a
  // future caller and reactivating Vaultr into a second-instance fight
  // with itself (see SELF_BUNDLE_IDS above for what that broke last time).
  if (!bundleId || SELF_BUNDLE_IDS.has(bundleId)) return Promise.resolve();
  return new Promise((resolve) => {
    execFile("open", ["-b", bundleId], (err) => {
      if (!err) return resolve();
      execFile("osascript", ["-e", `tell application id "${bundleId}" to activate`], () => resolve());
    });
  });
}

/**
 * POST a PNG file to Vaultr's existing image library endpoint
 * (internal/server/handler/vault_image.go's UploadImage, the same one
 * content_pane.js's editor image paste/drop already uses via
 * `fetch('/api/vault/upload-image', {body: FormData})`). No `fetch`/
 * `FormData` here since this runs in the main process, not a renderer —
 * same multipart/form-data shape built by hand instead. Local server, no
 * auth needed. Resolves with `{ src, name }` on success (`src` is the
 * vault-relative path other Vaultr APIs use to reference the image).
 */
function uploadImageToVaultr(imagePath, serverUrl) {
  return new Promise((resolve, reject) => {
    if (!serverUrl) return reject(new Error("no server URL"));
    let fileData;
    try {
      fileData = fs.readFileSync(imagePath);
    } catch (e) {
      return reject(e);
    }

    const boundary = `----vaultrCapture${Date.now()}`;
    const head = Buffer.from(
      `--${boundary}\r\n` +
      `Content-Disposition: form-data; name="file"; filename="${path.basename(imagePath)}"\r\n` +
      `Content-Type: image/png\r\n\r\n`
    );
    const tail = Buffer.from(`\r\n--${boundary}--\r\n`);
    const body = Buffer.concat([head, fileData, tail]);

    let url;
    try {
      url = new URL("/api/vault/upload-image", serverUrl);
    } catch (e) {
      return reject(e);
    }
    const mod = url.protocol === "https:" ? https : http;
    const req = mod.request(url, {
      method: "POST",
      headers: {
        "Content-Type": `multipart/form-data; boundary=${boundary}`,
        "Content-Length": body.length,
      },
    }, (res) => {
      const chunks = [];
      res.on("data", (c) => chunks.push(c));
      res.on("end", () => {
        const text = Buffer.concat(chunks).toString("utf8");
        if (res.statusCode !== 200) return reject(new Error(`upload failed (${res.statusCode}): ${text}`));
        try {
          resolve(JSON.parse(text));
        } catch (e) {
          reject(e);
        }
      });
    });
    req.on("error", reject);
    req.write(body);
    req.end();
  });
}

/** Runs `screencapture -i`; resolves with the temp PNG path, or null if the user cancelled (Esc / right-click). */
function runInteractiveScreencapture() {
  return new Promise((resolve) => {
    const tmpFile = path.join(os.tmpdir(), `vaultr-capture-${Date.now()}.png`);
    const proc = spawn("screencapture", ["-i", "-o", tmpFile]);
    proc.on("error", () => resolve(null)); // e.g. binary missing — not expected on macOS
    proc.on("close", () => {
      resolve(fs.existsSync(tmpFile) ? tmpFile : null);
    });
  });
}

/**
 * `restoreFocus`: true for the explicit in-panel actions (Send/Cancel click,
 * Esc, ⌘⏎) — that's the path where our own panel activated Vaultr as a
 * side effect, so we own putting focus back. Left false for the "blur"
 * auto-dismiss (user clicked into some *other* app) — there the OS is
 * already handling focus correctly on its own, and forcing it back to
 * `frontAppBeforeCapture` would fight whatever the user just clicked into.
 *
 * When true, activateApp() is awaited *before* win.close(): closing first
 * would leave Vaultr briefly active with no window of its own to offer
 * except the main one, which macOS then raises for the ~50-100ms it takes
 * the (async, subprocess-based) activateApp call to land elsewhere — a
 * visible flash. Sequencing it the other way means Vaultr is never the
 * active app at the moment the panel actually disappears, so there's
 * nothing to flash.
 */
function closePanel({ restoreFocus = false } = {}) {
  // Closing can itself trigger a 'blur' (also wired to closePanel) before
  // 'closed' fires — null out panelWin first so that reentrant call is a no-op.
  if (!panelWin) return;
  const win = panelWin;
  const imagePath = panelImagePath;
  const frontApp = frontAppBeforeCapture;
  panelWin = null;
  panelImagePath = null;
  frontAppBeforeCapture = null;

  const finish = () => {
    if (!win.isDestroyed()) win.close();
    if (imagePath) fs.unlink(imagePath, () => {});
  };
  if (restoreFocus && frontApp) activateApp(frontApp).then(finish);
  else finish();
}

function openPanelForImage(imagePath) {
  const image = nativeImage.createFromPath(imagePath);
  const { width: iw, height: ih } = image.getSize();

  const display = screen.getDisplayNearestPoint(screen.getCursorScreenPoint());
  const { x: dx, y: dy, width: dw, height: dh } = display.workArea;

  const { imgW, imgH, boxW, boxH } = computeLayout(iw, ih, { width: dw, height: dh });
  const panelWidth = boxW + PANEL_PADDING * 2;
  const panelHeight = PANEL_PADDING * 2 + HEADER_H + boxH + IMG_GAP + COMPOSER_H + ACTIONS_GAP + ACTIONS_H;

  const x = Math.round(dx + (dw - panelWidth) / 2);
  const y = Math.round(dy + (dh - panelHeight) / 2);

  panelWin = new BaseWindow({
    x, y,
    width: panelWidth,
    height: panelHeight,
    type: "panel",       // non-activating NSPanel — never steals foreground from the app under it
    frame: false,
    transparent: true,
    resizable: false,
    // Not movable: no element in index.html declares -webkit-app-region:
    // drag any more (see the comment there) — a plain click on a drag
    // region behaves like clicking a title bar, which activates the app
    // regardless of type:"panel", so this transient auto-centered popup
    // just doesn't offer dragging at all rather than fight that.
    movable: false,
    hasShadow: true,
    alwaysOnTop: true,
    skipTaskbar: true,
    show: false,
    // Same native glass material the main Vaultr window's sidebar uses
    // (main.js's createWindow) — index.html leaves #panel's own background
    // transparent so this shows through, instead of painting a flat color.
    // Tried "popover" (the material semantically meant for a floating panel
    // like this one) to get rid of this material's own edge stroke, but on
    // this window it rendered flat/opaque instead of translucent — a
    // straight regression, so back to "sidebar". The stroke is a separate,
    // still-open problem.
    vibrancy: "sidebar",
    visualEffectState: "active",
  });
  // This is the actual cause of the Dock-icon-disappearing bug: without
  // skipTransformProcessType, setVisibleOnAllWorkspaces({visibleOnFullScreen:
  // true}) makes Electron flip the app's macOS process type from
  // ForegroundApplication to UIElementApplication and back (Chromium's
  // Browser::DockHide()/DockShow()) on every call, to get windows to float
  // above a fullscreen Space. That's supposed to be a brief flicker, but on
  // this Electron version DockShow() never actually gets called back — a
  // known upstream bug (electron/electron#26350) — so the very first
  // capture permanently drops Vaultr out of the Dock, ⌘-Tab, and even its
  // own menu bar, with the process still running underneath. Vaultr is
  // already a regular (Dock-visible) app the whole time this panel is
  // open, so skipping the transform costs nothing here and sidesteps the
  // bug entirely — this is Electron's own documented escape hatch for it.
  panelWin.setVisibleOnAllWorkspaces(true, { visibleOnFullScreen: true, skipTransformProcessType: true });
  panelImagePath = imagePath;

  const view = new WebContentsView({ webPreferences: makePanelWebPrefs() });
  // BaseWindow's own transparent:true only covers the native window itself —
  // a child View still paints an opaque background unless told otherwise,
  // which would show up as a white box around the panel's rounded corners.
  view.setBackgroundColor("#00000000");
  panelWin.contentView.addChildView(view);
  view.setBounds({ x: 0, y: 0, width: panelWidth, height: panelHeight });
  view.webContents.loadFile(path.join(__dirname, "quick-capture", "index.html"));

  view.webContents.once("did-finish-load", () => {
    view.webContents.send("quick-capture:init", { imageDataUrl: image.toDataURL(), boxW, boxH, imgW, imgH });
  });

  panelWin.on("blur", () => closePanel());
  panelWin.on("closed", () => { panelWin = null; });

  panelWin.showInactive();
  // Not panelWin.focus(): that BaseWindow-level focus() call turned out to
  // also pull Vaultr's main window forward when opening the panel — despite
  // electron/electron#40307 saying a type:"panel" window's own focus()
  // shouldn't activate the owning app, BaseWindow (the newer API both this
  // panel and the main window are built on, rather than classic
  // BrowserWindow) apparently doesn't get that exemption cleanly here.
  // webContents.focus() is the narrower, already-proven-safe call instead —
  // it's the exact same one showSection() in main.js uses to focus a
  // section view without touching window-level activation at all.
  view.webContents.focus();

  if (appWasHiddenBeforeCapture) {
    const mainWin = getMainWindow();
    if (mainWin && !mainWin.isDestroyed()) mainWin.hide();
  }
}

async function triggerQuickCapture() {
  if (captureInFlight || panelWin) return;
  captureInFlight = true;
  try {
    // Both read *before* screencapture runs — screencapture's own selection
    // UI doesn't change the frontmost app or the app's ⌘H-hidden state, but
    // capturing early keeps this tied to the state the user was actually in.
    frontAppBeforeCapture = await getFrontmostAppId();
    appWasHiddenBeforeCapture = app.isHidden();
    const imagePath = await runInteractiveScreencapture();
    if (!imagePath) return; // user cancelled the selection
    openPanelForImage(imagePath);
  } finally {
    captureInFlight = false;
  }
}

function registerQuickCapture(ipcMain, { getMainWindow: getMainWindowOpt, getServerUrl: getServerUrlOpt } = {}) {
  if (process.platform !== "darwin") return; // macOS-only for now
  if (getMainWindowOpt) getMainWindow = getMainWindowOpt;
  if (getServerUrlOpt) getServerUrl = getServerUrlOpt;

  globalShortcut.register(QUICK_CAPTURE_ACCELERATOR, triggerQuickCapture);
  app.on("will-quit", () => globalShortcut.unregister(QUICK_CAPTURE_ACCELERATOR));

  ipcMain.on("quick-capture:submit", (_event, message) => {
    // TODO: forward { imagePath: panelImagePath, message } to the agent bot
    // trigger endpoint once that wiring exists server-side.
    console.log("[quick-capture] submit (not yet sent to agent bot):", { imagePath: panelImagePath, message });
    closePanel({ restoreFocus: true });
  });
  ipcMain.on("quick-capture:save", async () => {
    const imagePath = panelImagePath;
    // Uploaded and awaited *before* closePanel() — closing deletes the temp
    // file (closePanel's finish()), so this has to know the upload actually
    // landed first or a failed save would silently lose the screenshot.
    try {
      const result = await uploadImageToVaultr(imagePath, getServerUrl());
      console.log("[quick-capture] saved to Vaultr Images:", result);
    } catch (e) {
      // No visible failure state in the panel yet (no error UI built for
      // this first pass) — it still closes either way, so a failed save
      // is currently silent to the user beyond this log line.
      console.error("[quick-capture] save failed:", e);
    }
    closePanel({ restoreFocus: true });
  });
  ipcMain.on("quick-capture:cancel", () => closePanel({ restoreFocus: true }));
}

/**
 * Whether the capture panel is currently open — main.js's `did-become-active`
 * handler checks this before showing the (deliberately hidden) main window,
 * so an activation blip that happens *because of* the panel itself (still a
 * possibility despite the webContents.focus()/mousedown fixes elsewhere)
 * doesn't undo the very hiding this feature is trying to preserve.
 */
function isCapturePanelOpen() {
  return panelWin !== null;
}

module.exports = { registerQuickCapture, isCapturePanelOpen };
