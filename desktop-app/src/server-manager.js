"use strict";

const { app, ipcMain } = require("electron");
const path = require("node:path");
const fs = require("node:fs");
const http = require("node:http");
const https = require("node:https");
const { spawn, spawnSync, execFile } = require("node:child_process");
const os = require("node:os");
const { installCli } = require("./cli-installer");

// ── Diagnostics ───────────────────────────────────────────────────────────────

/** @type {{ [key: string]: number }} */
const shellDiagThrottle = {};
const CHECK_FAIL_LOG_GAP_MS = 12000;

function getShellDiagPaths() {
  const base = app.getPath("userData");
  return {
    userData: base,
    diagnostics: path.join(base, "hylo-shell-diagnostics.log"),
    serverOutput: path.join(base, "hylo-server-output.log"),
  };
}

function appendDiagFile(line) {
  try {
    const { diagnostics } = getShellDiagPaths();
    fs.appendFileSync(diagnostics, line);
  } catch (_) { /* noop */ }
}

function diagLog(...parts) {
  const text = `[${new Date().toISOString()}] ${parts.join(" ")}\n`;
  console.error("[hylo-shell]", ...parts);
  appendDiagFile(text);
}

function diagLogCheckFailThrottled(url, reason) {
  const now = Date.now();
  const key = `${reason}|${url}`;
  const prev = shellDiagThrottle[key];
  if (prev != null && now - prev < CHECK_FAIL_LOG_GAP_MS) return;
  shellDiagThrottle[key] = now;
  diagLog("check-server", reason, url);
}

// ── PATH expansion for GUI launch ────────────────────────────────────────────
// On macOS, apps launched from Finder/Dock don't source ~/.zshrc, so
// process.env.PATH is the minimal system PATH.  Expand it with common
// user-level tool directories so Electron can locate the hylo binary.
// Full shell env (http_proxy, tokens, etc.) is captured by the Go server
// at startup via agent.WarmShellEnv — no need to do it here.

// Computed once and cached — process.env.PATH doesn't change over the app's
// lifetime, and this is recomputed on every resolveHyloBin() candidate and
// every spawn, which added up to redundant work on the same static inputs.
let cachedExpandedEnv = null;

function expandedEnv() {
  if (cachedExpandedEnv) return cachedExpandedEnv;
  const home = os.homedir();
  const extra = [
    path.join(home, ".local", "bin"),
    path.join(home, "bin"),
    path.join(home, ".local", "share", "pnpm"),
    path.join(home, ".npm-global", "bin"),
    path.join(home, ".opencode", "bin"),
    path.join(home, ".volta", "bin"),
    "/opt/homebrew/bin",
    "/opt/homebrew/sbin",
    "/usr/local/bin",
  ];
  const current = process.env.PATH || "";
  const parts = new Set(current.split(path.delimiter).filter(Boolean));
  for (const p of extra) parts.add(p);
  cachedExpandedEnv = { ...process.env, PATH: [...parts].join(path.delimiter) };
  return cachedExpandedEnv;
}

// ── Hylo binary resolution ──────────────────────────────────────────────────

/**
 * Runs `bin --version` without blocking the main thread (unlike spawnSync,
 * which freezes the whole UI — window dragging, all IPC — for up to the
 * timeout if the target binary hangs). Resolves true iff it exits 0.
 */
function checkBinWorks(bin) {
  return new Promise((resolve) => {
    try {
      execFile(bin, ["--version"], { timeout: 8000, windowsHide: true, env: expandedEnv() }, (error) => {
        resolve(!error);
      });
    } catch {
      resolve(false);
    }
  });
}

/** Returns the first working hylo binary path, or null if none found. */
async function resolveHyloBin() {
  const candidates = [
    path.join(os.homedir(), ".local", "bin", "hylo"),
    "hylo",
  ];
  for (const bin of candidates) {
    if (await checkBinWorks(bin)) return bin;
  }
  return null;
}

// ── PID / process helpers ─────────────────────────────────────────────────────

function getServerPIDFilePath() {
  return path.join(app.getPath("userData"), "hylo-server.pid");
}

