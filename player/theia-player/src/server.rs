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

mod models;
pub use models::*;

use std::io::Read;
use std::time::{Duration, Instant};

const PREVIEW_LIMIT: u64 = 8 << 20;

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
    let daemon =
        mdns_sd::ServiceDaemon::new().map_err(|e| format!("starting the mDNS browser: {e}"))?;
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
                let version = info.get_property_val_str("version").map(|v| v.to_string());
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

/// The rung as a height, if one was asked for.
///
/// Zero is the ladder's own word for "the file as it is" and is never a rung:
/// the server offers none at that height, and the bare route is what serves the
/// file.
fn rung(height: Option<i64>) -> Option<i64> {
    height.filter(|height| *height > 0)
}

/// A converted stream's query: where to start, and how tall.
///
/// `t=` is not a convenience here, it is the film's clock: the pipe starts at
/// zero, so a viewer who changes quality an hour in is served an hour in.
fn converted_query(height: i64, start: f64) -> String {
    let at = if start.is_finite() && start > 0.0 {
        start
    } else {
        0.0
    };
    format!("t={at:.3}&h={height}")
}

/// How many films or series one request asks for, and the ceiling on a walk of
/// the whole library. A personal collection is a few hundred rows; the ceiling
/// exists so a server that always answers with a full page cannot keep the loop
/// going forever.
pub const LIBRARY_PAGE: u32 = 200;
pub const LIBRARY_CEILING: u32 = 5000;

fn collect_pages<T>(
    mut fetch: impl FnMut(u32, u32) -> Result<Vec<T>, String>,
) -> Result<Vec<T>, String> {
    let mut all = Vec::new();
    let mut offset = 0;
    loop {
        let mut batch = fetch(LIBRARY_PAGE, offset)?;
        let got = batch.len() as u32;
        all.append(&mut batch);
        offset += got;
        if got < LIBRARY_PAGE || all.len() as u32 >= LIBRARY_CEILING {
            return Ok(all);
        }
    }
}

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

/// Why a connection attempt was refused, in the two shapes the connect screen
/// tells apart.
///
/// The sentences belong to the interface, and the player answers a code
/// (decision 25, read the other way round). The distinction is not decoration:
/// on 27 September 2026 the maintainer typed `http://192.168.1.77.8383` - a dot
/// where the colon belongs - pressed Connect, and read "check the address, and
/// that Theia is running". The server had been up for four minutes and its log
/// held no request from that machine at all: the address never left the player,
/// and the one sentence sent the search to a machine that was never the
/// problem.
#[derive(Clone, Copy, Debug)]
enum RefusalKind {
    /// The address itself could not be read: a URL the parser refuses, a scheme
    /// this player has no stack for, or a name that does not resolve.
    Unreadable,
    /// The address was understood, and nothing usable came back from it.
    Silent,
}

impl RefusalKind {
    /// The code the interface owns a sentence for.
    fn code(self) -> &'static str {
        match self {
            RefusalKind::Unreadable => "address_unreadable",
            RefusalKind::Silent => "no_answer",
        }
    }
}

/// A refused connection: which of the two shapes, and what the transport said.
///
/// `Display` is the transport's own words, which is what the console and the
/// `--server` flag print; the OSD never shows them.
#[derive(Clone, Debug)]
pub struct Refusal {
    kind: RefusalKind,
    detail: String,
}

impl Refusal {
    /// The code the interface owns a sentence for.
    pub fn code(&self) -> &'static str {
        self.kind.code()
    }

    pub fn detail(&self) -> &str {
        &self.detail
    }

    /// A failure of something other than the address: the shape a call that got
    /// past the parser carries when it fails.
    pub fn silent(detail: String) -> Refusal {
        Refusal {
            kind: RefusalKind::Silent,
            detail,
        }
    }

    /// Reads a transport failure.
    ///
    /// `InvalidUrl`, `UnknownScheme` and `Dns` are about the address: the parser
    /// or the resolver could not make a host out of it. Everything else - a
    /// refused connection, a timeout, a status this player did not expect -
    /// means the address was understood and the sentence about the server is
    /// the true one.
    fn of(error: ureq::Error) -> Refusal {
        let kind = match error.kind() {
            ureq::ErrorKind::InvalidUrl | ureq::ErrorKind::UnknownScheme | ureq::ErrorKind::Dns => {
                RefusalKind::Unreadable
            }
            _ => RefusalKind::Silent,
        };
        Refusal {
            kind,
            detail: error.to_string(),
        }
    }
}

