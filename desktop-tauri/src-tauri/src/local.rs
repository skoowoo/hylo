// The Hylo server on this computer. Every install, spawn and kill goes through
// one lock here; waiting for a start to finish happens outside it, so Stop can
// cut a slow start short.
use std::fs::OpenOptions;
use std::io::Write;
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::sync::{Arc, Mutex, MutexGuard};
use std::thread;
use std::time::{Duration, Instant};

use serde::ser::SerializeStruct;
use serde::{Serialize, Serializer};
use tauri::{AppHandle, Manager};
use tokio::sync::OwnedMutexGuard;

use crate::cli::{self, InstallOutcome};
use crate::commands::ShellState;
use crate::config::{DEFAULT_SERVER_URL, LOCAL_ID};
use crate::remote::{self, ServerError};
use crate::server::{self, diag_log};

/// A large folder is indexed before the server starts listening.
const START_WAIT: Duration = Duration::from_secs(600);
const POLL: Duration = Duration::from_millis(400);

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum StartError {
    /// The user stopped it; only they start it again.
    StoppedByUser,
    NoCli(String),
    PortTaken,
    Failed(String),
    /// Stopped while it was starting.
    Cancelled,
    TimedOut,
}

impl StartError {
    pub fn code(&self) -> &'static str {
        match self {
            Self::StoppedByUser => "stopped_by_user",
            Self::NoCli(_) => "no_cli",
            Self::PortTaken => "port_taken",
            Self::Failed(_) => "failed",
            Self::Cancelled => "cancelled",
            Self::TimedOut => "timed_out",
        }
    }

    pub fn message(&self) -> String {
        match self {
            Self::StoppedByUser => "Hylo was stopped on this computer.".into(),
            Self::NoCli(why) => format!("Couldn't find Hylo's command-line tool. {why}"),
            Self::PortTaken => format!(
                "Another app is using {}, the address Hylo needs. Quit it and try again.",
                DEFAULT_SERVER_URL.trim_start_matches("http://")
            ),
            Self::Failed(why) => why.clone(),
            Self::Cancelled => "Hylo's local service was stopped while it was starting.".into(),
            Self::TimedOut => "Hylo's local service didn't start in time.".into(),
        }
    }
}

impl Serialize for StartError {
    fn serialize<S: Serializer>(&self, s: S) -> Result<S::Ok, S::Error> {
        let mut out = s.serialize_struct("StartError", 2)?;
        out.serialize_field("code", self.code())?;
        out.serialize_field("message", &self.message())?;
        out.end()
    }
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ProcessStatus {
    /// This app started it, now or in an earlier run.
    pub managed: bool,
    pub alive: bool,
    /// Why the server this app spawned exited, once it has.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub failure: Option<String>,
}

enum Proc {
    None,
    Running,
    Exited,
}

pub struct LocalServer {
    data_dir: PathBuf,
    archive: Option<PathBuf>,
    ops: Arc<tokio::sync::Mutex<()>>,
    /// Kept after it exits, so its failure can be reported.
    child: Mutex<Option<Child>>,
    /// Each lookup runs the CLI, so it is resolved once.
    bin: Mutex<Option<PathBuf>>,
}

fn lock<T>(m: &Mutex<T>) -> MutexGuard<'_, T> {
    m.lock().unwrap_or_else(|e| e.into_inner())
}

impl LocalServer {
    pub fn new(data_dir: PathBuf, archive: Option<PathBuf>) -> Self {
        Self {
            data_dir,
            archive,
            ops: Arc::new(tokio::sync::Mutex::new(())),
            child: Mutex::new(None),
            bin: Mutex::new(None),
        }
    }

    fn proc(&self) -> Proc {
        match lock(&self.child).as_mut().map(|c| c.try_wait()) {
            None => Proc::None,
            Some(Ok(None)) => Proc::Running,
            Some(_) => Proc::Exited,
        }
    }

    /// A server an earlier run of the app started and left running.
    fn leftover(&self) -> Option<u32> {
        server::read_pid(&self.data_dir).filter(|&p| server::pid_is_hylo(p))
    }

