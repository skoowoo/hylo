use std::collections::HashSet;

use serde::{Deserialize, Serialize};
use tauri::webview::cookie::{time::Duration as CookieAge, Cookie, SameSite};
use tauri::{AppHandle, Manager, State};

use crate::commands::ShellState;
use crate::config::{normalize_url, ServerEntry, ServerKind, LOCAL_ID};
use crate::local::{self, StartError};
use crate::notify::{Notifier, Target};
use crate::remote::{self, Health, ServerError, Session};
use crate::session::{self, show_start, show_start_at};

/// Rejection payload for the page: `code` picks the wording, `message` is the detail.
#[derive(Debug, Serialize)]
pub struct CmdError {
    code: &'static str,
    message: String,
}

impl From<ServerError> for CmdError {
    fn from(e: ServerError) -> Self {
        Self {
            code: e.code(),
            message: e.message(),
        }
    }
}

impl From<StartError> for CmdError {
    fn from(e: StartError) -> Self {
        Self {
            code: e.code(),
            message: e.message(),
        }
    }
}

impl From<String> for CmdError {
    fn from(message: String) -> Self {
        Self {
            code: "error",
            message,
        }
    }
}

impl From<&str> for CmdError {
    fn from(message: &str) -> Self {
        message.to_string().into()
    }
}

type CmdResult<T> = Result<T, CmdError>;

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ServerView {
    id: String,
    name: String,
    url: String,
    kind: ServerKind,
    has_key: bool,
    active: bool,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ServerList {
    servers: Vec<ServerView>,
    active_id: String,
}

#[derive(Debug, Serialize)]
pub struct Probe {
    id: String,
    health: Health,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AddOpts {
    name: Option<String>,
    url: String,
    key: Option<String>,
}

#[derive(Debug, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct CheckOpts {
    #[serde(default)]
    user_initiated: bool,
    /// Only look: the local server failed to start, and only the user starts it again.
    #[serde(default)]
    look_only: bool,
}

#[derive(Debug, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct UpdateOpts {
    name: Option<String>,
    key: Option<String>,
}

fn view(entry: &ServerEntry, active_id: &str) -> ServerView {
    ServerView {
        id: entry.id.clone(),
        name: entry.name.clone(),
        url: entry.url.clone(),
        kind: entry.kind,
        has_key: entry.key.is_some(),
        active: entry.id == active_id,
    }
}

fn snapshot(state: &ShellState) -> Result<ServerList, String> {
    state.read_config(|cfg| ServerList {
        servers: cfg
            .servers
            .iter()
            .map(|s| view(s, &cfg.active_id))
            .collect(),
        active_id: cfg.active_id.clone(),
    })
}

fn non_empty(s: Option<String>) -> Option<String> {
    s.map(|s| s.trim().to_string()).filter(|s| !s.is_empty())
}

#[tauri::command]
pub fn list_servers(state: State<ShellState>) -> CmdResult<ServerList> {
    Ok(snapshot(&state)?)
}

/// Leaves the server page for the shell's server manager, which is not served by any server.
#[tauri::command]
pub fn open_manager(app: AppHandle) {
    show_start_at(&app, "#servers");
}

#[tauri::command]
pub async fn add_server(app: AppHandle, opts: AddOpts) -> CmdResult<()> {
    let state = app.state::<ShellState>();
    let url = normalize_url(&opts.url)?;
    let key = non_empty(opts.key);
    remote::verify(&url, key.as_deref()).await?;

    let name = non_empty(opts.name).unwrap_or_default();
    state.edit_config(|cfg| cfg.add_remote(&name, &url, key).map(|_| ()))?;
    sync_notifiers(&app);
    Ok(())
}

