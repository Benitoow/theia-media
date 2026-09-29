//! JSON records exchanged with the Theia server.

use super::image_url;

/// What `/api/health` answers.
#[derive(Clone, serde::Deserialize, serde::Serialize)]
pub struct Health {
    pub status: String,
    pub version: String,
    #[serde(default)]
    pub uptime_seconds: u64,
    /// The language this installation was set up in, which the OSD opens in
    /// unless this machine's viewer has already chosen for itself. Absent from
    /// a server older than decision 137, and English answers then.
    #[serde(default)]
    pub language: String,
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

/// What was watched, for the settings sheet's viewing section.
///
/// Every field defaults, for the reason the home screen's rows do: the two
/// programs update separately, and a server without the route answers a 404
/// rather than this shape. An unfilled section is a sentence; a settings sheet
/// that refuses to open over a statistic would not be.
/// The calendar month the server is in: what was watched whose last report
/// falls inside it, which is the only sense "this month" has without a table
/// that records each evening separately.
#[derive(Clone, Default, serde::Deserialize, serde::Serialize)]
pub struct ThisMonth {
    #[serde(default)]
    pub movies: u32,
    #[serde(default)]
    pub episodes: u32,
    #[serde(default)]
    pub seconds: f64,
}

#[derive(Clone, Default, serde::Deserialize, serde::Serialize)]
pub struct WatchStats {
    #[serde(default)]
    pub movies: Watched,
    #[serde(default)]
    pub series: Watched,
    #[serde(default)]
    pub episodes: Watched,
    #[serde(default)]
    pub month: ThisMonth,
    #[serde(default)]
    pub top_series: Vec<SeriesWatch>,
}

/// How much of one kind of thing was watched: the time actually spent, and the
/// two counts the server's own rule for "watched" produced.
#[derive(Clone, Default, serde::Deserialize, serde::Serialize)]
pub struct Watched {
    #[serde(default)]
    pub started: u32,
    #[serde(default)]
    pub finished: u32,
    #[serde(default)]
    pub seconds: f64,
}

/// One line of the ranking: a series and how much of it was watched.
///
/// Episodes, not files - a file holding S01E01E02 counts as two - and `total`
/// is every episode the series has, so the line reads "1 sur 10".
///
/// The poster arrives as the path TMDB writes and leaves as a URL against this
/// server, the same division of labour the library's cards use: the interface
/// never learns the server's address.
#[derive(Clone, Default, serde::Deserialize, serde::Serialize)]
pub struct SeriesWatch {
    #[serde(default)]
    pub id: i64,
    #[serde(default)]
    pub title: String,
    #[serde(default)]
    pub poster_path: String,
    #[serde(default)]
    pub poster_url: String,
    #[serde(default)]
    pub episodes: u32,
    #[serde(default)]
    pub finished: u32,
    #[serde(default)]
    pub total: u32,
    #[serde(default)]
    pub seconds: f64,
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
    /// The same backdrop at the size a frame that fills a window draws it - the
    /// home's hero and the library's ambient picture. A card is 336 CSS pixels
    /// wide; those two are most of a window, and one size cannot serve both.
    #[serde(default)]
    pub hero_url: String,
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
    /// The window-sized frame, as on a film.
    #[serde(default)]
    pub hero_url: String,
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
    // A list that arrives as null is an empty list. The released 3.3.1 server
    // encoded an empty row list that way, and the two programs update
    // separately: without this, a 3.3.2 player talking to an installed 3.3.1
    // server said "the home screen could not be loaded" about a library that was
    // simply empty. The server sends `[]` now (internal/library.HomeScreen), and
    // a window must not refuse a screen over the difference.
    #[serde(default, deserialize_with = "de_null_as_empty")]
    pub rows: Vec<HomeRow>,
    #[serde(default)]
    pub total: i64,
}

/// de_null_as_empty reads a list that a server may have encoded as null.
fn de_null_as_empty<'de, D, T>(deserializer: D) -> Result<Vec<T>, D::Error>
where
    D: serde::Deserializer<'de>,
    T: serde::Deserialize<'de>,
{
    // Fully qualified: this module uses serde's derive paths rather than
    // importing the traits, and `Option::<Vec<T>>::deserialize` needs the trait
    // in scope to resolve.
    Ok(<Option<Vec<T>> as serde::Deserialize>::deserialize(deserializer)?.unwrap_or_default())
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
    /// The episode the server says comes after this one, which is what the
    /// player starts when the file ends and the viewer asked for it.
    ///
    /// Absent is an answer and not a gap: the last of a season, a special and a
    /// show with one episode have no next, and `nextEpisode` in
    /// `internal/library/series.go` says so by sending nothing. The field is
    /// populated from the running order rather than from progress, so it is
    /// already known when this episode is opened - which is the moment the
    /// player reads it.
    #[serde(default)]
    pub next_episode_id: Option<i64>,
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
    /// The language the film was made in, as TMDB stores it - `fr`, `en`.
    ///
    /// The one metadata field playback itself depends on: `vo` in the audio
    /// preferences asks the engine for this tongue, and nothing else can produce
    /// it. Empty when TMDB was never matched to the file, which is not a
    /// language and must not become one - the engine's own default answers then.
    #[serde(default)]
    pub original_language: String,
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
    /// The clip itself, as a data URL. Empty while the server is still building
    /// one, which is the state the interface keeps its still for.
    #[serde(default)]
    pub data_url: String,
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
    /// Fills in the artwork URLs a card and a window-sized frame need, once, as
    /// the film arrives. The alternative is the OSD knowing the server's
    /// address, which is the connection's business and not the interface's.
    ///
    /// Three sizes, because a screen holds three sizes of frame: a card's w780,
    /// a w500 poster, and `hero_url` for the home's hero and the library's
    /// ambient picture. The last is a measurement rather than a taste - on
    /// 29 September 2026 the maintainer sent the player's home photographed on
    /// a 2568-pixel window at 200% scaling, where the w1280 that used to serve
    /// the hero is stretched about 1.9x and reads as a picture too small for its
    /// frame. The interface cannot ask for another size when it draws (it has no
    /// address to build a URL from), so every size it may need is resolved here.
    pub(super) fn resolve_artwork(&mut self, base: &str) {
        self.backdrop_url = image_url(base, &self.metadata.backdrop_path, "w780");
        self.hero_url = image_url(base, &self.metadata.backdrop_path, "original");
        self.poster_url = image_url(base, &self.metadata.poster_path, "w500");
    }
}

impl Series {
    /// The same three sizes as a film: a card, a poster, and the frame a window
    /// draws.
    pub(super) fn resolve_artwork(&mut self, base: &str) {
        self.backdrop_url = image_url(base, &self.metadata.backdrop_path, "w780");
        self.hero_url = image_url(base, &self.metadata.backdrop_path, "original");
        self.poster_url = image_url(base, &self.metadata.poster_path, "w500");
    }
}

impl EpisodeItem {
    pub(super) fn resolve_artwork(&mut self, base: &str) {
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
    /// How long the film is, for the streams that cannot say.
    ///
    /// A converted stream is a pipe with no length, so this is the only end the
    /// clock can have. Zero when the server has never probed the file and TMDB
    /// knew no runtime either.
    #[serde(default)]
    pub duration_seconds: f64,
    /// The rungs this machine can actually produce for this file, as the server
    /// publishes them: height 0 is the file as it is, and every rung below it is
    /// a re-encode. Empty means no encoder is free on this machine, and the
    /// menu then offers no quality at all rather than a row that fails.
    #[serde(default)]
    pub qualities: Vec<VideoQuality>,
    /// What producing a rung would cost, which is the whole difference between
    /// a quality change being free and being a decision.
    #[serde(default)]
    pub transcode: Option<TranscodeInfo>,
}

/// One rung of the quality ladder, as `/info` publishes it.
///
/// `mode` is what playing this rung would do - "direct", "remux" or
/// "transcode" - and it is what the menu says a choice costs, rather than
/// guessing from the height.
#[derive(Clone, Default, serde::Deserialize, serde::Serialize)]
pub struct VideoQuality {
    #[serde(default)]
    pub height: i64,
    #[serde(default)]
    pub mode: String,
}

/// What this machine can encode, and whether a slot is free.
#[derive(Clone, Default, serde::Deserialize, serde::Serialize)]
pub struct TranscodeInfo {
    #[serde(default)]
    pub available: bool,
    /// "hardware" or "software", the word the menu shows beside the heading.
    #[serde(default)]
    pub kind: String,
    /// Every transcoding slot is taken, so the rungs that need one would stall
    /// somebody else's film. The interface greys them instead of letting a
    /// press fail.
    #[serde(default)]
    pub busy: bool,
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
pub(super) struct MovieList {
    #[serde(default)]
    pub(super) movies: Vec<Movie>,
}

#[derive(serde::Deserialize)]
pub(super) struct SeriesList {
    #[serde(default)]
    pub(super) series: Vec<Series>,
}

#[derive(serde::Deserialize)]
pub(super) struct ProfileList {
    #[serde(default)]
    pub(super) profiles: Vec<Profile>,
}
