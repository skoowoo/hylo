"use strict";

const path = require("node:path");
const fs = require("node:fs");
const os = require("node:os");
const { spawnSync } = require("node:child_process");

const BINARY_NAME = process.platform === "win32" ? "hylo.exe" : "hylo";

// app.getVersion() (package.json "version", x.y.z) is what users see and
// should only change on real releases. "hyloBuild" is a separate counter
// for re-shipping the same x.y.z with a fixed bundled CLI/skills payload
// (e.g. patching a bug found right after release, without bumping the public
// version number). Bump hyloBuild whenever bundled/ changes but version
// doesn't; reset it to "0" whenever version is bumped for a normal release.
const { hyloBuild: BUILD_ID = "0" } = require("../package.json");

/** Combines app version + build id so a build-only re-ship still invalidates old sentinels. */
function sentinelValueFor(appVersion) {
  return `${appVersion}-${BUILD_ID}`;
}

// Sentinel file: records which app version+build last installed the CLI.
// If it matches the current value, installation is skipped.
const SENTINEL_FILE = path.join(os.homedir(), ".hylo", ".cli_app_version");

function getBundledArchive() {
  const { app } = require("electron");
  if (!app.isPackaged) return null; // dev mode: no bundled archive
  const ext = process.platform === "win32" ? "zip" : "tar.gz";
  return path.join(process.resourcesPath, "cli", `hylo.${ext}`);
}

function getSystemInstallDir() {
  return path.join(os.homedir(), ".local", "bin");
}

// Serializes all installCli() calls onto one chain. Two call sites can race
// in practice — main.js's proactive install at every launch, and
// server-manager.js's "binary went missing" fallback — and both write to the
// same destBin/tmpBin path (tmpBin includes only process.pid, which is
// identical for concurrent calls in this process). Without serialization,
// concurrent copyFile/rename calls to that shared path can interleave and
// install a truncated/corrupt binary. installChain always resolves (never
// rejects) so one failed install doesn't block the next call from running;
// each caller still gets its own accurate result via `run`.
let installChain = Promise.resolve();

/**
 * Install the bundled hylo CLI by extracting the tar.gz from app resources.
 *
 * Skipped when the sentinel file matches the current app version + hyloBuild,
 * unless `force` is set (e.g. the sentinel matches but the binary itself is
 * missing — reinstalling is the only way to recover).
 * Binary is always overwritten (keeps CLI in sync with app version).
 * config.toml and skills/ files are only written if absent.
 *
 * Concurrent calls are serialized (see installChain above) rather than run
 * in parallel.
 *
 * @param {(msg: string) => void} [log]
 * @param {{ force?: boolean }} [opts]
 * @returns {Promise<{ ok: boolean, skipped?: boolean, installDir?: string, error?: string }>}
 */
function installCli(log = () => {}, opts = {}) {
  const run = installChain.then(() => doInstallCli(log, opts));
  installChain = run.then(() => {}, () => {});
  return run;
}

