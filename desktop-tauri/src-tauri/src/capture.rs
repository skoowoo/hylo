// macOS Alt+Shift+C: screencapture -i, then a floating panel served by GET /quick-capture.
// Send still only logs; the agent endpoint does not exist yet.

use std::path::PathBuf;
use std::process::Command;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Mutex;
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};

use base64::Engine;
use serde::Serialize;
use tauri::{AppHandle, Manager, WebviewUrl, WebviewWindow, WebviewWindowBuilder};

use crate::commands::ShellState;
use crate::server;

pub const WINDOW_LABEL: &str = "quick-capture";
const ACCELERATOR: &str = "Alt+Shift+C";
const INIT_SCRIPT: &str = include_str!("capture.js");

const PANEL_PADDING: f64 = 20.0;
const HEADER_H: f64 = 34.0;
const IMG_GAP: f64 = 16.0;
const COMPOSER_H: f64 = 76.0;
const ACTIONS_GAP: f64 = 12.0;
const ACTIONS_H: f64 = 32.0;
const IMG_INSET: f64 = 10.0;
const MIN_BOX_W: f64 = 260.0;
const MAX_BOX_W: f64 = 640.0;
const MIN_BOX_H: f64 = 90.0;
const MAX_BOX_H: f64 = 420.0;
const CORNER_RADIUS: f64 = 12.0;

struct Layout {
    img_w: u32,
    img_h: u32,
    box_w: u32,
    box_h: u32,
    panel_w: f64,
    panel_h: f64,
}

struct Session {
    image_path: PathBuf,
    front_app: Option<String>,
    // Blur during show() is not the user clicking away.
    blur_after: Option<Instant>,
    init_js: String,
}

static IN_FLIGHT: AtomicBool = AtomicBool::new(false);
static SESSION: Mutex<Option<Session>> = Mutex::new(None);

pub fn panel_open() -> bool {
    session().is_some()
}

pub fn register(app: &AppHandle) {
    #[cfg(target_os = "macos")]
    {
        use tauri_plugin_global_shortcut::{GlobalShortcutExt, ShortcutState};
        let data_dir = app.state::<ShellState>().data_dir.clone();
        if let Err(err) = app
            .global_shortcut()
            .on_shortcut(ACCELERATOR, |app, _shortcut, event| {
                if event.state == ShortcutState::Pressed {
                    trigger(app);
                }
            })
        {
            server::diag_log(&data_dir, &format!("quick-capture shortcut: {err}"));
        }
    }
    #[cfg(not(target_os = "macos"))]
    let _ = app;
}

pub fn trigger(app: &AppHandle) {
    #[cfg(target_os = "macos")]
    {
        let app = app.clone();
        if macos::is_main_thread() {
            macos::begin(app);
        } else {
            let queued = app.clone();
            let _ = app.run_on_main_thread(move || macos::begin(queued));
        }
    }
    #[cfg(not(target_os = "macos"))]
    let _ = app;
}

pub fn on_panel_blur(app: &AppHandle) {
    let dismiss = {
        let slot = session();
        match slot.as_ref() {
            Some(session) => session.blur_after.is_some_and(|at| Instant::now() >= at),
            None => false,
        }
    };
    // A click on the panel's own padding can blur a transparent window. Only a
    // pointer that has left the window is an outside click.
    if dismiss && !pointer_inside_panel(app) {
        close_panel(app, false);
    }
}

fn pointer_inside_panel(app: &AppHandle) -> bool {
    let Some(window) = app.get_webview_window(WINDOW_LABEL) else {
        return false;
    };
    let Ok(cursor) = window.cursor_position() else {
        return false;
    };
    let Ok(origin) = window.outer_position() else {
        return false;
    };
    let Ok(size) = window.outer_size() else {
        return false;
    };
    let left = origin.x as f64;
    let top = origin.y as f64;
    cursor.x >= left
        && cursor.x < left + size.width as f64
        && cursor.y >= top
        && cursor.y < top + size.height as f64
}

#[tauri::command]
pub fn quick_capture_submit(app: AppHandle, message: String) {
    if let Some(path) = image_path() {
        log(
            &app,
            &format!(
                "quick-capture submit (not yet sent to agent bot): image={} message={message}",
                path.display()
            ),
        );
    }
    close_panel(&app, true);
}