function readServerPID() {
  try {
    const content = fs.readFileSync(getServerPIDFilePath(), "utf8").trim();
    const pid = parseInt(content, 10);
    return isNaN(pid) || pid <= 0 ? null : pid;
  } catch {
    return null;
  }
}

function isProcessAlive(pid) {
  try { process.kill(pid, 0); return true; } catch { return false; }
}

/**
 * Best-effort check that `pid` actually looks like a hylo process, not an
 * unrelated process that reused this PID after the real server exited
 * without cleaning up its PID file (isProcessAlive only checks existence).
 * Returns true (assume it's ours) if the platform lookup itself fails —
 * a failed check shouldn't be treated as proof it's NOT ours.
 */
function looksLikeHyloProcess(pid) {
  try {
    const r = process.platform === "win32"
      ? spawnSync("wmic", ["process", "where", `ProcessId=${pid}`, "get", "ExecutablePath"], { encoding: "utf8", timeout: 3000, windowsHide: true })
      : spawnSync("ps", ["-p", String(pid), "-o", "comm="], { encoding: "utf8", timeout: 3000 });
    if (r.error || r.status !== 0) return true; // couldn't verify — don't block on it
    return /hylo/i.test((r.stdout || "").trim());
  } catch {
    return true;
  }
}

/** Combines existence + identity: is `pid` alive AND does it look like our hylo server? */
function pidIsHyloServer(pid) {
  return isProcessAlive(pid) && looksLikeHyloProcess(pid);
}

// ── Server spawn ──────────────────────────────────────────────────────────────

/** PID of the hylo server process most recently spawned by this Electron instance. */
let managedServerChildPid = 0;

/** Callbacks registered via register() — used by restartServerAfterCliUpdate. */
let _registeredOpts = {};

/** Shared SIGTERM → wait → optional SIGKILL logic. Returns true when the process is gone. */
async function killProcess(pid) {
  if (!pidIsHyloServer(pid)) return true;
  try { process.kill(pid, "SIGTERM"); } catch { return false; }
  const deadline = Date.now() + 8000;
  while (Date.now() < deadline) {
    await new Promise((r) => setTimeout(r, 150));
    if (!isProcessAlive(pid)) return true;
  }
  diagLog("kill-process: still alive after 8s, sending SIGKILL to pid", pid);
  try { process.kill(pid, "SIGKILL"); } catch { /* noop */ }
  await new Promise((r) => setTimeout(r, 300));
  return !isProcessAlive(pid);
}

/**
 * Detached subprocess; stdout/stderr go to userData log. Survives after Electron exits.
 *
 * If no `hylo` binary can be resolved, this falls back to a forced re-install of the
 * bundled CLI (bypassing the sentinel — the sentinel may say this version+build is
 * already installed even though the binary itself has since gone missing) and retries
 * resolution once before giving up.
 */
