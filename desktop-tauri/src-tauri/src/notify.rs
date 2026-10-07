use std::path::Path;
use std::process::Command;
use std::time::Duration;

use serde::Deserialize;
use tauri::AppHandle;
use tokio::sync::watch;

use crate::config::ConfigFile;

/// Connection to one server's inbox stream.
#[derive(Clone, PartialEq, Eq)]
pub struct Target {
    pub base_url: String,
    pub key: Option<String>,
    /// Prefixes notification titles so several servers stay distinguishable.
    pub label: Option<String>,
}

pub struct InboxHandle {
    stop: watch::Sender<bool>,
    /// What the loop was started with, to detect a changed URL, key or label.
    target: Target,
}

impl InboxHandle {
    pub fn start(app: AppHandle, data_dir: std::path::PathBuf, target: Target) -> Self {
        let (stop, rx) = watch::channel(false);
        let handle = Self { stop, target: target.clone() };
        tauri::async_runtime::spawn(async move {
            run_loop(app, data_dir, target, rx).await;
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
    Retry,
}

async fn run_loop(
    app: AppHandle,
    data_dir: std::path::PathBuf,
    target: Target,
    mut stop: watch::Receiver<bool>,
) {
    let Some(endpoint) = join_notifications(&target.base_url) else {
        return;
    };
    let Ok(client) = reqwest::Client::builder().no_proxy().build() else {
        return;
    };
    loop {
        if *stop.borrow() {
            break;
        }
        match connect(&client, &app, &data_dir, &endpoint, &target, &mut stop).await {
            Loop::Stopped => break,
            Loop::Retry => {
                tokio::select! {
                    _ = stop.changed() => {
                        if *stop.borrow() { break; }
                    }
                    _ = tokio::time::sleep(Duration::from_secs(5)) => {}
                }
            }
        }
    }
}

async fn connect(
    client: &reqwest::Client,
    app: &AppHandle,
    data_dir: &Path,
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
        _ => return Loop::Retry,
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
                            show_message(app, data_dir, &raw, target.label.as_deref());
                        }
                    }
                    _ => return Loop::Retry,
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

#[derive(Debug, Deserialize)]
struct InboxMsg {
    title: Option<String>,
    body: Option<String>,
    source: Option<String>,
}

pub fn show_message(app: &AppHandle, data_dir: &Path, raw: &str, label: Option<&str>) {
    let Ok(msg) = serde_json::from_str::<InboxMsg>(raw) else {
        return;
    };
    let settings = ConfigFile::load(data_dir).inbox_notify;
    if settings.text_enabled {
        let title = msg
            .title
            .filter(|s| !s.is_empty())
            .or(msg.source)
            .unwrap_or_else(|| "Inbox".to_string());
        let title = match label {
            Some(l) => format!("[{l}] {title}"),
            None => title,
        };
        let body: String = msg
            .body
            .unwrap_or_default()
            .trim()
            .chars()
            .take(150)
            .collect();
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

fn join_notifications(base: &str) -> Option<String> {
    let base = base.trim().trim_end_matches('/');
    if base.is_empty() {
        return None;
    }
    Some(format!("{base}/api/inbox/notifications"))
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
}
