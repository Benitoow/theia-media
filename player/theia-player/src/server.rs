//! Finding a `theia-server` on the network, and talking to it.
//!
//! Two mechanisms, deliberately unequal in status - the same asymmetry the
//! server itself documents for mDNS versus the QR code:
//!
//! * browsing for [`SERVICE_TYPE`] is the pleasant path. When exactly one
//!   server answers, the player connects without asking anybody anything.
//! * an address typed by hand is the contract. Multicast is blocked on plenty
//!   of networks - and on at least one development machine, where a query for
//!   any service at all returns nothing - so a player that could only be found
//!   that way would be a player nobody could use.
//!
//! The HTTP side is deliberately plain: the API is JSON over HTTP on the local
//! network, the film itself is fetched by mpv rather than by this process, and
//! no TLS stack is linked in because there is no TLS to speak.

use std::time::{Duration, Instant};

/// The service the server announces. It is the product's own name rather than
/// `_http._tcp`, so a browser finds Theia instead of finding "a web server".
pub const SERVICE_TYPE: &str = "_theia._tcp.local.";

/// A server seen on the local network.
#[derive(Clone, serde::Serialize)]
pub struct Discovered {
    /// The instance name, which is the server's hostname.
    pub name: String,
    /// The first usable address, already turned into a base URL.
    pub url: String,
    /// The TXT `version` record, when the announcement carried one.
    pub version: Option<String>,
}

/// Browses for servers. Best-effort by construction: an error here means "no
/// server was found this way", never "the player cannot work".
pub fn discover(timeout: Duration) -> Result<Vec<Discovered>, String> {
    let daemon = mdns_sd::ServiceDaemon::new()
        .map_err(|e| format!("starting the mDNS browser: {e}"))?;
    let receiver = daemon
        .browse(SERVICE_TYPE)
        .map_err(|e| format!("browsing for {SERVICE_TYPE}: {e}"))?;

    let mut found: Vec<Discovered> = Vec::new();
    let deadline = Instant::now() + timeout;
    while let Some(remaining) = deadline.checked_duration_since(Instant::now()) {
        match receiver.recv_timeout(remaining) {
            Ok(mdns_sd::ServiceEvent::ServiceResolved(info)) => {
                // IPv4 first: it is what every home network still carries, and a
                // link-local IPv6 address typed into a URL needs brackets.
                let address = info
                    .get_addresses()
                    .iter()
                    .find(|a| a.is_ipv4())
                    .or_else(|| info.get_addresses().iter().next());
                let Some(address) = address else { continue };
                let version = info
                    .get_property_val_str("version")
                    .map(|v| v.to_string());
                let url = format!("http://{}:{}", address, info.get_port());
                if found.iter().any(|s| s.url == url) {
                    continue;
                }
                found.push(Discovered {
                    name: info.get_fullname().to_string(),
                    url,
                    version,
                });
            }
            Ok(_) => {}
            // A timeout or a closed channel both mean "nothing more is coming".
            Err(_) => break,
        }
    }
    let _ = daemon.shutdown();
    Ok(found)
}

/// What `/api/health` answers.
#[derive(Clone, serde::Deserialize, serde::Serialize)]
pub struct Health {
    pub status: String,
    pub version: String,
    #[serde(default)]
    pub uptime_seconds: u64,
}

/// A household profile, the identity viewing history belongs to.
#[derive(Clone, serde::Deserialize, serde::Serialize)]
pub struct Profile {
    pub id: i64,
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub is_default: bool,
    #[serde(default)]
    pub has_avatar: bool,
    #[serde(default)]
    pub avatar_version: i64,
}

/// The updater state already exposed by the server's settings page. The native
/// shell mirrors that contract instead of inventing a second release checker.
#[derive(Clone, Default, serde::Deserialize, serde::Serialize)]
pub struct UpdateStatus {
    #[serde(default)]
    pub state: String,
    #[serde(default)]
    pub reason: String,
    #[serde(default)]
    pub current_version: String,
    #[serde(default)]
    pub latest_version: String,
    #[serde(default)]
    pub available: bool,
    #[serde(default)]
    pub message: String,
    #[serde(default)]
    pub release_url: String,
    #[serde(default)]
    pub checked_at: String,
}

