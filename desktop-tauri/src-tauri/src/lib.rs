mod agent_setup;
mod cli;
mod commands;
mod config;
mod local;
mod locale;
mod nav;
mod notify;
mod onboarding;
mod remote;
mod server;
mod servers;
mod session;
mod shell_path;
mod vault_setup;
mod window_size;
mod zoom;

use std::sync::atomic::Ordering;
use std::sync::Mutex;

use tauri::{Manager, RunEvent, WebviewUrl, WebviewWindowBuilder, WindowEvent};

use crate::commands::ShellState;
use crate::config::ConfigFile;

const INIT_SCRIPT: &str = include_str!("shim.js");

pub fn run() {
    let mut builder = tauri::Builder::default()
        .plugin(tauri_plugin_single_instance::init(|app, _args, _cwd| {
            if let Some(window) = app.get_webview_window("main") {
                let _ = window.unminimize();
                let _ = window.show();
                let _ = window.set_focus();
            }
        }))
        .plugin(tauri_plugin_dialog::init())
        .plugin(tauri_plugin_notification::init())
        .plugin(tauri_plugin_opener::init())
        .plugin(window_state());

    builder = builder
        .invoke_handler(commands::invoke_handler())
        .setup(|app| {
            let data_dir = app.path().app_data_dir()?;
            let _ = std::fs::create_dir_all(&data_dir);
            let archive = bundled_archive(app.handle());
            let mut config = ConfigFile::load(&data_dir);
            if onboarding::should_start(&data_dir) {
                config.onboarding.pending = true;
                // A new install starts dark; its first page keeps that as its own theme.
                config.theme = "dark".into();
                let _ = config.save(&data_dir);
            }
            let theme = config::theme_choice(&config.theme);
            let compact = config.onboarding.pending;
            onboarding::STARTED_COMPACT.store(compact, Ordering::Relaxed);
            app.manage(ShellState {
                data_dir: data_dir.clone(),
                config: Mutex::new(config),
                local: local::LocalServer::new(data_dir.clone(), archive),
                connected_url: Mutex::new(None),
                landing: Mutex::new(None),
                inboxes: Mutex::new(std::collections::HashMap::new()),
            });
            servers::sync_inboxes(app.handle());

            install_menu(app.handle())?;

            let theme_name = match theme {
                Some(tauri::Theme::Dark) => "dark",
                Some(tauri::Theme::Light) => "light",
                _ => "",
            };
            let script = format!(
                "window.__HYLO_PLATFORM__=\"{}\";\nwindow.__HYLO_THEME__=\"{theme_name}\";\n{INIT_SCRIPT}",
                platform_name()
            );
            let main_size = window_size::launch_main_size(app.handle());
            let handle = app.handle().clone();
            let mut window =
                WebviewWindowBuilder::new(app, "main", WebviewUrl::App("index.html".into()))
                    .title("Hylo")
                    .theme(theme)
                    .initialization_script(script)
                    // Default handler returns true and never forwards the drag to
                    // WKWebView, so an in-page drop never reaches a sidebar folder.
                    .disable_drag_drop_handler()
                    .on_navigation(move |url| session::allow_url(&handle, url))
                    .on_new_window(|url, _features| {
                        let _ = open::that(url.as_str());
                        tauri::webview::NewWindowResponse::Deny
                    });

            window = if compact {
                let (w, h) = onboarding::COMPACT_SIZE;
                window
                    .inner_size(w, h)
                    .resizable(false)
                    .maximizable(false)
                    .center()
            } else {
                let (w, h) = main_size;
                let (min_w, min_h) = window_size::MAIN_MIN_SIZE;
                window.inner_size(w, h).min_inner_size(min_w, min_h).center()
            };
            #[cfg(target_os = "macos")]
            {
                window = window
                    .transparent(true)
                    .title_bar_style(tauri::TitleBarStyle::Overlay)
                    .hidden_title(true)
                    .traffic_light_position(tauri::LogicalPosition::new(16.0, 20.0));
            }
            #[cfg(not(target_os = "macos"))]
            {
                window = window.background_color(tauri::window::Color(0x18, 0x19, 0x1e, 255));
            }
            #[cfg(debug_assertions)]
            {
                window = window.devtools(true);
            }

            let window = window.build()?;
            if !compact {
                // The window-state plugin restores from a main-thread task queued by
                // build(); queuing behind it checks the restored size, not ours.
                let restored = window.clone();
                let _ = window.run_on_main_thread(move || window_size::keep_on_screen(&restored));
            }
            #[cfg(target_os = "macos")]
            {
                use tauri::window::{Effect, EffectState, EffectsBuilder};
                let _ = window.set_effects(
                    EffectsBuilder::new()
                        .effect(Effect::Sidebar)
                        .state(EffectState::Active)
                        .build(),
                );
            }
            #[cfg(not(target_os = "macos"))]
            {
                let _ = window;
            }

            let app_handle = app.handle().clone();
            #[cfg(target_os = "macos")]
            watch_app_activation(app_handle.clone());
            session::start_heartbeat(app_handle.clone());
            let install = local::hold_for_install(&app_handle);
            tauri::async_runtime::spawn(local::install_on_launch(app_handle, install));
            Ok(())
        })
        .on_window_event(|window, event| {
            if let WindowEvent::CloseRequested { api, .. } = event {
                #[cfg(target_os = "macos")]
                if window.label() == "main" {
                    let _ = window.hide();
                    api.prevent_close();
                }
                #[cfg(not(target_os = "macos"))]
                let _ = (window, api);
            }
        });

    builder
        .build(tauri::generate_context!())
        .expect("error while running Hylo")
        .run(|app, event| {
            if let RunEvent::Reopen { .. } = event {
                if let Some(window) = app.get_webview_window("main") {
                    let _ = window.show();
                    let _ = window.set_focus();
                }
            }
        });
}

