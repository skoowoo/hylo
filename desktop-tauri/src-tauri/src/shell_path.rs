// The CLI installs to ~/.local/bin (cli.rs), which a terminal only finds if the
// user's shell puts it on PATH. With consent, onboarding adds that one line.
use std::fs;
use std::io::Write;
use std::path::{Path, PathBuf};

use crate::server;

const MARKER: &str = "# Added by Hylo: the hylo CLI";

fn rc_files(home: &Path) -> Vec<PathBuf> {
    [
        ".zshrc",
        ".zprofile",
        ".bash_profile",
        ".bashrc",
        ".profile",
        ".config/fish/config.fish",
        ".config/fish/conf.d/hylo.fish",
    ]
    .iter()
    .map(|f| home.join(f))
    .collect()
}

/// The file the user's login shell reads, and the line that adds the CLI.
fn target(home: &Path, shell: &str) -> (PathBuf, &'static str) {
    const POSIX: &str = "export PATH=\"$HOME/.local/bin:$PATH\"";
    match shell.rsplit('/').next().unwrap_or_default() {
        "zsh" => (home.join(".zshrc"), POSIX),
        "bash" if cfg!(target_os = "macos") => (home.join(".bash_profile"), POSIX),
        "bash" => (home.join(".bashrc"), POSIX),
        "fish" => (
            home.join(".config/fish/conf.d/hylo.fish"),
            "fish_add_path -g $HOME/.local/bin",
        ),
        "" if cfg!(target_os = "macos") => (home.join(".zshrc"), POSIX),
        _ => (home.join(".profile"), POSIX),
    }
}

fn needed_in(home: &Path) -> bool {
    !rc_files(home)
        .iter()
        .filter_map(|f| fs::read_to_string(f).ok())
        .any(|s| s.contains(".local/bin"))
}

fn add_in(home: &Path, shell: &str) -> Result<(), String> {
    if !needed_in(home) {
        return Ok(());
    }
    let (file, line) = target(home, shell);
    if let Some(dir) = file.parent() {
        fs::create_dir_all(dir).map_err(|e| e.to_string())?;
    }
    let mut f = fs::OpenOptions::new()
        .create(true)
        .append(true)
        .open(&file)
        .map_err(|e| e.to_string())?;
    write!(f, "\n{MARKER}\n{line}\n").map_err(|e| e.to_string())
}

/// Whether the terminal still can't find `hylo`. Windows installs put it on PATH themselves.
#[tauri::command]
pub fn onboarding_path_needed() -> bool {
    cfg!(unix) && needed_in(&server::home_dir())
}

#[tauri::command]
pub fn onboarding_add_to_path() -> Result<(), String> {
    if !cfg!(unix) {
        return Ok(());
    }
    add_in(
        &server::home_dir(),
        &std::env::var("SHELL").unwrap_or_default(),
    )
}

#[cfg(test)]
mod tests {
    use super::*;

    fn home(tag: &str) -> PathBuf {
        let dir = std::env::temp_dir().join(format!("hylo-path-{tag}-{}", std::process::id()));
        let _ = fs::remove_dir_all(&dir);
        fs::create_dir_all(&dir).unwrap();
        dir
    }

    #[test]
    fn adds_once_to_the_login_shell_rc() {
        let h = home("zsh");
        assert!(needed_in(&h));
        add_in(&h, "/bin/zsh").unwrap();
        add_in(&h, "/bin/zsh").unwrap();
        let rc = fs::read_to_string(h.join(".zshrc")).unwrap();
        assert_eq!(rc.matches(MARKER).count(), 1);
        assert!(!needed_in(&h));
        let _ = fs::remove_dir_all(&h);
    }

    #[test]
    fn existing_local_bin_anywhere_means_done() {
        let h = home("bash");
        fs::write(h.join(".profile"), "PATH=$HOME/.local/bin:$PATH\n").unwrap();
        add_in(&h, "/bin/bash").unwrap();
        assert!(!h.join(".bashrc").exists() && !h.join(".bash_profile").exists());
        let _ = fs::remove_dir_all(&h);
    }

    #[test]
    fn fish_gets_its_own_conf_file() {
        let h = home("fish");
        add_in(&h, "/opt/homebrew/bin/fish").unwrap();
        let conf = fs::read_to_string(h.join(".config/fish/conf.d/hylo.fish")).unwrap();
        assert!(conf.contains("fish_add_path"));
        let _ = fs::remove_dir_all(&h);
    }
}