/// The parts of a film the player needs to offer it and to play it. The server
/// sends more; a struct that mirrors the whole record would be a second copy of
/// the API to keep in step.
#[derive(Clone, serde::Deserialize, serde::Serialize)]
pub struct Movie {
    pub id: i64,
    #[serde(default)]
    pub title: String,
    #[serde(default)]
    pub year: i32,
    #[serde(default)]
    pub metadata: Metadata,
    #[serde(default)]
    pub files: Vec<MovieFile>,
    #[serde(default)]
    pub progress: Progress,
    /// The artwork, already resolved against the server this client is talking
    /// to. The OSD draws them; it does not build URLs, and it cannot know which
    /// server answered.
    #[serde(default)]
    pub backdrop_url: String,
    #[serde(default)]
    pub poster_url: String,
}

/// A show in the library. The list deliberately stays light; seasons arrive
/// only after somebody opens the show.
#[derive(Clone, Default, serde::Deserialize, serde::Serialize)]
pub struct Series {
    pub id: i64,
    #[serde(default)]
    pub title: String,
    #[serde(default)]
    pub year: i32,
    #[serde(default)]
    pub metadata: SeriesMetadata,
    #[serde(default)]
    pub seasons: Vec<Season>,
    #[serde(default)]
    pub backdrop_url: String,
    #[serde(default)]
    pub poster_url: String,
}

#[derive(Clone, Default, serde::Deserialize, serde::Serialize)]
pub struct SeriesMetadata {
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub poster_path: String,
    #[serde(default)]
    pub backdrop_path: String,
    // Same door, same rule: the series preview names its synopsis and its year
    // from these, and the year comes from the air date rather than the name.
    #[serde(default)]
    pub overview: String,
    #[serde(default)]
    pub tagline: String,
    #[serde(default)]
    pub first_air_date: String,
    #[serde(default)]
    pub release_date: String,
}

/// A row of the home screen, exactly as the server names it. The kind travels
/// as a code - `continue`, `recent`, `top_rated`, `tonight` - and the OSD owns
/// the sentence that names it (decision 25).
#[derive(serde::Deserialize, serde::Serialize)]
pub struct HomeRow {
    #[serde(default)]
    pub kind: String,
    #[serde(default)]
    pub movies: Vec<Movie>,
}

/// The home screen: one hero and short rows, the shape the server builds.
///
/// The hero is a film even when the series rows below it are the thing being
/// continued; that is the server's choice and this side does not second-guess
/// it. `hero_kind` says whether the hero is being resumed or is simply on,
/// which changes what the screen offers - see the design system.
#[derive(serde::Deserialize, serde::Serialize)]
pub struct HomeScreen {
    #[serde(default)]
    pub hero: Option<Movie>,
    #[serde(default)]
    pub hero_kind: String,
    #[serde(default)]
    pub rows: Vec<HomeRow>,
    #[serde(default)]
    pub total: i64,
}

/// The series half of the home screen: episodes to continue, shows that are
/// new. A separate request because the server keeps the two answers in
/// separate tables, and a server without series simply answers empty.
#[derive(serde::Deserialize, serde::Serialize)]
pub struct SeriesHome {
    #[serde(default)]
    pub continue_watching: Vec<EpisodeItem>,
    #[serde(default)]
    pub recent_series: Vec<Series>,
}

#[derive(Clone, Default, serde::Deserialize, serde::Serialize)]
pub struct Season {
    pub id: i64,
    pub series_id: i64,
    #[serde(default)]
    pub season_number: i32,
    #[serde(default)]
    pub metadata: SeasonMetadata,
    #[serde(default, rename = "episodes")]
    pub items: Vec<EpisodeItem>,
}

#[derive(Clone, Default, serde::Deserialize, serde::Serialize)]
pub struct SeasonMetadata {
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub episode_count: i32,
}

