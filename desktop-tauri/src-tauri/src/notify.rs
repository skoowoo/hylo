use std::process::Command;
use std::time::Duration;

use serde::Deserialize;
use tauri::{AppHandle, Manager};
use tokio::sync::watch;

use crate::commands::ShellState;

const RETRY_FIRST: Duration = Duration::from_secs(5);
/// A server that stays away (a laptop's remote, off for the night) is asked rarely.
const RETRY_MAX: Duration = Duration::from_secs(120);

/// Connection to one server's live feed, for bots reaching out.
#[derive(Clone, PartialEq, Eq)]
pub struct Target {
    pub base_url: String,
    pub key: Option<String>,
    /// Prefixes notification titles so several servers stay distinguishable.
    pub label: Option<String>,
}

pub struct Notifier {
    stop: watch::Sender<bool>,
    /// What the loop was started with, to detect a changed URL, key or label.
    target: Target,
}

impl Notifier {
    pub fn start(app: AppHandle, target: Target) -> Self {
        let (stop, rx) = watch::channel(false);
        let handle = Self {
            stop,
            target: target.clone(),
        };
        tauri::async_runtime::spawn(async move {
            run_loop(app, target, rx).await;
        });
        handle
    }

    pub fn matches(&self, target: &Target) -> bool {
        self.target == *target
    }

    pub fn shutdown(self) {
        let _ = self.stop.send(true);
    }
}

enum Loop {
    Stopped,
    /// `true` once the stream was open: the server was there, so retry soon.
    Retry(bool),
}

async fn run_loop(app: AppHandle, target: Target, mut stop: watch::Receiver<bool>) {
    let Some(endpoint) = join_live(&target.base_url) else {
        return;
    };
    let Ok(client) = reqwest::Client::builder().no_proxy().build() else {
        return;
    };
    let mut wait = RETRY_FIRST;
    loop {
        if *stop.borrow() {
            break;
        }
        match connect(&client, &app, &endpoint, &target, &mut stop).await {
            Loop::Stopped => break,
            Loop::Retry(was_open) => {
                wait = if was_open {
                    RETRY_FIRST
                } else {
                    (wait * 2).min(RETRY_MAX)
                };
                tokio::select! {
                    _ = stop.changed() => {
                        if *stop.borrow() { break; }
                    }
                    _ = tokio::time::sleep(wait) => {}
                }
            }
        }
    }
}

async fn connect(
    client: &reqwest::Client,
    app: &AppHandle,
    endpoint: &str,
    target: &Target,
    stop: &mut watch::Receiver<bool>,
) -> Loop {
    let mut req = client.get(endpoint);
    if let Some(key) = target.key.as_deref().filter(|k| !k.is_empty()) {
        req = req.header("X-Hylo-API-Key", key);
    }
    let mut resp = match req.send().await {
        Ok(r) if r.status().is_success() => r,
        _ => return Loop::Retry(false),
    };
    let mut buf = String::new();
    loop {
        if *stop.borrow() {
            return Loop::Stopped;
        }
        tokio::select! {
            changed = stop.changed() => {
                if changed.is_err() || *stop.borrow() {
                    return Loop::Stopped;
                }
            }
            chunk = tokio::time::timeout(Duration::from_secs(45), resp.chunk()) => {
                match chunk {
                    Ok(Ok(Some(bytes))) => {
                        buf.push_str(&String::from_utf8_lossy(&bytes));
                        for raw in drain_events(&mut buf) {
                            show_wake(app, &raw, target.label.as_deref());
                        }
                    }
                    _ => return Loop::Retry(true),
                }
            }
        }
    }
}

pub fn drain_events(buf: &mut String) -> Vec<String> {
    let mut out = Vec::new();
    while let Some(idx) = buf.find("\n\n") {
        let block = buf[..idx].to_string();
        buf.replace_range(..idx + 2, "");
        for line in block.lines() {
            let Some(rest) = line.strip_prefix("data:") else {
                continue;
            };
            let data = rest.trim();
            if !data.is_empty() {
                out.push(data.to_string());
            }
        }
    }
    out
}

/// One change on the server's live feed (internal/live).
#[derive(Debug, Deserialize)]
struct Change {
    topic: String,
    op: String,
    data: Option<WakeEnd>,
}

