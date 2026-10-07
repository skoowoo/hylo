use std::fs;
use std::path::{Path, PathBuf};

use serde::{Deserialize, Serialize};

pub const DEFAULT_SERVER_URL: &str = "http://127.0.0.1:54321";

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct InboxNotify {
    #[serde(default = "default_true")]
    pub text_enabled: bool,
    #[serde(default = "default_true")]
    pub sound_enabled: bool,
    #[serde(default = "default_sound")]
    pub sound: String,
}

impl Default for InboxNotify {
    fn default() -> Self {
        Self {
            text_enabled: true,
            sound_enabled: true,
            sound: default_sound(),
        }
    }
}

fn default_true() -> bool {
    true
}

fn default_sound() -> String {
    "beep".to_string()
}

pub const LOCAL_ID: &str = "local";

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum ServerKind {
    Local,
    Remote,
}

/// One Hylo server (a vault). `key` is the remote's API key, kept in this file
/// (see `save` for permissions) and never sent to the page.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct ServerEntry {
    pub id: String,
    pub name: String,
    pub url: String,
    pub kind: ServerKind,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub key: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct ConfigFile {
    #[serde(default)]
    pub servers: Vec<ServerEntry>,
    #[serde(default)]
    pub active_id: String,
    /// Missing means on.
    #[serde(default = "default_true")]
    pub auto_start_server: bool,
    #[serde(default)]
    pub inbox_notify: InboxNotify,
}

impl Default for ConfigFile {
    fn default() -> Self {
        let mut cfg = Self {
            servers: Vec::new(),
            active_id: String::new(),
            auto_start_server: true,
            inbox_notify: InboxNotify::default(),
        };
        cfg.ensure_valid();
        cfg
    }
}

/// Trims, drops a trailing slash and requires http(s) with a host.
pub fn normalize_url(raw: &str) -> Result<String, String> {
    let url = raw.trim().trim_end_matches('/');
    let parsed = url::Url::parse(url).map_err(|_| format!("invalid URL: {raw}"))?;
    if !matches!(parsed.scheme(), "http" | "https") || parsed.host_str().is_none() {
        return Err("URL must start with http:// or https://".into());
    }
    Ok(url.to_string())
}

impl ConfigFile {
    pub fn load(data_dir: &Path) -> Self {
        let path = config_path(data_dir);
        let Ok(raw) = fs::read_to_string(path) else {
            return Self::default();
        };
        let mut cfg: Self = serde_json::from_str(&raw).unwrap_or_default();
        cfg.ensure_valid();
        cfg
    }

    /// Guarantees a local entry and a valid active server.
    fn ensure_valid(&mut self) {
        if !self.servers.iter().any(|s| s.id == LOCAL_ID) {
            self.servers.insert(
                0,
                ServerEntry {
                    id: LOCAL_ID.into(),
                    name: "Local".into(),
                    url: DEFAULT_SERVER_URL.to_string(),
                    kind: ServerKind::Local,
                    key: None,
                },
            );
        }
        if !self.servers.iter().any(|s| s.id == self.active_id) {
            self.active_id = LOCAL_ID.into();
        }
    }

    fn new_id(&self) -> String {
        (1..)
            .map(|n| format!("srv-{n}"))
            .find(|id| !self.servers.iter().any(|s| &s.id == id))
            .expect("unbounded range")
    }

    pub fn active(&self) -> &ServerEntry {
        self.find(&self.active_id).expect("ensure_valid keeps active_id valid")
    }

    pub fn find(&self, id: &str) -> Option<&ServerEntry> {
        self.servers.iter().find(|s| s.id == id)
    }

    pub fn find_by_url(&self, url: &str) -> Option<&ServerEntry> {
        self.servers.iter().find(|s| s.url == url)
    }

    pub fn add_remote(
        &mut self,
        name: &str,
        url: &str,
        key: Option<String>,
    ) -> Result<ServerEntry, String> {
        let url = normalize_url(url)?;
        if self.find_by_url(&url).is_some() {
            return Err("this server is already in the list".into());
        }
        let name = match name.trim() {
            "" => host_label(&url),
            n => n.to_string(),
        };
        let entry = ServerEntry {
            id: self.new_id(),
            name,
            url,
            kind: ServerKind::Remote,
            key,
        };
        self.servers.push(entry.clone());
        Ok(entry)
    }

    pub fn set_key(&mut self, id: &str, key: String) -> Result<(), String> {
        let entry = self.servers.iter_mut().find(|s| s.id == id).ok_or("unknown server")?;
        entry.key = Some(key);
        Ok(())
    }

    pub fn rename(&mut self, id: &str, name: &str) -> Result<(), String> {
        let name = name.trim();
        if name.is_empty() {
            return Err("name cannot be empty".into());
        }
        let entry = self.servers.iter_mut().find(|s| s.id == id).ok_or("unknown server")?;
        entry.name = name.to_string();
        Ok(())
    }

    /// The local server cannot be removed. Removing the active one falls back to it.
    pub fn remove(&mut self, id: &str) -> Result<(), String> {
        if id == LOCAL_ID {
            return Err("the local server cannot be removed".into());
        }
        let before = self.servers.len();
        self.servers.retain(|s| s.id != id);
        if self.servers.len() == before {
            return Err("unknown server".into());
        }
        if self.active_id == id {
            self.active_id = LOCAL_ID.into();
        }
        Ok(())
    }

