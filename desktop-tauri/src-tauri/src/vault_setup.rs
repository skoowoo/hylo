use std::collections::HashMap;
use std::fs;
use std::io::ErrorKind;
use std::path::{Path, PathBuf};

use serde::{Deserialize, Serialize};
use tauri::AppHandle;

use crate::server;

/// Mirrors config.DefaultVaultPath on the Go side.
pub const DEFAULT_VAULT: &str = "~/Documents/Hylo Vault";

#[derive(Debug, Serialize, PartialEq)]
#[serde(rename_all = "camelCase")]
pub struct VaultChoice {
    name: String,
    path: String,
    display: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    last_opened: Option<u64>,
    /// The vault config.toml already points at (a replayed onboarding).
    #[serde(skip_serializing_if = "std::ops::Not::not")]
    current: bool,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct VaultChoices {
    default: VaultChoice,
    obsidian: Vec<VaultChoice>,
    /// The current vault when it is neither the default nor an Obsidian one.
    #[serde(skip_serializing_if = "Option::is_none")]
    current: Option<VaultChoice>,
}

#[derive(Deserialize)]
struct Registry {
    #[serde(default)]
    vaults: HashMap<String, RegistryEntry>,
}

#[derive(Deserialize)]
struct RegistryEntry {
    path: String,
    #[serde(default)]
    ts: Option<u64>,
}

fn obsidian_registry(home: &Path) -> Option<PathBuf> {
    if cfg!(target_os = "macos") {
        return Some(home.join("Library/Application Support/obsidian/obsidian.json"));
    }
    if cfg!(windows) {
        return std::env::var_os("APPDATA")
            .map(|d| PathBuf::from(d).join("obsidian").join("obsidian.json"));
    }
    let config = std::env::var_os("XDG_CONFIG_HOME")
        .map(PathBuf::from)
        .unwrap_or_else(|| home.join(".config"));
    Some(config.join("obsidian").join("obsidian.json"))
}

fn display_path(path: &str, home: &Path) -> String {
    let home = home.to_string_lossy();
    match path.strip_prefix(home.as_ref()) {
        Some(rest) if rest.is_empty() || rest.starts_with('/') => format!("~{rest}"),
        _ => path.to_string(),
    }
}

fn choice(path: &str, last_opened: Option<u64>, home: &Path) -> VaultChoice {
    let name = Path::new(path.trim_end_matches('/'))
        .file_name()
        .map(|n| n.to_string_lossy().into_owned())
        .unwrap_or_else(|| path.to_string());
    VaultChoice {
        name,
        path: path.to_string(),
        display: display_path(path, home),
        last_opened,
        current: false,
    }
}

/// Reads Obsidian's own list of vaults, most recently opened first. The vault
/// folders are never touched: most sit under Documents or iCloud Drive, and
/// reading them would raise macOS's folder-access prompt before the user has
/// picked anything.
fn parse_registry(raw: &str, home: &Path) -> Vec<VaultChoice> {
    let Ok(reg) = serde_json::from_str::<Registry>(raw) else {
        return Vec::new();
    };
    let mut entries: Vec<RegistryEntry> = reg
        .vaults
        .into_values()
        .filter(|e| Path::new(&e.path).is_absolute())
        .collect();
    entries.sort_by(|a, b| b.ts.cmp(&a.ts).then_with(|| a.path.cmp(&b.path)));
    entries.dedup_by(|a, b| a.path == b.path);
    entries
        .iter()
        .map(|e| choice(&e.path, e.ts, home))
        .collect()
}

fn config_file(home: &Path) -> PathBuf {
    home.join(".hylo").join("config.toml")
}

fn read_doc(file: &Path) -> Result<toml_edit::DocumentMut, String> {
    let raw = match fs::read_to_string(file) {
        Ok(raw) => raw,
        Err(e) if e.kind() == ErrorKind::NotFound => String::new(),
        Err(e) => return Err(e.to_string()),
    };
    raw.parse()
        .map_err(|e| format!("{} is not valid TOML: {e}", file.display()))
}

fn vault_path(doc: &toml_edit::DocumentMut) -> Option<String> {
    doc.get("vault")?.get("path")?.as_str().map(str::to_string)
}

#[tauri::command]
pub fn onboarding_vaults() -> VaultChoices {
    let home = server::home_dir();
    let mut obsidian = obsidian_registry(&home)
        .and_then(|p| fs::read_to_string(p).ok())
        .map(|raw| parse_registry(&raw, &home))
        .unwrap_or_default();
    let mut default = choice(DEFAULT_VAULT, None, &home);
    default.display = DEFAULT_VAULT.to_string();

    let doc = read_doc(&config_file(&home)).unwrap_or_default();
    let mut current = None;
    if let Some(path) = vault_path(&doc) {
        let target = expand(&path, &home);
        if target == expand(DEFAULT_VAULT, &home) {
            default.current = true;
        } else if let Some(o) = obsidian.iter_mut().find(|o| Path::new(&o.path) == target) {
            o.current = true;
        } else {
            let mut c = choice(&target.to_string_lossy(), None, &home);
            c.path = path;
            c.current = true;
            current = Some(c);
        }
    }
    VaultChoices {
        default,
        obsidian,
        current,
    }
}

/// The Obsidian vaults Hylo isn't using, most recently opened first, for the
/// hello: someone who kept their vault back is likely still deciding.
pub fn other_obsidian_vaults() -> Vec<serde_json::Value> {
    let home = server::home_dir();
    let current = read_doc(&config_file(&home))
        .ok()
        .and_then(|d| vault_path(&d))
        .map(|p| expand(&p, &home));
    let vaults = obsidian_registry(&home)
        .and_then(|p| fs::read_to_string(p).ok())
        .map(|raw| parse_registry(&raw, &home))
        .unwrap_or_default();
    others(vaults, current.as_deref())
}

fn others(vaults: Vec<VaultChoice>, current: Option<&Path>) -> Vec<serde_json::Value> {
    vaults
        .into_iter()
        .filter(|v| current != Some(Path::new(&v.path)))
        .map(|v| serde_json::json!({ "name": v.name, "lastOpened": v.last_opened.unwrap_or(0) }))
        .collect()
}

/// Note apps by macOS app bundle. Apple Notes ships with every Mac, so its
/// presence says nothing about where someone keeps notes.
const NOTE_APPS: &[(&str, &str)] = &[
    ("Notion.app", "Notion"),
    ("Bear.app", "Bear"),
    ("Logseq.app", "Logseq"),
    ("Evernote.app", "Evernote"),
    ("印象笔记.app", "印象笔记"),
    ("Yinxiang Biji.app", "印象笔记"),
    ("Craft.app", "Craft"),
    ("Joplin.app", "Joplin"),
    ("Microsoft OneNote.app", "OneNote"),
    ("Heptabase.app", "Heptabase"),
    ("Reflect.app", "Reflect"),
    ("Tana.app", "Tana"),
    ("Anytype.app", "Anytype"),
    ("Capacities.app", "Capacities"),
    ("UpNote.app", "UpNote"),
    ("NotePlan 3.app", "NotePlan"),
    ("Drafts.app", "Drafts"),
    ("Agenda.app", "Agenda"),
    ("Typora.app", "Typora"),
    ("语雀.app", "语雀"),
    ("flomo.app", "flomo"),
];

/// Note apps installed on this Mac. Listing the Applications folders needs no
/// permission prompt.
pub fn note_apps() -> Vec<&'static str> {
    if !cfg!(target_os = "macos") {
        return Vec::new();
    }
    let home = server::home_dir();
    installed(&[
        PathBuf::from("/Applications"),
        PathBuf::from("/Applications/Setapp"),
        home.join("Applications"),
    ])
}

