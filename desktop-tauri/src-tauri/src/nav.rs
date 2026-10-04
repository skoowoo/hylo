use url::Url;

pub const DEV_START_URL: &str = "http://127.0.0.1:1430/index.html";

pub fn shell_start_url() -> &'static str {
    // `tauri dev` clears the custom-protocol feature, which sets cfg(dev)
    // and loads devUrl. A plain debug build still serves the embedded page.
    if cfg!(dev) {
        DEV_START_URL
    } else if cfg!(windows) {
        "http://tauri.localhost/index.html"
    } else {
        "tauri://localhost/index.html"
    }
}

pub fn is_shell_url(url: &Url) -> bool {
    let s = url.as_str();
    s.starts_with("tauri://localhost")
        || s.starts_with("http://tauri.localhost")
        || s.starts_with("https://tauri.localhost")
        || s.starts_with("http://127.0.0.1:1430")
        || s.starts_with("http://localhost:1430")
}

/// `scheme://host:port` with brackets around IPv6, no path.
pub fn http_origin(url: &Url) -> Option<String> {
    if !matches!(url.scheme(), "http" | "https") {
        return None;
    }
    let host = match url.host()? {
        url::Host::Ipv6(addr) => format!("[{addr}]"),
        other => other.to_string(),
    };
    match url.port() {
        Some(port) => Some(format!("{}://{host}:{port}", url.scheme())),
        None => Some(format!("{}://{host}", url.scheme())),
    }
}

pub fn same_origin(page: &Url, server: &str) -> bool {
    let Ok(base) = Url::parse(server) else {
        return false;
    };
    page.scheme() == base.scheme()
        && page.host_str() == base.host_str()
        && page.port_or_known_default() == base.port_or_known_default()
}

/// Main-frame navigations stay inside the shell page or the configured Hylo origin.
/// Anything else is handed to the OS browser by the caller.
pub fn allow_navigation(url: &Url, server: Option<&str>) -> bool {
    if is_shell_url(url) {
        return true;
    }
    match server {
        Some(server) => same_origin(url, server),
        None => false,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn allows_shell_and_saved_origin_only() {
        let home = Url::parse("http://127.0.0.1:54321/home").unwrap();
        let other = Url::parse("https://example.com/").unwrap();
        let start = Url::parse(DEV_START_URL).unwrap();
        assert!(allow_navigation(&start, None));
        assert!(allow_navigation(&home, Some("http://127.0.0.1:54321")));
        assert!(!allow_navigation(&other, Some("http://127.0.0.1:54321")));
        assert!(!allow_navigation(&home, None));
    }

    #[test]
    fn http_origin_keeps_port_and_ipv6_brackets() {
        let v4 = Url::parse("http://127.0.0.1:54321/home").unwrap();
        let v6 = Url::parse("http://[::1]:54321/home").unwrap();
        let named = Url::parse("https://notes.example/a/b").unwrap();
        assert_eq!(http_origin(&v4).as_deref(), Some("http://127.0.0.1:54321"));
        assert_eq!(http_origin(&v6).as_deref(), Some("http://[::1]:54321"));
        assert_eq!(
            http_origin(&named).as_deref(),
            Some("https://notes.example")
        );
        assert!(http_origin(&Url::parse("tauri://localhost/index.html").unwrap()).is_none());
    }
}
