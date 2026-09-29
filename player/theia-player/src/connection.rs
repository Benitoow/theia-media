//! The viewer's chosen server belongs to the native application, across WebView
//! rebuilds and player restarts. It contains an address, never a credential.

use std::path::{Path, PathBuf};
use tauri::Manager;

/// Keep native proof runs off the viewer's server choice and device history.
pub(super) fn config_dir(app: &tauri::AppHandle) -> Result<PathBuf, String> {
    if let Some(path) = std::env::var_os("THEIA_PLAYER_CONFIG_DIR").filter(|p| !p.is_empty()) {
        return Ok(PathBuf::from(path));
    }
    app.path()
        .app_config_dir()
        .map_err(|error| format!("finding the player configuration: {error}"))
}

fn choice_path(app: &tauri::AppHandle) -> Result<PathBuf, String> {
    config_dir(app).map(|dir| dir.join("server-address.txt"))
}

pub(super) fn remembered(app: &tauri::AppHandle) -> Result<Option<String>, String> {
    read(&choice_path(app)?)
}

pub(super) fn remember(app: &tauri::AppHandle, address: &str) -> Result<(), String> {
    write(&choice_path(app)?, address)
}

pub(super) fn forget(app: &tauri::AppHandle) -> Result<(), String> {
    let path = choice_path(app)?;
    match std::fs::remove_file(&path) {
        Ok(()) => Ok(()),
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => Ok(()),
        Err(error) => Err(format!("removing {}: {error}", path.display())),
    }
}

fn read(path: &Path) -> Result<Option<String>, String> {
    match std::fs::read_to_string(path) {
        Ok(address) => Ok(Some(address.trim().to_string()).filter(|value| !value.is_empty())),
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => Ok(None),
        Err(error) => Err(format!("reading {}: {error}", path.display())),
    }
}

fn write(path: &Path, address: &str) -> Result<(), String> {
    let address = address.trim().trim_end_matches('/');
    if address.is_empty() {
        return Err("the server address is empty".into());
    }
    // Server addresses never need URL user-info. Keep accidental credentials
    // out of the native configuration, even if a caller bypasses the form.
    if address.contains('@') {
        return Err("server addresses cannot contain credentials".into());
    }
    let parent = path
        .parent()
        .ok_or("the player configuration has no directory")?;
    std::fs::create_dir_all(parent)
        .map_err(|error| format!("creating {}: {error}", parent.display()))?;
    std::fs::write(path, address).map_err(|error| format!("saving {}: {error}", path.display()))
}

#[cfg(test)]
mod tests {
    use super::{read, write};

    #[test]
    fn a_chosen_address_survives_a_new_reader() {
        let path =
            std::env::temp_dir().join(format!("theia-server-choice-{}.txt", std::process::id()));
        write(&path, "  http://192.0.2.7:8395/  ").unwrap();
        assert_eq!(
            read(&path).unwrap().as_deref(),
            Some("http://192.0.2.7:8395")
        );
        std::fs::remove_file(&path).unwrap();
        assert_eq!(read(&path).unwrap(), None);
    }

    #[test]
    fn credentials_are_not_written() {
        let path =
            std::env::temp_dir().join(format!("theia-server-secret-{}.txt", std::process::id()));
        assert!(write(&path, "http://user:password@example.test:8395").is_err());
        assert_eq!(read(&path).unwrap(), None);
    }
}
