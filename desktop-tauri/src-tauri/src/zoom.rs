// AppKit's zoom: animation blocks the main thread, so WKWebView can't commit a
// frame until it ends: the vibrancy background grows first and the page snaps
// afterwards. The window animator runs off the run loop instead, letting the
// page repaint at every step.

#[cfg(target_os = "macos")]
pub fn toggle(window: &tauri::WebviewWindow) {
    use std::sync::Mutex;

    use objc2_app_kit::{NSAnimatablePropertyContainer, NSAnimationContext, NSWindow};
    use objc2_foundation::NSRect;

    static RESTORE: Mutex<Option<NSRect>> = Mutex::new(None);

    let Ok(ptr) = window.ns_window() else {
        return;
    };
    let ptr = ptr as usize;
    let _ = window.run_on_main_thread(move || {
        let ns = unsafe { &*(ptr as *const NSWindow) };
        let Some(screen) = ns.screen() else {
            return;
        };
        let mut restore = RESTORE.lock().unwrap();
        let target = if ns.isZoomed() {
            match restore.take() {
                Some(frame) => frame,
                // Zoomed by AppKit itself (menu, green button): only it knows the old frame.
                None => return ns.zoom(None),
            }
        } else {
            *restore = Some(ns.frame());
            screen.visibleFrame()
        };
        NSAnimationContext::beginGrouping();
        NSAnimationContext::currentContext().setDuration(0.25);
        ns.animator().setFrame_display(target, true);
        NSAnimationContext::endGrouping();
    });
}

#[cfg(not(target_os = "macos"))]
pub fn toggle(window: &tauri::WebviewWindow) {
    if window.is_maximized().unwrap_or(false) {
        let _ = window.unmaximize();
    } else {
        let _ = window.maximize();
    }
}