#[derive(Clone, Default, serde::Deserialize, serde::Serialize)]
pub struct EpisodeItem {
    pub id: i64,
    pub series_id: i64,
    #[serde(default)]
    pub series_title: String,
    #[serde(default)]
    pub season_number: i32,
    #[serde(default)]
    pub episode_numbers: Vec<i32>,
    #[serde(default, rename = "episode_metadata")]
    pub episodes: Vec<Episode>,
    #[serde(default)]
    pub files: Vec<EpisodeFile>,
    #[serde(default)]
    pub progress: Progress,
    #[serde(default)]
    pub still_url: String,
}

#[derive(Clone, Default, serde::Deserialize, serde::Serialize)]
pub struct Episode {
    pub id: i64,
    #[serde(default)]
    pub episode_number: i32,
    #[serde(default)]
    pub local_title: String,
    #[serde(default)]
    pub metadata: EpisodeMetadata,
}

#[derive(Clone, Default, serde::Deserialize, serde::Serialize)]
pub struct EpisodeMetadata {
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub still_path: String,
    #[serde(default)]
    pub runtime_minutes: i32,
    #[serde(default)]
    pub overview: String,
}

#[derive(Clone, Default, serde::Deserialize, serde::Serialize)]
pub struct EpisodeFile {
    pub id: i64,
    #[serde(default)]
    pub file_name: String,
    #[serde(default)]
    pub is_primary: bool,
}

/// The two TMDB paths a card can be drawn from, exactly as the server stores
/// them - a leading slash and no host.
#[derive(Clone, Default, serde::Deserialize, serde::Serialize)]
pub struct Metadata {
    #[serde(
        default,
        rename(deserialize = "tmdb_title", serialize = "title"),
        alias = "title"
    )]
    pub title: String,
    #[serde(default)]
    pub poster_path: String,
    #[serde(default)]
    pub backdrop_path: String,
    // The rest of what the interface draws. This struct is the only door the
    // server's metadata comes through - anything missing here is dropped before
    // the OSD ever sees it, whatever the API sent. Measured on 20 September
    // 2026: the hero and the card preview could not show a synopsis because
    // `overview` was not in this list, and the harness passed anyway because it
    // mocks the invoke layer and answers with its own fixtures.
    #[serde(default)]
    pub overview: String,
    /// TMDB's release date, `YYYY-MM-DD`.
    #[serde(default)]
    pub release_date: String,
    #[serde(default)]
    pub runtime_minutes: i32,
    #[serde(default)]
    pub vote_average: f64,
    #[serde(default)]
    pub director: String,
}

/// What the server says about a card preview.
///
/// Three states and no fourth: `ready` with a URL, `building` while it makes
/// one, and an absence the interface answers by showing the still it already
/// has.
#[derive(Clone, Default, serde::Deserialize, serde::Serialize)]
pub struct PreviewState {
    #[serde(default)]
    pub state: String,
    #[serde(default)]
    pub clip_url: String,
}

#[derive(Clone, serde::Deserialize, serde::Serialize)]
pub struct MovieFile {
    pub id: i64,
    #[serde(default)]
    pub file_name: String,
    #[serde(default)]
    pub is_primary: bool,
}

#[derive(Clone, Default, serde::Deserialize, serde::Serialize)]
pub struct Progress {
    #[serde(default)]
    pub position_seconds: f64,
    /// Absent for a film nobody has opened, and taken from the film itself
    /// rather than from the viewing history - which is why a card can draw its
    /// progress rule before the film has ever been played.
    #[serde(default)]
    pub duration_seconds: f64,
    #[serde(default)]
    pub finished: bool,
}

impl Movie {
    /// Fills in the two artwork URLs a card needs, once, as the film arrives.
    /// The alternative is the OSD knowing the server's address, which is the
    /// connection's business and not the interface's.
    fn resolve_artwork(&mut self, base: &str) {
        self.backdrop_url = image_url(base, &self.metadata.backdrop_path, "w780");
        self.poster_url = image_url(base, &self.metadata.poster_path, "w500");
    }
}

impl Series {
    fn resolve_artwork(&mut self, base: &str) {
        self.backdrop_url = image_url(base, &self.metadata.backdrop_path, "w780");
        self.poster_url = image_url(base, &self.metadata.poster_path, "w500");
    }
}