    pub fn status(&self) -> ProcessStatus {
        let proc = self.proc();
        let leftover = self.leftover().is_some();
        ProcessStatus {
            managed: !matches!(proc, Proc::None) || leftover,
            alive: matches!(proc, Proc::Running) || leftover,
            failure: matches!(proc, Proc::Exited).then(|| self.exit_reason().message()),
        }
    }

    pub fn bin(&self) -> Option<PathBuf> {
        let mut bin = lock(&self.bin);
        if bin.is_none() {
            *bin = server::resolve_hylo_bin();
        }
        bin.clone()
    }

    /// The local server's key, from the CLI. A key is never served over HTTP
    /// without credentials; the CLI reads the 0600 config file the server wrote
    /// it to, so only this OS user can get it.
    pub fn read_key(&self) -> Option<String> {
        read_key(&self.bin()?)
    }

    fn bin_or_install(&self) -> Result<PathBuf, StartError> {
        if let Some(bin) = self.bin() {
            return Ok(bin);
        }
        diag_log(&self.data_dir, "start-server: hylo not found, installing the bundled CLI");
        match cli::install(&self.data_dir, self.archive.clone(), true) {
            Ok(InstallOutcome::Installed) => self
                .bin()
                .ok_or_else(|| StartError::NoCli("Reinstalling it didn't give a working one.".into())),
            Ok(InstallOutcome::Skipped) => Err(StartError::NoCli(
                "Put `hylo` on your PATH or in ~/.local/bin.".into(),
            )),
            Err(e) => Err(StartError::NoCli(format!("Installing it failed: {e}"))),
        }
    }

    fn spawn(&self) -> Result<(), StartError> {
        let bin = self.bin_or_install()?;
        let out = server::diag_paths(&self.data_dir).server_output;
        let log = OpenOptions::new()
            .create(true)
            .append(true)
            .open(&out)
            .map_err(|e| StartError::Failed(format!("Couldn't open Hylo's log ({e}).")))?;
        // server::last_error reads from the last of these markers.
        let _ = writeln!(&log, "\n--- {} spawn hylo start server ---", server::stamp());
        let log_err = log
            .try_clone()
            .map_err(|e| StartError::Failed(format!("Couldn't open Hylo's log ({e}).")))?;

        let mut cmd = Command::new(&bin);
        cmd.args(["start", "server", "--pid-file"])
            .arg(server::pid_path(&self.data_dir))
            .env("PATH", server::expanded_path())
            .stdin(Stdio::null())
            .stdout(Stdio::from(log))
            .stderr(Stdio::from(log_err));
        #[cfg(unix)]
        {
            use std::os::unix::process::CommandExt;
            // Its own process group, so quitting the app leaves the server running.
            cmd.process_group(0);
        }
        #[cfg(windows)]
        {
            use std::os::windows::process::CommandExt;
            cmd.creation_flags(0x00000008 | 0x08000000);
        }
        let child = cmd.spawn().map_err(|e| {
            *lock(&self.bin) = None;
            StartError::Failed(format!("Couldn't run {}: {e}", bin.display()))
        })?;
        diag_log(&self.data_dir, &format!("start-server: pid {}, output in {out}", child.id()));
        *lock(&self.child) = Some(child);
        Ok(())
    }

    /// Stops the server this app is responsible for. Returns whether one was running.
    fn kill(&self) -> Result<bool, String> {
        let mut running = false;
        let child = lock(&self.child).take();
        if let Some(mut child) = child {
            if matches!(child.try_wait(), Ok(None)) {
                running = true;
                diag_log(&self.data_dir, &format!("stop-server: pid {}", child.id()));
                self.terminate(&mut child);
            }
        }
        if let Some(pid) = self.leftover() {
            running = true;
            diag_log(&self.data_dir, &format!("stop-server: pid {pid} from an earlier run"));
            if !server::kill_hylo(&self.data_dir, pid) {
                return Err("Hylo's local service did not exit.".into());
            }
        }
        Ok(running)
    }

