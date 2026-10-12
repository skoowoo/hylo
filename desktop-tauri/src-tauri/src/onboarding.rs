use std::path::Path;
use std::sync::atomic::AtomicBool;

use serde::Serialize;
use tauri::{LogicalSize, State, WebviewWindow};

use crate::commands::ShellState;
use crate::config::{config_path, Onboarding};
use crate::server;
use crate::window_size;
use crate::zoom;

pub const COMPACT_SIZE: (f64, f64) = (520.0, 620.0);

/// Set for a run that opened on onboarding's compact window.
pub static STARTED_COMPACT: AtomicBool = AtomicBool::new(false);
/// What the window-state plugin calls the main window in such a run, so its
/// compact size never lands in, or is restored from, the main window's record.
pub const COMPACT_STATE_LABEL: &str = "main-onboarding";

/// `HYLO_ONBOARDING=1` replays the flow on an existing install.
const FORCE_ENV: &str = "HYLO_ONBOARDING";

/// Only a machine that has never run Hylo onboards; an upgrade or a CLI user
/// who already has a vault goes straight in.
pub fn should_start(data_dir: &Path) -> bool {
    if std::env::var(FORCE_ENV).is_ok_and(|v| v == "1") {
        return true;
    }
    if config_path(data_dir).exists() {
        return false;
    }
    let hylo = server::home_dir().join(".hylo");
    !hylo.join("config.toml").exists() && !hylo.join("root").exists()
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct OnboardingView {
    pending: bool,
    step: String,
}

#[tauri::command]
pub fn onboarding_get(state: State<ShellState>) -> Result<OnboardingView, String> {
    state.read_config(|cfg| OnboardingView {
        pending: cfg.onboarding.pending,
        step: cfg.onboarding.step.clone(),
    })
}

#[tauri::command]
pub fn onboarding_set_step(state: State<ShellState>, step: String) -> Result<(), String> {
    state.edit_config(|cfg| {
        if cfg.onboarding.pending {
            cfg.onboarding.step = step;
        }
        Ok(())
    })
}

/// Marks onboarding done and grows the compact window into the main one.
/// `landing` is the app path the first page opens on (see servers::enter).
#[tauri::command]
pub fn onboarding_finish(
    state: State<ShellState>,
    window: WebviewWindow,
    landing: Option<String>,
) -> Result<(), String> {
    state.edit_config(|cfg| {
        cfg.onboarding = Onboarding::default();
        Ok(())
    })?;
    if let Some(path) = landing.filter(|p| p.starts_with("/home")) {
        if let Ok(mut slot) = state.landing.lock() {
            *slot = Some(path);
        }
    }
    let _ = window.set_resizable(true);
    let _ = window.set_maximizable(true);
    let after = window.clone();
    zoom::grow_centered(&window, window_size::fit_main_size, move || {
        let (w, h) = window_size::MAIN_MIN_SIZE;
        let _ = after.set_min_size(Some(LogicalSize::new(w, h)));
    });
    Ok(())
}