fn installed(dirs: &[PathBuf]) -> Vec<&'static str> {
    let mut found = Vec::new();
    for (bundle, name) in NOTE_APPS {
        if !found.contains(name) && dirs.iter().any(|d| d.join(bundle).exists()) {
            found.push(*name);
        }
    }
    found
}

fn expand(path: &str, home: &Path) -> PathBuf {
    match path.strip_prefix("~/") {
        Some(rest) => home.join(rest),
        None => PathBuf::from(path),
    }
}

fn access_error(e: std::io::Error) -> String {
    match e.kind() {
        ErrorKind::PermissionDenied => "Hylo can't open this folder. Allow access in System Settings › Privacy & Security › Files & Folders, or pick another folder.".into(),
        ErrorKind::NotFound => "This folder no longer exists. Pick another one.".into(),
        _ => e.to_string(),
    }
}

/// Points the server at `path`; false when config.toml already did. A
/// different vault already on file is kept in config.toml.bak, so switching
/// never loses where the old notes were.
fn write_vault_path(file: &Path, path: &str, home: &Path) -> Result<bool, String> {
    let mut doc = read_doc(file)?;
    let prev = vault_path(&doc);
    if prev
        .as_deref()
        .is_some_and(|p| expand(p, home) == expand(path, home))
    {
        return Ok(false);
    }
    if prev.is_some() {
        fs::copy(file, file.with_extension("toml.bak")).map_err(|e| e.to_string())?;
    }
    doc["vault"]["path"] = toml_edit::value(path);
    if let Some(dir) = file.parent() {
        fs::create_dir_all(dir).map_err(|e| e.to_string())?;
    }
    fs::write(file, doc.to_string()).map_err(|e| e.to_string())?;
    Ok(true)
}