impl EpisodeItem {
    fn resolve_artwork(&mut self, base: &str) {
        let path = self
            .episodes
            .first()
            .map(|episode| episode.metadata.still_path.as_str())
            .unwrap_or_default();
        self.still_url = image_url(base, path, "w780");
    }

    pub fn title(&self) -> String {
        self.episodes
            .iter()
            .map(|episode| {
                if episode.metadata.name.is_empty() {
                    episode.local_title.clone()
                } else {
                    episode.metadata.name.clone()
                }
            })
            .filter(|title| !title.is_empty())
            .collect::<Vec<_>>()
            .join(" / ")
    }
}

/// What `/api/stream/{id}/files/{file_id}/info` answers, as far as this player
/// needs it.
///
/// The request is not only for the answer: the server refreshes the subtitle
/// files sitting beside a media file when a player asks how it will be
/// delivered, which is how a `.srt` dropped into the folder this afternoon is
/// offered this evening without a rescan. Asking is what makes the sidecar
/// visible at all, so the player asks on every load.
#[derive(Clone, Default, serde::Deserialize, serde::Serialize)]
pub struct StreamInfo {
    #[serde(default)]
    pub subtitle_tracks: Vec<SubtitleTrack>,
}

#[derive(Clone, Default, serde::Deserialize, serde::Serialize)]
pub struct SubtitleTrack {
    pub id: i64,
    #[serde(default)]
    pub language: String,
    #[serde(default)]
    pub title: String,
    #[serde(default)]
    pub codec: String,
    /// A `.srt` beside the film rather than a stream inside it. Only these are
    /// fetched: mpv already reads everything embedded in the container, and it
    /// reads bitmap subtitles the server deliberately refuses to serve at all
    /// (decision 3).
    #[serde(default)]
    pub is_external: bool,
    /// "text" or "image".
    #[serde(default)]
    pub kind: String,
}

impl SubtitleTrack {
    /// Whether this is a track mpv should be handed by URL.
    pub fn fetchable(&self) -> bool {
        self.is_external && self.kind == "text"
    }
}

#[derive(serde::Deserialize)]
struct MovieList {
    #[serde(default)]
    movies: Vec<Movie>,
}

#[derive(serde::Deserialize)]
struct SeriesList {
    #[serde(default)]
    series: Vec<Series>,
}

#[derive(serde::Deserialize)]
struct ProfileList {
    #[serde(default)]
    profiles: Vec<Profile>,
}

/// How many films one request asks for, and the ceiling on the walk that reads
/// the whole library. A personal collection is a few hundred rows; the ceiling
/// exists so a server that always answers with a full page cannot keep the loop
/// going forever.
pub const LIBRARY_PAGE: u32 = 200;
pub const LIBRARY_CEILING: u32 = 5000;

/// Builds the URL for a cached TMDB image, or an empty string when there is
/// nothing to draw. A free function rather than a method because resolving a
/// hundred films' artwork should not mean building a hundred HTTP agents.
///
/// The sizes are not free choices: the server whitelists 342, 500 and 780, a
/// card is a 16/9 backdrop and therefore `w780`, and a poster is only ever the
/// fallback - cased in the artwork frame rather than covering it (design system
/// 6.1) - so it is fetched at the width a card can actually show.
fn image_url(base: &str, path: &str, size: &str) -> String {
    if path.is_empty() {
        return String::new();
    }
    // Both ends are trimmed because both are somebody else's: a connection
    // stores its base without a trailing slash, and TMDB writes its paths with
    // a leading one. Trusting either produced `http://host:8395//api/...`, which
    // the server answers with a redirect - a wasted round trip per card.
    format!(
        "{}/api/images/{size}/{}",
        base.trim_end_matches('/'),
        path.trim_start_matches('/')
    )
}

/// A connection to one server.
pub struct Client {
    agent: ureq::Agent,
    base: String,
    /// The profile viewing history belongs to, sent as `?profile=`. Never a
    /// credential: decision 49 has the profile travelling in the open.
    profile: Option<i64>,
}

