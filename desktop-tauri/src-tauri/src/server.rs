use std::fs::{self, OpenOptions};
use std::io::Write;
use std::path::{Path, PathBuf};
use std::process::{Command, Stdio};
use std::thread;
use std::time::{Duration, Instant};

use serde::Serialize;

use crate::cli;
use crate::config::ConfigFile;

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct SpawnResult {
    pub ok: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub reason: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub pid: Option<u32>,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ProcessStatus {
    pub managed: bool,
    pub pid: Option<u32>,
    pub alive: bool,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct DiagPaths {
    pub user_data: String,
    pub diagnostics: String,
    pub server_output: String,
}

pub fn diag_paths(data_dir: &Path) -> DiagPaths {
    DiagPaths {
        user_data: data_dir.display().to_string(),
        diagnostics: data_dir
            .join("hylo-shell-diagnostics.log")
            .display()
            .to_string(),
        server_output: data_dir
            .join("hylo-server-output.log")
            .display()
            .to_string(),
    }
}

pub fn diag_log(data_dir: &Path, msg: &str) {
    let line = format!("[{}] {msg}\n", chrono_stamp());
    eprintln!("[hylo-tauri] {msg}");
    if let Ok(mut f) = OpenOptions::new()
        .create(true)
        .append(true)
        .open(data_dir.join("hylo-shell-diagnostics.log"))
    {
        let _ = f.write_all(line.as_bytes());
    }
}

fn chrono_stamp() -> String {
    // Avoid a time crate: ISO-ish local stamp is enough for the shell log.
    let d = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap_or_default()
        .as_secs();
    format!("{d}")
}

fn home_dir() -> PathBuf {
    std::env::var_os("HOME")
        .or_else(|| std::env::var_os("USERPROFILE"))
        .map(PathBuf::from)
        .unwrap_or_else(|| PathBuf::from("."))
}

pub fn expanded_path() -> String {
    let home = home_dir();
    let extra = [
        home.join(".local").join("bin"),
        home.join("bin"),
        home.join(".local").join("share").join("pnpm"),
        home.join(".npm-global").join("bin"),
        home.join(".opencode").join("bin"),
        home.join(".volta").join("bin"),
        PathBuf::from("/opt/homebrew/bin"),
        PathBuf::from("/opt/homebrew/sbin"),
        PathBuf::from("/usr/local/bin"),
    ];
    let current = std::env::var("PATH").unwrap_or_default();
    let mut parts: Vec<String> = current
        .split(if cfg!(windows) { ';' } else { ':' })
        .filter(|s| !s.is_empty())
        .map(|s| s.to_string())
        .collect();
    for p in extra {
        let s = p.display().to_string();
        if !parts.iter().any(|e| e == &s) {
            parts.push(s);
        }
    }
    parts.join(if cfg!(windows) { ";" } else { ":" })
}

fn pid_path(data_dir: &Path) -> PathBuf {
    data_dir.join("hylo-server.pid")
}

pub fn read_pid(data_dir: &Path) -> Option<u32> {
    let raw = fs::read_to_string(pid_path(data_dir)).ok()?;
    let pid: u32 = raw.trim().parse().ok()?;
    if pid == 0 {
        None
    } else {
        Some(pid)
    }
}

pub fn is_alive(pid: u32) -> bool {
    #[cfg(unix)]
    {
        let rc = unsafe { libc::kill(pid as i32, 0) };
        if rc == 0 {
            return true;
        }
        std::io::Error::last_os_error().raw_os_error() == Some(libc::EPERM)
    }
    #[cfg(windows)]
    {
        let out = Command::new("tasklist")
            .args(["/FI", &format!("PID eq {pid}"), "/NH"])
            .output();
        match out {
            Ok(o) => String::from_utf8_lossy(&o.stdout).contains(&pid.to_string()),
            Err(_) => false,
        }
    }
}

/// A failed lookup assumes the pid is ours, matching the Electron shell.
pub fn looks_like_hylo(pid: u32) -> bool {
    #[cfg(unix)]
    {
        let out = Command::new("ps")
            .args(["-p", &pid.to_string(), "-o", "comm="])
            .output();
        match out {
            Ok(o) if o.status.success() => String::from_utf8_lossy(&o.stdout)
                .to_lowercase()
                .contains("hylo"),
            _ => true,
        }
    }
    #[cfg(windows)]
    {
        let out = Command::new("tasklist")
            .args(["/FI", &format!("PID eq {pid}"), "/NH", "/FO", "CSV"])
            .output();
        match out {
            Ok(o) if o.status.success() => String::from_utf8_lossy(&o.stdout)
                .to_lowercase()
                .contains("hylo"),
            _ => true,
        }
    }
}

pub fn pid_is_hylo(pid: u32) -> bool {
    is_alive(pid) && looks_like_hylo(pid)
}

pub fn status(data_dir: &Path) -> ProcessStatus {
    let pid = read_pid(data_dir);
    ProcessStatus {
        managed: pid.is_some(),
        alive: pid.is_some_and(pid_is_hylo),
        pid,
    }
}

fn bin_works(bin: &Path, path_env: &str) -> bool {
    let mut cmd = Command::new(bin);
    cmd.arg("--version")
        .env("PATH", path_env)
        .stdin(Stdio::null())
        .stdout(Stdio::null())
        .stderr(Stdio::null());
    let Ok(mut child) = cmd.spawn() else {
        return false;
    };
    let start = Instant::now();
    loop {
        match child.try_wait() {
            Ok(Some(status)) => return status.success(),
            Ok(None) if start.elapsed() > Duration::from_secs(8) => {
                let _ = child.kill();
                let _ = child.wait();
                return false;
            }
            Ok(None) => thread::sleep(Duration::from_millis(40)),
            Err(_) => return false,
        }
    }
}

pub fn resolve_hylo_bin() -> Option<PathBuf> {
    let path_env = expanded_path();
    let home = home_dir();
    let mut candidates = vec![home.join(".local").join("bin").join(bin_name())];
    candidates.push(PathBuf::from(bin_name()));
    for bin in candidates {
        if bin_works(&bin, &path_env) {
            return Some(bin);
        }
    }
    None
}

fn bin_name() -> &'static str {
    if cfg!(windows) {
        "hylo.exe"
    } else {
        "hylo"
    }
}

pub fn kill_hylo(data_dir: &Path, pid: u32) -> bool {
    if !pid_is_hylo(pid) {
        return true;
    }
    signal(pid, false);
    let deadline = Instant::now() + Duration::from_secs(8);
    while Instant::now() < deadline {
        if !is_alive(pid) {
            return true;
        }
        thread::sleep(Duration::from_millis(150));
    }
    diag_log(
        data_dir,
        &format!("kill-process: still alive after 8s, forcing pid {pid}"),
    );
    signal(pid, true);
    thread::sleep(Duration::from_millis(300));
    !is_alive(pid)
}

fn signal(pid: u32, force: bool) {
    #[cfg(unix)]
    {
        let sig = if force { libc::SIGKILL } else { libc::SIGTERM };
        unsafe { libc::kill(pid as i32, sig) };
    }
    #[cfg(windows)]
    {
        let mut cmd = Command::new("taskkill");
        cmd.args(["/PID", &pid.to_string()]);
        if force {
            cmd.arg("/F");
        }
        let _ = cmd.output();
    }
}

pub fn start_server(data_dir: &Path, archive: Option<PathBuf>) -> SpawnResult {
    let paths = diag_paths(data_dir);
    let _ = fs::create_dir_all(data_dir);
    if let Ok(mut f) = OpenOptions::new()
        .create(true)
        .append(true)
        .open(&paths.server_output)
    {
        let _ = writeln!(f, "\n--- {} spawn hylo start server ---", chrono_stamp());
    }

    let bin = match resolve_hylo_bin() {
        Some(bin) => bin,
        None => {
            diag_log(
                data_dir,
                "start-server: hylo binary not found, attempting fallback CLI install",
            );
            match cli::install(data_dir, archive, true) {
                Ok(cli::InstallOutcome::Skipped) => {
                    return fail("`hylo` not found in PATH, /usr/local/bin, or ~/.local/bin (no bundled CLI archive available to install (dev mode?)).");
                }
                Ok(cli::InstallOutcome::Installed) => {
                    let mut found = resolve_hylo_bin();
                    for delay in [300, 600] {
                        if found.is_some() {
                            break;
                        }
                        thread::sleep(Duration::from_millis(delay));
                        found = resolve_hylo_bin();
                    }
                    match found {
                        Some(bin) => bin,
                        None => {
                            return fail(
                                "reinstalling the bundled CLI did not produce a working binary",
                            );
                        }
                    }
                }
                Err(e) => {
                    return fail(&format!(
                        "`hylo` not found, and fallback install failed: {e}"
                    ))
                }
            }
        }
    };

    let log = match OpenOptions::new()
        .create(true)
        .append(true)
        .open(&paths.server_output)
    {
        Ok(f) => f,
        Err(e) => return fail(&format!("cannot open log file ({e})")),
    };
    let log_err = match log.try_clone() {
        Ok(f) => f,
        Err(e) => return fail(&e.to_string()),
    };

    let mut cmd = Command::new(&bin);
    cmd.args(["start", "server", "--pid-file"])
        .arg(pid_path(data_dir))
        .env("PATH", expanded_path())
        .stdin(Stdio::null())
        .stdout(Stdio::from(log))
        .stderr(Stdio::from(log_err));
    #[cfg(unix)]
    {
        use std::os::unix::process::CommandExt;
        // New process group so quitting the shell does not take the server with it.
        cmd.process_group(0);
    }
    #[cfg(windows)]
    {
        use std::os::windows::process::CommandExt;
        cmd.creation_flags(0x00000008 | 0x08000000);
    }

    match cmd.spawn() {
        Ok(child) => {
            let pid = child.id();
            diag_log(
                data_dir,
                &format!(
                    "started hylo server, pid={pid} logs append to {}",
                    paths.server_output
                ),
            );
            SpawnResult {
                ok: true,
                reason: None,
                error: None,
                pid: Some(pid),
            }
        }
        Err(e) => fail(&e.to_string()),
    }
}

fn fail(msg: &str) -> SpawnResult {
    SpawnResult {
        ok: false,
        reason: None,
        error: Some(msg.to_string()),
        pid: None,
    }
}

pub fn autostart_enabled(data_dir: &Path) -> bool {
    ConfigFile::load(data_dir).auto_start_server
}

pub fn set_autostart(data_dir: &Path, on: bool) -> Result<(), String> {
    let mut cfg = ConfigFile::load(data_dir);
    cfg.auto_start_server = on;
    cfg.save(data_dir)
}