/// A bot's turn it started on its own, now over (handler.WakeEnd).
#[derive(Debug, Deserialize)]
struct WakeEnd {
    bot: Option<String>,
    status: Option<String>,
    #[serde(default)]
    silent: bool,
    /// The user ran it again and is watching it.
    #[serde(default)]
    retry: bool,
    text: Option<String>,
}

/// What a bot sent on its own, unless it chose to say nothing.
fn wake_message(raw: &str) -> Option<(String, String)> {
    let c = serde_json::from_str::<Change>(raw).ok()?;
    if c.topic != "chat" || c.op != "wake" {
        return None;
    }
    let w = c.data?;
    if w.silent || w.retry || w.status.as_deref() != Some("succeeded") {
        return None;
    }
    let title = w
        .bot
        .filter(|s| !s.is_empty())
        .unwrap_or_else(|| "Hylo".to_string());
    let body = w
        .text
        .unwrap_or_default()
        .trim()
        .chars()
        .take(150)
        .collect();
    Some((title, body))
}

fn show_wake(app: &AppHandle, raw: &str, label: Option<&str>) {
    let Some((title, body)) = wake_message(raw) else {
        return;
    };
    let Ok(settings) = app
        .state::<ShellState>()
        .read_config(|cfg| cfg.notify.clone())
    else {
        return;
    };
    if settings.text_enabled {
        let title = match label {
            Some(l) => format!("[{l}] {title}"),
            None => title,
        };
        let handle = app.clone();
        let _ = app.run_on_main_thread(move || {
            let _ = notify(&handle, &title, &body);
        });
    }
    if settings.sound_enabled {
        play_sound(&settings.sound);
    }
}

fn notify(app: &AppHandle, title: &str, body: &str) -> Result<(), String> {
    use tauri_plugin_notification::NotificationExt;
    app.notification()
        .builder()
        .title(title)
        .body(body)
        .show()
        .map_err(|e| e.to_string())
}

pub fn play_sound(sound: &str) {
    if sound.is_empty() || sound == "none" {
        return;
    }
    #[cfg(target_os = "macos")]
    {
        let path = if sound == "beep" {
            "/System/Library/Sounds/Funk.aiff".to_string()
        } else if sound.starts_with('/') {
            sound.to_string()
        } else {
            format!("/System/Library/Sounds/{sound}.aiff")
        };
        let _ = Command::new("/usr/bin/afplay")
            .arg(path)
            .stdin(std::process::Stdio::null())
            .stdout(std::process::Stdio::null())
            .stderr(std::process::Stdio::null())
            .spawn();
    }
    #[cfg(windows)]
    {
        let _ = sound;
        let _ = Command::new("powershell")
            .args(["-NoProfile", "-Command", "[console]::beep(880,120)"])
            .spawn();
    }
}

fn join_live(base: &str) -> Option<String> {
    let base = base.trim().trim_end_matches('/');
    if base.is_empty() {
        return None;
    }
    Some(format!("{base}/api/live"))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn splits_sse_blocks() {
        let mut buf = "data: {\"title\":\"A\"}\n\ndata: {\"title\":\"B\"}\n\npartial".to_string();
        let ev = drain_events(&mut buf);
        assert_eq!(ev, vec!["{\"title\":\"A\"}", "{\"title\":\"B\"}"]);
        assert_eq!(buf, "partial");
    }

    #[test]
    fn notifies_only_what_a_bot_said() {
        let said = r#"{"topic":"chat","op":"wake","data":{"bot":"Recall","status":"succeeded","text":" Two notes disagree. "}}"#;
        assert_eq!(
            wake_message(said),
            Some(("Recall".to_string(), "Two notes disagree.".to_string()))
        );
        let silent = r#"{"topic":"chat","op":"wake","data":{"bot":"Recall","status":"succeeded","silent":true}}"#;
        assert_eq!(wake_message(silent), None);
        let retried = r#"{"topic":"chat","op":"wake","data":{"bot":"Recall","status":"succeeded","retry":true,"text":"Again."}}"#;
        assert_eq!(wake_message(retried), None);
        assert_eq!(
            wake_message(r#"{"topic":"notes","op":"write","path":"/a.md"}"#),
            None
        );
        assert_eq!(wake_message("{}"), None);
    }
}