/// Creating the default folder here, on the user's click, is what raises
/// macOS's Documents prompt at a moment it makes sense. Returns whether the
/// vault changed.
#[tauri::command]
pub async fn onboarding_set_vault(app: AppHandle, path: String) -> Result<bool, String> {
    let home = server::home_dir();
    let target = expand(&path, &home);
    if path == DEFAULT_VAULT {
        fs::create_dir_all(&target).map_err(access_error)?;
    } else {
        fs::read_dir(&target).map_err(access_error)?;
    }
    if !write_vault_path(&config_file(&home), &path, &home)? {
        return Ok(false);
    }

    // A server already up or still indexing (a replayed onboarding, or Back
    // from the AI step) has the old vault open.
    crate::local::kill(&app).await?;
    Ok(true)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn registry_newest_first_with_home_shortened() {
        let home = Path::new("/Users/me");
        let raw = r#"{"vaults":{
            "a":{"path":"/Users/me/Documents/Old","ts":100},
            "b":{"path":"/Users/me/Notes","ts":300,"open":true},
            "c":{"path":"relative/skip","ts":999},
            "d":{"path":"/Volumes/Ext/Work","ts":200}
        }}"#;
        let got = parse_registry(raw, home);
        let names: Vec<_> = got.iter().map(|c| c.name.as_str()).collect();
        assert_eq!(names, ["Notes", "Work", "Old"]);
        assert_eq!(got[0].display, "~/Notes");
        assert_eq!(got[1].display, "/Volumes/Ext/Work");
        assert_eq!(got[0].last_opened, Some(300));
        assert!(parse_registry("not json", home).is_empty());
    }

    #[test]
    fn other_vaults_skip_the_one_in_use() {
        let home = Path::new("/Users/me");
        let raw = r#"{"vaults":{"a":{"path":"/Users/me/Notes","ts":300},"b":{"path":"/Users/me/Work","ts":200}}}"#;
        let got = others(
            parse_registry(raw, home),
            Some(Path::new("/Users/me/Notes")),
        );
        assert_eq!(
            got,
            vec![serde_json::json!({ "name": "Work", "lastOpened": 200 })]
        );
    }

    #[test]
    fn note_apps_found_once_across_folders() {
        let dir = std::env::temp_dir().join(format!("hylo-apps-{}", std::process::id()));
        let _ = fs::remove_dir_all(&dir);
        for p in [
            "a/Bear.app",
            "a/印象笔记.app",
            "b/Yinxiang Biji.app",
            "b/Notes.app",
        ] {
            fs::create_dir_all(dir.join(p)).unwrap();
        }
        let got = installed(&[dir.join("a"), dir.join("b"), dir.join("missing")]);
        assert_eq!(got, ["Bear", "印象笔记"]);
        let _ = fs::remove_dir_all(&dir);
    }

    #[test]
    fn display_only_shortens_whole_home_segment() {
        let home = Path::new("/Users/me");
        assert_eq!(display_path("/Users/me", home), "~");
        assert_eq!(display_path("/Users/meg/x", home), "/Users/meg/x");
    }

    #[test]
    fn vault_path_written_and_old_one_backed_up() {
        let dir = std::env::temp_dir().join(format!("hylo-vault-{}", std::process::id()));
        let _ = fs::remove_dir_all(&dir);
        let file = dir.join("config.toml");

        let home = Path::new("/Users/me");
        assert!(write_vault_path(&file, DEFAULT_VAULT, home).unwrap());
        assert!(fs::read_to_string(&file)
            .unwrap()
            .contains("path = \"~/Documents/Hylo Vault\""));
        assert!(!file.with_extension("toml.bak").exists());
        // The same folder spelled out in full is no switch.
        assert!(!write_vault_path(&file, "/Users/me/Documents/Hylo Vault", home).unwrap());
        assert!(!file.with_extension("toml.bak").exists());

        fs::write(
            &file,
            "# keep me\n[server]\nport = 1\n\n[vault]\npath = \"~/.hylo/root\"\n",
        )
        .unwrap();
        assert!(write_vault_path(&file, "/Users/me/Notes", home).unwrap());
        let out = fs::read_to_string(&file).unwrap();
        assert!(out.contains("# keep me") && out.contains("port = 1"));
        assert!(out.contains("path = \"/Users/me/Notes\""));
        assert!(fs::read_to_string(file.with_extension("toml.bak"))
            .unwrap()
            .contains("~/.hylo/root"));
        let _ = fs::remove_dir_all(&dir);
    }
}
