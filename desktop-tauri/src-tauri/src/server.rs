use std::fs::{self, OpenOptions};
use std::io::Write;
use std::path::{Path, PathBuf};
use std::process::{Command, Stdio};
use std::thread;
use std::time::{Duration, Instant};

use serde::Serialize;

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
    let line = format!("[{}] {msg}\n", stamp());
    eprintln!("[hylo-tauri] {msg}");
    if let Ok(mut f) = OpenOptions::new()
        .create(true)
        .append(true)
        .open(data_dir.join("hylo-shell-diagnostics.log"))
    {
        let _ = f.write_all(line.as_bytes());
    }
}

pub fn stamp() -> String {
    // Avoid a time crate: ISO-ish local stamp is enough for the shell log.
    let d = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap_or_default();
    format!("{}.{:03}", d.as_secs(), d.subsec_millis())
}

pub fn home_dir() -> PathBuf {
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

pub fn pid_path(data_dir: &Path) -> PathBuf {
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

/// A failed lookup assumes the pid is ours.
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

/// The `Error: …` line the server printed when its latest start failed.
pub fn last_error(data_dir: &Path) -> Option<String> {
    let raw = fs::read(diag_paths(data_dir).server_output).ok()?;
    let tail = String::from_utf8_lossy(&raw[raw.len().saturating_sub(8192)..]).into_owned();
    let latest = tail.rfind("spawn hylo start server").map_or(tail.as_str(), |i| &tail[i..]);
    latest
        .lines()
        .rev()
        .find_map(|l| l.strip_prefix("Error: "))
        .map(str::to_string)
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
    candidates.into_iter().find(|bin| bin_works(bin, &path_env))
}

fn bin_name() -> &'static str {
    if cfg!(windows) {
        "hylo.exe"
    } else {
        "hylo"
    }
}

/// The server's own graceful shutdown gives up after 15s; most finish in well under this.
pub const STOP_GRACE: Duration = Duration::from_secs(10);

pub fn kill_hylo(data_dir: &Path, pid: u32) -> bool {
    if !pid_is_hylo(pid) {
        return true;
    }
    signal(pid, false);
    let deadline = Instant::now() + STOP_GRACE;
    while Instant::now() < deadline {
        if !is_alive(pid) {
            return true;
        }
        thread::sleep(Duration::from_millis(150));
    }
    diag_log(
        data_dir,
        &format!("kill-process: pid {pid} ignored SIGTERM, killing"),
    );
    signal(pid, true);
    thread::sleep(Duration::from_millis(300));
    !is_alive(pid)
}

pub fn signal(pid: u32, force: bool) {
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
