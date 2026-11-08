//! Banners via UserNotifications. The process must be an app bundle; a raw
//! `tauri dev` binary has no bundle id, and `currentNotificationCenter` aborts
//! without one, so that case falls back to `osascript`.

use std::process::{Command, Stdio};
use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
use std::sync::OnceLock;
use std::time::{SystemTime, UNIX_EPOCH};

use block2::RcBlock;
use objc2::rc::Retained;
use objc2::runtime::{Bool, NSObjectProtocol, ProtocolObject};
use objc2::{define_class, msg_send, AnyThread};
use objc2_foundation::{NSBundle, NSError, NSObject, NSString};
use objc2_user_notifications::{
    UNAuthorizationOptions, UNAuthorizationStatus, UNMutableNotificationContent, UNNotification,
    UNNotificationPresentationOptions, UNNotificationRequest, UNNotificationResponse,
    UNNotificationSettings, UNNotificationTrigger, UNUserNotificationCenter,
    UNUserNotificationCenterDelegate,
};
use tauri::{AppHandle, Manager};

static APP: OnceLock<AppHandle> = OnceLock::new();
static DELEGATE: OnceLock<Retained<BannerDelegate>> = OnceLock::new();
static DENIED_LOGGED: AtomicBool = AtomicBool::new(false);
static NEXT_ID: AtomicU64 = AtomicU64::new(1);

pub fn install(app: &AppHandle) {
    let _ = APP.set(app.clone());
    let Some(center) = center() else {
        return;
    };
    set_delegate(&center);
    let handler = RcBlock::new(|_granted: Bool, _err: *mut NSError| {});
    center.requestAuthorizationWithOptions_completionHandler(auth_options(), &handler);
}

pub fn show(title: &str, body: &str) -> Result<(), String> {
    let Some(center) = center() else {
        return script_notify(title, body);
    };
    set_delegate(&center);
    let content = UNMutableNotificationContent::new();
    content.setTitle(&NSString::from_str(title));
    content.setBody(&NSString::from_str(body));
    let request = UNNotificationRequest::requestWithIdentifier_content_trigger(
        &NSString::from_str(&next_id()),
        &content,
        None::<&UNNotificationTrigger>,
    );
    let posted = center.clone();
    let handler = RcBlock::new(move |settings: std::ptr::NonNull<UNNotificationSettings>| {
        let status = unsafe { settings.as_ref() }.authorizationStatus();
        enqueue(posted.clone(), request.clone(), status);
    });
    center.getNotificationSettingsWithCompletionHandler(&handler);
    Ok(())
}

fn center() -> Option<Retained<UNUserNotificationCenter>> {
    NSBundle::mainBundle().bundleIdentifier()?;
    Some(UNUserNotificationCenter::currentNotificationCenter())
}

fn auth_options() -> UNAuthorizationOptions {
    UNAuthorizationOptions::Alert | UNAuthorizationOptions::Sound
}

fn set_delegate(center: &UNUserNotificationCenter) {
    let delegate = DELEGATE.get_or_init(BannerDelegate::new);
    center.setDelegate(Some(ProtocolObject::from_ref(&**delegate)));
}

fn enqueue(
    center: Retained<UNUserNotificationCenter>,
    request: Retained<UNNotificationRequest>,
    status: UNAuthorizationStatus,
) {
    if delivers(status) {
        add(&center, &request);
        return;
    }
    if status != UNAuthorizationStatus::NotDetermined {
        note_denied();
        return;
    }
    let asking = center.clone();
    let handler = RcBlock::new(move |granted: Bool, _err: *mut NSError| {
        if granted.as_bool() {
            add(&asking, &request);
        } else {
            note_denied();
        }
    });
    center.requestAuthorizationWithOptions_completionHandler(auth_options(), &handler);
}

fn delivers(status: UNAuthorizationStatus) -> bool {
    status == UNAuthorizationStatus::Authorized
        || status == UNAuthorizationStatus::Provisional
        || status == UNAuthorizationStatus::Ephemeral
}

fn add(center: &UNUserNotificationCenter, request: &UNNotificationRequest) {
    let handler = RcBlock::new(|err: *mut NSError| {
        if let Some(err) = unsafe { err.as_ref() } {
            eprintln!("[hylo-tauri] notification: {}", err.localizedDescription());
        }
    });
    center.addNotificationRequest_withCompletionHandler(request, Some(&handler));
}

fn note_denied() {
    if DENIED_LOGGED.swap(true, Ordering::Relaxed) {
        return;
    }
    eprintln!("[hylo-tauri] notifications are turned off for Hylo in System Settings");
}

fn next_id() -> String {
    let ms = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|d| d.as_millis())
        .unwrap_or(0);
    format!("hylo-{ms}-{}", NEXT_ID.fetch_add(1, Ordering::Relaxed))
}

fn reveal() {
    let Some(app) = APP.get().cloned() else {
        return;
    };
    let _ = app.clone().run_on_main_thread(move || {
        let Some(window) = app.get_webview_window("main") else {
            return;
        };
        let _ = window.unminimize();
        let _ = window.show();
        let _ = window.set_focus();
    });
}

/// `display notification` still banners when this process is not an app bundle.
fn script_notify(title: &str, body: &str) -> Result<(), String> {
    let script = format!(
        "display notification \"{}\" with title \"{}\"",
        apple_string(body),
        apple_string(title),
    );
    Command::new("/usr/bin/osascript")
        .arg("-e")
        .arg(script)
        .stdin(Stdio::null())
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .spawn()
        .map(|_| ())
        .map_err(|e| e.to_string())
}

fn apple_string(s: &str) -> String {
    let mut out = String::with_capacity(s.len());
    for c in s.chars() {
        match c {
            '\\' => out.push_str("\\\\"),
            '"' => out.push_str("\\\""),
            '\n' | '\r' => out.push(' '),
            c => out.push(c),
        }
    }
    out
}

define_class!(
    #[unsafe(super(NSObject))]
    #[ivars = ()]
    struct BannerDelegate;

    unsafe impl NSObjectProtocol for BannerDelegate {}

    unsafe impl UNUserNotificationCenterDelegate for BannerDelegate {
        #[allow(non_snake_case)]
        #[unsafe(method(userNotificationCenter:willPresentNotification:withCompletionHandler:))]
        fn userNotificationCenter_willPresentNotification_withCompletionHandler(
            &self,
            _center: &UNUserNotificationCenter,
            _notification: &UNNotification,
            completion_handler: &block2::DynBlock<dyn Fn(UNNotificationPresentationOptions)>,
        ) {
            // Frontmost apps get no banner unless the delegate asks for one.
            // Sound stays off here: the shell plays its own.
            let opts =
                UNNotificationPresentationOptions::Banner | UNNotificationPresentationOptions::List;
            completion_handler.call((opts,));
        }

        #[allow(non_snake_case)]
        #[unsafe(method(userNotificationCenter:didReceiveNotificationResponse:withCompletionHandler:))]
        fn userNotificationCenter_didReceiveNotificationResponse_withCompletionHandler(
            &self,
            _center: &UNUserNotificationCenter,
            _response: &UNNotificationResponse,
            completion_handler: &block2::DynBlock<dyn Fn()>,
        ) {
            reveal();
            completion_handler.call(());
        }
    }
);

impl BannerDelegate {
    fn new() -> Retained<Self> {
        let this = Self::alloc().set_ivars(());
        unsafe { msg_send![super(this), init] }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn escapes_applescript_strings() {
        assert_eq!(apple_string(r#"say "hi"\"#), r#"say \"hi\"\\"#);
        assert_eq!(apple_string("a\nb"), "a b");
    }
}
