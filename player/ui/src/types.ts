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

export type Server = {
	url: string;
	health: { version: string; status?: string; language?: string };
	profile?: number | null;
	profiles: Profile[];
};

export type DiscoveredServer = { name: string; url: string; version?: string };
