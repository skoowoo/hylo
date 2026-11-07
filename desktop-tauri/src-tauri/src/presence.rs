//! Tells a server whether the user is at this computer and which app is in
//! front, so the main bot's rounds pick their moment. App names only, never
//! window titles. A server that doesn't keep them says so, and is asked again
//! much later.

use std::time::Duration;

use serde::Serialize;
use tauri::AppHandle;
use tokio::sync::watch;

use crate::notify::Target;

const EVERY: Duration = Duration::from_secs(30);
/// How long a server that turned reports away waits before the next one.
const OFF_EVERY: Duration = Duration::from_secs(15 * 60);

#[derive(Serialize, Default, Debug)]
pub struct Sample {
    app: String,
    /// Seconds since the last keyboard or mouse input, anywhere.
    idle: u64,
    locked: bool,
}

/// Reports a sample every 30s until stop. A failed post is dropped: the next
/// one carries the same news. 410 (presence off) or 404 (an older server)
/// backs off to OFF_EVERY.
pub async fn run(app: AppHandle, target: Target, mut stop: watch::Receiver<bool>) {
    let Some(endpoint) = join_presence(&target.base_url) else {
        return;
    };
    let Ok(client) = reqwest::Client::builder()
        .no_proxy()
        .timeout(Duration::from_secs(10))
        .build()
    else {
        return;
    };
    loop {
        if *stop.borrow() {
            return;
        }
        let mut wait = EVERY;
        if let Some(s) = sample(&app).await {
            let mut req = client.post(&endpoint).json(&s);
            if let Some(key) = target.key.as_deref().filter(|k| !k.is_empty()) {
                req = req.header("X-Hylo-API-Key", key);
            }
            if let Ok(res) = req.send().await {
                if turned_away(res.status()) {
                    wait = OFF_EVERY;
                }
            }
        }
        tokio::select! {
            changed = stop.changed() => {
                if changed.is_err() || *stop.borrow() {
                    return;
                }
            }
            _ = tokio::time::sleep(wait) => {}
        }
    }
}

fn turned_away(status: reqwest::StatusCode) -> bool {
    status == reqwest::StatusCode::GONE || status == reqwest::StatusCode::NOT_FOUND
}

fn join_presence(base: &str) -> Option<String> {
    let base = base.trim().trim_end_matches('/');
    if base.is_empty() {
        return None;
    }
    Some(format!("{base}/api/desktop/presence"))
}

#[cfg(target_os = "macos")]
async fn sample(app: &AppHandle) -> Option<Sample> {
    // AppKit is asked on the main thread.
    let (tx, rx) = tokio::sync::oneshot::channel();
    app.run_on_main_thread(move || {
        let _ = tx.send(mac::sample());
    })
    .ok()?;
    rx.await.ok()
}

#[cfg(not(target_os = "macos"))]
async fn sample(_app: &AppHandle) -> Option<Sample> {
    None
}

#[cfg(target_os = "macos")]
mod mac {
    use std::ffi::c_void;

    use objc2::rc::Retained;
    use objc2::runtime::AnyObject;
    use objc2_app_kit::NSWorkspace;
    use objc2_foundation::{ns_string, NSDictionary, NSNumber, NSString};

    use super::Sample;

    #[link(name = "CoreGraphics", kind = "framework")]
    extern "C" {
        fn CGEventSourceSecondsSinceLastEventType(state: i32, event_type: u32) -> f64;
        fn CGSessionCopyCurrentDictionary() -> *mut c_void;
    }

    const COMBINED_SESSION_STATE: i32 = 0;
    const ANY_INPUT_EVENT: u32 = u32::MAX;

    pub fn sample() -> Sample {
        let mut s = Sample {
            idle: idle_seconds(),
            locked: locked(),
            ..Default::default()
        };
        let Some(front) = NSWorkspace::sharedWorkspace().frontmostApplication() else {
            return s;
        };
        s.app = front
            .localizedName()
            .map(|n| n.to_string())
            .unwrap_or_default();
        s
    }

    fn idle_seconds() -> u64 {
        let secs = unsafe {
            CGEventSourceSecondsSinceLastEventType(COMBINED_SESSION_STATE, ANY_INPUT_EVENT)
        };
        if secs.is_finite() && secs > 0.0 {
            secs as u64
        } else {
            0
        }
    }

    /// No session dictionary means no one is logged in at the screen.
    fn locked() -> bool {
        let raw = unsafe { CGSessionCopyCurrentDictionary() };
        // The copy hands us ownership; CFDictionary is toll-free bridged to NSDictionary.
        let Some(dict) =
            (unsafe { Retained::from_raw(raw.cast::<NSDictionary<NSString, AnyObject>>()) })
        else {
            return true;
        };
        dict.objectForKey(ns_string!("CGSSessionScreenIsLocked"))
            .and_then(|o| o.downcast::<NSNumber>().ok())
            .is_some_and(|n| n.boolValue())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn presence_endpoint() {
        assert_eq!(
            join_presence("http://127.0.0.1:54321/").as_deref(),
            Some("http://127.0.0.1:54321/api/desktop/presence")
        );
        assert_eq!(join_presence("  "), None);
    }

    #[test]
    fn backs_off_when_turned_away() {
        assert!(turned_away(reqwest::StatusCode::GONE));
        assert!(turned_away(reqwest::StatusCode::NOT_FOUND));
        assert!(!turned_away(reqwest::StatusCode::NO_CONTENT));
        assert!(!turned_away(reqwest::StatusCode::UNAUTHORIZED));
    }
}