async function startHyloServerDetached() {
  const { serverOutput } = getShellDiagPaths();
  try {
    fs.mkdirSync(path.dirname(serverOutput), { recursive: true });
    fs.appendFileSync(
      serverOutput,
      `\n--- ${new Date().toISOString()} spawn hylo start server ---\n`
    );
  } catch (e) {
    diagLog("server output log preamble failed:", e.message);
  }

  let bin = await resolveHyloBin();
  if (!bin) {
    diagLog("start-server: hylo binary not found, attempting fallback CLI install");
    let installResult;
    try {
      installResult = await installCli((msg) => diagLog(msg), { force: true });
    } catch (e) {
      return { ok: false, error: `\`hylo\` not found, and fallback install threw: ${e.message}` };
    }
    if (!installResult.ok) {
      return { ok: false, error: `\`hylo\` not found, and fallback install failed: ${installResult.error || "unknown error"}` };
    }
    bin = await resolveHyloBin();
    // A freshly-written binary can occasionally fail its first exec attempt
    // (e.g. macOS briefly scanning a newly-extracted executable before
    // allowing it to run) even though the install itself succeeded. Give it
    // a couple of short retries before giving up — but only when an install
    // actually ran; retrying after a `skipped` no-op (e.g. dev mode with no
    // bundled archive) can't change the outcome.
    if (!bin && !installResult.skipped) {
      for (const delayMs of [300, 600]) {
        await new Promise((r) => setTimeout(r, delayMs));
        bin = await resolveHyloBin();
        if (bin) break;
      }
    }
    if (!bin) {
      const reason = installResult.skipped
        ? "no bundled CLI archive available to install (dev mode?)"
        : "reinstalling the bundled CLI did not produce a working binary";
      return { ok: false, error: `\`hylo\` not found in PATH, /usr/local/bin, or ~/.local/bin (${reason}).` };
    }
    diagLog("start-server: fallback install succeeded, resolved hylo at", bin);
  }

  return new Promise((resolve) => {
    let settled = false;
    const finish = (/** @type {{ ok: boolean, error?: string, pid?: number }} */ out) => {
      if (settled) return;
      settled = true;
      resolve(out);
    };

    /** @type {number | undefined} */
    let logFd;
    try {
      logFd = fs.openSync(serverOutput, "a");
    } catch (e) {
      diagLog("open server output log:", e.message);
      finish({ ok: false, error: `cannot open log file (${e.message})` });
      return;
    }

    let child;
    try {
      child = spawn(bin, ["start", "server", "--pid-file", getServerPIDFilePath()], {
        detached: true,
        stdio: ["ignore", logFd, logFd],
        windowsHide: true,
        env: expandedEnv(),
      });
    } catch (e) {
      try { fs.closeSync(logFd); } catch (_) { /* noop */ }
      diagLog("spawn hylo start server threw:", e.message);
      finish({ ok: false, error: e.message });
      return;
    }

    try { fs.closeSync(logFd); } catch (_) { /* noop */ }

    child.once("error", (err) => {
      diagLog("spawn hylo start server event error:", err.message);
      finish({ ok: false, error: err.message });
    });
    child.unref();

    setImmediate(() => {
      if (settled) return;
      const pid = child.pid;
      if (typeof pid === "number" && pid > 0) {
        diagLog("started hylo server, pid=", pid, "logs append to", serverOutput);
        finish({ ok: true, pid });
      } else {
        diagLog("spawn produced no pid");
        finish({ ok: false, error: "spawn failed (no process id)" });
      }
    });
  });
}

// ── IPC registration ──────────────────────────────────────────────────────────

/**
 * @param {{
 *   onRestartDone?: () => void,
 *   onServerStopped?: () => void,
 *   onBeforeStop?: () => void,
 *   getAutoStart?: () => boolean,
 *   setAutoStart?: (v: boolean) => void,
 * }} [opts]
 */