#[cfg(target_os = "macos")]
fn watch_app_activation(app: tauri::AppHandle) {
    use std::ptr::NonNull;

    use block2::RcBlock;
    use objc2_app_kit::NSApplicationDidBecomeActiveNotification;
    use objc2_foundation::{NSNotification, NSNotificationCenter, NSOperationQueue};

    let queue = NSOperationQueue::mainQueue();
    let center = NSNotificationCenter::defaultCenter();
    let block = RcBlock::new(move |_note: NonNull<NSNotification>| {
        // ⌘-Tab activates the app without Reopen. A window hidden by the
        // close button stays hidden unless we show it here.
        show_main_if_hidden(&app);
    });
    let name = unsafe { NSApplicationDidBecomeActiveNotification };
    let observer = unsafe {
        center.addObserverForName_object_queue_usingBlock(Some(name), None, Some(&queue), &block)
    };
    // Dropping the token removes the observer.
    std::mem::forget(observer);
}

#[cfg(target_os = "macos")]
fn show_main_if_hidden(app: &tauri::AppHandle) {
    use tauri::Manager;
    let Some(window) = app.get_webview_window("main") else {
        return;
    };
    if window.is_visible().unwrap_or(true) {
        return;
    }
    let _ = window.unminimize();
    let _ = window.show();
    let _ = window.set_focus();
}

/// Remembers where and how big the main window was. Visibility is left out:
/// on macOS closing only hides the window, and it must come back shown.
fn window_state() -> tauri::plugin::TauriPlugin<tauri::Wry> {
    use tauri_plugin_window_state::StateFlags;
    tauri_plugin_window_state::Builder::new()
        .with_state_flags(
            StateFlags::SIZE
                | StateFlags::POSITION
                | StateFlags::MAXIMIZED
                | StateFlags::FULLSCREEN,
        )
        .map_label(|label| {
            if onboarding::STARTED_COMPACT.load(Ordering::Relaxed) {
                onboarding::COMPACT_STATE_LABEL
            } else {
                label
            }
        })
        .with_filter(|label| label != onboarding::COMPACT_STATE_LABEL)
        .build()
}

fn platform_name() -> &'static str {
    if cfg!(target_os = "macos") {
        "darwin"
    } else if cfg!(windows) {
        "win32"
    } else {
        "linux"
    }
}

fn bundled_archive(app: &tauri::AppHandle) -> Option<std::path::PathBuf> {
    let name = if cfg!(windows) {
        "hylo.zip"
    } else {
        "hylo.tar.gz"
    };
    let path = app.path().resource_dir().ok()?.join("cli").join(name);
    path.exists().then_some(path)
}

