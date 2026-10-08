// AppKit's zoom: animation blocks the main thread, so WKWebView can't commit a
// frame until it ends: the vibrancy background grows first and the page snaps
// afterwards. The window animator runs off the run loop instead, letting the
// page repaint at every step.
//
// Even then WKWebView renders out of process and trails the window frame, so a
// growing window exposes the vibrancy layer on its right and bottom edges. When
// growing, the webview is sized to the target first and the window grows over
// an already painted page.

#[cfg(target_os = "macos")]
pub fn toggle(window: &tauri::WebviewWindow) {
    use std::sync::Mutex;

    use block2::RcBlock;
    use objc2::{msg_send, runtime::NSObjectProtocol, sel};
    use objc2_app_kit::{
        NSAnimatablePropertyContainer, NSAnimationContext, NSAutoresizingMaskOptions, NSView,
        NSWindow,
    };
    use objc2_foundation::{NSRect, NSSize};

    static RESTORE: Mutex<Option<NSRect>> = Mutex::new(None);

    fn animate(ns: &NSWindow, target: NSRect, done: Option<&block2::DynBlock<dyn Fn()>>) {
        let changes = RcBlock::new(move |ctx: std::ptr::NonNull<NSAnimationContext>| {
            unsafe { ctx.as_ref() }.setDuration(0.25);
            ns.animator().setFrame_display(target, true);
        });
        NSAnimationContext::runAnimationGroup_completionHandler(&changes, done);
    }

    let _ = window.with_webview(move |pw| {
        let ns = unsafe { &*(pw.ns_window() as *const NSWindow) };
        let web = unsafe { &*(pw.inner() as *const NSView) };
        let Some(screen) = ns.screen() else {
            return;
        };
        let mut restore = RESTORE.lock().unwrap();
        if ns.isZoomed() {
            match restore.take() {
                Some(frame) => animate(ns, frame, None),
                // Zoomed by AppKit itself (menu, green button): only it knows the old frame.
                None => ns.zoom(None),
            }
            return;
        }
        let current = ns.frame();
        *restore = Some(current);
        let target = screen.visibleFrame();
        let dw = (target.size.width - current.size.width).max(0.0);
        let dh = (target.size.height - current.size.height).max(0.0);
        let Some(parent) = (unsafe { web.superview() }) else {
            return animate(ns, target, None);
        };
        let flipped = parent.isFlipped();
        let mut frame = web.frame();
        frame.size = NSSize::new(frame.size.width + dw, frame.size.height + dh);
        if !flipped {
            frame.origin.y -= dh;
        }
        // Fixed size, pinned to the top edge while the window catches up.
        web.setAutoresizingMask(if flipped {
            NSAutoresizingMaskOptions::ViewMaxYMargin
        } else {
            NSAutoresizingMaskOptions::ViewMinYMargin
        });
        web.setFrame(frame);

        let ns_ptr = ns as *const NSWindow as usize;
        let web_ptr = web as *const NSView as usize;
        let start = RcBlock::new(move || {
            let ns = unsafe { &*(ns_ptr as *const NSWindow) };
            let done = RcBlock::new(move || {
                let web = unsafe { &*(web_ptr as *const NSView) };
                web.setAutoresizingMask(
                    NSAutoresizingMaskOptions::ViewWidthSizable
                        | NSAutoresizingMaskOptions::ViewHeightSizable,
                );
                if let Some(parent) = unsafe { web.superview() } {
                    web.setFrame(parent.bounds());
                }
            });
            animate(ns, target, Some(&done));
        });
        // Private WebKit hook that fires once the resized page is on screen.
        let after_paint = sel!(_doAfterNextPresentationUpdate:);
        if web.respondsToSelector(after_paint) {
            let _: () = unsafe { msg_send![web, _doAfterNextPresentationUpdate: &*start] };
        } else {
            start.call(());
        }
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