impl std::fmt::Display for Refusal {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(&self.detail)
    }
}

/// A connection to one server.
#[derive(Clone)]
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
    ///
    /// The address is trimmed, and that is not politeness. The contract is an
    /// address somebody typed or copied - the server prints one at startup and
    /// the console line it sits on carries a space - and `ureq` refuses
    /// `http://host:8383 ` with `InvalidPort`, the space becoming part of the
    /// port. A correct address then failed to connect while the screen blamed
    /// the server. Nothing else is guessed at: no scheme is invented, no port is
    /// moved, and an address that is still unreadable is reported as such.
    pub fn with_timeout(base: &str, timeout: Duration) -> Client {
        Client {
            agent: ureq::AgentBuilder::new().timeout(timeout).build(),
            base: base.trim().trim_end_matches('/').to_string(),
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

    pub fn episode_history(&self, id: i64, watched: bool) -> Result<(), String> {
        let path = format!(
            "/api/library/episodes/{id}/{}",
            if watched { "watched" } else { "progress" }
        );
        let response = self
            .agent
            .request(if watched { "PUT" } else { "DELETE" }, &self.url(&path))
            .call();
        response.map(|_| ()).map_err(|error| error.to_string())
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

    /// Asks the server who it is, keeping the failure's shape.
    ///
    /// [`Client::health`] answers a sentence, which is what every other caller
    /// wants. This one exists for `connect_to`, which has to tell an address it
    /// could not read from a server that did not answer - see [`Refusal`].
    pub fn probe(&self) -> Result<Health, Refusal> {
        let response = self
            .agent
            .get(&self.url("/api/health"))
            .call()
            .map_err(Refusal::of)?;
        response.into_json::<Health>().map_err(|error| {
            Refusal::silent(format!(
                "the answer was not the JSON this player expects: {error}"
            ))
        })
    }

    pub fn health(&self) -> Result<Health, String> {
        self.probe()
            .map_err(|refusal| format!("/api/health: {}", refusal.detail()))
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
        let mut list: MovieList = self.get_json(&format!(
            "/api/library/movies?limit={limit}&offset={offset}"
        ))?;
        for movie in &mut list.movies {
            crate::progress_queue::overlay(self, false, movie.id, &mut movie.progress);
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
        collect_pages(|limit, offset| self.movies(limit, offset))
    }

    /// First inspection can prepare FFmpeg; ordinary catalogue timeouts stay short.
    pub fn inspect_file(
        &self,
        kind: &str,
        id: i64,
        file_id: i64,
    ) -> Result<serde_json::Value, String> {
        let resource = match kind {
            "movie" => "movies",
            "episode" => "episodes",
            _ => return Err("invalid_media_kind".into()),
        };
        let mut client = Self::with_timeout(&self.base, Duration::from_secs(180));
        client.set_profile(self.profile);
        let value: serde_json::Value = client.post_json(&format!(
            "/api/library/{resource}/{id}/files/{file_id}/inspect"
        ))?;
        if let Some(error) = value.get("error").and_then(|error| error.as_str()) {
            return Err(error.to_string());
        }
        if value.get("id").and_then(|id| id.as_i64()) != Some(file_id)
            || value
                .pointer("/media/status")
                .and_then(|status| status.as_str())
                != Some("ok")
        {
            return Err("invalid_inspection_response".into());
        }
        Ok(value)
    }

    pub fn movie(&self, id: i64) -> Result<Movie, String> {
        let mut movie: Movie = self.get_json(&format!("/api/library/movies/{id}"))?;
        crate::progress_queue::overlay(self, false, movie.id, &mut movie.progress);
        movie.resolve_artwork(&self.base);
        Ok(movie)
    }

    pub fn series(&self) -> Result<Vec<Series>, String> {
        collect_pages(|limit, offset| self.series_page(limit, offset))
    }

    fn series_page(&self, limit: u32, offset: u32) -> Result<Vec<Series>, String> {
        let mut list: SeriesList = self.get_json(&format!(
            "/api/library/series?limit={limit}&offset={offset}"
        ))?;
        for series in &mut list.series {
            if let Some(episode) = &mut series.resume_episode {
                crate::progress_queue::overlay(self, true, episode.id, &mut episode.progress);
            }
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
    /// Nothing here is re-resolved at a size of its own: the hero draws the
    /// `hero_url` every item already carries, and the rows draw the card sizes.
    pub fn home(&self) -> Result<HomeScreen, String> {
        let mut home: HomeScreen = self.get_json("/api/library/home")?;
        if let Some(hero) = &mut home.hero {
            crate::progress_queue::overlay(self, false, hero.id, &mut hero.progress);
            hero.resolve_artwork(&self.base);
        }
        for row in &mut home.rows {
            for movie in &mut row.movies {
                crate::progress_queue::overlay(self, false, movie.id, &mut movie.progress);
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
            crate::progress_queue::overlay(self, true, episode.id, &mut episode.progress);
            episode.resolve_artwork(&self.base);
        }
        for series in &mut home.recent_series {
            series.resolve_artwork(&self.base);
        }
        Ok(home)
    }

    /// What this profile has watched, for the settings sheet.
    ///
    /// Per viewer, because that is how the server keeps a history: the client
    /// carries the profile on every call, so this asks for the one watching
    /// here rather than for the household's total.
    pub fn watch_stats(&self) -> Result<WatchStats, String> {
        let mut stats: WatchStats = self.get_json("/api/library/watching")?;
        for one in &mut stats.top_series {
            // w185, the smallest poster TMDB serves: the ranking draws it at
            // about forty pixels, and a card's w342 would be four times the
            // bytes for the same picture.
            one.poster_url = image_url(&self.base, &one.poster_path, "w185");
        }
        Ok(stats)
    }

    pub fn season(&self, series_id: i64, season_number: i32) -> Result<Season, String> {
        let mut season: Season = self.get_json(&format!(
            "/api/library/series/{series_id}/seasons/{season_number}"
        ))?;
        for episode in &mut season.items {
            crate::progress_queue::overlay(self, true, episode.id, &mut episode.progress);
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
        // One algorithm for every kind of item: each resolves to a file - a
        // series to its earliest episode's - and the server names the route.
        let path = match kind {
            "episode" => format!("/api/library/episodes/{id}/preview/clip"),
            "series" => format!("/api/library/series/{id}/preview/clip"),
            _ => format!("/api/stream/{id}/preview/clip"),
        };
        let mut payload: PreviewState = self.get_json(&path)?;
        if !payload.clip_url.is_empty() {
            let url = format!("{}{}", self.base, payload.clip_url);
            // The bytes come back through here rather than being left to a
            // `<video src>` pointing at the server, and that is not tidiness.
            // WebView2 refused the media with `MEDIA_ELEMENT_ERROR: Format
            // error` without sending a request at all - measured on 20 September
            // 2026, on a clip another Chromium decodes and with the response
            // carrying `Access-Control-Allow-Origin` and
            // `Cross-Origin-Resource-Policy: cross-origin` - so the address
            // never reaches the page and the element is handed the bytes it
            // already trusts. It is also the rule this client already follows
            // everywhere else: the interface never learns the server's address.
            payload.data_url = self.preview_data(&url)?;
            payload.clip_url = url;
        }
        Ok(payload)
    }

    /// Fetches a clip and returns it as a data URL.
    ///
    /// Bounded, because the only thing that ever answers this is a six-second
    /// card preview and a client that will read whatever it is given should say
    /// what it is willing to read.
    fn preview_data(&self, url: &str) -> Result<String, String> {
        let response = self
            .agent
            .get(url)
            .call()
            .map_err(|e| format!("fetching the preview: {e}"))?;
        let bytes = read_preview(response.into_reader())?;
        Ok(format!("data:video/mp4;base64,{}", base64(&bytes)))
    }

    pub fn episode(&self, id: i64) -> Result<EpisodeItem, String> {
        let mut episode: EpisodeItem = self.get_json(&format!("/api/library/episodes/{id}"))?;
        crate::progress_queue::overlay(self, true, episode.id, &mut episode.progress);
        episode.resolve_artwork(&self.base);
        Ok(episode)
    }

    /// The URL mpv should open, at one rung of the ladder.
    ///
    /// Handing mpv the URL rather than streaming through this process is
    /// deliberate: mpv does its own range requests, buffering and seeking, and
    /// it is better at all three than anything written here would be.
    ///
    /// `None` is the file itself, which is what makes this player worth having:
    /// mpv reads the container directly, so TrueHD, Atmos and DTS-HD MA reach
    /// the amplifier untouched. A rung is a *different route* - `/remux`, which
    /// the server answers with a converted pipe that honours `h=` - because the
    /// bare route serves bytes and ignores the height entirely (measured: `h=1080`
    /// on it returns the file, `Content-Range` and all). A pipe starts at zero
    /// and has no length, so the film's own clock is `t=` on the address plus
    /// whatever mpv counts, and seeking it means asking again from another `t=`.
    pub fn stream_url(
        &self,
        movie_id: i64,
        file_id: i64,
        height: Option<i64>,
        start: f64,
    ) -> String {
        match rung(height) {
            Some(height) => self.url(&format!(
                "/api/stream/{movie_id}/files/{file_id}/remux?{}",
                converted_query(height, start)
            )),
            None => self.url(&format!("/api/stream/{movie_id}/files/{file_id}")),
        }
    }

    pub fn episode_stream_url(
        &self,
        episode_id: i64,
        file_id: i64,
        height: Option<i64>,
        start: f64,
    ) -> String {
        match rung(height) {
            Some(height) => self.url(&format!(
                "/api/library/episodes/{episode_id}/files/{file_id}/stream/remux?{}",
                converted_query(height, start)
            )),
            None => self.url(&format!(
                "/api/library/episodes/{episode_id}/files/{file_id}/stream"
            )),
        }
    }

    /// How the server will deliver one file, and what can be chosen while
    /// watching it. Also the call that refreshes the sidecar subtitle list.
    pub fn stream_info(&self, movie_id: i64, file_id: i64) -> Result<StreamInfo, String> {
        self.get_json(&format!("/api/stream/{movie_id}/files/{file_id}/info"))
    }

    pub fn episode_stream_info(&self, episode_id: i64, file_id: i64) -> Result<StreamInfo, String> {
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

    pub fn episode_subtitle_url(&self, episode_id: i64, file_id: i64, track_id: i64) -> String {
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

/// Read one complete preview or reject it. Reading one byte past the limit
/// distinguishes an exact-fit clip from one that would be silently truncated.
fn read_preview(reader: impl Read) -> Result<Vec<u8>, String> {
    let mut bytes = Vec::new();
    reader
        .take(PREVIEW_LIMIT + 1)
        .read_to_end(&mut bytes)
        .map_err(|e| format!("reading the preview: {e}"))?;
    if bytes.is_empty() {
        return Err("the preview was empty".into());
    }
    if bytes.len() as u64 > PREVIEW_LIMIT {
        return Err("the preview exceeds 8 MiB".into());
    }
    Ok(bytes)
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
    use std::io::Read;

    #[test]
    fn a_full_series_page_does_not_hide_the_next_one() {
        let mut offsets = Vec::new();
        let ids: Vec<u32> = collect_pages(|limit, offset| {
            offsets.push(offset);
            Ok((offset..(offset + limit).min(501)).collect())
        })
        .unwrap();
        assert_eq!(ids.len(), 501);
        assert_eq!(offsets, [0, 200, 400]);
        assert_eq!(ids[500], 500);
    }

    #[test]
    fn a_preview_is_complete_or_refused() {
        assert_eq!(read_preview(&b"clip"[..]).unwrap(), b"clip");
        assert!(read_preview(std::io::empty())
            .unwrap_err()
            .contains("empty"));
        assert_eq!(
            read_preview(std::io::repeat(0).take(PREVIEW_LIMIT))
                .unwrap()
                .len() as u64,
            PREVIEW_LIMIT
        );
        assert!(read_preview(std::io::repeat(0).take(PREVIEW_LIMIT + 1))
            .unwrap_err()
            .contains("exceeds 8 MiB"));
    }

    #[test]
    fn a_null_row_list_from_a_released_server_is_an_empty_one() {
        // What 3.3.1 sent for a library with no rows. The player and the server
        // update separately, so this is a real combination until the installed
        // server updates itself - and it was "the home screen could not be
        // loaded" until this test existed.
        let released: HomeScreen = serde_json::from_str(include_str!(
            "../../contract-fixtures/home-null-released.json"
        ))
        .expect("a released server's empty home screen has to parse");
        assert!(released.rows.is_empty());
        assert_eq!(released.total, 0);
        let empty: HomeScreen =
            serde_json::from_str(include_str!("../../contract-fixtures/home-empty.json")).unwrap();
        assert!(empty.hero.is_none() && empty.rows.is_empty() && empty.total == 0);

        // And the shape the fixed server sends keeps working, with a row in it.
        let current: HomeScreen = serde_json::from_str(
            r#"{"hero":null,"hero_kind":"","rows":[{"kind":"recent","movies":[{"id":7,"title":"A film"}]}],"total":1}"#,
        )
        .expect("the current shape parses");
        assert_eq!(current.rows.len(), 1);
        assert_eq!(current.rows[0].movies[0].title, "A film");
    }

    #[test]
    fn the_watching_payload_is_read_as_the_server_writes_it() {
        // The literal is what the server answered on 24 September 2026, copied
        // rather than paraphrased: the shape is the contract between the two
        // programs, and a field the player forgets to read is a section that
        // silently draws nothing - which is exactly how the ranking's poster
        // showed a placeholder while the code looked right.
        let payload = r#"{
            "movies": {"started": 1, "finished": 0, "seconds": 4132.295},
            "series": {"started": 1, "finished": 0, "seconds": 2079.958},
            "episodes": {"started": 1, "finished": 0, "seconds": 2079.958},
            "month": {"movies": 1, "episodes": 1, "seconds": 6212.253},
            "top_series": [{"id": 1, "title": "Sh\u014dgun",
                            "poster_path": "/7O4iVfOMQmdCSxhOg1WnzG1AgYT.jpg",
                            "episodes": 1, "finished": 0, "total": 10,
                            "seconds": 2079.958}]
        }"#;
        let stats: WatchStats = serde_json::from_str(payload).expect("the server's own answer");
        assert_eq!(stats.month.movies, 1);
        assert_eq!(stats.month.seconds, 6212.253);
        assert_eq!(stats.top_series[0].title, "Shōgun");
        assert_eq!(
            stats.top_series[0].poster_path,
            "/7O4iVfOMQmdCSxhOg1WnzG1AgYT.jpg"
        );
        // And the URL the interface is handed, built the way watch_stats builds
        // it: an empty path means an empty URL, which is the placeholder.
        assert_eq!(
            image_url(
                "http://127.0.0.1:8383",
                &stats.top_series[0].poster_path,
                "w185"
            ),
            "http://127.0.0.1:8383/api/images/w185/7O4iVfOMQmdCSxhOg1WnzG1AgYT.jpg"
        );
    }

    #[test]
    fn an_image_url_trims_both_ends_and_keeps_the_size() {
        // The size is the caller's: the ranking draws a forty-pixel poster and
        // asks for w185, the cards ask for w342 and w500, and the hero for
        // w1280. Both ends are somebody else's - a connection's base carries no
        // trailing slash, a TMDB path carries a leading one - and trusting
        // either produced a double slash the server answers with a redirect,
        // one wasted round trip per image.
        assert_eq!(
            image_url("http://host:8395", "/x.jpg", "w185"),
            "http://host:8395/api/images/w185/x.jpg"
        );
        assert_eq!(
            image_url("http://host:8395/", "x.jpg", "w342"),
            "http://host:8395/api/images/w342/x.jpg"
        );
        // A series TMDB never matched has no path, and no URL is the honest
        // answer: the interface draws its own placeholder rather than asking
        // the server for an image that does not exist.
        assert_eq!(image_url("http://host:8395", "", "w185"), "");
    }

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

    /// The two shapes a refused connection has, on the addresses that produced
    /// them.
    ///
    /// The first is the maintainer's own typo on 27 September 2026, copied
    /// character for character: a dot where the colon belongs. The URL parser
    /// refuses it before any socket is opened, which is why the server's log
    /// held nothing from that machine and why "check that Theia is running"
    /// sent the search to the wrong computer. The second is a port nothing
    /// listens on: the address was understood, so the sentence about the server
    /// is the true one.
    #[test]
    fn an_address_that_cannot_be_read_is_not_a_server_that_is_down() {
        let Err(typo) = Client::new("http://192.168.1.77.8383").probe() else {
            panic!("a dot where the colon belongs is not a server address");
        };
        assert_eq!(typo.code(), "address_unreadable");

        let Err(closed) =
            Client::with_timeout("http://127.0.0.1:1", Duration::from_millis(500)).probe()
        else {
            panic!("nothing listens on port 1");
        };
        assert_eq!(closed.code(), "no_answer");
    }

    /// An address somebody copied arrives with the space the console line
    /// carried, and that space is not part of the port.
    ///
    /// `ureq` reads `http://host:8383 ` as a port called "8383 " and refuses it
    /// with `InvalidPort`, so the trim is what makes a correct address work at
    /// all. What it does not do is invent anything: an address that is still
    /// unreadable stays unreadable, and is reported as an address problem
    /// rather than as a server that is down.
    #[test]
    fn a_copied_address_loses_the_space_it_was_copied_with() {
        let client = Client::new("  http://host:8395/  ");
        assert_eq!(client.base(), "http://host:8395");
        assert_eq!(client.url("/api/health"), "http://host:8395/api/health");

        let spaced = Client::new("http://host:83 83");
        assert_eq!(spaced.base(), "http://host:83 83");
        let Err(refused) = spaced.probe() else {
            panic!("a port with a space in it is not a port");
        };
        assert_eq!(refused.code(), "address_unreadable");
    }

    #[test]
    fn the_stream_url_names_both_ids() {
        let client = Client::new("http://host:8395");
        assert_eq!(
            client.stream_url(7, 9, None, 0.0),
            "http://host:8395/api/stream/7/files/9"
        );
    }

    /// A rung is the converted route, and it carries the film's clock.
    ///
    /// Both halves are asserted because both were wrong in the first version of
    /// this: `h=` on the *bare* route is ignored by the server - measured, it
    /// answers with the file itself, `Content-Range` and all - and a converted
    /// pipe starts at zero, so a rung without `t=` would send somebody who
    /// changed quality an hour in back to the beginning. The profile is appended
    /// after the query (`Client::url`), which is the other thing that breaks
    /// silently if it is ever "simplified".
    #[test]
    fn a_rung_is_the_converted_route_and_carries_the_clock() {
        let mut client = Client::new("http://host:8395");
        assert_eq!(
            client.stream_url(7, 9, Some(1080), 3600.5),
            "http://host:8395/api/stream/7/files/9/remux?t=3600.500&h=1080"
        );
        // Zero is the ladder's own word for "the file as it is", and the file
        // is what the bare route serves.
        assert_eq!(
            client.stream_url(7, 9, Some(0), 12.0),
            "http://host:8395/api/stream/7/files/9"
        );
        client.set_profile(Some(3));
        assert_eq!(
            client.stream_url(7, 9, Some(720), 0.0),
            "http://host:8395/api/stream/7/files/9/remux?t=0.000&h=720&profile=3"
        );
        assert_eq!(
            client.episode_stream_url(5, 6, Some(720), 61.25),
            "http://host:8395/api/library/episodes/5/files/6/stream/remux?t=61.250&h=720&profile=3"
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
    fn native_detail_keeps_measured_files_credits_and_portrait_urls() {
        let mut movie: Movie = serde_json::from_value(serde_json::json!({
            "id": 1, "metadata": {
                "cast": [{"name":"Actor", "character":"Lead", "profile_path":"/actor.jpg"}],
                "crew": [{"name":"Writer", "role":"writing"}],
                "collection": {"name":"Saga"}
            },
            "files":[{"id":11,"size_bytes":123,"media":{"status":"ok","video":{"codec":"hevc","width":3840,"height":1604,"color_transfer":"smpte2084"}}}],
            "collection_parts":[{"id":2,"metadata":{"poster_path":"/part.jpg"}}]
        })).unwrap();
        movie.resolve_artwork("http://host:8395");
        let detail = serde_json::to_value(movie).unwrap();
        assert_eq!(detail["files"][0]["media"]["video"]["width"], 3840);
        assert_eq!(
            detail["metadata"]["cast"][0]["profile_url"],
            "http://host:8395/api/images/w185/actor.jpg"
        );
        assert_eq!(detail["metadata"]["crew"][0]["role"], "writing");
        assert_eq!(
            detail["collection_parts"][0]["poster_url"],
            "http://host:8395/api/images/w500/part.jpg"
        );
        let mut series: Series = serde_json::from_value(serde_json::json!({
            "id":1,"metadata":{"original_name":"Original","last_air_date":"2025-01-01","air_status":"ended","genres":["Drama"],"vote_average":8.5,"creators":["Creator"],"networks":["Network"],"cast":[{"name":"Lead","profile_path":"/lead.jpg"}]}
        })).unwrap();
        series.resolve_artwork("http://host:8395");
        let detail = serde_json::to_value(series).unwrap();
        assert_eq!(detail["metadata"]["creators"][0], "Creator");
        assert_eq!(
            detail["metadata"]["cast"][0]["profile_url"],
            "http://host:8395/api/images/w185/lead.jpg"
        );
        assert_eq!(detail["metadata"]["air_status"], "ended");
    }

    /// The frames that fill a window - the home's hero and the library's ambient
    /// picture - are not the frame a card draws, and one size cannot serve both:
    /// the w1280 that used to serve the hero is stretched about 1.9x on a
    /// 2568-pixel window at 200% scaling, which is the picture the maintainer
    /// reported as too small for its frame on 29 September 2026. The card keeps
    /// w780, because a hero-sized picture in every card would be paid for on
    /// every row.
    #[test]
    fn a_window_sized_frame_is_resolved_wider_than_a_card() {
        let mut movie: Movie = serde_json::from_str(
            r#"{"id":4,"title":"Dune: Part Two","metadata":{"backdrop_path":"/b.jpg"}}"#,
        )
        .expect("the server's own shape should parse");
        movie.resolve_artwork("http://host:8395/");
        assert_eq!(movie.hero_url, "http://host:8395/api/images/original/b.jpg");
        assert_eq!(movie.backdrop_url, "http://host:8395/api/images/w780/b.jpg");

        // A film TMDB never matched gets no hero URL rather than one that 404s.
        let mut unmatched: Movie = serde_json::from_str(r#"{"id":5,"title":"Unmatched"}"#)
            .expect("a film TMDB never matched still has to parse");
        unmatched.resolve_artwork("http://host:8395");
        assert_eq!(unmatched.hero_url, "");
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

/// Base64, written here rather than pulled in: it is twenty lines, it is the
/// only use this crate has for it, and a dependency for one function is a
/// dependency to keep patched for the life of the project.
fn base64(bytes: &[u8]) -> String {
    const ALPHABET: &[u8; 64] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
    let mut out = String::with_capacity(bytes.len().div_ceil(3) * 4);
    for chunk in bytes.chunks(3) {
        let b = [
            chunk[0],
            *chunk.get(1).unwrap_or(&0),
            *chunk.get(2).unwrap_or(&0),
        ];
        let n = ((b[0] as u32) << 16) | ((b[1] as u32) << 8) | b[2] as u32;
        out.push(ALPHABET[(n >> 18) as usize & 63] as char);
        out.push(ALPHABET[(n >> 12) as usize & 63] as char);
        out.push(if chunk.len() > 1 {
            ALPHABET[(n >> 6) as usize & 63] as char
        } else {
            '='
        });
        out.push(if chunk.len() > 2 {
            ALPHABET[n as usize & 63] as char
        } else {
            '='
        });
    }
    out
}

#[cfg(test)]
mod base64_tests {
    use super::base64;

    #[test]
    fn the_three_cases_and_a_real_padding_run() {
        // The RFC 4648 vectors, plus the lengths a video file actually has: a
        // preview is not a multiple of three bytes and the padding has to be
        // exactly right or the data URL decodes to a corrupt file.
        assert_eq!(base64(b""), "");
        assert_eq!(base64(b"f"), "Zg==");
        assert_eq!(base64(b"fo"), "Zm8=");
        assert_eq!(base64(b"foo"), "Zm9v");
        assert_eq!(base64(b"foob"), "Zm9vYg==");
        assert_eq!(base64(b"fooba"), "Zm9vYmE=");
        assert_eq!(base64(b"foobar"), "Zm9vYmFy");
    }
}
