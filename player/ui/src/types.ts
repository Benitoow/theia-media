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
	pos?: number | null;
	duration?: number | null;
	audioMode?: 'pcm' | 'passthrough' | string;
};

export type Track = {
	id: number;
	type: 'audio' | 'sub' | 'video';
	title?: string;
	lang?: string;
	codec?: string;
	selected?: boolean;
	external?: boolean;
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
	health: { version: string; status?: string };
	profile?: number | null;
	profiles: Profile[];
};

export type DiscoveredServer = { name: string; url: string; version?: string };