async function doInstallCli(log, opts) {
  const { force = false } = opts;
  const { app } = require("electron");
  const appVersion = app.getVersion();
  const sentinelValue = sentinelValueFor(appVersion);
  const fsp = fs.promises;

  // Skip if this app version+build has already installed the CLI
  if (!force) {
    try {
      const installed = (await fsp.readFile(SENTINEL_FILE, "utf8")).trim();
      if (installed === sentinelValue) {
        log(`cli-installer: already up to date (v${sentinelValue}), skipping`);
        return { ok: true, skipped: true };
      }
    } catch { /* sentinel absent or unreadable → proceed */ }
  }

  const archive = getBundledArchive();
  if (!archive) {
    log("cli-installer: no bundled archive (dev mode), skipping");
    return { ok: true, skipped: true };
  }

  try { await fsp.access(archive); } catch {
    log(`cli-installer: bundled archive not found at ${archive}`);
    return { ok: false, error: "Bundled CLI archive not found in app resources" };
  }

  // Extract archive to a temp directory
  let tmpDir;
  try {
    tmpDir = await fsp.mkdtemp(path.join(os.tmpdir(), "hylo-install-"));
  } catch (e) {
    log(`cli-installer: mkdtemp failed: ${e.message}`);
    return { ok: false, error: e.message };
  }

  try {
    let r;
    if (process.platform === "win32") {
      r = spawnSync("powershell", [
        "-NoProfile", "-Command",
        `Expand-Archive -Path "${archive}" -DestinationPath "${tmpDir}" -Force`,
      ], { encoding: "utf8" });
    } else {
      r = spawnSync("tar", ["xzf", archive, "-C", tmpDir], { encoding: "utf8" });
    }
    if (r.error || r.status !== 0) {
      const reason = r.error?.message || r.stderr || `exit code ${r.status}`;
      log(`cli-installer: extraction failed: ${reason}`);
      return { ok: false, error: `archive extraction failed: ${reason}` };
    }
  } catch (e) {
    log(`cli-installer: extraction spawn failed: ${e.message}`);
    return { ok: false, error: e.message };
  }

  // Install binary (always overwrite — keeps CLI version in sync with app)
  const installDir = getSystemInstallDir();
  try {
    await fsp.mkdir(installDir, { recursive: true });
  } catch (e) {
    log(`cli-installer: mkdir ${installDir} failed: ${e.message}`);
    return { ok: false, error: `Cannot create install dir: ${e.message}` };
  }

  const destBin = path.join(installDir, BINARY_NAME);
  const tmpBin = destBin + ".tmp." + process.pid;
  try {
    await fsp.copyFile(path.join(tmpDir, BINARY_NAME), tmpBin);
    if (process.platform !== "win32") await fsp.chmod(tmpBin, 0o755);
    await fsp.rename(tmpBin, destBin); // atomic replace — safe even if old binary is running
    log(`cli-installer: installed to ${destBin}`);
  } catch (e) {
    try { await fsp.unlink(tmpBin); } catch { /* noop */ }
    log(`cli-installer: binary copy failed: ${e.message}`);
    return { ok: false, error: e.message };
  }

  // config.toml — only write if not already present (never overwrite user config)
  try {
    const hyloDir = path.join(os.homedir(), ".hylo");
    await fsp.mkdir(hyloDir, { recursive: true });

    const configDst = path.join(hyloDir, "config.toml");
    try {
      await fsp.access(configDst);
      // file exists — skip
    } catch {
      try {
        await fsp.copyFile(path.join(tmpDir, "config.example.toml"), configDst);
        log(`cli-installer: installed default config to ${configDst}`);
      } catch { /* config.example.toml absent in archive */ }
    }

    // skills/ — only write files that do not already exist
    const skillsSrc = path.join(tmpDir, "skills");
    try {
      await fsp.access(skillsSrc);
      fs.cpSync(skillsSrc, path.join(hyloDir, "skills"), {
        recursive: true,
        force: false,
        errorOnExist: false,
      });
      log(`cli-installer: installed built-in skills to ${path.join(hyloDir, "skills")}`);
    } catch { /* skills dir absent in archive */ }
  } catch (e) {
    log(`cli-installer: config/skills copy failed (non-fatal): ${e.message}`);
  }

  // Clean up temp dir
  try { fs.rmSync(tmpDir, { recursive: true, force: true }); } catch { /* noop */ }

  // Write sentinel so subsequent launches with the same app version+build skip install
  try {
    await fsp.mkdir(path.dirname(SENTINEL_FILE), { recursive: true });
    await fsp.writeFile(SENTINEL_FILE, sentinelValue, "utf8");
  } catch (e) {
    log(`cli-installer: sentinel write failed (non-fatal): ${e.message}`);
  }

  return { ok: true, installDir };
}

module.exports = { installCli, BINARY_NAME };