#[tauri::command]
pub async fn quick_capture_save(app: AppHandle) {
    let path = image_path();
    let server = server_url(&app);
    let bytes = path.as_ref().and_then(|p| std::fs::read(p).ok());
    match (path, server, bytes) {
        (Some(path), Some(server), Some(bytes)) => {
            let name = path
                .file_name()
                .and_then(|s| s.to_str())
                .unwrap_or("capture.png");
            match upload_image(&server, name, &bytes).await {
                Ok(body) => log(&app, &format!("quick-capture saved to Hylo Images: {body}")),
                Err(err) => log(&app, &format!("quick-capture save failed: {err}")),
            }
        }
        _ => log(&app, "quick-capture save failed: missing image or server"),
    }
    close_panel(&app, true);
}

#[tauri::command]
pub fn quick_capture_cancel(app: AppHandle) {
    close_panel(&app, true);
}

fn close_panel(app: &AppHandle, restore_focus: bool) {
    let Some(session) = session().take() else {
        return;
    };
    let app = app.clone();
    let path = session.image_path;
    let app_for_finish = app.clone();
    let finish = move || {
        if let Some(window) = app_for_finish.get_webview_window(WINDOW_LABEL) {
            let _ = window.destroy();
        }
        let _ = std::fs::remove_file(path);
    };
    if restore_focus {
        if let Some(bundle) = session.front_app {
            std::thread::spawn(move || {
                activate_bundle(&bundle);
                let _ = app.run_on_main_thread(finish);
            });
            return;
        }
    }
    if cfg!(target_os = "macos") && !macos_main_thread() {
        let _ = app.run_on_main_thread(finish);
    } else {
        finish();
    }
}

fn macos_main_thread() -> bool {
    #[cfg(target_os = "macos")]
    {
        macos::is_main_thread()
    }
    #[cfg(not(target_os = "macos"))]
    {
        false
    }
}

fn image_path() -> Option<PathBuf> {
    session().as_ref().map(|s| s.image_path.clone())
}

fn server_url(app: &AppHandle) -> Option<String> {
    app.state::<ShellState>().connected()
}

fn log(app: &AppHandle, msg: &str) {
    server::diag_log(&app.state::<ShellState>().data_dir, msg);
}

fn session() -> std::sync::MutexGuard<'static, Option<Session>> {
    SESSION.lock().unwrap_or_else(|err| err.into_inner())
}

#[cfg(target_os = "macos")]
mod macos {
    use super::*;
    use objc2::MainThreadMarker;
    use objc2_app_kit::{NSApplication, NSWindow, NSWindowCollectionBehavior, NSWorkspace};
    use objc2_foundation::NSBundle;
    use std::ptr::NonNull;
    use tauri::webview::PageLoadEvent;

    pub fn is_main_thread() -> bool {
        MainThreadMarker::new().is_some()
    }

    pub fn begin(app: AppHandle) {
        if IN_FLIGHT
            .compare_exchange(false, true, Ordering::SeqCst, Ordering::SeqCst)
            .is_err()
        {
            return;
        }
        if panel_open() {
            IN_FLIGHT.store(false, Ordering::SeqCst);
            return;
        }
        let Some(server) = server_url(&app) else {
            IN_FLIGHT.store(false, Ordering::SeqCst);
            return;
        };
        // Read both before screencapture. Its UI does not change them, and the
        // panel has to restore the app the user was actually in.
        let front = frontmost_bundle();
        let hidden = app_is_hidden();
        std::thread::spawn(move || {
            let shot = capture_screen();
            let opened = app.clone();
            let queued = app.run_on_main_thread(move || {
                if let Some(shot) = shot {
                    open_panel(&opened, shot, front, hidden, server);
                }
                IN_FLIGHT.store(false, Ordering::SeqCst);
            });
            if queued.is_err() {
                IN_FLIGHT.store(false, Ordering::SeqCst);
            }
        });
    }

    struct Shot {
        path: PathBuf,
        bytes: Vec<u8>,
        width: u32,
        height: u32,
    }

    fn capture_screen() -> Option<Shot> {
        let stamp = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .map(|d| d.as_millis())
            .unwrap_or(0);
        let path =
            std::env::temp_dir().join(format!("hylo-capture-{}-{stamp}.png", std::process::id()));
        let _ = Command::new("screencapture")
            .args(["-i", "-o"])
            .arg(&path)
            .status();
        let bytes = std::fs::read(&path).ok()?;
        let Some((width, height)) = png_size(&bytes) else {
            let _ = std::fs::remove_file(&path);
            return None;
        };
        if width == 0 || height == 0 {
            let _ = std::fs::remove_file(&path);
            return None;
        }
        Some(Shot {
            path,
            bytes,
            width,
            height,
        })
    }

