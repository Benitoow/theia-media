export type Progress = {
	position_seconds?: number;
	duration_seconds?: number;
	finished?: boolean;
};

/**
 * What a card or a hero may state about a film. Deliberately a subset of the
 * server's record: the player parses what a screen actually says, never the
 * whole API - a mirror of it would be a second copy to keep in step.
 */
export type MovieMetadata = {
	title?: string;
	tmdb_title?: string;
	tmdb_name?: string;
	overview?: string;
	release_date?: string;
	first_air_date?: string;
	runtime_minutes?: number;
	vote_average?: number;
	director?: string;
	backdrop_path?: string;
	poster_path?: string;
	still_path?: string;
};

export type Movie = {
	id: number;
	title: string;
	year?: number;
	metadata?: MovieMetadata;
	backdrop_url?: string;
	poster_url?: string;
	progress?: Progress;
};

export type Series = {
	id: number;
	title: string;
	year?: number;
	metadata?: {
		name?: string;
		tmdb_title?: string;
		tmdb_name?: string;
		release_date?: string;
		first_air_date?: string;
		backdrop_path?: string;
		poster_path?: string;
		// The server sends these on a series; the preview shows them when they
		// are there and stays quiet when they are not.
		overview?: string;
		tagline?: string;
	};
	backdrop_url?: string;
	poster_url?: string;
	seasons?: Season[];
};

export type Season = {
	id: number;
	series_id: number;
	season_number: number;
	metadata?: { name?: string; episode_count?: number };
	episodes?: Episode[];
};

export type Episode = {
	id: number;
	series_id: number;
	series_title: string;
	season_number: number;
	episode_numbers?: number[];
	episode_metadata?: Array<{
		id: number;
		episode_number: number;
		local_title?: string;
		metadata?: { name?: string; still_path?: string; runtime_minutes?: number; overview?: string };
	}>;
	still_url?: string;
	progress?: Progress;
};

/** One short row of the home screen; `kind` is the server's word for it. */
export type HomeRow = { kind: string; movies: Movie[] };

/**
 * The home screen: one hero and the short rows, exactly the shape the server
 * builds. `hero_kind` says whether the hero is `resume` (a film under way) or
 * `featured` (nothing is, so it is simply on) - which changes what the screen
 * offers, not what it is.
 */
export type Home = {
	hero?: Movie | null;
	hero_kind?: string;
	rows: HomeRow[];
	total?: number;
};

/** The series half of the home screen: a separate request, a separate shape. */
export type SeriesHome = {
	continue_watching: Episode[];
	recent_series: Series[];
};

/** Which audio track the engine is asked for. `auto` is the file's own answer,
 *  `vf` is French, and `vo` follows the original language of the title. */
export type AudioLanguage = 'auto' | 'vf' | 'vo';

/** Which subtitle track the engine is asked for. `none` keeps them off until
 *  somebody asks for one in the menu, which is not the same as absent. */
export type SubtitleLanguage = 'auto' | 'none' | 'fr' | 'en';

/** What detaches the letters from the picture. The engine has no borderless
 *  style of its own, so `none` is a transparent box - see the note in Rust. */
export type SubtitleOutline = 'shadow' | 'outline' | 'none';

/** Three families the engine can resolve by name. The fonts this interface
 *  ships are web fonts, which the engine cannot load, so it offers none of
 *  them: "standard" is the engine's own sans. */
export type SubtitleFont = 'standard' | 'serif' | 'mono';

/**
 * How subtitles are drawn, as the settings sheet decides it.
 *
 * The two sizes are pixels on a picture 1080 pixels tall, and the engine's own
 * unit is a scaled pixel - the player converts once, in Rust, and the preview
 * in this interface is drawn at exactly the same proportion so that what the
 * sheet shows and what the film does cannot drift apart.
 */
export type SubtitleStyle = {
	sizePx: number;
	heightPx: number;
	/** `#RRGGBB`; the alpha the engine needs is added where the value is used. */
	colour: string;
	outline: SubtitleOutline;
	/** `none`, or a `#RRGGBB` band behind the text. */
	background: string;
	font: SubtitleFont;
	bold: boolean;
};

/**
 * What the settings sheet decides about playing, and what the engine is told
 * through `player_set_playback`.
 *
 * The shape is a contract with Rust: the fields are camelCase on the wire and
 * the defaults below are the ones `PlaybackPreferences::default` carries, so a
 * preferences object that never reaches the engine still means the same thing
 * there.
 */
export type PlaybackPreferences = {
	autoPlayNext: boolean;
	audioLanguage: AudioLanguage;
	subtitleLanguage: SubtitleLanguage;
	subtitleStyle: SubtitleStyle;
};

/** The look a subtitle has before anybody changes anything: the engine's own
 *  size, the reference the sheet was built from, and no band. 36 and 120 are
 *  pixels at 1080, which is where the two sliders start. */
export const PLAYBACK_DEFAULTS: PlaybackPreferences = {
	autoPlayNext: true,
	audioLanguage: 'auto',
	subtitleLanguage: 'auto',
	subtitleStyle: {
		sizePx: 36,
		heightPx: 120,
		colour: '#EDE7DC',
		outline: 'shadow',
		background: 'none',
		font: 'standard',
		bold: false,
	},
};