// Page-owned shortcuts the user can rebind get no accelerator, so the menu never shows a stale key.
// The page sees a key first and a menu accelerator only fires when it declines the event.
#[cfg(target_os = "macos")]
fn install_menu(app: &tauri::AppHandle) -> tauri::Result<()> {
    use tauri::menu::{Menu, MenuItem, PredefinedMenuItem, Submenu};

    let item = |id: &str, text: &str, accel: Option<&str>| {
        MenuItem::with_id(app, id, text, true, accel)
    };
    let sep = || PredefinedMenuItem::separator(app);

    let app_menu = Submenu::with_items(
        app,
        "Hylo",
        true,
        &[
            &PredefinedMenuItem::about(app, None, None)?,
            &sep()?,
            &item("open-settings", "Settings…", Some("Cmd+,"))?,
            &sep()?,
            &PredefinedMenuItem::services(app, None)?,
            &sep()?,
            &PredefinedMenuItem::hide(app, None)?,
            &PredefinedMenuItem::hide_others(app, None)?,
            &PredefinedMenuItem::show_all(app, None)?,
            &sep()?,
            &PredefinedMenuItem::quit(app, None)?,
        ],
    )?;
    let file = Submenu::with_items(
        app,
        "File",
        true,
        &[
            &item("new-note", "New Note", None)?,
            &sep()?,
            &item("close-tab", "Close Tab", None)?,
            &PredefinedMenuItem::close_window(app, None)?,
        ],
    )?;
    let edit = Submenu::with_items(
        app,
        "Edit",
        true,
        &[
            &item("undo", "Undo", Some("Cmd+Z"))?,
            &item("redo", "Redo", Some("Cmd+Shift+Z"))?,
            &sep()?,
            &PredefinedMenuItem::cut(app, None)?,
            &PredefinedMenuItem::copy(app, None)?,
            &PredefinedMenuItem::paste(app, None)?,
            &PredefinedMenuItem::select_all(app, None)?,
            &sep()?,
            &item("find", "Find", Some("Cmd+F"))?,
        ],
    )?;
    let view = Submenu::with_items(
        app,
        "View",
        true,
        &[
            &item("toggle-search", "Search", Some("Cmd+K"))?,
            &sep()?,
            &item("toggle-editor", "Toggle Editor", None)?,
            &item("expand-editor", "Expand Editor", None)?,
            &item("reading-mode", "Reading Mode", None)?,
            &sep()?,
            &item("refresh", "Refresh", Some("Cmd+R"))?,
        ],
    )?;
    let window_menu = Submenu::with_items(
        app,
        "Window",
        true,
        &[
            &PredefinedMenuItem::minimize(app, None)?,
            &item("zoom", "Zoom", None)?,
            &sep()?,
            &PredefinedMenuItem::fullscreen(app, None)?,
            &sep()?,
            &PredefinedMenuItem::bring_all_to_front(app, None)?,
        ],
    )?;
    let help = Submenu::with_items(
        app,
        "Help",
        true,
        &[
            &item("help-site", "Hylo on GitHub", None)?,
            &item("help-issue", "Report an Issue", None)?,
        ],
    )?;
    let menu = Menu::with_items(app, &[&app_menu, &file, &edit, &view, &window_menu, &help])?;
    app.set_menu(menu)?;
    window_menu.set_as_windows_menu_for_nsapp()?;
    help.set_as_help_menu_for_nsapp()?;

    app.on_menu_event(|app, event| {
        let id = event.id().0.as_str();
        let link = match id {
            "help-site" => Some("https://github.com/skoowoo/hylo"),
            "help-issue" => Some("https://github.com/skoowoo/hylo/issues"),
            _ => None,
        };
        if let Some(url) = link {
            let _ = open::that(url);
            return;
        }
        let Some(window) = app.get_webview_window("main") else {
            return;
        };
        let script = match id {
            "zoom" => return zoom::toggle(&window),
            // Pages without the editor still have native fields to undo in.
            "undo" => "window.__hyloUndo ? __hyloUndo() : document.execCommand('undo')".to_string(),
            "redo" => "window.__hyloRedo ? __hyloRedo() : document.execCommand('redo')".to_string(),
            _ => format!("window.__hyloHotkeys && window.__hyloHotkeys.run({id:?})"),
        };
        let _ = window.eval(script);
    });
    Ok(())
}

// Elsewhere the webview handles editing keys itself, and a menu bar holding
// only Edit would clash with the app's own chrome.
#[cfg(not(target_os = "macos"))]
fn install_menu(_app: &tauri::AppHandle) -> tauri::Result<()> {
    Ok(())
}