#[tauri::command]
pub async fn update_server(app: AppHandle, id: String, opts: UpdateOpts) -> CmdResult<()> {
    let state = app.state::<ShellState>();
    let key = non_empty(opts.key);
    if let Some(key) = &key {
        let url = state
            .read_config(|cfg| cfg.find(&id).map(|s| s.url.clone()))?
            .ok_or("unknown server")?;
        remote::verify(&url, Some(key)).await?;
    }
    state.edit_config(|cfg| {
        if let Some(name) = &opts.name {
            cfg.rename(&id, name)?;
        }
        if let Some(key) = key {
            cfg.set_key(&id, key)?;
        }
        Ok(())
    })?;
    sync_notifiers(&app);
    Ok(())
}

#[tauri::command]
pub fn remove_server(app: AppHandle, id: String) -> CmdResult<ServerList> {
    let state = app.state::<ShellState>();
    state.edit_config(|cfg| cfg.remove(&id))?;
    sync_notifiers(&app);
    Ok(snapshot(&state)?)
}

/// A key the local server accepts. The stored one is tried first; the CLI is
/// asked (each read spawns it) only when there is none or it was refused,
/// since the server writes its key on first start and it can be reset.
pub async fn local_key(app: &AppHandle, base: &str) -> Result<Option<String>, ServerError> {
    let state = app.state::<ShellState>();
    let stored = state
        .read_config(|cfg| cfg.find(LOCAL_ID).and_then(|s| s.key.clone()))
        .ok()
        .flatten();
    if let Some(key) = &stored {
        match remote::check(base, Some(key)).await {
            Err(ServerError::BadKey) => {}
            other => return other.map(|_| stored),
        }
    }
    let handle = app.clone();
    let fresh =
        tauri::async_runtime::spawn_blocking(move || handle.state::<ShellState>().local.read_key())
            .await
            .ok()
            .flatten();
    remote::check(base, fresh.as_deref()).await?;
    if let Some(key) = &fresh {
        if fresh != stored {
            let _ = state.edit_config(|cfg| cfg.set_key(LOCAL_ID, key.clone()));
            sync_notifiers(app);
        }
    }
    Ok(fresh)
}

fn entry(app: &AppHandle, id: &str) -> Result<ServerEntry, String> {
    app.state::<ShellState>()
        .read_config(|cfg| cfg.find(id).cloned())?
        .ok_or_else(|| "unknown server".into())
}

/// Whether the server can be entered now: up, Hylo, and taking its key. The
/// local one is started first, and waited for.
#[tauri::command(rename_all = "camelCase")]
pub async fn check_server(app: AppHandle, id: String, opts: Option<CheckOpts>) -> CmdResult<()> {
    let opts = opts.unwrap_or_default();
    let entry = entry(&app, &id)?;
    match entry.kind {
        ServerKind::Local => {
            if !opts.look_only {
                local::ensure(&app, opts.user_initiated).await?;
            }
            local_key(&app, &entry.url).await?;
        }
        ServerKind::Remote => remote::check(&entry.url, entry.key.as_deref()).await?,
    }
    Ok(())
}

/// Makes `id` the server Hylo opens.
fn make_active(app: &AppHandle, id: &str) -> Result<ServerEntry, String> {
    app.state::<ShellState>().edit_config(|cfg| {
        let entry = cfg.find(id).cloned().ok_or("unknown server")?;
        cfg.active_id = entry.id.clone();
        if entry.kind == ServerKind::Local {
            cfg.auto_start_server = true;
        }
        Ok(entry)
    })
}

/// From a server's page. The shell page takes over and enters `id` behind the
/// logo, like every other way in; a remote that can't be entered is reported
/// here instead, so the current page stays.
#[tauri::command]
pub async fn switch_server(app: AppHandle, id: String) -> CmdResult<()> {
    let entry = entry(&app, &id)?;
    if entry.kind == ServerKind::Remote {
        remote::check(&entry.url, entry.key.as_deref()).await?;
    }
    make_active(&app, &id)?;
    show_start(&app);
    Ok(())
}