/**
 * What the interface measures about itself, and what the engine measures about
 * the picture. Both are diagnostics: the OSD shows none of them, `--diagnostics`
 * prints them, and a claim of fluidity without them is an opinion.
 */
export type FluidStats = {
	/** The rate mpv is actually presenting, not the file's own. */
	fps?: number | null;
	/** Frames the video output and the decoder had to throw away. */
	drops?: number;
	decoderDrops?: number;
	/** The worst frame interval seen while somebody was doing something, in ms. */
	osdWorstFrameMs?: number | null;
	/** How many frames in the window took longer than two frames at 60 Hz. */
	osdSlowFrames?: number | null;
	/** How many frames the sampler drew: zero with a zero worst frame means the
	 *  instrument never ran, which is not the same answer as "it was smooth". */
	osdFrames?: number | null;
	/** How many window-resize events arrived: one drag is not one event. */
	osdResizeEvents?: number | null;
};

export type PlayerStatus = {
	ready?: boolean;
	media?: string | null;
	title?: string | null;
	pause?: boolean;
	mute?: boolean;
	/** The volume as a fraction of full scale, which is the unit
	 * `player_set_volume` accepts: the engine's own percent is converted once,
	 * in Rust, so no interface has to know about it. */
	volume?: number;
	pos?: number | null;
	duration?: number | null;
	/** Which rung of the quality ladder is loaded, `null` being the file itself.
	 * Session state on the Rust side rather than an mpv property: the rung is a
	 * fact about the address the film was fetched from. */
	quality?: number | null;
	audioMode?: 'pcm' | 'passthrough' | string;
};

/** One entry of mpv's `track-list`, passed through as it answers.
 *
 * The fields past `external` are the ones the menu reads to say *what* a track
 * is: mpv names a channel layout `demux-channels` ("7.1", "stereo", and
 * "unknown6" when the container wrote none), and a sidecar arrives as
 * `external`. */
export type Track = {
	id: number;
	type: 'audio' | 'sub' | 'video';
	title?: string;
	lang?: string;
	codec?: string;
	selected?: boolean;
	external?: boolean;
	forced?: boolean;
	default?: boolean;
	'audio-channels'?: number;
	'demux-channels'?: string;
	'demux-channel-count'?: number;
};

/** One rung of the server's quality ladder, as `/info` publishes it.
 *
 * `mode` is what playing it would do - "direct", "remux" or "transcode" - and
 * the menu says what a choice costs from that word rather than from the height,
 * because a rung is not always an encode. */
export type VideoQuality = { height: number; mode?: string };

/** What this machine can encode, and whether a slot is free. */
export type TranscodeInfo = { available?: boolean; kind?: string; busy?: boolean };

/** The answer to `player_qualities`: which rung is loaded, and what the others
 * would cost. `null` means the server offered no ladder at all - no encoder, or
 * a file it cannot measure - and the menu then has no quality tab to draw. */
export type QualityLadder = {
	current: number | null;
	qualities: VideoQuality[];
	transcode: TranscodeInfo | null;
};

/** The words a track's label is built from, per language. The shape of
 * `vocabulary` in `lib/catalogues.js`, which cannot carry a type of its own. */
export type TrackVocabulary = {
	languages: Record<string, string>;
	channels: Record<string, string>;
	channelCount: string;
	codecs: Record<string, string>;
	commentary: string;
	forced: string;
	external: string;
	unnamedAudio: string;
	unnamedSubtitle: string;
	/** The words a quality rung is said with. `qualityHeight` carries an {n}
	 * for the same reason `channelCount` does: "720p" is a number and a letter,
	 * and every language writes it that way. */
	qualityHeight: string;
	qualityReencoded: string;
	qualityHardware: string;
	qualitySoftware: string;
};

export type Profile = {
	id: number;
	name?: string;
	is_default: boolean;
	has_avatar?: boolean;
	avatar_version?: number;
	avatar_url?: string;
};

export type UpdateStatus = {
	state: 'idle' | 'checking' | 'available' | 'downloading' | 'ready' | 'deferred' | 'failed' | 'unsupported' | string;
	reason?: string;
	current_version: string;
	latest_version?: string;
	available: boolean;
	message?: string;
	release_url?: string;
	checked_at?: string;
};

export type Watched = {
	started: number;
	finished: number;
	seconds: number;
};

export type SeriesWatch = {
	id: number;
	title: string;
	/** The TMDB path, and the URL this server answers for it: the ranking is
	 *  drawn as the series, so it needs the picture and not only the name. */
	poster_path?: string;
	poster_url?: string;
	episodes: number;
	finished: number;
	total: number;
	seconds: number;
};

/**
 * What a profile has watched, as the server counted it.
 *
 * The rules are the server's: a film is watched when it has under two minutes
 * or five per cent left, a series when every one of its episodes is. The player
 * asks and prints; a player that counted for itself would be a second
 * definition of "watched" to keep in step with the first.
 */
export type ThisMonth = {
	movies: number;
	episodes: number;
	seconds: number;
};

export type WatchStats = {
	movies: Watched;
	series: Watched;
	episodes: Watched;
	/** What was watched this calendar month, by the server's own clock. */
	month: ThisMonth;
	top_series: SeriesWatch[];
};

export type Server = {
	url: string;
	health: { version: string; status?: string; language?: string };
	profile?: number | null;
	profiles: Profile[];
};

export type DiscoveredServer = { name: string; url: string; version?: string };