impl Client {
    pub fn new(base: &str) -> Client {
        Self::with_timeout(base, Duration::from_secs(10))
    }

    /// Builds a client whose calls fail within a caller-chosen window.
    ///
    /// A manual connection may legitimately take a few seconds over a tunnel,
    /// while startup probes must move on quickly to the next candidate. Keeping
    /// both on the same ten-second timeout made an offline remembered server
    /// look like a frozen application.
    pub fn with_timeout(base: &str, timeout: Duration) -> Client {
        Client {
            agent: ureq::AgentBuilder::new()
                .timeout(timeout)
                .build(),
            base: base.trim_end_matches('/').to_string(),
            profile: None,
        }
    }

    pub fn base(&self) -> &str {
        &self.base
    }

    pub fn profile(&self) -> Option<i64> {
        self.profile
    }

    pub fn set_profile(&mut self, id: Option<i64>) {
        self.profile = id;
    }

    /// Appends the profile, preserving any query the path already carries.
    fn url(&self, path: &str) -> String {
        let full = format!("{}{}", self.base, path);
        match self.profile {
            None => full,
            Some(id) => format!(
                "{}{}profile={id}",
                full,
                if full.contains('?') { '&' } else { '?' }
            ),
        }
    }

    pub fn health(&self) -> Result<Health, String> {
        self.get_json("/api/health")
    }

    pub fn profiles(&self) -> Result<Vec<Profile>, String> {
        let list: ProfileList = self.get_json("/api/profiles")?;
        Ok(list.profiles)
    }

    pub fn rename_profile(&self, id: i64, name: &str) -> Result<Profile, String> {
        self.request_json(
            "PATCH",
            &format!("/api/profiles/{id}"),
            Some(serde_json::json!({ "name": name })),
        )
    }

    pub fn set_profile_avatar(
        &self,
        id: i64,
        content_type: &str,
        data: &[u8],
    ) -> Result<Profile, String> {
        let path = format!("/api/profiles/{id}/avatar");
        let response = match self
            .agent
            .put(&self.url(&path))
            .set("Content-Type", content_type)
            .send_bytes(data)
        {
            Ok(response) => response,
            Err(ureq::Error::Status(_, response)) => response,
            Err(error) => return Err(format!("{path}: {error}")),
        };
        response
            .into_json::<Profile>()
            .map_err(|e| format!("{path}: the answer was not the JSON this player expects: {e}"))
    }

    pub fn clear_profile_avatar(&self, id: i64) -> Result<Profile, String> {
        self.request_json("DELETE", &format!("/api/profiles/{id}/avatar"), None)
    }

    pub fn update_status(&self) -> Result<UpdateStatus, String> {
        self.get_json("/api/update")
    }

    pub fn check_update(&self) -> Result<UpdateStatus, String> {
        self.post_json("/api/update/check")
    }

    pub fn apply_update(&self) -> Result<UpdateStatus, String> {
        self.post_json("/api/update/apply")
    }

    pub fn movies(&self, limit: u32, offset: u32) -> Result<Vec<Movie>, String> {
        let mut list: MovieList = self.get_json(&format!("/api/library/movies?limit={limit}&offset={offset}"))?;
        for movie in &mut list.movies {
            movie.resolve_artwork(&self.base);
        }
        Ok(list.movies)
    }

    /// Every film the server holds, one page at a time.
    ///
    /// The API is paginated and the player used to read the first page only,
    /// which on a real library meant most of it was invisible. A page that comes
    /// back short is the last one; the ceiling is what stops a server that always
    /// answers with a full page from looping forever.
    pub fn all_movies(&self) -> Result<Vec<Movie>, String> {
        let mut all = Vec::new();
        let mut offset = 0;
        loop {
            let mut batch = self.movies(LIBRARY_PAGE, offset)?;
            let got = batch.len() as u32;
            all.append(&mut batch);
            offset += got;
            if got < LIBRARY_PAGE || all.len() as u32 >= LIBRARY_CEILING {
                return Ok(all);
            }
        }
    }