    fn open_panel(
        app: &AppHandle,
        shot: Shot,
        front: Option<String>,
        hidden: bool,
        server: String,
    ) {
        let monitor = monitor_under_cursor(app);
        let (dx, dy, dw, dh) = monitor
            .as_ref()
            .map(work_area_points)
            .unwrap_or((0.0, 0.0, 1440.0, 900.0));
        let layout = compute_layout(shot.width as f64, shot.height as f64, dw, dh);
        let data_url = format!(
            "data:image/png;base64,{}",
            base64::engine::general_purpose::STANDARD.encode(&shot.bytes)
        );
        let init_js = match serde_json::to_string(&InitPayload {
            image_data_url: data_url,
            box_w: layout.box_w,
            box_h: layout.box_h,
            img_w: layout.img_w,
            img_h: layout.img_h,
        }) {
            Ok(json) => {
                format!("window.__hyloQuickCaptureInit&&window.__hyloQuickCaptureInit({json})")
            }
            Err(_) => {
                let _ = std::fs::remove_file(&shot.path);
                return;
            }
        };
        let x = (dx + (dw - layout.panel_w) / 2.0).round();
        let y = (dy + (dh - layout.panel_h) / 2.0).round();

        if let Some(existing) = app.get_webview_window(WINDOW_LABEL) {
            let _ = existing.destroy();
        }

        let page = format!("{}/quick-capture", server.trim_end_matches('/'));
        let Ok(url) = page.parse() else {
            let _ = std::fs::remove_file(&shot.path);
            return;
        };
        let server_for_nav = server.clone();
        *session() = Some(Session {
            image_path: shot.path,
            front_app: front,
            blur_after: None,
            init_js,
        });
        let built = WebviewWindowBuilder::new(app, WINDOW_LABEL, WebviewUrl::External(url))
            .title("Quick Capture")
            .inner_size(layout.panel_w, layout.panel_h)
            .position(x, y)
            .decorations(false)
            .transparent(true)
            .resizable(false)
            .shadow(true)
            .always_on_top(true)
            .skip_taskbar(true)
            .visible(false)
            .visible_on_all_workspaces(true)
            .initialization_script(INIT_SCRIPT)
            .on_navigation(move |url| allow_panel_url(url, &server_for_nav))
            .on_page_load(|window, payload| {
                if payload.event() == PageLoadEvent::Finished
                    && payload.url().path() == "/quick-capture"
                {
                    deliver_init(&window);
                }
            })
            .on_new_window(|url, _features| {
                let _ = open::that(url.as_str());
                tauri::webview::NewWindowResponse::Deny
            })
            .build();

        let window = match built {
            Ok(window) => window,
            Err(err) => {
                log(app, &format!("quick-capture panel: {err}"));
                close_panel(app, true);
                return;
            }
        };

        tune_panel(&window);
        {
            use tauri::window::{Effect, EffectState, EffectsBuilder};
            let _ = window.set_effects(
                EffectsBuilder::new()
                    .effect(Effect::Sidebar)
                    .state(EffectState::Active)
                    .radius(CORNER_RADIUS)
                    .build(),
            );
        }
        if let Err(err) = window.show() {
            log(app, &format!("quick-capture show: {err}"));
            close_panel(app, true);
            return;
        }
        tune_panel(&window);
        if hidden {
            if let Some(main) = app.get_webview_window("main") {
                let _ = main.hide();
            }
        }
        if let Some(session) = session().as_mut() {
            session.blur_after = Some(Instant::now() + Duration::from_millis(200));
        }
        watch_panel_load(app.clone(), server);
    }

    fn deliver_init(window: &WebviewWindow) {
        let Some(js) = session().as_ref().map(|s| s.init_js.clone()) else {
            return;
        };
        let _ = window.eval(js);
    }

    fn watch_panel_load(app: AppHandle, server: String) {
        tauri::async_runtime::spawn(async move {
            if !page_unreachable(&server).await || !panel_open() {
                return;
            }
            let failed = app.clone();
            let _ = app.run_on_main_thread(move || {
                log(&failed, "quick-capture page failed, closing");
                close_panel(&failed, true);
            });
        });
    }