    fn terminate(&self, child: &mut Child) {
        server::signal(child.id(), false);
        let deadline = Instant::now() + server::STOP_GRACE;
        while Instant::now() < deadline {
            if !matches!(child.try_wait(), Ok(None)) {
                return;
            }
            thread::sleep(Duration::from_millis(150));
        }
        diag_log(&self.data_dir, &format!("stop-server: pid {} ignored SIGTERM, killing", child.id()));
        let _ = child.kill();
        let _ = child.wait();
    }

    fn exit_reason(&self) -> StartError {
        let err = server::last_error(&self.data_dir).unwrap_or_default();
        if err.contains("address already in use") {
            StartError::PortTaken
        } else if err.is_empty() {
            StartError::Failed("Hylo's local service stopped unexpectedly.".into())
        } else {
            StartError::Failed(format!("Hylo's local service couldn't start: {err}"))
        }
    }
}

fn read_key(bin: &Path) -> Option<String> {
    // A GUI app starts with a minimal PATH; use the same one that launched the server.
    let out = Command::new(bin)
        .args(["auth", "show", "--raw"])
        .env("PATH", server::expanded_path())
        .stdin(Stdio::null())
        .output()
        .ok()?;
    if !out.status.success() {
        return None;
    }
    let key = String::from_utf8(out.stdout).ok()?.trim().to_string();
    (!key.is_empty() && !key.contains(char::is_whitespace)).then_some(key)
}

fn base_url(state: &ShellState) -> String {
    state
        .read_config(|cfg| cfg.find(LOCAL_ID).map(|s| s.url.clone()))
        .ok()
        .flatten()
        .unwrap_or_else(|| DEFAULT_SERVER_URL.into())
}

fn set_autostart(state: &ShellState, on: bool) {
    if state.read_config(|cfg| cfg.auto_start_server != on).unwrap_or(true) {
        let _ = state.edit_config(|cfg| {
            cfg.auto_start_server = on;
            Ok(())
        });
    }
}

async fn blocking<T: Send + 'static>(
    app: &AppHandle,
    f: impl FnOnce(&LocalServer) -> T + Send + 'static,
) -> T {
    let app = app.clone();
    tauri::async_runtime::spawn_blocking(move || f(&app.state::<ShellState>().local))
        .await
        .expect("local server task panicked")
}

/// Whether a Hylo answers at `base`; something else answering is an error.
async fn probe(base: &str) -> Result<bool, StartError> {
    match remote::ensure_hylo(base).await {
        Ok(()) => Ok(true),
        Err(ServerError::NotHylo) => Err(StartError::PortTaken),
        Err(_) => Ok(false),
    }
}

/// Returns once a Hylo answers on the local address, starting one if none
/// does. A start the app makes on its own respects the user having stopped it.
pub async fn ensure(app: &AppHandle, user_initiated: bool) -> Result<(), StartError> {
    let result = start_and_wait(app, user_initiated).await;
    if let Err(e) = &result {
        diag_log(&app.state::<ShellState>().data_dir, &format!("start-server: {}", e.message()));
    }
    result
}

async fn start_and_wait(app: &AppHandle, user_initiated: bool) -> Result<(), StartError> {
    let state = app.state::<ShellState>();
    let base = base_url(&state);
    {
        let _op = state.local.ops.lock().await;
        if probe(&base).await? {
            return Ok(());
        }
        if user_initiated {
            set_autostart(&state, true);
        } else if !state.read_config(|cfg| cfg.auto_start_server).unwrap_or(true) {
            return Err(StartError::StoppedByUser);
        }
        if !matches!(state.local.proc(), Proc::Running) {
            blocking(app, |l| l.spawn()).await?;
        }
    }
    let deadline = Instant::now() + START_WAIT;
    loop {
        if probe(&base).await? {
            return Ok(());
        }
        match state.local.proc() {
            Proc::Running => {}
            // Another Hylo may have taken the port as ours gave up.
            Proc::Exited if probe(&base).await? => return Ok(()),
            Proc::Exited => return Err(state.local.exit_reason()),
            Proc::None => return Err(StartError::Cancelled),
        }
        if Instant::now() >= deadline {
            return Err(StartError::TimedOut);
        }
        tokio::time::sleep(POLL).await;
    }
}