    pub fn movie(&self, id: i64) -> Result<Movie, String> {
        let mut movie: Movie = self.get_json(&format!("/api/library/movies/{id}"))?;
        movie.resolve_artwork(&self.base);
        Ok(movie)
    }

    pub fn series(&self) -> Result<Vec<Series>, String> {
        let mut list: SeriesList = self.get_json("/api/library/series?limit=500&offset=0")?;
        for series in &mut list.series {
            series.resolve_artwork(&self.base);
        }
        Ok(list.series)
    }

    pub fn series_detail(&self, id: i64) -> Result<Series, String> {
        let mut series: Series = self.get_json(&format!("/api/library/series/{id}"))?;
        series.resolve_artwork(&self.base);
        Ok(series)
    }

    /// The home screen: one hero and the short rows the server built, in the
    /// order it built them.
    ///
    /// The hero is drawn across most of the window, so its backdrop is
    /// resolved again at w1280 - the w780 a card lives on would be soft at
    /// that size. Everything else stays on the card sizes.
    pub fn home(&self) -> Result<HomeScreen, String> {
        let mut home: HomeScreen = self.get_json("/api/library/home")?;
        if let Some(hero) = &mut home.hero {
            hero.resolve_artwork(&self.base);
            hero.backdrop_url = image_url(&self.base, &hero.metadata.backdrop_path, "w1280");
        }
        for row in &mut home.rows {
            for movie in &mut row.movies {
                movie.resolve_artwork(&self.base);
            }
        }
        Ok(home)
    }

    /// The series half of the home screen: episodes to continue, shows that
    /// are new.
    pub fn series_home(&self) -> Result<SeriesHome, String> {
        let mut home: SeriesHome = self.get_json("/api/library/series/home")?;
        for episode in &mut home.continue_watching {
            episode.resolve_artwork(&self.base);
        }
        for series in &mut home.recent_series {
            series.resolve_artwork(&self.base);
        }
        Ok(home)
    }

    pub fn season(&self, series_id: i64, season_number: i32) -> Result<Season, String> {
        let mut season: Season = self.get_json(&format!(
            "/api/library/series/{series_id}/seasons/{season_number}"
        ))?;
        for episode in &mut season.items {
            episode.resolve_artwork(&self.base);
        }
        Ok(season)
    }

    /// The card preview for one item, with its URL resolved against this
    /// server.
    ///
    /// The state machine stays the server's - `building`, `ready`, or nothing -
    /// and this only makes the URL absolute, for the same reason artwork is
    /// resolved here: the interface never learns the server's address.
    pub fn preview_clip(&self, kind: &str, id: i64) -> Result<PreviewState, String> {
        let path = match kind {
            "episode" => format!("/api/library/episodes/{id}/preview/clip"),
            _ => format!("/api/stream/{id}/preview/clip"),
        };
        let mut payload: PreviewState = self.get_json(&path)?;
        if !payload.clip_url.is_empty() {
            payload.clip_url = format!("{}{}", self.base, payload.clip_url);
        }
        Ok(payload)
    }

    pub fn episode(&self, id: i64) -> Result<EpisodeItem, String> {
        let mut episode: EpisodeItem = self.get_json(&format!("/api/library/episodes/{id}"))?;
        episode.resolve_artwork(&self.base);
        Ok(episode)
    }

    /// The URL mpv should open. Handing mpv the URL rather than streaming
    /// through this process is deliberate: mpv does its own range requests,
    /// buffering and seeking, and it is better at all three than anything
    /// written here would be.
    pub fn stream_url(&self, movie_id: i64, file_id: i64) -> String {
        self.url(&format!("/api/stream/{movie_id}/files/{file_id}"))
    }

    pub fn episode_stream_url(&self, episode_id: i64, file_id: i64) -> String {
        self.url(&format!(
            "/api/library/episodes/{episode_id}/files/{file_id}/stream"
        ))
    }

    /// How the server will deliver one file, and what can be chosen while
    /// watching it. Also the call that refreshes the sidecar subtitle list.
    pub fn stream_info(&self, movie_id: i64, file_id: i64) -> Result<StreamInfo, String> {
        self.get_json(&format!("/api/stream/{movie_id}/files/{file_id}/info"))
    }

