//! Durable latest-position outbox. Network work never runs on the window thread.
use crate::server::Client;
use std::path::PathBuf;
use std::sync::{
    atomic::{AtomicBool, Ordering},
    Mutex, OnceLock,
};

#[derive(Clone, serde::Serialize, serde::Deserialize, PartialEq, Debug)]
struct Position {
    server: String,
    profile: Option<i64>,
    episode: bool,
    id: i64,
    position: f64,
    duration: f64,
    revision: u64,
}
#[derive(Default)]
struct Outbox {
    path: Option<PathBuf>,
    entries: Vec<Position>,
    revision: u64,
}
static OUTBOX: OnceLock<Mutex<Outbox>> = OnceLock::new();
static FLUSHING: AtomicBool = AtomicBool::new(false);
static STORAGE_FAILED: AtomicBool = AtomicBool::new(false);
static NETWORK: Mutex<()> = Mutex::new(());
fn outbox() -> &'static Mutex<Outbox> {
    OUTBOX.get_or_init(|| Mutex::new(Outbox::default()))
}

impl Outbox {
    fn persist(&self) -> Result<(), String> {
        let Some(path) = &self.path else {
            return Ok(());
        };
        if let Some(parent) = path.parent() {
            std::fs::create_dir_all(parent).map_err(|e| e.to_string())?;
        }
        let temporary = path.with_extension("json.tmp");
        std::fs::write(
            &temporary,
            serde_json::to_vec(&self.entries).map_err(|e| e.to_string())?,
        )
        .map_err(|e| e.to_string())?;
        std::fs::rename(temporary, path).map_err(|e| e.to_string())
    }
    fn put(&mut self, mut entry: Position) {
        self.revision += 1;
        entry.revision = self.revision;
        self.entries.retain(|old| {
            !(old.server == entry.server
                && old.profile == entry.profile
                && old.episode == entry.episode
                && old.id == entry.id)
        });
        self.entries.push(entry);
    }
    fn acknowledge(&mut self, entry: &Position) {
        self.entries.retain(|old| old != entry);
    }
}
fn persist(boxed: &Outbox) {
    STORAGE_FAILED.store(boxed.persist().is_err(), Ordering::Relaxed);
}
pub(super) fn init(path: PathBuf) {
    let mut boxed = outbox().lock().unwrap();
    boxed.path = Some(path.clone());
    match std::fs::read(&path) {
        Ok(bytes) => match serde_json::from_slice::<Vec<Position>>(&bytes) {
            Ok(entries) => {
                boxed.entries = entries
                    .into_iter()
                    .filter(|e| {
                        !e.server.contains('@')
                            && e.id > 0
                            && e.position.is_finite()
                            && e.duration.is_finite()
                            && e.position >= 0.0
                            && e.duration >= 0.0
                    })
                    .collect();
                boxed.revision = boxed.entries.iter().map(|e| e.revision).max().unwrap_or(0);
            }
            Err(_) => STORAGE_FAILED.store(true, Ordering::Relaxed),
        },
        Err(e) if e.kind() == std::io::ErrorKind::NotFound => {}
        Err(_) => STORAGE_FAILED.store(true, Ordering::Relaxed),
    }
}
pub(super) fn enqueue(client: &Client, episode: bool, id: i64, position: f64, duration: f64) {
    if !position.is_finite()
        || !duration.is_finite()
        || position < 0.0
        || duration < 0.0
        || client.base().contains('@')
    {
        return;
    }
    {
        let mut boxed = outbox().lock().unwrap();
        boxed.put(Position {
            server: client.base().into(),
            profile: client.profile(),
            episode,
            id,
            position,
            duration,
            revision: 0,
        });
        persist(&boxed);
    }
    flush();
}
/// Pending device history wins over an older server answer after a failed write.
pub(super) fn overlay(
    client: &Client,
    episode: bool,
    id: i64,
    progress: &mut crate::server::Progress,
) {
    if let Ok(boxed) = outbox().lock() {
        boxed.overlay(client, episode, id, progress);
    }
}
impl Outbox {
    fn overlay(
        &self,
        client: &Client,
        episode: bool,
        id: i64,
        progress: &mut crate::server::Progress,
    ) {
        if let Some(entry) = self.entries.iter().find(|e| {
            e.server == client.base()
                && e.profile == client.profile()
                && e.episode == episode
                && e.id == id
        }) {
            progress.position_seconds = entry.position;
            progress.duration_seconds = entry.duration;
            progress.finished = entry.duration > 0.0
                && entry.duration - entry.position <= 120.0_f64.min(entry.duration * 0.05);
        }
    }
}
pub(super) fn pending() -> usize {
    outbox()
        .lock()
        .map(|boxed| boxed.entries.len())
        .unwrap_or(0)
}
pub(super) fn storage_failed() -> bool {
    STORAGE_FAILED.load(Ordering::Relaxed)
}
pub(super) fn flush() {
    if FLUSHING.swap(true, Ordering::SeqCst) {
        return;
    }
    std::thread::spawn(|| {
        flush_blocking();
        FLUSHING.store(false, Ordering::SeqCst);
    });
}
pub(super) fn flush_blocking() {
    let _network = NETWORK.lock().unwrap();
    let entries = outbox()
        .lock()
        .map(|boxed| boxed.entries.clone())
        .unwrap_or_default();
    // Retry only this server/profile. An old server must never hold up the
    // current one or receive history because another device reused an id.
    if let Ok(current) = crate::client_snapshot() {
        for entry in entries
            .into_iter()
            .filter(|e| e.server == current.base() && e.profile == current.profile())
        {
            if !outbox().lock().unwrap().entries.contains(&entry) {
                continue;
            }
            let result = if entry.episode {
                current.save_episode_progress(entry.id, entry.position, entry.duration)
            } else {
                current.save_progress(entry.id, entry.position, entry.duration)
            };
            if result.is_err() {
                break;
            }
            if let Ok(mut boxed) = outbox().lock() {
                boxed.acknowledge(&entry);
                persist(&boxed);
            }
        }
    }
}
pub(super) fn history(client: &Client, id: i64, watched: bool) -> Result<(), String> {
    let _network = NETWORK.lock().map_err(|e| e.to_string())?;
    client.episode_history(id, watched)?;
    discard(client, true, id);
    Ok(())
}
pub(super) fn discard(client: &Client, episode: bool, id: i64) {
    let mut boxed = outbox().lock().unwrap();
    boxed.entries.retain(|e| {
        !(e.server == client.base()
            && e.profile == client.profile()
            && e.episode == episode
            && e.id == id)
    });
    persist(&boxed);
}