/// Stops the server and keeps it stopped until the user starts it.
pub async fn stop(app: &AppHandle) -> Result<bool, String> {
    let state = app.state::<ShellState>();
    let _op = state.local.ops.lock().await;
    set_autostart(&state, false);
    blocking(app, |l| l.kill()).await
}

/// Stops it without the user asking, e.g. to reopen it on another folder.
pub async fn kill(app: &AppHandle) -> Result<bool, String> {
    let state = app.state::<ShellState>();
    let _op = state.local.ops.lock().await;
    blocking(app, |l| l.kill()).await
}

/// Restarts the server this app runs. A server it didn't start (the CLI's) is
/// left alone: Ok(false).
pub async fn restart(app: &AppHandle) -> Result<bool, StartError> {
    let state = app.state::<ShellState>();
    let base = base_url(&state);
    let _op = state.local.ops.lock().await;
    let ours = blocking(app, |l| matches!(l.proc(), Proc::Running) || l.leftover().is_some()).await;
    if !ours && probe(&base).await? {
        return Ok(false);
    }
    blocking(app, |l| l.kill()).await.map_err(StartError::Failed)?;
    set_autostart(&state, true);
    blocking(app, |l| l.spawn()).await?;
    Ok(true)
}

/// Taken in setup, before any page can ask for a start, so the first start
/// runs the CLI this version of the app bundles.
pub fn hold_for_install(app: &AppHandle) -> OwnedMutexGuard<()> {
    app.state::<ShellState>()
        .local
        .ops
        .clone()
        .try_lock_owned()
        .expect("nothing else runs before setup")
}

pub async fn install_on_launch(app: AppHandle, _op: OwnedMutexGuard<()>) {
    let installed = blocking(&app, |l| cli::install(&l.data_dir, l.archive.clone(), false)).await;
    match installed {
        Ok(InstallOutcome::Installed) => {
            *lock(&app.state::<ShellState>().local.bin) = None;
            // A server left from an earlier run is the old version; the next start runs the new one.
            if let Err(e) = blocking(&app, |l| l.kill()).await {
                diag_log(&app.state::<ShellState>().data_dir, &format!("cli-upgrade: {e}"));
            }
        }
        Ok(InstallOutcome::Skipped) => {}
        Err(e) => diag_log(&app.state::<ShellState>().data_dir, &format!("cli auto-install failed: {e}")),
    }
}

#[cfg(all(test, unix))]
mod tests {
    use super::*;

    fn script(body: &str) -> PathBuf {
        use std::os::unix::fs::PermissionsExt;
        use std::sync::atomic::{AtomicUsize, Ordering};
        static N: AtomicUsize = AtomicUsize::new(0);
        let path = std::env::temp_dir().join(format!(
            "hylo-stub-{}-{}",
            std::process::id(),
            N.fetch_add(1, Ordering::Relaxed)
        ));
        std::fs::write(&path, format!("#!/bin/sh\n{body}\n")).unwrap();
        std::fs::set_permissions(&path, std::fs::Permissions::from_mode(0o755)).unwrap();
        path
    }

    #[test]
    fn local_key_comes_from_the_cli() {
        let ok = script(r#"[ "$1 $2 $3" = "auth show --raw" ] && echo hylo_abc123"#);
        assert_eq!(read_key(&ok).as_deref(), Some("hylo_abc123"));

        let no_key = script("echo 'Error: no API key yet' >&2; exit 1");
        assert_eq!(read_key(&no_key), None);

        let noisy = script("echo 'two words'");
        assert_eq!(read_key(&noisy), None);

        assert_eq!(read_key(Path::new("/nonexistent/hylo")), None);
    }
}