    pub fn episode_stream_info(
        &self,
        episode_id: i64,
        file_id: i64,
    ) -> Result<StreamInfo, String> {
        self.get_json(&format!(
            "/api/library/episodes/{episode_id}/files/{file_id}/stream/info"
        ))
    }

    /// One subtitle track as WebVTT, which is what the server serves and what
    /// mpv reads.
    ///
    /// No `?t=`: the server rebases a track when the stream is a remux whose
    /// clock restarts at zero on every seek, and it only does that when asked.
    /// The native player takes the file untouched, so the subtitles have to keep
    /// the film's own clock.
    pub fn subtitle_url(&self, movie_id: i64, file_id: i64, track_id: i64) -> String {
        self.url(&format!(
            "/api/library/movies/{movie_id}/files/{file_id}/subtitles/{track_id}"
        ))
    }

    pub fn episode_subtitle_url(
        &self,
        episode_id: i64,
        file_id: i64,
        track_id: i64,
    ) -> String {
        self.url(&format!(
            "/api/library/episodes/{episode_id}/files/{file_id}/subtitles/{track_id}"
        ))
    }

    pub fn save_progress(
        &self,
        movie_id: i64,
        position_seconds: f64,
        duration_seconds: f64,
    ) -> Result<(), String> {
        let body = serde_json::json!({
            "position_seconds": position_seconds,
            "duration_seconds": duration_seconds,
        });
        self.agent
            .put(&self.url(&format!("/api/library/movies/{movie_id}/progress")))
            .set("Content-Type", "application/json")
            .send_json(body)
            .map(|_| ())
            .map_err(|e| format!("saving progress: {e}"))
    }

    pub fn save_episode_progress(
        &self,
        episode_id: i64,
        position_seconds: f64,
        duration_seconds: f64,
    ) -> Result<(), String> {
        let body = serde_json::json!({
            "position_seconds": position_seconds,
            "duration_seconds": duration_seconds,
        });
        self.agent
            .put(&self.url(&format!("/api/library/episodes/{episode_id}/progress")))
            .set("Content-Type", "application/json")
            .send_json(body)
            .map(|_| ())
            .map_err(|e| format!("saving episode progress: {e}"))
    }

    fn get_json<T: serde::de::DeserializeOwned>(&self, path: &str) -> Result<T, String> {
        self.agent
            .get(&self.url(path))
            .call()
            .map_err(|e| format!("{path}: {e}"))?
            .into_json::<T>()
            .map_err(|e| format!("{path}: the answer was not the JSON this player expects: {e}"))
    }

    /// Update application may legitimately answer 409 (playback in progress)
    /// or 500 while still returning the useful structured updater state. Keep
    /// that body instead of flattening it into an opaque HTTP error.
    fn post_json<T: serde::de::DeserializeOwned>(&self, path: &str) -> Result<T, String> {
        let response = match self.agent.post(&self.url(path)).call() {
            Ok(response) => response,
            Err(ureq::Error::Status(_, response)) => response,
            Err(error) => return Err(format!("{path}: {error}")),
        };
        response
            .into_json::<T>()
            .map_err(|e| format!("{path}: the answer was not the JSON this player expects: {e}"))
    }

    fn request_json<T: serde::de::DeserializeOwned>(
        &self,
        method: &str,
        path: &str,
        body: Option<serde_json::Value>,
    ) -> Result<T, String> {
        let request = self
            .agent
            .request(method, &self.url(path))
            .set("Content-Type", "application/json");
        let response = match body {
            Some(body) => request.send_json(body),
            None => request.call(),
        };
        let response = match response {
            Ok(response) => response,
            Err(ureq::Error::Status(_, response)) => response,
            Err(error) => return Err(format!("{path}: {error}")),
        };
        response
            .into_json::<T>()
            .map_err(|e| format!("{path}: the answer was not the JSON this player expects: {e}"))
    }
}