#[cfg(test)]
mod tests {
    use super::*;
    fn entry(profile: i64, position: f64) -> Position {
        Position {
            server: "http://example.test".into(),
            profile: Some(profile),
            episode: true,
            id: 7,
            position,
            duration: 1200.0,
            revision: 0,
        }
    }
    #[test]
    fn newer_position_survives_an_older_ack_and_profiles_stay_separate() {
        let mut boxed = Outbox::default();
        boxed.put(entry(1, 100.0));
        let sent = boxed.entries[0].clone();
        boxed.put(entry(1, 200.0));
        boxed.put(entry(2, 50.0));
        boxed.acknowledge(&sent);
        assert_eq!(boxed.entries.len(), 2);
        assert_eq!(boxed.entries[0].position, 200.0);
    }
    #[test]
    fn pending_resume_overrides_stale_server_only_for_its_owner() {
        let mut boxed = Outbox::default();
        boxed.put(entry(1, 420.0));
        let mut client = Client::new("http://example.test");
        client.set_profile(Some(1));
        let mut progress = crate::server::Progress::default();
        boxed.overlay(&client, true, 7, &mut progress);
        assert_eq!(progress.position_seconds, 420.0);
        client.set_profile(Some(2));
        progress.position_seconds = 0.0;
        boxed.overlay(&client, true, 7, &mut progress);
        assert_eq!(progress.position_seconds, 0.0);
        client.set_profile(Some(1));
        boxed.overlay(&client, false, 7, &mut progress);
        assert_eq!(progress.position_seconds, 0.0);
    }
    #[test]
    fn outbox_survives_a_new_reader() {
        let path = std::env::temp_dir().join(format!("theia-outbox-{}.json", std::process::id()));
        let mut boxed = Outbox {
            path: Some(path.clone()),
            ..Default::default()
        };
        boxed.put(entry(1, 420.0));
        boxed.persist().unwrap();
        let entries: Vec<Position> =
            serde_json::from_slice(&std::fs::read(&path).unwrap()).unwrap();
        assert_eq!(entries, boxed.entries);
        std::fs::remove_file(path).unwrap();
    }
}
