// Language signals for the server; input methods matter because many people
// type another language on an English system.
use serde::Serialize;
use tauri::AppHandle;

#[derive(Debug, Default, Serialize)]
pub struct LangSignals {
    /// Preferred UI languages, most preferred first.
    languages: Vec<String>,
    /// The formats locale, whose region says where they live.
    locale: String,
    /// Languages of the enabled keyboard input sources.
    inputs: Vec<String>,
}

pub async fn lang_signals(app: &AppHandle) -> LangSignals {
    LangSignals {
        languages: sys_locale::get_locales().collect(),
        locale: current_locale(),
        inputs: input_languages(app).await,
    }
}

fn dedup(langs: impl IntoIterator<Item = String>) -> Vec<String> {
    let mut out: Vec<String> = Vec::new();
    for l in langs {
        if !l.is_empty() && !out.contains(&l) {
            out.push(l);
        }
    }
    out
}

#[cfg(target_os = "macos")]
async fn input_languages(app: &AppHandle) -> Vec<String> {
    // Text Input Sources asserts it runs on the main thread.
    let (tx, rx) = tokio::sync::oneshot::channel();
    if app
        .run_on_main_thread(move || {
            let _ = tx.send(mac::input_languages());
        })
        .is_err()
    {
        return Vec::new();
    }
    rx.await.unwrap_or_default()
}

#[cfg(target_os = "macos")]
fn current_locale() -> String {
    mac::current_locale()
}

#[cfg(windows)]
async fn input_languages(_app: &AppHandle) -> Vec<String> {
    win::input_languages()
}

#[cfg(windows)]
fn current_locale() -> String {
    win::current_locale()
}

#[cfg(not(any(target_os = "macos", windows)))]
async fn input_languages(_app: &AppHandle) -> Vec<String> {
    Vec::new()
}

#[cfg(not(any(target_os = "macos", windows)))]
fn current_locale() -> String {
    String::new()
}

#[cfg(target_os = "macos")]
mod mac {
    use std::ffi::{c_char, c_void, CStr};

    type CFTypeRef = *const c_void;
    const UTF8: u32 = 0x0800_0100;

    #[link(name = "Carbon", kind = "framework")]
    extern "C" {
        static kTISPropertyInputSourceCategory: CFTypeRef;
        static kTISCategoryKeyboardInputSource: CFTypeRef;
        static kTISPropertyInputSourceLanguages: CFTypeRef;
        fn TISCreateInputSourceList(properties: CFTypeRef, include_all_installed: u8) -> CFTypeRef;
        fn TISGetInputSourceProperty(source: CFTypeRef, key: CFTypeRef) -> CFTypeRef;
    }

    #[link(name = "CoreFoundation", kind = "framework")]
    extern "C" {
        fn CFArrayGetCount(array: CFTypeRef) -> isize;
        fn CFArrayGetValueAtIndex(array: CFTypeRef, index: isize) -> CFTypeRef;
        fn CFEqual(a: CFTypeRef, b: CFTypeRef) -> u8;
        fn CFRelease(cf: CFTypeRef);
        fn CFStringGetCString(s: CFTypeRef, buf: *mut c_char, size: isize, encoding: u32) -> u8;
        fn CFLocaleCopyCurrent() -> CFTypeRef;
        fn CFLocaleGetIdentifier(locale: CFTypeRef) -> CFTypeRef;
    }

    unsafe fn string(s: CFTypeRef) -> Option<String> {
        if s.is_null() {
            return None;
        }
        let mut buf = [0 as c_char; 64];
        if CFStringGetCString(s, buf.as_mut_ptr(), buf.len() as isize, UTF8) == 0 {
            return None;
        }
        Some(CStr::from_ptr(buf.as_ptr()).to_string_lossy().into_owned())
    }

    pub fn input_languages() -> Vec<String> {
        unsafe {
            let list = TISCreateInputSourceList(std::ptr::null(), 0);
            if list.is_null() {
                return Vec::new();
            }
            let mut langs = Vec::new();
            for i in 0..CFArrayGetCount(list) {
                let src = CFArrayGetValueAtIndex(list, i);
                let category = TISGetInputSourceProperty(src, kTISPropertyInputSourceCategory);
                if category.is_null() || CFEqual(category, kTISCategoryKeyboardInputSource) == 0 {
                    continue;
                }
                // The first language is the one the source is for; the rest it can merely type.
                let ls = TISGetInputSourceProperty(src, kTISPropertyInputSourceLanguages);
                if !ls.is_null() && CFArrayGetCount(ls) > 0 {
                    langs.extend(string(CFArrayGetValueAtIndex(ls, 0)));
                }
            }
            CFRelease(list);
            super::dedup(langs)
        }
    }

    pub fn current_locale() -> String {
        unsafe {
            let locale = CFLocaleCopyCurrent();
            if locale.is_null() {
                return String::new();
            }
            let id = string(CFLocaleGetIdentifier(locale)).unwrap_or_default();
            CFRelease(locale);
            id
        }
    }
}

#[cfg(windows)]
mod win {
    use windows_sys::Win32::Globalization::{GetUserDefaultLocaleName, LCIDToLocaleName};
    use windows_sys::Win32::UI::Input::KeyboardAndMouse::GetKeyboardLayoutList;

    const NAME_LEN: usize = 85;

    pub fn input_languages() -> Vec<String> {
        unsafe {
            let n = GetKeyboardLayoutList(0, std::ptr::null_mut());
            if n <= 0 {
                return Vec::new();
            }
            let mut layouts = vec![std::ptr::null_mut(); n as usize];
            let n = GetKeyboardLayoutList(n, layouts.as_mut_ptr());
            let langs = layouts
                .into_iter()
                .take(n.max(0) as usize)
                .filter_map(|hkl| {
                    // The low word of a layout handle is its language id.
                    let lang_id = (hkl as usize & 0xffff) as u32;
                    let mut buf = [0u16; NAME_LEN];
                    let len = LCIDToLocaleName(lang_id, buf.as_mut_ptr(), buf.len() as i32, 0);
                    (len > 1).then(|| String::from_utf16_lossy(&buf[..len as usize - 1]))
                });
            super::dedup(langs)
        }
    }

    pub fn current_locale() -> String {
        let mut buf = [0u16; NAME_LEN];
        let len = unsafe { GetUserDefaultLocaleName(buf.as_mut_ptr(), buf.len() as i32) };
        if len <= 1 {
            return String::new();
        }
        String::from_utf16_lossy(&buf[..len as usize - 1])
    }
}
