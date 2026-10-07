use std::path::Path;
use std::process::Command;
use std::time::Duration;

use reqwest::{redirect::Policy, Client, StatusCode};
use serde::Deserialize;

/// Why a server could not be used. The codes are the UI contract.
#[derive(Debug, PartialEq, Eq)]
pub enum ServerError {
    Unreachable(String),
    NotHylo,
    BadKey,
}

impl ServerError {
    pub fn code(&self) -> &'static str {
        match self {
            Self::Unreachable(_) => "unreachable",
            Self::NotHylo => "not_hylo",
            Self::BadKey => "bad_key",
        }
    }

    pub fn message(&self) -> String {
        match self {
            Self::Unreachable(e) => format!("cannot reach the server ({e})"),
            Self::NotHylo => "this address is not a Hylo server".into(),
            Self::BadKey => "the API key was rejected".into(),
        }
    }
}

#[derive(Deserialize)]
struct Identity {
    app: String,
}

fn client() -> Result<Client, ServerError> {
    Client::builder()
        .timeout(Duration::from_secs(5))
        .redirect(Policy::none())
        .no_proxy()
        .build()
        .map_err(|e| ServerError::Unreachable(e.to_string()))
}

/// POST /version is unauthenticated and says whether the URL is a Hylo server.
async fn ensure_hylo(base: &str) -> Result<(), ServerError> {
    let resp = client()?
        .post(format!("{base}/version"))
        .send()
        .await
        .map_err(|e| ServerError::Unreachable(e.to_string()))?;
    let ident: Identity = resp.json().await.map_err(|_| ServerError::NotHylo)?;
    if ident.app == "hylo" {
        Ok(())
    } else {
        Err(ServerError::NotHylo)
    }
}

/// Whether the server accepts `key` (none is sent when absent).
async fn check_key(base: &str, key: Option<&str>) -> Result<(), ServerError> {
    let mut req = client()?.post(format!("{base}/api/status"));
    if let Some(k) = key.filter(|k| !k.is_empty()) {
        req = req.header("X-Hylo-API-Key", k);
    }
    let resp = req
        .send()
        .await
        .map_err(|e| ServerError::Unreachable(e.to_string()))?;
    match resp.status() {
        StatusCode::UNAUTHORIZED | StatusCode::FORBIDDEN => Err(ServerError::BadKey),
        _ => Ok(()),
    }
}

/// Confirms `base` is a Hylo server and that `key` (if any) is accepted.
pub async fn verify(base: &str, key: Option<&str>) -> Result<(), ServerError> {
    ensure_hylo(base).await?;
    check_key(base, key).await
}

/// A session cookie obtained by logging in with `key`.
#[derive(Debug, PartialEq, Eq)]
pub struct Session {
    pub name: String,
    pub value: String,
}

pub async fn login(base: &str, key: &str) -> Result<Session, ServerError> {
    let resp = client()?
        .post(format!("{base}/login"))
        .form(&[("key", key), ("next", "/home")])
        .send()
        .await
        .map_err(|e| ServerError::Unreachable(e.to_string()))?;
    match resp.status() {
        StatusCode::UNAUTHORIZED => return Err(ServerError::BadKey),
        _ => {}
    }
    resp.headers()
        .get_all(reqwest::header::SET_COOKIE)
        .iter()
        .filter_map(|v| v.to_str().ok())
        .filter_map(|v| v.split(';').next())
        .filter_map(|pair| pair.split_once('='))
        .find(|(name, _)| *name == "hylo_session")
        .map(|(name, value)| Session { name: name.into(), value: value.into() })
        .ok_or(ServerError::NotHylo)
}

/// Asks the installed CLI for the local server's key. A key is never served
/// over HTTP without credentials; the CLI reads the same 0600 config file the
/// server wrote it to, so only this OS user can get it.
pub fn read_local_key(bin: &Path) -> Option<String> {
    // A GUI app starts with a minimal PATH; use the same one that launched the server.
    let out = Command::new(bin)
        .args(["auth", "show", "--raw"])
        .env("PATH", crate::server::expanded_path())
        .stdin(std::process::Stdio::null())
        .output()
        .ok()?;
    if !out.status.success() {
        return None;
    }
    let key = String::from_utf8(out.stdout).ok()?.trim().to_string();
    (!key.is_empty() && !key.contains(char::is_whitespace)).then_some(key)
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, serde::Serialize)]
#[serde(rename_all = "snake_case")]
pub enum Health {
    Online,
    Offline,
    AuthRequired,
}