    async fn page_unreachable(server: &str) -> bool {
        let url = format!("{}/quick-capture", server.trim_end_matches('/'));
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

    fn allow_panel_url(url: &url::Url, server: &str) -> bool {
        crate::nav::same_origin(url, server) && url.path() == "/quick-capture"
    }

    fn tune_panel(window: &WebviewWindow) {
        let Ok(ptr) = window.ns_window() else {
            return;
        };
        let Some(ptr) = NonNull::new(ptr) else {
            return;
        };
        let ns: &NSWindow = unsafe { ptr.cast().as_ref() };
        let behavior = ns.collectionBehavior()
            | NSWindowCollectionBehavior::CanJoinAllSpaces
            | NSWindowCollectionBehavior::FullScreenAuxiliary;
        ns.setCollectionBehavior(behavior);
        ns.setMovable(false);
        ns.setHidesOnDeactivate(false);
        ns.setExcludedFromWindowsMenu(true);
    }

    fn monitor_under_cursor(app: &AppHandle) -> Option<tauri::Monitor> {
        let cursor = app.cursor_position().ok()?;
        let monitors = app.available_monitors().ok()?;
        let hit = monitors.iter().find(|monitor| {
            let x = monitor.position().x as f64;
            let y = monitor.position().y as f64;
            let w = monitor.size().width as f64;
            let h = monitor.size().height as f64;
            cursor.x >= x && cursor.x < x + w && cursor.y >= y && cursor.y < y + h
        });
        hit.cloned().or_else(|| monitors.into_iter().next())
    }

    fn work_area_points(monitor: &tauri::Monitor) -> (f64, f64, f64, f64) {
        let scale = monitor.scale_factor().max(1.0);
        let area = monitor.work_area();
        (
            area.position.x as f64 / scale,
            area.position.y as f64 / scale,
            area.size.width as f64 / scale,
            area.size.height as f64 / scale,
        )
    }

    fn frontmost_bundle() -> Option<String> {
        let own = own_bundle_id();
        let app = NSWorkspace::sharedWorkspace().frontmostApplication()?;
        let id = app.bundleIdentifier()?.to_string();
        if is_self_bundle(&id, own.as_deref()) {
            None
        } else {
            Some(id)
        }
    }

    fn own_bundle_id() -> Option<String> {
        NSBundle::mainBundle()
            .bundleIdentifier()
            .map(|id| id.to_string())
    }

    pub(super) fn own_is(id: &str) -> bool {
        own_bundle_id().as_deref() == Some(id)
    }

    fn app_is_hidden() -> bool {
        let Some(mtm) = MainThreadMarker::new() else {
            return false;
        };
        NSApplication::sharedApplication(mtm).isHidden()
    }

    #[derive(Serialize)]
    #[serde(rename_all = "camelCase")]
    struct InitPayload {
        image_data_url: String,
        box_w: u32,
        box_h: u32,
        img_w: u32,
        img_h: u32,
    }
}

fn activate_bundle(bundle_id: &str) {
    if !is_safe_bundle_id(bundle_id) || is_self_bundle(bundle_id, None) {
        return;
    }
    #[cfg(target_os = "macos")]
    if macos::own_is(bundle_id) {
        return;
    }
    let opened = Command::new("open")
        .args(["-b", bundle_id])
        .status()
        .map(|status| status.success())
        .unwrap_or(false);
    if opened {
        return;
    }
    let script = format!("tell application id \"{bundle_id}\" to activate");
    let _ = Command::new("osascript").args(["-e", &script]).status();
}

fn compute_layout(iw: f64, ih: f64, work_w: f64, work_h: f64) -> Layout {
    let max_box_w = MAX_BOX_W.min((work_w * 0.6).round());
    let max_box_h = MAX_BOX_H.min((work_h * 0.5).round());
    let scale = 1.0_f64
        .min((max_box_w - IMG_INSET * 2.0) / iw.max(1.0))
        .min((max_box_h - IMG_INSET * 2.0) / ih.max(1.0));
    let img_w = (iw * scale).round().max(1.0) as u32;
    let img_h = (ih * scale).round().max(1.0) as u32;
    let box_w = max_box_w.min((img_w as f64 + IMG_INSET * 2.0).max(MIN_BOX_W)) as u32;
    let box_h = max_box_h.min((img_h as f64 + IMG_INSET * 2.0).max(MIN_BOX_H)) as u32;
    let panel_w = box_w as f64 + PANEL_PADDING * 2.0;
    let panel_h = PANEL_PADDING * 2.0
        + HEADER_H
        + box_h as f64
        + IMG_GAP
        + COMPOSER_H
        + ACTIONS_GAP
        + ACTIONS_H;
    Layout {
        img_w,
        img_h,
        box_w,
        box_h,
        panel_w,
        panel_h,
    }
}

fn png_size(bytes: &[u8]) -> Option<(u32, u32)> {
    if bytes.len() < 24 || &bytes[0..8] != b"\x89PNG\r\n\x1a\n" || &bytes[12..16] != b"IHDR" {
        return None;
    }
    let width = u32::from_be_bytes(bytes[16..20].try_into().ok()?);
    let height = u32::from_be_bytes(bytes[20..24].try_into().ok()?);
    Some((width, height))
}

fn is_self_bundle(id: &str, own: Option<&str>) -> bool {
    id == "dev.hardhacker.hylo" || id == "com.github.Electron" || own == Some(id)
}

fn is_safe_bundle_id(id: &str) -> bool {
    !id.is_empty()
        && id
            .chars()
            .all(|c| c.is_ascii_alphanumeric() || c == '.' || c == '-')
}

async fn upload_image(server: &str, file_name: &str, bytes: &[u8]) -> Result<String, String> {
    let mut url = url::Url::parse(server).map_err(|err| err.to_string())?;
    url.set_path("/api/vault/upload-image");
    url.set_query(None);
    url.set_fragment(None);

    let stamp = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|d| d.as_millis())
        .unwrap_or(0);
    let boundary = format!("----hyloCapture{}{stamp}", std::process::id());
    let mut body = Vec::with_capacity(bytes.len() + 256);
    body.extend(
        format!(
            "--{boundary}\r\nContent-Disposition: form-data; name=\"file\"; filename=\"{file_name}\"\r\nContent-Type: image/png\r\n\r\n"
        )
        .into_bytes(),
    );
    body.extend_from_slice(bytes);
    body.extend(format!("\r\n--{boundary}--\r\n").into_bytes());

