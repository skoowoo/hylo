// The window's session with a server: which pages it may show, and noticing
// when that server goes away so the shell page can take over again.
use std::time::Duration;

use tauri::{AppHandle, Manager};
use tauri_plugin_opener::OpenerExt;
use url::Url;

use crate::commands::ShellState;
use crate::nav::{allow_navigation, is_shell_url, same_origin, shell_start_url};
use crate::remote;
use crate::server::diag_log;

const HEARTBEAT_EVERY: Duration = Duration::from_secs(10);
/// Two misses in a row, so a blip after waking from sleep doesn't kick the user out.
const HEARTBEAT_MISSES: u32 = 2;

/// Signs the window in to `base` and loads `target` there.
pub fn open(app: &AppHandle, base: &str, target: &str) {
    let state = app.state::<ShellState>();
    if let Ok(mut slot) = state.connected_url.lock() {
        *slot = Some(base.to_string());
    }
    diag_log(&state.data_dir, &format!("navigate {target}"));
    navigate(app, target);
}

pub fn show_start(app: &AppHandle) {
    show_start_at(app, "");
}

/// `hash` picks the shell page's view, e.g. "#servers". Leaving the server's
/// pages always ends the session; see allow_url.
pub fn show_start_at(app: &AppHandle, hash: &str) {
    app.state::<ShellState>().end_session();
    let url = format!("{}{hash}", shell_start_url());
    let app2 = app.clone();
    // Lets the command that asked for this answer its page before the page goes.
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

pub fn allow_url(app: &AppHandle, url: &Url) -> bool {
    let state = app.state::<ShellState>();
    let server = state.connected();
    // Only we send the window back to the shell, and we end the session first;
    // a shell page while still connected is the mouse's back button.
    if is_shell_url(url) && server.is_some() {
        return false;
    }
    if allow_navigation(url, server.as_deref()) {
        if !is_shell_url(url) {
            // Tauri has no load-failure event, so a server that is down as its
            // page loads is the signal to fall back to the start page.
            let (app, url) = (app.clone(), url.clone());
            tauri::async_runtime::spawn(async move {
                if connected_down(&app).await {
                    leave(&app, &url, "page load failed");
                }
            });
        }
        return true;
    }
    // History (the mouse's back button) can reach a page of a server we're no
    // longer signed in to; that's not a link to open in the browser.
    let registered = state
        .read_config(|cfg| cfg.servers.iter().any(|s| same_origin(url, &s.url)))
        .unwrap_or(false);
    if registered {
        diag_log(
            &state.data_dir,
            &format!("navigation to {url} blocked: not connected"),
        );
        return false;
    }
    let _ = app.opener().open_url(url.as_str(), None::<&str>);
    false
}

/// A server can die after its page loaded; nothing else notices until the next
/// navigation. The servers view then reconnects, or restarts the local one.
pub fn start_heartbeat(app: AppHandle) {
    tauri::async_runtime::spawn(async move {
        let mut misses = 0;
        loop {
            tokio::time::sleep(HEARTBEAT_EVERY).await;
            let Some(page) = server_page(&app) else {
                misses = 0;
                continue;
            };
            if !connected_down(&app).await {
                misses = 0;
                continue;
            }
            misses += 1;
            if misses >= HEARTBEAT_MISSES {
                misses = 0;
                leave(&app, &page, "heartbeat: server down");
            }
        }
    });
}

/// Down for sure, not merely slow: a busy server must keep its session.
async fn connected_down(app: &AppHandle) -> bool {
    let state = app.state::<ShellState>();
    let Some(base) = state.connected() else {
        return false;
    };
    let key = state
        .read_config(|cfg| cfg.find_by_url(&base).and_then(|s| s.key.clone()))
        .ok()
        .flatten();
    remote::check(&base, key.as_deref())
        .await
        .is_err_and(|e| e.is_down())
}

/// The page on screen, when it belongs to the connected server.
fn server_page(app: &AppHandle) -> Option<Url> {
    let saved = app.state::<ShellState>().connected()?;
    let current = app.get_webview_window("main")?.url().ok()?;
    (!is_shell_url(&current) && same_origin(&current, &saved)).then_some(current)
}

/// Back to the shell page, unless the window already left `url`'s server.
fn leave(app: &AppHandle, url: &Url, why: &str) {
    let state = app.state::<ShellState>();
    let Some(saved) = state.connected() else {
        return;
    };
    if !same_origin(url, &saved) {
        return;
    }
    let on_shell = app
        .get_webview_window("main")
        .and_then(|w| w.url().ok())
        .is_some_and(|current| is_shell_url(&current));
    if on_shell {
        return;
    }
    diag_log(
        &state.data_dir,
        &format!("{why}, returning to start ({url})"),
    );
    show_start(app);
}
