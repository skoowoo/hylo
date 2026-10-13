// The main window's size: fit to the screen it opens on, in the proportions
// its layout is designed for.
use tauri::{LogicalSize, WebviewWindow};

/// Only when no screen can be read.
const MAIN_SIZE: (f64, f64) = (1440.0, 932.0);
/// A 15" MacBook Air's default 1440×932: the shape the main window is laid out for.
const MAIN_ASPECT: f64 = 1440.0 / 932.0;
const MAIN_MAX_WIDTH: f64 = 1440.0;
/// Room left around the window so it reads as a window, not a takeover.
const SCREEN_FILL: f64 = 0.88;
pub const MAIN_MIN_SIZE: (f64, f64) = (960.0, 640.0);

/// The main window for a screen's visible area: 15-inch proportions, as big
/// as fits with a margin, never wider than the layout needs.
pub fn fit_main_size(visible_w: f64, visible_h: f64) -> (f64, f64) {
    let mut w = (visible_w * SCREEN_FILL).min(MAIN_MAX_WIDTH);
    let mut h = w / MAIN_ASPECT;
    if h > visible_h * SCREEN_FILL {
        h = visible_h * SCREEN_FILL;
        w = h * MAIN_ASPECT;
    }
    let (min_w, min_h) = MAIN_MIN_SIZE;
    (
        w.max(min_w).min(visible_w).round(),
        h.max(min_h).min(visible_h).round(),
    )
}

/// The main window's size at launch, from the primary screen's work area;
/// the same fit the onboarding window grows into.
pub fn launch_main_size(app: &tauri::AppHandle) -> (f64, f64) {
    app.primary_monitor()
        .ok()
        .flatten()
        .map(|m| {
            let area = m.work_area().size.to_logical::<f64>(m.scale_factor());
            fit_main_size(area.width, area.height)
        })
        .unwrap_or(MAIN_SIZE)
}

/// A restored size can come from a bigger display than the one the window
/// opens on now; refit it rather than spill past the screen.
pub fn keep_on_screen(window: &WebviewWindow) {
    let (Ok(Some(m)), Ok(size)) = (window.current_monitor(), window.outer_size()) else {
        return;
    };
    let area = m.work_area().size;
    if size.width <= area.width && size.height <= area.height {
        return;
    }
    let logical = area.to_logical::<f64>(m.scale_factor());
    let (w, h) = fit_main_size(logical.width, logical.height);
    let _ = window.set_size(LogicalSize::new(w, h));
    let _ = window.center();
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn main_window_fits_the_screen_in_15_inch_proportions() {
        // 15" MacBook Air, menu bar and dock taken out.
        let (w, h) = fit_main_size(1440.0, 875.0);
        assert!(h <= 875.0 * SCREEN_FILL + 1.0 && w <= 1440.0);
        assert!((w / h - MAIN_ASPECT).abs() < 0.01);
        // A big display stops at the layout's width.
        assert_eq!(fit_main_size(2560.0, 1415.0), (1440.0, 932.0));
        // A small one never goes under the minimum the app supports.
        assert_eq!(fit_main_size(1000.0, 700.0), MAIN_MIN_SIZE);
    }
}