/// The shell page's last step in, once check_server passed: signs the window
/// in and loads the server.
#[tauri::command]
pub async fn enter_server(app: AppHandle, id: String) -> CmdResult<()> {
    let entry = make_active(&app, &id)?;
    let key = match entry.kind {
        ServerKind::Local => local_key(&app, &entry.url).await?,
        ServerKind::Remote => entry.key.clone(),
    };
    // With a key there is no login page: hand the window a ready session.
    if let Some(key) = &key {
        let session = remote::login(&entry.url, key).await?;
        inject_session(&app, &entry.url, &session);
    }
    sync_notifiers(&app);
    let state = app.state::<ShellState>();
    let landing = match entry.kind {
        ServerKind::Local => state.landing.lock().ok().and_then(|mut l| l.take()),
        ServerKind::Remote => None,
    };
    let target = format!("{}{}", entry.url, landing.as_deref().unwrap_or("/home"));
    session::open(&app, &entry.url, &target);
    Ok(())
}

#[tauri::command]
pub async fn probe_servers(app: AppHandle) -> CmdResult<Vec<Probe>> {
    let state = app.state::<ShellState>();
    let jobs: Vec<(String, String, Option<String>)> = state.read_config(|cfg| {
        cfg.servers
            .iter()
            .map(|s| (s.id.clone(), s.url.clone(), s.key.clone()))
            .collect()
    })?;
    let handles: Vec<_> = jobs
        .into_iter()
        .map(|(id, url, key)| {
            tauri::async_runtime::spawn(async move {
                Probe {
                    health: remote::health(&url, key.as_deref()).await,
                    id,
                }
            })
        })
        .collect();
    let mut out = Vec::new();
    for h in handles {
        if let Ok(p) = h.await {
            out.push(p);
        }
    }
    Ok(out)
}

fn inject_session(app: &AppHandle, url: &str, session: &Session) {
    let Some(window) = app.get_webview_window("main") else {
        return;
    };
    let Ok(parsed) = url::Url::parse(url) else {
        return;
    };
    let Some(host) = parsed.host_str() else {
        return;
    };
    let mut cookie = Cookie::build((session.name.clone(), session.value.clone()))
        .domain(host.to_string())
        .path("/")
        .http_only(true)
        .same_site(SameSite::Lax)
        .max_age(CookieAge::days(30));
    // wry hands any Some(secure) to NSHTTPCookie, which treats even `false` as Secure.
    if parsed.scheme() == "https" {
        cookie = cookie.secure(true);
    }
    let cookie = cookie.build();
    let _ = window.set_cookie(cookie);
}

/// One notification stream per server that can authenticate. Safe to call whenever
/// the registry or a key changes; unchanged streams are left running.
pub fn sync_notifiers(app: &AppHandle) {
    let state = app.state::<ShellState>();
    let wanted: Vec<(String, Target)> = {
        let Ok(cfg) = state.config.lock() else {
            return;
        };
        let multi = cfg.servers.len() > 1;
        cfg.servers
            .iter()
            .filter_map(|s| {
                // No key yet (the local server's first start): picked up on the next switch.
                let key = s.key.clone()?;
                Some((
                    s.id.clone(),
                    Target {
                        base_url: s.url.clone(),
                        key: Some(key),
                        label: multi.then(|| s.name.clone()),
                    },
                ))
            })
            .collect()
    };

    let Ok(mut live) = state.notifiers.lock() else {
        return;
    };
    let keep: HashSet<&String> = wanted.iter().map(|(id, _)| id).collect();
    let stale: Vec<String> = live
        .keys()
        .filter(|id| !keep.contains(id))
        .cloned()
        .collect();
    for id in stale {
        if let Some(h) = live.remove(&id) {
            h.shutdown();
        }
    }
    for (id, target) in wanted {
        if live
            .get(&id)
            .is_some_and(|h: &Notifier| h.matches(&target))
        {
            continue;
        }
        if let Some(old) = live.remove(&id) {
            old.shutdown();
        }
        live.insert(id, Notifier::start(app.clone(), target));
    }
}
