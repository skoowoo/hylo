use std::path::PathBuf;
use std::sync::Mutex;
use std::time::Duration;

use serde::{Deserialize, Serialize};
use tauri::{AppHandle, Manager, State};
use tauri_plugin_dialog::DialogExt;
use tauri_plugin_opener::OpenerExt;

use crate::cli::{self, InstallOutcome};
use crate::config::{ConfigFile, InboxNotify};
use crate::nav::{allow_navigation, http_origin, is_shell_url, shell_start_url};
use crate::notify::{self, InboxHandle};
use crate::server::{self, DiagPaths, ProcessStatus, SpawnResult};

pub struct ShellState {
    pub data_dir: PathBuf,
    pub archive: Option<PathBuf>,
    pub config: Mutex<ConfigFile>,
    pub managed_pid: Mutex<u32>,
    pub connected_url: Mutex<Option<String>>,
    pub inbox: Mutex<Option<InboxHandle>>,
}

impl ShellState {
    pub fn connected(&self) -> Option<String> {
        self.connected_url.lock().ok().and_then(|g| g.clone())
    }

    fn replace_inbox(&self, next: Option<InboxHandle>) {
        if let Ok(mut slot) = self.inbox.lock() {
            if let Some(prev) = slot.take() {
                prev.shutdown();
            }
            *slot = next;
        }
    }

    /// Drop the live server session the way Electron's resetToStartScreen does.
    pub fn end_session(&self) {
        if let Ok(mut slot) = self.connected_url.lock() {
            *slot = None;
        }
        self.replace_inbox(None);
    }
}

#[derive(Debug, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
struct StartOpts {
    #[serde(default)]
    user_initiated: bool,
}

