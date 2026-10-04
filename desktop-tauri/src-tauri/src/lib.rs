mod cli;
mod commands;
mod config;
mod nav;
mod notify;
mod server;
mod zoom;

use std::sync::Mutex;

use tauri::menu::{Menu, MenuItem, PredefinedMenuItem, Submenu};
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
        .plugin(tauri_plugin_opener::init());

    builder = builder
        .invoke_handler(commands::invoke_handler())
        .setup(|app| {
            let data_dir = config::shell_data_dir();
            let _ = std::fs::create_dir_all(&data_dir);
            let archive = bundled_archive(app.handle());
            app.manage(ShellState {
                data_dir: data_dir.clone(),
                archive,
                config: Mutex::new(ConfigFile::load(&data_dir)),
                managed_pid: Mutex::new(0),
                connected_url: Mutex::new(None),
                inbox: Mutex::new(None),
            });

            install_menu(app.handle())?;

            let script = format!(
                "window.__HYLO_PLATFORM__=\"{}\";\n{INIT_SCRIPT}",
                platform_name()
            );
            let handle = app.handle().clone();
            let mut window =
                WebviewWindowBuilder::new(app, "main", WebviewUrl::App("index.html".into()))
                    .title("Hylo")
                    .inner_size(1440.0, 960.0)
                    .min_inner_size(960.0, 640.0)
                    .initialization_script(script)
                    .on_navigation(move |url| commands::allow_url(&handle, url))
                    .on_new_window(|url, _features| {
                        let _ = open::that(url.as_str());
                        tauri::webview::NewWindowResponse::Deny
                    });

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
            std::thread::spawn(move || commands::install_cli_on_launch(&app_handle));
            Ok(())
        })
        .on_window_event(|window, event| {
            match event {
                WindowEvent::CloseRequested { api, .. } => {
                    #[cfg(target_os = "macos")]
                    if window.label() == "main" {
                        let _ = window.hide();
                        api.prevent_close();
                    }
                    #[cfg(not(target_os = "macos"))]
                    let _ = (window, api);
                }
                _ => {}
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
        // close button stays hidden unless we show it here. Electron listens
        // for did-become-active for the same case.
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

fn install_menu(app: &tauri::AppHandle) -> tauri::Result<()> {
    let undo = MenuItem::with_id(app, "undo", "Undo", true, None::<&str>)?;
    let redo = MenuItem::with_id(app, "redo", "Redo", true, None::<&str>)?;
    let edit = Submenu::with_items(
        app,
        "Edit",
        true,
        &[
            &undo,
            &redo,
            &PredefinedMenuItem::separator(app)?,
            &PredefinedMenuItem::cut(app, None)?,
            &PredefinedMenuItem::copy(app, None)?,
            &PredefinedMenuItem::paste(app, None)?,
            &PredefinedMenuItem::select_all(app, None)?,
        ],
    )?;

    let menu = {
        #[cfg(target_os = "macos")]
        {
            let app_menu = Submenu::with_items(
                app,
                "Hylo",
                true,
                &[
                    &PredefinedMenuItem::about(app, None, None)?,
                    &PredefinedMenuItem::separator(app)?,
                    &PredefinedMenuItem::hide(app, None)?,
                    &PredefinedMenuItem::hide_others(app, None)?,
                    &PredefinedMenuItem::show_all(app, None)?,
                    &PredefinedMenuItem::separator(app)?,
                    &PredefinedMenuItem::quit(app, None)?,
                ],
            )?;
            let zoom = MenuItem::with_id(app, "zoom", "Zoom", true, None::<&str>)?;
            let window_menu = Submenu::with_items(
                app,
                "Window",
                true,
                &[
                    &PredefinedMenuItem::minimize(app, None)?,
                    &zoom,
                    &PredefinedMenuItem::separator(app)?,
                    &PredefinedMenuItem::close_window(app, None)?,
                ],
            )?;
            Menu::with_items(app, &[&app_menu, &edit, &window_menu])?
        }
        #[cfg(not(target_os = "macos"))]
        {
            Menu::with_items(app, &[&edit])?
        }
    };
    app.set_menu(menu)?;
    app.on_menu_event(|app, event| {
        if event.id().0 == "zoom" {
            if let Some(window) = app.get_webview_window("main") {
                zoom::toggle(&window);
            }
            return;
        }
        let script = match event.id().0.as_str() {
            "undo" => "window.__hyloUndo && window.__hyloUndo()",
            "redo" => "window.__hyloRedo && window.__hyloRedo()",
            _ => return,
        };
        if let Some(window) = app.get_webview_window("main") {
            let _ = window.eval(script);
        }
    });
    Ok(())
}