    pub fn save(&self, data_dir: &Path) -> Result<(), String> {
        fs::create_dir_all(data_dir).map_err(|e| e.to_string())?;
        let raw = serde_json::to_vec_pretty(self).map_err(|e| e.to_string())?;
        write_private(&config_path(data_dir), &raw).map_err(|e| e.to_string())
    }
}

/// The file holds API keys, so keep it readable by the owner only. Also
/// tightens a file an older version created with default permissions.
fn write_private(path: &Path, data: &[u8]) -> std::io::Result<()> {
    #[cfg(unix)]
    {
        use std::io::Write;
        use std::os::unix::fs::{OpenOptionsExt, PermissionsExt};
        let mut f = fs::OpenOptions::new()
            .write(true)
            .create(true)
            .truncate(true)
            .mode(0o600)
            .open(path)?;
        f.write_all(data)?;
        fs::set_permissions(path, fs::Permissions::from_mode(0o600))
    }
    #[cfg(not(unix))]
    {
        fs::write(path, data)
    }
}

fn host_label(url: &str) -> String {
    url::Url::parse(url)
        .ok()
        .and_then(|u| u.host_str().map(str::to_string))
        .unwrap_or_else(|| url.to_string())
}

pub fn config_path(data_dir: &Path) -> PathBuf {
    data_dir.join("config.json")
}

#[cfg(test)]
mod tests {
    use super::*;

    fn parse(json: &str) -> ConfigFile {
        let mut cfg: ConfigFile = serde_json::from_str(json).unwrap();
        cfg.ensure_valid();
        cfg
    }

    #[test]
    fn missing_autostart_stays_on() {
        let cfg = parse("{}");
        assert!(cfg.auto_start_server);
        assert!(cfg.inbox_notify.text_enabled);
        assert_eq!(cfg.inbox_notify.sound, "beep");
    }

    #[test]
    fn empty_config_gets_the_local_server() {
        let cfg = parse("{}");
        assert_eq!(cfg.servers.len(), 1);
        assert_eq!(cfg.active_id, LOCAL_ID);
        assert_eq!(cfg.active().url, DEFAULT_SERVER_URL);
        assert_eq!(cfg.active().kind, ServerKind::Local);
    }

    #[test]
    fn dangling_active_id_falls_back_to_local() {
        let cfg = parse(r#"{"activeId":"gone"}"#);
        assert_eq!(cfg.active_id, LOCAL_ID);
    }

    #[test]
    fn add_remove_rename() {
        let mut cfg = parse("{}");
        let a = cfg.add_remote("", "https://a.example.com/", None).unwrap();
        assert_eq!(a.name, "a.example.com");
        assert!(cfg.add_remote("dup", "https://a.example.com", None).is_err());
        assert!(cfg.add_remote("bad", "ftp://x", None).is_err());
        let b = cfg.add_remote("Work", "http://10.0.0.2:54321", None).unwrap();
        assert_ne!(a.id, b.id);

        cfg.rename(&b.id, " Office ").unwrap();
        assert_eq!(cfg.find(&b.id).unwrap().name, "Office");
        assert!(cfg.rename(&b.id, "  ").is_err());

        cfg.active_id = b.id.clone();
        cfg.remove(&b.id).unwrap();
        assert_eq!(cfg.active_id, LOCAL_ID);
        assert!(cfg.remove(LOCAL_ID).is_err());
        assert!(cfg.remove("nope").is_err());
    }

    #[test]
    fn keys_roundtrip() {
        let mut cfg = parse("{}");
        let a = cfg.add_remote("A", "https://a.example.com", Some("k1".into())).unwrap();
        assert_eq!(cfg.find(&a.id).unwrap().key.as_deref(), Some("k1"));
        cfg.set_key(&a.id, "k2".into()).unwrap();
        assert_eq!(cfg.find(&a.id).unwrap().key.as_deref(), Some("k2"));
        cfg.set_key(LOCAL_ID, "local-key".into()).unwrap();
        assert!(cfg.set_key("nope", "x".into()).is_err());

        let again = parse(&serde_json::to_string(&cfg).unwrap());
        assert_eq!(again.find(&a.id).unwrap().key.as_deref(), Some("k2"));
        assert_eq!(again.find(LOCAL_ID).unwrap().key.as_deref(), Some("local-key"));
        // Keyless entries do not serialize a "key" field at all.
        assert!(!serde_json::to_string(&parse("{}")).unwrap().contains("\"key\""));
    }

    #[cfg(unix)]
    #[test]
    fn saved_config_is_owner_only() {
        use std::os::unix::fs::PermissionsExt;
        let dir = std::env::temp_dir().join(format!("hylo-cfg-{}", std::process::id()));
        fs::create_dir_all(&dir).unwrap();
        fs::write(config_path(&dir), "{}").unwrap();
        fs::set_permissions(config_path(&dir), fs::Permissions::from_mode(0o644)).unwrap();

        let mut cfg = ConfigFile::default();
        cfg.add_remote("A", "https://a.example.com", Some("secret".into())).unwrap();
        cfg.save(&dir).unwrap();

        let mode = fs::metadata(config_path(&dir)).unwrap().permissions().mode() & 0o777;
        assert_eq!(mode, 0o600);
        assert_eq!(ConfigFile::load(&dir).servers.len(), 2);
        let _ = fs::remove_dir_all(&dir);
    }
}
