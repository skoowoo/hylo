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

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct ConfigFile {
    #[serde(default = "default_server_url")]
    pub server_url: String,
    /// Missing means on, matching the Electron shell (`!== false`).
    #[serde(default = "default_true")]
    pub auto_start_server: bool,
    #[serde(default)]
    pub inbox_notify: InboxNotify,
}

fn default_server_url() -> String {
    DEFAULT_SERVER_URL.to_string()
}

impl Default for ConfigFile {
    fn default() -> Self {
        Self {
            server_url: default_server_url(),
            auto_start_server: true,
            inbox_notify: InboxNotify::default(),
        }
    }
}

impl ConfigFile {
    pub fn load(data_dir: &Path) -> Self {
        let path = config_path(data_dir);
        let Ok(raw) = fs::read_to_string(path) else {
            return Self::default();
        };
        serde_json::from_str(&raw).unwrap_or_default()
    }

    pub fn save(&self, data_dir: &Path) -> Result<(), String> {
        fs::create_dir_all(data_dir).map_err(|e| e.to_string())?;
        let raw = serde_json::to_vec_pretty(self).map_err(|e| e.to_string())?;
        fs::write(config_path(data_dir), raw).map_err(|e| e.to_string())
    }
}

pub fn config_path(data_dir: &Path) -> PathBuf {
    data_dir.join("config.json")
}

/// Same folder Electron uses after `app.setName("Hylo")`.
pub fn shell_data_dir() -> PathBuf {
    #[cfg(target_os = "macos")]
    {
        home_dir()
            .join("Library")
            .join("Application Support")
            .join("Hylo")
    }
    #[cfg(target_os = "windows")]
    {
        std::env::var_os("APPDATA")
            .map(PathBuf::from)
            .unwrap_or_else(home_dir)
            .join("Hylo")
    }
    #[cfg(all(unix, not(target_os = "macos")))]
    {
        std::env::var_os("XDG_CONFIG_HOME")
            .map(PathBuf::from)
            .unwrap_or_else(|| home_dir().join(".config"))
            .join("Hylo")
    }
}

fn home_dir() -> PathBuf {
    std::env::var_os("HOME")
        .or_else(|| std::env::var_os("USERPROFILE"))
        .map(PathBuf::from)
        .unwrap_or_else(std::env::temp_dir)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn missing_autostart_stays_on() {
        let cfg: ConfigFile =
            serde_json::from_str(r#"{"serverUrl":"http://127.0.0.1:9"}"#).unwrap();
        assert!(cfg.auto_start_server);
        assert_eq!(cfg.server_url, "http://127.0.0.1:9");
        assert!(cfg.inbox_notify.text_enabled);
        assert_eq!(cfg.inbox_notify.sound, "beep");
    }

    #[test]
    fn data_dir_matches_electron_user_data() {
        let dir = shell_data_dir();
        #[cfg(target_os = "macos")]
        assert!(dir.ends_with("Library/Application Support/Hylo"));
        #[cfg(not(target_os = "macos"))]
        assert!(dir.ends_with("Hylo"));
    }
}