/// Reachability plus whether the stored key (if any) is still accepted.
pub async fn health(base: &str, key: Option<&str>) -> Health {
    match check_key(base, key).await {
        Ok(()) => Health::Online,
        Err(ServerError::BadKey) => Health::AuthRequired,
        Err(_) => Health::Offline,
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::io::{Read, Write};
    use std::net::TcpListener;

    /// One-route fake: answers every request with the closure's raw HTTP response.
    fn serve(respond: impl Fn(&str) -> String + Send + 'static) -> String {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let addr = listener.local_addr().unwrap();
        std::thread::spawn(move || {
            for stream in listener.incoming() {
                let Ok(mut s) = stream else { break };
                let mut buf = [0u8; 4096];
                let n = s.read(&mut buf).unwrap_or(0);
                let req = String::from_utf8_lossy(&buf[..n]).to_string();
                let _ = s.write_all(respond(&req).as_bytes());
            }
        });
        format!("http://{addr}")
    }

    fn http(status: &str, headers: &str, body: &str) -> String {
        format!(
            "HTTP/1.1 {status}\r\nContent-Length: {}\r\nConnection: close\r\n{headers}\r\n{body}",
            body.len()
        )
    }

    const HYLO: &str = r#"{"app":"hylo"}"#;

    fn hylo_server(accept_key: &'static str) -> String {
        serve(move |req| {
            if req.starts_with("POST /version") {
                http("200 OK", "Content-Type: application/json\r\n", HYLO)
            } else if req.to_lowercase().contains(&format!("x-hylo-api-key: {accept_key}")) {
                http("200 OK", "", "{}")
            } else {
                http("401 Unauthorized", "", "Unauthorized")
            }
        })
    }

    #[tokio::test]
    async fn verify_accepts_good_key() {
        let base = hylo_server("secret");
        verify(&base, Some("secret")).await.unwrap();
    }

    #[tokio::test]
    async fn verify_rejects_bad_or_missing_key() {
        let base = hylo_server("secret");
        assert_eq!(verify(&base, Some("nope")).await.unwrap_err(), ServerError::BadKey);
        assert_eq!(verify(&base, None).await.unwrap_err(), ServerError::BadKey);
    }

    #[tokio::test]
    async fn non_hylo_server_is_detected() {
        let base = serve(|_| http("200 OK", "", "<html>hi</html>"));
        assert_eq!(ensure_hylo(&base).await.unwrap_err(), ServerError::NotHylo);
        let other = serve(|_| http("200 OK", "", r#"{"app":"nginx"}"#));
        assert_eq!(ensure_hylo(&other).await.unwrap_err(), ServerError::NotHylo);
    }

    #[tokio::test]
    async fn closed_port_is_unreachable() {
        let l = TcpListener::bind("127.0.0.1:0").unwrap();
        let addr = l.local_addr().unwrap();
        drop(l);
        let err = verify(&format!("http://{addr}"), None).await.unwrap_err();
        assert_eq!(err.code(), "unreachable");
    }

    #[tokio::test]
    async fn login_extracts_session_cookie() {
        let base = serve(|req| {
            if req.contains("key=good") {
                http(
                    "303 See Other",
                    "Location: /home\r\nSet-Cookie: hylo_session=abc123; Path=/; HttpOnly\r\n",
                    "",
                )
            } else {
                http("401 Unauthorized", "", "")
            }
        });
        let s = login(&base, "good").await.unwrap();
        assert_eq!(s, Session { name: "hylo_session".into(), value: "abc123".into() });
        assert_eq!(login(&base, "bad").await.unwrap_err(), ServerError::BadKey);
    }

    #[cfg(unix)]
    fn script(body: &str) -> std::path::PathBuf {
        use std::os::unix::fs::PermissionsExt;
        use std::sync::atomic::{AtomicUsize, Ordering};
        static N: AtomicUsize = AtomicUsize::new(0);
        let path = std::env::temp_dir().join(format!(
            "hylo-stub-{}-{}",
            std::process::id(),
            N.fetch_add(1, Ordering::Relaxed)
        ));
        std::fs::write(&path, format!("#!/bin/sh\n{body}\n")).unwrap();
        std::fs::set_permissions(&path, std::fs::Permissions::from_mode(0o755)).unwrap();
        path
    }

    #[cfg(unix)]
    #[test]
    fn local_key_comes_from_the_cli() {
        let ok = script(r#"[ "$1 $2 $3" = "auth show --raw" ] && echo hylo_abc123"#);
        assert_eq!(read_local_key(&ok).as_deref(), Some("hylo_abc123"));

        let no_key = script("echo 'Error: no API key yet' >&2; exit 1");
        assert_eq!(read_local_key(&no_key), None);

        let noisy = script("echo 'two words'");
        assert_eq!(read_local_key(&noisy), None);

        assert_eq!(read_local_key(Path::new("/nonexistent/hylo")), None);
    }

    #[tokio::test]
    async fn health_maps_errors() {
        let base = hylo_server("secret");
        assert_eq!(health(&base, Some("secret")).await, Health::Online);
        assert_eq!(health(&base, Some("x")).await, Health::AuthRequired);
        assert_eq!(health("http://127.0.0.1:1", None).await, Health::Offline);
    }
}