/// A bounded health probe for startup orchestration.
///
/// It deliberately does not mutate the connected client. The OSD owns the
/// decision to connect; this only answers whether an all-in-one server is ready
/// to receive that connection.
pub fn reachable(base: &str, timeout: Duration) -> bool {
    Client::with_timeout(base, timeout).health().is_ok()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn the_profile_is_appended_without_breaking_a_query() {
        let mut client = Client::new("http://host:8395/");
        assert_eq!(client.url("/api/health"), "http://host:8395/api/health");

        client.set_profile(Some(3));
        assert_eq!(
            client.url("/api/health"),
            "http://host:8395/api/health?profile=3"
        );
        assert_eq!(
            client.url("/api/library/movies?limit=5"),
            "http://host:8395/api/library/movies?limit=5&profile=3"
        );
    }

    #[test]
    fn the_stream_url_names_both_ids() {
        let client = Client::new("http://host:8395");
        assert_eq!(
            client.stream_url(7, 9),
            "http://host:8395/api/stream/7/files/9"
        );
    }

    #[test]
    fn a_card_is_drawn_from_the_backdrop_at_the_width_a_card_shows() {
        let mut movie: Movie = serde_json::from_str(
            r#"{"id":4,"title":"Multi Track","year":2021,
                "metadata":{"tmdb_title":"Le film traduit","poster_path":"/p.jpg","backdrop_path":"/b.jpg"},
                "progress":{"position_seconds":0,"finished":false}}"#,
        )
        .expect("the server's own shape should parse");

        // The paths arrive with a leading slash, as TMDB writes them, and the
        // URL must not end up with two.
        movie.resolve_artwork("http://host:8395/");
        assert_eq!(movie.metadata.title, "Le film traduit");
        let card = serde_json::to_value(&movie).expect("the player card should serialize");
        assert_eq!(card["metadata"]["title"], "Le film traduit");
        assert!(card["metadata"].get("tmdb_title").is_none());
        assert_eq!(movie.backdrop_url, "http://host:8395/api/images/w780/b.jpg");
        assert_eq!(movie.poster_url, "http://host:8395/api/images/w500/p.jpg");
    }

    #[test]
    fn a_film_with_no_artwork_gets_no_url_rather_than_a_broken_one() {
        let mut movie: Movie = serde_json::from_str(r#"{"id":1,"title":"Unmatched"}"#)
            .expect("a film TMDB never matched still has to parse");
        movie.resolve_artwork("http://host:8395");
        assert_eq!(movie.backdrop_url, "");
        assert_eq!(movie.poster_url, "");
    }

    #[test]
    fn only_a_text_file_beside_the_film_is_fetched() {
        // What the server sends for a film with a `.srt` beside it, an embedded
        // text track, and a bitmap one. The first two carry no `kind` when the
        // file has never been inspected, which must read as "text".
        let info: StreamInfo = serde_json::from_str(
            r#"{"mode":"direct","subtitle_tracks":[
                 {"id":6,"language":"fra","codec":"srt","is_external":true,"kind":"text"},
                 {"id":7,"language":"eng","codec":"subrip","is_external":false,"kind":"text"},
                 {"id":8,"codec":"hdmv_pgs_subtitle","is_external":true,"kind":"image"}]}"#,
        )
        .expect("the server's own shape should parse");

        let fetched: Vec<i64> = info
            .subtitle_tracks
            .iter()
            .filter(|track| track.fetchable())
            .map(|track| track.id)
            .collect();
        // The embedded track is already in the container mpv is reading, and a
        // bitmap one cannot be served at all (decision 3): handing either to mpv
        // by URL would fetch something worse than what it already has.
        assert_eq!(fetched, vec![6]);
    }

    #[test]
    fn a_sidecar_is_fetched_as_webvtt_and_keeps_the_films_clock() {
        let client = Client::new("http://host:8395");
        assert_eq!(
            client.subtitle_url(4, 9, 6),
            "http://host:8395/api/library/movies/4/files/9/subtitles/6"
        );
        // No `?t=`: the rebase is for a remux whose clock restarts at zero, and
        // the native player takes the file itself, so a rebased track would sit
        // as far from the picture as the viewer has travelled into the film.
        assert!(!client.subtitle_url(4, 9, 6).contains("t="));
    }
}
