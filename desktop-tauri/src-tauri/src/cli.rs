use std::fs;
use std::io;
use std::path::{Path, PathBuf};
use std::process::Command;
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::Mutex;

use crate::server;

pub const APP_VERSION: &str = "1.1.0";
pub const BUILD_ID: &str = "0";

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum InstallOutcome {
    Skipped,
    Installed,
}

pub fn sentinel_value() -> String {
    format!("{APP_VERSION}-{BUILD_ID}")
}

fn sentinel_path() -> PathBuf {
    home().join(".hylo").join(".cli_app_version")
}

fn home() -> PathBuf {
    std::env::var_os("HOME")
        .or_else(|| std::env::var_os("USERPROFILE"))
        .map(PathBuf::from)
        .unwrap_or_else(|| PathBuf::from("."))
}

fn binary_name() -> &'static str {
    if cfg!(windows) {
        "hylo.exe"
    } else {
        "hylo"
    }
}

pub fn install(
    data_dir: &Path,
    archive: Option<PathBuf>,
    force: bool,
) -> Result<InstallOutcome, String> {
    // Launch install and the "binary missing" fallback both write the same
    // dest, so serialize them.
    static LOCK: Mutex<()> = Mutex::new(());
    let _guard = LOCK.lock().unwrap_or_else(|err| err.into_inner());
    let sentinel = sentinel_value();
    if !force {
        if let Ok(installed) = fs::read_to_string(sentinel_path()) {
            if installed.trim() == sentinel {
                server::diag_log(
                    data_dir,
                    &format!("cli-installer: already up to date (v{sentinel}), skipping"),
                );
                return Ok(InstallOutcome::Skipped);
            }
        }
    }

    let Some(archive) = archive.filter(|p| p.exists()) else {
        server::diag_log(
            data_dir,
            "cli-installer: no bundled archive (dev mode), skipping",
        );
        return Ok(InstallOutcome::Skipped);
    };
    if !archive.exists() {
        return Err("Bundled CLI archive not found in app resources".into());
    }

    static NEXT_TMP: AtomicU64 = AtomicU64::new(0);
    let stamp = NEXT_TMP.fetch_add(1, Ordering::Relaxed);
    let tmp = std::env::temp_dir().join(format!("hylo-install-{}-{stamp}", std::process::id()));
    let _ = fs::remove_dir_all(&tmp);
    fs::create_dir_all(&tmp).map_err(|e| e.to_string())?;

    let extracted = (|| {
        extract(&archive, &tmp)?;
        let install_dir = home().join(".local").join("bin");
        fs::create_dir_all(&install_dir).map_err(|e| format!("Cannot create install dir: {e}"))?;
        let dest = install_dir.join(binary_name());
        let tmp_bin = install_dir.join(format!(
            "{}.tmp.{}-{stamp}",
            binary_name(),
            std::process::id()
        ));
        fs::copy(tmp.join(binary_name()), &tmp_bin).map_err(|e| e.to_string())?;
        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;
            let mut perms = fs::metadata(&tmp_bin)
                .map_err(|e| e.to_string())?
                .permissions();
            perms.set_mode(0o755);
            fs::set_permissions(&tmp_bin, perms).map_err(|e| e.to_string())?;
        }
        fs::rename(&tmp_bin, &dest).map_err(|e| e.to_string())?;
        server::diag_log(
            data_dir,
            &format!("cli-installer: installed to {}", dest.display()),
        );
        copy_defaults(&tmp, data_dir);
        Ok::<(), String>(())
    })();

    let _ = fs::remove_dir_all(&tmp);
    extracted?;

    if let Err(e) = fs::create_dir_all(sentinel_path().parent().unwrap_or(Path::new(".")))
        .and_then(|_| fs::write(sentinel_path(), &sentinel))
    {
        server::diag_log(
            data_dir,
            &format!("cli-installer: sentinel write failed (non-fatal): {e}"),
        );
    }
    Ok(InstallOutcome::Installed)
}

fn extract(archive: &Path, dest: &Path) -> Result<(), String> {
    let status = if cfg!(windows) {
        Command::new("powershell")
            .args([
                "-NoProfile",
                "-Command",
                &format!(
                    "Expand-Archive -Path \"{}\" -DestinationPath \"{}\" -Force",
                    archive.display(),
                    dest.display()
                ),
            ])
            .status()
    } else {
        Command::new("tar")
            .args([
                "xzf",
                &archive.display().to_string(),
                "-C",
                &dest.display().to_string(),
            ])
            .status()
    };
    match status {
        Ok(s) if s.success() => Ok(()),
        Ok(s) => Err(format!(
            "archive extraction failed: exit code {}",
            s.code().unwrap_or(-1)
        )),
        Err(e) => Err(format!("archive extraction failed: {e}")),
    }
}

fn copy_defaults(tmp: &Path, data_dir: &Path) {
    let hylo_dir = home().join(".hylo");
    if let Err(e) = fs::create_dir_all(&hylo_dir) {
        server::diag_log(
            data_dir,
            &format!("cli-installer: config/skills copy failed (non-fatal): {e}"),
        );
        return;
    }
    let config_dst = hylo_dir.join("config.toml");
    if !config_dst.exists() {
        if fs::copy(tmp.join("config.example.toml"), &config_dst).is_ok() {
            server::diag_log(
                data_dir,
                &format!(
                    "cli-installer: installed default config to {}",
                    config_dst.display()
                ),
            );
        }
    }
    let skills_src = tmp.join("skills");
    if skills_src.is_dir() {
        let skills_dst = hylo_dir.join("skills");
        if let Err(e) = copy_missing(&skills_src, &skills_dst) {
            server::diag_log(
                data_dir,
                &format!("cli-installer: config/skills copy failed (non-fatal): {e}"),
            );
        } else {
            server::diag_log(
                data_dir,
                &format!(
                    "cli-installer: installed built-in skills to {}",
                    skills_dst.display()
                ),
            );
        }
    }
}

/// Copy files that are not already present. Never overwrites user edits.
fn copy_missing(src: &Path, dst: &Path) -> io::Result<()> {
    fs::create_dir_all(dst)?;
    for entry in fs::read_dir(src)? {
        let entry = entry?;
        let from = entry.path();
        let to = dst.join(entry.file_name());
        if from.is_dir() {
            copy_missing(&from, &to)?;
        } else if !to.exists() {
            fs::copy(&from, &to)?;
        }
    }
    Ok(())
}