function register(opts = {}) {
  _registeredOpts = opts;
  const { onRestartDone, onServerStopped, onBeforeStop, getAutoStart, setAutoStart } = opts;

  /** Schedule a UI-transition callback after the current IPC promise resolves. */
  function deferUiTransition(fn) {
    if (fn) setTimeout(() => { try { fn(); } catch { /* noop */ } }, 0);
  }

  // ── check-server ────────────────────────────────────────────────────────────

  ipcMain.handle("check-server", async (_event, url) => {
    return new Promise((resolve) => {
      try {
        const parsed = new URL(url);
        const mod = parsed.protocol === "https:" ? https : http;
        const req = mod.request(
          {
            method: "GET",
            hostname: parsed.hostname,
            port: parsed.port || (parsed.protocol === "https:" ? 443 : 80),
            path: "/home",
            timeout: 3000,
          },
          (res) => { res.resume(); resolve(true); }
        );
        req.on("error", (err) => {
          diagLogCheckFailThrottled(url, `http error: ${err?.message || err}`);
          resolve(false);
        });
        req.on("timeout", () => {
          diagLogCheckFailThrottled(url, "timeout GET /home (3s)");
          req.destroy();
          resolve(false);
        });
        req.end();
      } catch (err) {
        diagLog("check-server bad URL:", url, err);
        resolve(false);
      }
    });
  });

  // ── start-hylo-server-detached ────────────────────────────────────────────

  ipcMain.handle("start-hylo-server-detached", async (_event, invokeOpts = {}) => {
    const { userInitiated = false } = invokeOpts;

    // Block automatic start when the user intentionally stopped the server.
    // A user-initiated attempt (clicking Connect) re-enables auto-start.
    if (!userInitiated && getAutoStart && !getAutoStart()) {
      diagLog("start-server: autoStart disabled, skip spawn");
      return { ok: false, reason: "stopped_by_user" };
    }
    if (userInitiated && setAutoStart) {
      setAutoStart(true);
    }

    // If Electron already spawned a server that is still initialising, skip the
    // duplicate spawn and let the start screen keep polling.
    if (managedServerChildPid > 0 && pidIsHyloServer(managedServerChildPid)) {
      diagLog("start-server: managed server pid=%d alive, skip duplicate spawn", managedServerChildPid);
      return { ok: true };
    }
    const result = await startHyloServerDetached();
    if (result.ok && result.pid) managedServerChildPid = result.pid;
    return result;
  });

  // ── get-shell-debug-paths ───────────────────────────────────────────────────

  ipcMain.handle("get-shell-debug-paths", () => getShellDiagPaths());

  // ── get-server-process-status ───────────────────────────────────────────────

  ipcMain.handle("get-server-process-status", () => {
    const pid = readServerPID();
    return {
      managed: pid !== null,
      pid,
      alive: pid !== null && pidIsHyloServer(pid),
    };
  });

  // ── stop-hylo-server ──────────────────────────────────────────────────────

  ipcMain.handle("stop-hylo-server", async () => {
    const pid = readServerPID();
    if (!pid) {
      return { ok: false, reason: "no_pid", error: "No managed server process found" };
    }

    // Close any persistent connections to the server (e.g. SSE streams) before
    // sending SIGTERM so the server's graceful-shutdown doesn't stall waiting
    // for long-lived connections to drain.
    if (onBeforeStop) onBeforeStop();

    diagLog("stop-server: stopping managed server pid", pid);
    await killProcess(pid);
    managedServerChildPid = 0;
    if (setAutoStart) setAutoStart(false);
    deferUiTransition(onServerStopped);
    return { ok: true };
  });

  // ── restart-hylo-server ───────────────────────────────────────────────────

  ipcMain.handle("restart-hylo-server", async () => {
    const pid = readServerPID();
    if (!pid) {
      return { ok: false, reason: "no_pid", error: "No valid PID file — server was not started by the desktop app" };
    }

    // Close persistent connections before SIGTERM for the same reason as stop.
    if (onBeforeStop) onBeforeStop();

    if (pidIsHyloServer(pid)) {
      diagLog("restart-server: stopping pid", pid);
      const killed = await killProcess(pid);
      if (!killed) {
        diagLog("restart-server: could not confirm process death, proceeding anyway");
      }
    } else {
      diagLog("restart-server: pid", pid, "already not running, spawning fresh");
    }

    diagLog("restart-server: spawning new hylo server");
    managedServerChildPid = 0;
    const spawnResult = await startHyloServerDetached();
    if (!spawnResult.ok) {
      return { ok: false, reason: "spawn_failed", error: spawnResult.error };
    }
    managedServerChildPid = spawnResult.pid;
    deferUiTransition(onRestartDone);
    return { ok: true, pid: spawnResult.pid };
  });
}

/**
 * Called after a CLI upgrade: kills the currently running server (if any) so the
 * start screen's auto-restart loop picks up the new binary.  autoStart is left
 * untouched — the start screen will re-spawn the server automatically.
 */
async function restartServerAfterCliUpdate() {
  const { onBeforeStop, onServerStopped } = _registeredOpts;
  const pid = readServerPID();
  if (!pid || !pidIsHyloServer(pid)) return; // no server running — nothing to do
  diagLog("cli-upgrade: killing old server pid", pid, "to pick up new binary");
  if (onBeforeStop) onBeforeStop();
  await killProcess(pid);
  managedServerChildPid = 0;
  if (onServerStopped) setTimeout(() => { try { onServerStopped(); } catch { /* noop */ } }, 0);
}

module.exports = { register, restartServerAfterCliUpdate };