    let client = reqwest::Client::builder()
        .timeout(Duration::from_secs(30))
        .no_proxy()
        .build()
        .map_err(|err| err.to_string())?;
    let response = client
        .post(url)
        .header(
            "Content-Type",
            format!("multipart/form-data; boundary={boundary}"),
        )
        .body(body)
        .send()
        .await
        .map_err(|err| err.to_string())?;
    let status = response.status();
    let text = response.text().await.map_err(|err| err.to_string())?;
    if status != reqwest::StatusCode::OK {
        return Err(format!("upload failed ({status}): {text}"));
    }
    Ok(text)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn layout_matches_electron_bounds() {
        let large = compute_layout(4000.0, 3000.0, 1440.0, 900.0);
        assert_eq!(large.img_w, 533);
        assert_eq!(large.img_h, 400);
        assert_eq!(large.box_w, 553);
        assert_eq!(large.box_h, 420);
        assert_eq!(large.panel_w, 593.0);
        assert_eq!(large.panel_h, 630.0);

        let small = compute_layout(80.0, 40.0, 1440.0, 900.0);
        assert_eq!(small.img_w, 80);
        assert_eq!(small.img_h, 40);
        assert_eq!(small.box_w, 260);
        assert_eq!(small.box_h, 90);
    }

    #[test]
    fn png_size_reads_ihdr() {
        let mut bytes = vec![0x89, b'P', b'N', b'G', b'\r', b'\n', 0x1a, b'\n'];
        bytes.extend_from_slice(&13u32.to_be_bytes());
        bytes.extend_from_slice(b"IHDR");
        bytes.extend_from_slice(&1800u32.to_be_bytes());
        bytes.extend_from_slice(&900u32.to_be_bytes());
        bytes.extend_from_slice(&[8, 2, 0, 0, 0]);
        assert_eq!(png_size(&bytes), Some((1800, 900)));
        assert_eq!(png_size(b"not a png"), None);
    }

    #[test]
    fn self_bundle_is_not_a_restore_target() {
        assert!(is_self_bundle("dev.hardhacker.hylo", None));
        assert!(is_self_bundle("com.github.Electron", None));
        assert!(is_self_bundle("com.example.Dev", Some("com.example.Dev")));
        assert!(!is_self_bundle(
            "com.apple.Safari",
            Some("dev.hardhacker.hylo")
        ));
        assert!(is_safe_bundle_id("com.apple.Safari"));
        assert!(!is_safe_bundle_id("com.apple.Safari\"; activate"));
    }
}