#[derive(Debug, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
struct PickOpts {
    title: Option<String>,
    default_path: Option<String>,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct PickResult {
    canceled: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    path: Option<String>,
}

#[derive(Debug, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
struct InboxPatch {
    text_enabled: Option<bool>,
    sound_enabled: Option<bool>,
    sound: Option<String>,
}

#[tauri::command]
async fn check_server(url: String) -> bool {
    let url = format!("{}/home", url.trim().trim_end_matches('/'));
    // Any HTTP response means the process is up. Electron treats it the same way.
    http_responds(&url).await
}

async fn http_responds(url: &str) -> bool {
    let Ok(client) = reqwest::Client::builder()
        .timeout(Duration::from_secs(3))
        .no_proxy()
        .build()
    else {
        return false;
    };
    client.get(url).send().await.is_ok()
}

/// True only for a refused/unreachable host. Timeouts stay false so a slow
/// server is not treated as a failed navigation.
async fn origin_unreachable(base: &str) -> bool {
    let url = format!("{}/home", base.trim_end_matches('/'));
    let Ok(client) = reqwest::Client::builder()
        .timeout(Duration::from_secs(4))
        .connect_timeout(Duration::from_secs(2))
        .no_proxy()
        .build()
    else {
        return false;
    };
    match client.get(url).send().await {
        Ok(_) => false,
        Err(err) if err.is_timeout() => false,
        Err(err) => err.is_connect(),
    }
}

#[tauri::command(rename_all = "camelCase")]
async fn start_hylo_server_detached(app: AppHandle, opts: Option<StartOpts>) -> SpawnResult {
    let state = app.state::<ShellState>();
    let opts = opts.unwrap_or_default();
    if !opts.user_initiated && !server::autostart_enabled(&state.data_dir) {
        server::diag_log(
            &state.data_dir,
            "start-server: autoStart disabled, skip spawn",
        );
        return SpawnResult {
            ok: false,
            reason: Some("stopped_by_user".into()),
            error: None,
            pid: None,
        };
    }
    if opts.user_initiated {
        let _ = server::set_autostart(&state.data_dir, true);
        if let Ok(mut cfg) = state.config.lock() {
            cfg.auto_start_server = true;
        }
    }
    let managed = *state.managed_pid.lock().unwrap_or_else(|e| e.into_inner());
    if managed > 0 && server::pid_is_hylo(managed) {
        server::diag_log(
            &state.data_dir,
            &format!("start-server: managed server pid={managed} alive, skip duplicate spawn"),
        );
        return SpawnResult {
            ok: true,
            reason: None,
            error: None,
            pid: Some(managed),
        };
    }
    let data_dir = state.data_dir.clone();
    let archive = state.archive.clone();
    let result =
        tauri::async_runtime::spawn_blocking(move || server::start_server(&data_dir, archive))
            .await
            .unwrap_or_else(|e| SpawnResult {
                ok: false,
                reason: None,
                error: Some(e.to_string()),
                pid: None,
            });
    if let Some(pid) = result.pid {
        if let Ok(mut slot) = state.managed_pid.lock() {
            *slot = pid;
        }
    }
    result
}

#[tauri::command]
fn get_shell_debug_paths(state: State<ShellState>) -> DiagPaths {
    server::diag_paths(&state.data_dir)
}

#[tauri::command]
fn get_server_process_status(state: State<ShellState>) -> ProcessStatus {
    server::status(&state.data_dir)
}

#[tauri::command]
fn get_server_url(state: State<ShellState>) -> String {
    state
        .config
        .lock()
        .map(|c| c.server_url.clone())
        .unwrap_or_else(|_| crate::config::DEFAULT_SERVER_URL.to_string())
}

#[tauri::command]
fn set_server_url(app: AppHandle, url: String) -> Result<(), String> {
    let state = app.state::<ShellState>();
    let url = url.trim().trim_end_matches('/').to_string();
    {
        let mut cfg = state.config.lock().map_err(|e| e.to_string())?;
        cfg.server_url = url.clone();
        cfg.auto_start_server = true;
        cfg.save(&state.data_dir)?;
    }
    if let Ok(mut slot) = state.connected_url.lock() {
        *slot = Some(url.clone());
    }
    let inbox = InboxHandle::start(app.clone(), state.data_dir.clone(), url.clone());
    state.replace_inbox(Some(inbox));

    let target = format!("{url}/home");
    server::diag_log(&state.data_dir, &format!("navigate {target}"));
    navigate(&app, &target);
    Ok(())
}

#[tauri::command]
async fn stop_hylo_server(app: AppHandle) -> SpawnResult {
    let state = app.state::<ShellState>();
    let Some(pid) = server::read_pid(&state.data_dir) else {
        return SpawnResult {
            ok: false,
            reason: Some("no_pid".into()),
            error: Some("No managed server process found".into()),
            pid: None,
        };
    };
    state.end_session();
    server::diag_log(
        &state.data_dir,
        &format!("stop-server: stopping managed server pid {pid}"),
    );
    let data_dir = state.data_dir.clone();
    let killed = tauri::async_runtime::spawn_blocking(move || server::kill_hylo(&data_dir, pid))
        .await
        .unwrap_or(false);
    if let Ok(mut slot) = state.managed_pid.lock() {
        *slot = 0;
    }
    let _ = server::set_autostart(&state.data_dir, false);
    if let Ok(mut cfg) = state.config.lock() {
        cfg.auto_start_server = false;
    }
    show_start(&app);
    SpawnResult {
        ok: killed,
        reason: None,
        error: if killed {
            None
        } else {
            Some("process did not exit".into())
        },
        pid: None,
    }
}

#[tauri::command]
async fn restart_hylo_server(app: AppHandle) -> SpawnResult {
    let state = app.state::<ShellState>();
    let Some(pid) = server::read_pid(&state.data_dir) else {
        return SpawnResult {
            ok: false,
            reason: Some("no_pid".into()),
            error: Some("No valid PID file — server was not started by the desktop app".into()),
            pid: None,
        };
    };
    state.replace_inbox(None);
    if server::pid_is_hylo(pid) {
        server::diag_log(
            &state.data_dir,
            &format!("restart-server: stopping pid {pid}"),
        );
        let data_dir = state.data_dir.clone();
        let _ =
            tauri::async_runtime::spawn_blocking(move || server::kill_hylo(&data_dir, pid)).await;
    }
    if let Ok(mut slot) = state.managed_pid.lock() {
        *slot = 0;
    }
    let data_dir = state.data_dir.clone();
    let archive = state.archive.clone();
    let spawned =
        tauri::async_runtime::spawn_blocking(move || server::start_server(&data_dir, archive))
            .await
            .unwrap_or_else(|e| SpawnResult {
                ok: false,
                reason: Some("spawn_failed".into()),
                error: Some(e.to_string()),
                pid: None,
            });
    if !spawned.ok {
        return SpawnResult {
            ok: false,
            reason: Some("spawn_failed".into()),
            error: spawned.error,
            pid: None,
        };
    }
    if let Some(pid) = spawned.pid {
        if let Ok(mut slot) = state.managed_pid.lock() {
            *slot = pid;
        }
    }
    state.end_session();
    show_start(&app);
    spawned
}

#[tauri::command(rename_all = "camelCase")]
async fn pick_folder(app: AppHandle, opts: Option<PickOpts>) -> PickResult {
    let opts = opts.unwrap_or_default();
    let mut dialog = app.dialog().file();
    if let Some(title) = opts.title.filter(|s| !s.is_empty()) {
        dialog = dialog.set_title(title);
    } else {
        dialog = dialog.set_title("Select Folder");
    }
    let default_path = opts
        .default_path
        .filter(|s| !s.is_empty())
        .unwrap_or_else(|| {
            std::env::var("HOME")
                .or_else(|_| std::env::var("USERPROFILE"))
                .unwrap_or_else(|_| ".".into())
        });
    dialog = dialog.set_directory(default_path);
    dialog = dialog.set_can_create_directories(true);
    // Callback form: blocking_pick_folder from a sync command deadlocks the main thread.
    let (tx, rx) = tokio::sync::oneshot::channel();
    dialog.pick_folder(move |file| {
        let _ = tx.send(file);
    });
    match rx.await.ok().flatten() {
        Some(file) => match file.into_path() {
            Ok(path) => PickResult {
                canceled: false,
                path: Some(path.display().to_string()),
            },
            Err(file) => PickResult {
                canceled: false,
                path: Some(file.to_string()),
            },
        },
        None => PickResult {
            canceled: true,
            path: None,
        },
    }
}

#[tauri::command]
fn set_view_bg_color(app: AppHandle, color: String, theme: String) {
    let _ = color;
    let parsed = match theme.as_str() {
        "light" => Some(tauri::Theme::Light),
        "dark" => Some(tauri::Theme::Dark),
        _ => None,
    };
    if let Some(window) = app.get_webview_window("main") {
        let _ = window.set_theme(parsed);
    }
}

#[tauri::command]
fn toggle_maximize(window: tauri::WebviewWindow) {
    crate::zoom::toggle(&window);
}

#[tauri::command]
fn set_window_button_visibility(_visible: bool) {
    // The Go UI exposes this and never calls it. Kept so the shim matches Electron.
}

#[tauri::command]
fn inbox_notify_get_settings(state: State<ShellState>) -> InboxNotify {
    state
        .config
        .lock()
        .map(|c| c.inbox_notify.clone())
        .unwrap_or_default()
}

#[tauri::command(rename_all = "camelCase")]
fn inbox_notify_set_settings(
    state: State<ShellState>,
    settings: InboxPatch,
) -> Result<InboxNotify, String> {
    let mut cfg = state.config.lock().map_err(|e| e.to_string())?;
    if let Some(v) = settings.text_enabled {
        cfg.inbox_notify.text_enabled = v;
    }
    if let Some(v) = settings.sound_enabled {
        cfg.inbox_notify.sound_enabled = v;
    }
    if let Some(v) = settings.sound {
        cfg.inbox_notify.sound = v;
    }
    let out = cfg.inbox_notify.clone();
    cfg.save(&state.data_dir)?;
    Ok(out)
}

#[tauri::command]
fn inbox_notify_preview_sound(sound: String) {
    notify::play_sound(&sound);
}

pub fn show_start(app: &AppHandle) {
    let url = shell_start_url().to_string();
    let app2 = app.clone();
    std::thread::spawn(move || {
        std::thread::sleep(Duration::from_millis(60));
        let nav = app2.clone();
        let _ = app2.run_on_main_thread(move || navigate(&nav, &url));
    });
}

fn navigate(app: &AppHandle, url: &str) {
    let Ok(parsed) = url.parse() else {
        return;
    };
    if let Some(window) = app.get_webview_window("main") {
        let _ = window.navigate(parsed);
    }
}

pub fn allow_url(app: &AppHandle, url: &url::Url) -> bool {
    let state = app.state::<ShellState>();
    let server = state.connected();
    if allow_navigation(url, server.as_deref()) {
        if !is_shell_url(url) {
            // Electron resets to the start screen on did-fail-load. Tauri has
            // no failure event, so a refused connection to this origin is the signal.
            watch_server_navigation(app.clone(), url.clone());
        }
        return true;
    }
    let _ = app.opener().open_url(url.as_str(), None::<&str>);
    false
}

fn watch_server_navigation(app: AppHandle, url: url::Url) {
    let Some(origin) = http_origin(&url) else {
        return;
    };
    tauri::async_runtime::spawn(async move {
        if !origin_unreachable(&origin).await {
            return;
        }
        recover_failed_navigation(&app, &url);
    });
}

fn recover_failed_navigation(app: &AppHandle, url: &url::Url) {
    let state = app.state::<ShellState>();
    let Some(saved) = state.connected() else {
        return;
    };
    if !crate::nav::same_origin(url, &saved) {
        return;
    }
    if let Some(window) = app.get_webview_window("main") {
        if let Ok(current) = window.url() {
            if is_shell_url(&current) {
                return;
            }
        }
    }
    server::diag_log(
        &state.data_dir,
        &format!("page load failed, returning to start ({url})"),
    );
    state.end_session();
    show_start(app);
}

pub fn install_cli_on_launch(app: &AppHandle) {
    let state = app.state::<ShellState>();
    let data_dir = state.data_dir.clone();
    let archive = state.archive.clone();
    match cli::install(&data_dir, archive, false) {
        Ok(InstallOutcome::Installed) => {
            if let Some(pid) = server::read_pid(&data_dir) {
                if server::pid_is_hylo(pid) {
                    server::diag_log(
                        &data_dir,
                        &format!("cli-upgrade: killing old server pid {pid}"),
                    );
                    let _ = server::kill_hylo(&data_dir, pid);
                    if let Ok(mut slot) = app.state::<ShellState>().managed_pid.lock() {
                        *slot = 0;
                    }
                    show_start(app);
                }
            }
        }
        Ok(InstallOutcome::Skipped) => {}
        Err(e) => server::diag_log(&data_dir, &format!("cli auto-install failed: {e}")),
    }
}

pub fn invoke_handler() -> impl Fn(tauri::ipc::Invoke<tauri::Wry>) -> bool + Send + Sync + 'static {
    tauri::generate_handler![
        check_server,
        start_hylo_server_detached,
        stop_hylo_server,
        get_server_process_status,
        get_shell_debug_paths,
        get_server_url,
        set_server_url,
        restart_hylo_server,
        pick_folder,
        set_view_bg_color,
        set_window_button_visibility,
        inbox_notify_get_settings,
        inbox_notify_set_settings,
        inbox_notify_preview_sound,
        toggle_maximize,
    ]
}
