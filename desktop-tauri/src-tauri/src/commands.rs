use std::collections::HashMap;
use std::path::PathBuf;
use std::sync::Mutex;

use serde::{Deserialize, Serialize};
use tauri::{AppHandle, Manager, State};
use tauri_plugin_dialog::DialogExt;

use crate::config::{ConfigFile, InboxNotify, LOCAL_ID};
use crate::local::{self, LocalServer, ProcessStatus};
use crate::notify::{self, InboxHandle};
use crate::server::{self, DiagPaths};
use crate::servers;
use crate::session::show_start;

pub struct ShellState {
    pub data_dir: PathBuf,
    pub config: Mutex<ConfigFile>,
    pub local: LocalServer,
    pub connected_url: Mutex<Option<String>>,
    /// Where the next entry into the local server lands, set once by onboarding.
    pub landing: Mutex<Option<String>>,
    /// Inbox stream per server id; see servers::sync_inboxes.
    pub inboxes: Mutex<HashMap<String, InboxHandle>>,
}

impl ShellState {
    pub fn connected(&self) -> Option<String> {
        self.connected_url.lock().ok().and_then(|g| g.clone())
    }

    pub fn read_config<T>(&self, f: impl FnOnce(&ConfigFile) -> T) -> Result<T, String> {
        let cfg = self.config.lock().map_err(|e| e.to_string())?;
        Ok(f(&cfg))
    }

    /// Runs `f` under the config lock and persists only if it succeeds.
    pub fn edit_config<T>(
        &self,
        f: impl FnOnce(&mut ConfigFile) -> Result<T, String>,
    ) -> Result<T, String> {
        let mut cfg = self.config.lock().map_err(|e| e.to_string())?;
        let out = f(&mut cfg)?;
        cfg.save(&self.data_dir)?;
        Ok(out)
    }

    /// Drop the live server session; the shell goes back to its start page.
    pub fn end_session(&self) {
        if let Ok(mut slot) = self.connected_url.lock() {
            *slot = None;
        }
    }
}

#[derive(Debug, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
struct ControlOpts {
    /// The servers view acts in place; the app's own pages are sent back to it.
    #[serde(default)]
    stay: bool,
}

/// Stop and restart answers; the Go settings page reads these fields.
#[derive(Debug, Serialize, Default)]
#[serde(rename_all = "camelCase")]
struct ControlResult {
    ok: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    reason: Option<&'static str>,
    #[serde(skip_serializing_if = "Option::is_none")]
    error: Option<String>,
}

impl ControlResult {
    fn failed(reason: &'static str, error: String) -> Self {
        Self { ok: false, reason: Some(reason), error: Some(error) }
    }
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
fn get_shell_debug_paths(state: State<ShellState>) -> DiagPaths {
    server::diag_paths(&state.data_dir)
}

#[tauri::command]
fn get_server_process_status(state: State<ShellState>) -> ProcessStatus {
    state.local.status()
}

/// Remote pages may call these too (see capabilities/remote.json); only the
/// shell page and the local server's own pages control this computer's server.
fn may_control(app: &AppHandle) -> bool {
    let state = app.state::<ShellState>();
    match state.connected() {
        None => true,
        Some(url) => state
            .read_config(|cfg| cfg.find(LOCAL_ID).is_some_and(|s| s.url == url))
            .unwrap_or(false),
    }
}

#[tauri::command]
async fn stop_hylo_server(app: AppHandle, opts: Option<ControlOpts>) -> ControlResult {
    if !may_control(&app) {
        return ControlResult::failed("forbidden", "Only this computer's Hylo can stop it.".into());
    }
    let result = match local::stop(&app).await {
        Ok(true) => ControlResult { ok: true, ..Default::default() },
        Ok(false) => ControlResult::failed("no_pid", "No server started by this app is running.".into()),
        Err(e) => ControlResult::failed("stop_failed", e),
    };
    if !opts.unwrap_or_default().stay {
        show_start(&app);
    }
    result
}

#[tauri::command]
async fn restart_hylo_server(app: AppHandle, opts: Option<ControlOpts>) -> ControlResult {
    if !may_control(&app) {
        return ControlResult::failed("forbidden", "Only this computer's Hylo can restart it.".into());
    }
    let result = match local::restart(&app).await {
        Ok(true) => ControlResult { ok: true, ..Default::default() },
        // Started from the CLI: it picks up the new config when restarted there.
        Ok(false) => ControlResult::failed("no_pid", "Hylo wasn't started by this app.".into()),
        Err(e) => ControlResult::failed(e.code(), e.message()),
    };
    if result.ok && !opts.unwrap_or_default().stay {
        show_start(&app);
    }
    result
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
fn set_view_bg_color(app: AppHandle, state: State<ShellState>, color: String, theme: String) {
    let _ = color;
    let parsed = crate::config::theme_choice(&theme);
    if let Some(window) = app.get_webview_window("main") {
        let _ = window.set_theme(parsed);
    }
    // Every page load calls this; only a real change touches the file.
    let theme = if parsed.is_some() { theme } else { String::new() };
    if state.read_config(|cfg| cfg.theme != theme).unwrap_or(false) {
        let _ = state.edit_config(|cfg| {
            cfg.theme = theme;
            Ok(())
        });
    }
}

#[tauri::command]
fn toggle_maximize(window: tauri::WebviewWindow) {
    crate::zoom::toggle(&window);
}

#[tauri::command]
fn set_window_button_visibility(_visible: bool) {
    // The Go UI exposes this and never calls it; kept so the shim API stays complete.
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

pub fn invoke_handler() -> impl Fn(tauri::ipc::Invoke<tauri::Wry>) -> bool + Send + Sync + 'static {
    tauri::generate_handler![
        servers::check_server,
        stop_hylo_server,
        get_server_process_status,
        get_shell_debug_paths,
        servers::list_servers,
        servers::add_server,
        servers::update_server,
        servers::remove_server,
        servers::switch_server,
        servers::enter_server,
        servers::probe_servers,
        servers::open_manager,
        restart_hylo_server,
        pick_folder,
        set_view_bg_color,
        set_window_button_visibility,
        inbox_notify_get_settings,
        inbox_notify_set_settings,
        inbox_notify_preview_sound,
        toggle_maximize,
        crate::onboarding::onboarding_get,
        crate::onboarding::onboarding_set_step,
        crate::onboarding::onboarding_finish,
        crate::vault_setup::onboarding_vaults,
        crate::vault_setup::onboarding_set_vault,
        crate::shell_path::onboarding_path_needed,
        crate::shell_path::onboarding_add_to_path,
        crate::agent_setup::onboarding_ready,
        crate::agent_setup::onboarding_agents,
        crate::agent_setup::onboarding_hello,
        crate::agent_setup::onboarding_create_bot,
    ]
}
