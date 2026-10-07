use std::collections::HashSet;

use serde::{Deserialize, Serialize};
use tauri::webview::cookie::{time::Duration as CookieAge, Cookie, SameSite};
use tauri::{AppHandle, Manager, State};

use crate::commands::{navigate, show_start_at, ShellState};
use crate::config::{normalize_url, ServerEntry, ServerKind, LOCAL_ID};
use crate::notify::{InboxHandle, Target};
use crate::remote::{self, Health, ServerError, Session};
use crate::server;

/// Rejection payload for the page: `code` picks the wording, `message` is the detail.
#[derive(Debug, Serialize)]
pub struct CmdError {
    code: &'static str,
    message: String,
}

impl From<ServerError> for CmdError {
    fn from(e: ServerError) -> Self {
        Self { code: e.code(), message: e.message() }
    }
}

impl From<String> for CmdError {
    fn from(message: String) -> Self {
        Self { code: "error", message }
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
        servers: cfg.servers.iter().map(|s| view(s, &cfg.active_id)).collect(),
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
    app.state::<ShellState>().end_session();
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
    sync_inboxes(&app);
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
    sync_inboxes(&app);
    Ok(())
}

#[tauri::command]
pub async fn remove_server(app: AppHandle, id: String) -> CmdResult<ServerList> {
    let state = app.state::<ShellState>();
    let was_active = state.edit_config(|cfg| {
        let was_active = cfg.active_id == id;
        cfg.remove(&id)?;
        Ok(was_active)
    })?;
    sync_inboxes(&app);
    if was_active {
        let local = state.read_config(|cfg| cfg.active().clone())?;
        enter(&app, &local).await;
    }
    Ok(snapshot(&state)?)
}

/// The local server writes its own key on first start; ask the CLI for the
/// current one and remember it. Falls back to the last stored key.
async fn refresh_local_key(app: &AppHandle) -> Option<String> {
    let state = app.state::<ShellState>();
    let fresh = match server::resolve_hylo_bin() {
        Some(bin) => tauri::async_runtime::spawn_blocking(move || remote::read_local_key(&bin))
            .await
            .ok()
            .flatten(),
        None => None,
    };
    match fresh {
        Some(key) => {
            let stale = state
                .read_config(|cfg| cfg.find(LOCAL_ID).and_then(|s| s.key.as_deref()) != Some(key.as_str()))
                .ok()?;
            if stale {
                state.edit_config(|cfg| cfg.set_key(LOCAL_ID, key.clone())).ok()?;
            }
            Some(key)
        }
        None => state.read_config(|cfg| cfg.find(LOCAL_ID).and_then(|s| s.key.clone())).ok()?,
    }
}

#[tauri::command]
pub async fn switch_server(app: AppHandle, id: String) -> CmdResult<()> {
    let state = app.state::<ShellState>();
    let entry = state.edit_config(|cfg| {
        let entry = cfg.find(&id).cloned().ok_or("unknown server")?;
        cfg.active_id = entry.id.clone();
        cfg.auto_start_server = true;
        Ok(entry)
    })?;
    enter(&app, &entry).await;
    Ok(())
}

/// Signs the webview in when a key is available, then loads the server.
async fn enter(app: &AppHandle, entry: &ServerEntry) {
    let state = app.state::<ShellState>();
    let key = match entry.kind {
        ServerKind::Local => refresh_local_key(app).await,
        ServerKind::Remote => entry.key.clone(),
    };

    // With a key there is no login page: hand the webview a ready session.
    if let Some(key) = key {
        match remote::login(&entry.url, &key).await {
            Ok(session) => inject_session(app, &entry.url, &session),
            Err(e) => server::diag_log(
                &state.data_dir,
                &format!("switch {}: auto-login skipped: {}", entry.name, e.code()),
            ),
        }
    }

    if let Ok(mut slot) = state.connected_url.lock() {
        *slot = Some(entry.url.clone());
    }
    sync_inboxes(app);
    let target = format!("{}/home", entry.url);
    server::diag_log(&state.data_dir, &format!("navigate {target}"));
    navigate(app, &target);
}

#[tauri::command]
pub async fn probe_servers(app: AppHandle) -> CmdResult<Vec<Probe>> {
    let state = app.state::<ShellState>();
    let jobs: Vec<(String, String, Option<String>)> = state.read_config(|cfg| {
        cfg.servers.iter().map(|s| (s.id.clone(), s.url.clone(), s.key.clone())).collect()
    })?;
    let handles: Vec<_> = jobs
        .into_iter()
        .map(|(id, url, key)| {
            tauri::async_runtime::spawn(async move {
                Probe { health: remote::health(&url, key.as_deref()).await, id }
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

/// One inbox stream per server that can authenticate. Safe to call whenever
/// the registry or a key changes; unchanged streams are left running.
pub fn sync_inboxes(app: &AppHandle) {
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
                    Target { base_url: s.url.clone(), key: Some(key), label: multi.then(|| s.name.clone()) },
                ))
            })
            .collect()
    };

    let Ok(mut live) = state.inboxes.lock() else {
        return;
    };
    let keep: HashSet<&String> = wanted.iter().map(|(id, _)| id).collect();
    let stale: Vec<String> = live.keys().filter(|id| !keep.contains(id)).cloned().collect();
    for id in stale {
        if let Some(h) = live.remove(&id) {
            h.shutdown();
        }
    }
    for (id, target) in wanted {
        if live.get(&id).is_some_and(|h: &InboxHandle| h.matches(&target)) {
            continue;
        }
        if let Some(old) = live.remove(&id) {
            old.shutdown();
        }
        live.insert(id, InboxHandle::start(app.clone(), state.data_dir.clone(), target));
    }
}
