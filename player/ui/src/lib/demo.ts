import type { Episode, Home, HomeRow, Movie, PlayerStatus, Season, Series, SeriesHome, Server, Track } from '../types';

export const isDemoLibrary = import.meta.env.VITE_DEMO_LIBRARY === '1';

type EventHandler = (event: { payload: unknown }) => void;

const movieTitles = [
	'Ashes of the Sun',
	'Midnight on Europa',
	'The City Without Shadow',
	'Neptune Express',
	'The Last Signal',
	'After the Rain',
	'The Watchers of the North',
	'Room 404',
	'A Summer in Kyoto',
	'The Memory of the Waves',
	'Orbit Zero',
	'The Kingdom of Glass',
	'Magnetic Silence',
	'The Birds of Mars',
	'The Road of Giants',
	'We Were Tomorrow',
	'The Scent of the Void',
	'Constellation 9',
	'The Night of the Comets',
	'Beyond the Strait',
	'The Heirs of Fire',
	'A World on Reprieve',
	'Vanishing Line',
	'The Garden of Machines',
	'A Hundred Days of Mist',
	'The Ghosts of Seoul',
	'Last Train to Dawn',
	'The Sound of the Stars',
	'Latitude 66',
	'Those Who Remain',
];

const seriesTitles = [
	'The House of Tides',
	'The Orion Archives',
	'Red District',
	'After Midnight',
	'The Atlas Protocol',
	'Dead Seasons',
	'The Children of the Signal',
	'Polarity',
	'The Glass Frontier',
	'Neon Riviera',
];

const demoDirectors = ['C. Aubert', 'M. Delacroix', 'S. Novak', 'L. Fontaine', 'R. Mercier', 'A. Beaumont'];



export const demoMovies: Movie[] = movieTitles.map((title, index) => {
	const year = 1994 + ((index * 7) % 32);
	return {
		id: index + 1,
		title,
		year,
		metadata: {
			title,
			// The demonstration invents these. Each
			// sentence says it is one: a demo may look plausible, it may not
			// pretend to be a real record.
			overview: `"${title}" is a demonstration film, invented to judge the interface without touching the real library.`,
			release_date: `${year}-05-${String((index % 27) + 1).padStart(2, '0')}`,
			runtime_minutes: 92 + (index % 9) * 7,
			vote_average: Number((6.4 + (index % 17) * 0.2).toFixed(1)),
			director: demoDirectors[index % demoDirectors.length],
		},
		progress:
			index % 7 === 2
				? { position_seconds: 2460 + index * 31, duration_seconds: 7200 + index * 47 }
				: index % 11 === 4
					? { position_seconds: 7800, duration_seconds: 7800, finished: true }
					: { position_seconds: 0, duration_seconds: 6600 + index * 53 },
	};
});

function seasonsFor(seriesId: number): Season[] {
	const count = 2 + (seriesId % 3);
	return Array.from({ length: count }, (_, index) => ({
		id: seriesId * 100 + index + 1,
		series_id: seriesId,
		season_number: index + 1,
		metadata: { name: `Season ${index + 1}`, episode_count: 8 + ((seriesId + index) % 5) },
	}));
}

export const demoSeries: Series[] = seriesTitles.map((title, index) => ({
	id: index + 101,
	title,
	year: 2012 + ((index * 3) % 14),
	metadata: { name: title },
	seasons: seasonsFor(index + 101),
}));

function seasonFor(seriesId: number, seasonNumber: number): Season {
	const series = demoSeries.find((entry) => entry.id === seriesId) ?? demoSeries[0];
	const season = series.seasons?.find((entry) => entry.season_number === seasonNumber) ?? series.seasons?.[0];
	const count = season?.metadata?.episode_count ?? 8;
	const episodes: Episode[] = Array.from({ length: count }, (_, index) => {
		const episodeNumber = index + 1;
		const title = ['The Passage', 'Reverse Shot', 'The Breach', 'The Echo', 'Blind Spot', 'The Crossing', 'The Pact', 'Return at Dawn'][index % 8];
		return {
			id: seriesId * 10_000 + seasonNumber * 100 + episodeNumber,
			series_id: seriesId,
			series_title: series.metadata?.name ?? series.title,
			season_number: seasonNumber,
			episode_numbers: [episodeNumber],
			episode_metadata: [
				{
					id: seriesId * 10_000 + seasonNumber * 100 + episodeNumber,
					episode_number: episodeNumber,
					local_title: title,
					metadata: { name: title, runtime_minutes: 42 + (episodeNumber % 4) * 5 },
				},
			],
			progress:
				episodeNumber === 3
					? { position_seconds: 1040, duration_seconds: 2940 }
					: { position_seconds: 0, duration_seconds: 2700 },
		};
	});
	return { ...(season ?? seasonsFor(seriesId)[0]), episodes };
}

/// The demonstration's home screen: the same shape the server sends, built
/// from the fake library. Films already under way lead, and the hero is the
/// first of them, so what gets judged is the resume composition - the state a
/// real household is in most evenings.
export function demoHome(): Home {
	const continuing = demoMovies.filter(
		(movie) => (movie.progress?.position_seconds ?? 0) > 0 && !movie.progress?.finished
	);
	const recent = demoMovies.filter((_, index) => index % 3 === 1).slice(0, 10);
	const topRated = [...demoMovies]
		.sort((a, b) => (b.metadata?.vote_average ?? 0) - (a.metadata?.vote_average ?? 0))
		.slice(0, 10);
	const rows = [
		continuing.length ? { kind: 'continue', movies: continuing } : null,
		recent.length ? { kind: 'recent', movies: recent } : null,
		topRated.length ? { kind: 'top_rated', movies: topRated } : null,
		{ kind: 'tonight', movies: [demoMovies[11]] },
	].filter((row): row is HomeRow => row !== null);
	return {
		hero: continuing[0] ?? demoMovies[0],
		hero_kind: continuing.length ? 'resume' : 'featured',
		rows,
		total: demoMovies.length,
	};
}

export function demoSeriesHome(): SeriesHome {
	const continue_watching: Episode[] = [];
	for (const series of demoSeries) {
		const episode = seasonFor(series.id, 1).episodes?.find((entry) => entry.episode_numbers?.[0] === 3);
		if (episode) continue_watching.push(episode);
		if (continue_watching.length >= 8) break;
	}
	return { continue_watching, recent_series: demoSeries.slice(0, 6) };
}

const demoServer: Server = {
	url: 'http://demo.theia',
	health: { version: '3.3.0 \u00b7 demo', status: 'ok' },
	profile: 1,
	profiles: [
		{ id: 1, name: 'Alex', is_default: true, has_avatar: false, avatar_version: 0 },
		{ id: 2, name: 'Lina', is_default: false, has_avatar: false, avatar_version: 0 },
		{ id: 3, name: 'Guests', is_default: false, has_avatar: false, avatar_version: 0 },
		{ id: 4, name: 'Kids', is_default: false, has_avatar: false, avatar_version: 0 },
	],
};

let demoUpdate = {
	state: 'available',
	current_version: '3.3.0',
	latest_version: '3.3.1',
	available: true,
	message: 'Theia 3.3.1 is ready to install.',
};

const demoTracks: Track[] = [
	{ id: 1, type: 'audio', title: 'French 5.1', lang: 'fra', codec: 'eac3', selected: true },
	{ id: 2, type: 'audio', title: 'Original version', lang: 'eng', codec: 'truehd' },
	{ id: 3, type: 'sub', title: 'Forced French', lang: 'fra', codec: 'subrip', selected: true, external: true },
	{ id: 4, type: 'sub', title: 'English', lang: 'eng', codec: 'subrip', external: true },
];

let demoStatus: PlayerStatus = { ready: true, media: null, pause: false, mute: false, pos: null, duration: null };
const handlers = new Map<string, Set<EventHandler>>();
let clock: number | null = null;

function emit(event: string, payload: unknown) {
	for (const handler of handlers.get(event) ?? []) handler({ payload });
}

function emitStatus() {
	emit('player-status', JSON.stringify(demoStatus));
}

function startClock() {
	if (clock !== null) return;
	clock = window.setInterval(() => {
		if (!demoStatus.media || demoStatus.pause) return;
		demoStatus = { ...demoStatus, pos: Math.min(Number(demoStatus.duration) || 0, (Number(demoStatus.pos) || 0) + 1) };
		emitStatus();
	}, 1000);
}

export async function demoListen(event: string, handler: EventHandler): Promise<() => void> {
	const set = handlers.get(event) ?? new Set<EventHandler>();
	set.add(handler);
	handlers.set(event, set);
	if (event === 'player-status') {
		queueMicrotask(emitStatus);
		startClock();
	}
	return () => set.delete(handler);
}

export async function demoInvoke<T>(command: string, args?: Record<string, unknown>): Promise<T> {
	let result: unknown;
	switch (command) {
		case 'player_local_server':
			result = demoServer.url;
			break;
		case 'player_connect':
			result = JSON.stringify(demoServer);
			break;
		case 'player_set_profile':
			demoServer.profile = Number(args?.id);
			result = null;
			break;
		case 'player_profile_rename': {
			const profile = demoServer.profiles.find((entry) => entry.id === Number(args?.id));
			if (!profile) throw new Error('profile not found');
			profile.name = String(args?.name || '').trim();
			result = JSON.stringify(profile);
			break;
		}
		case 'player_profile_set_avatar': {
			const profile = demoServer.profiles.find((entry) => entry.id === Number(args?.id));
			if (!profile) throw new Error('profile not found');
			if (profile.avatar_url?.startsWith('blob:')) URL.revokeObjectURL(profile.avatar_url);
			const bytes = Uint8Array.from((args?.data as number[]) ?? []);
			profile.has_avatar = true;
			profile.avatar_version = (profile.avatar_version ?? 0) + 1;
			profile.avatar_url = URL.createObjectURL(new Blob([bytes], { type: String(args?.contentType || 'application/octet-stream') }));
			result = JSON.stringify(profile);
			break;
		}
		case 'player_profile_clear_avatar': {
			const profile = demoServer.profiles.find((entry) => entry.id === Number(args?.id));
			if (!profile) throw new Error('profile not found');
			if (profile.avatar_url?.startsWith('blob:')) URL.revokeObjectURL(profile.avatar_url);
			profile.has_avatar = false;
			profile.avatar_url = undefined;
			profile.avatar_version = (profile.avatar_version ?? 0) + 1;
			result = JSON.stringify(profile);
			break;
		}
		case 'player_update_status':
		case 'player_update_check':
			result = JSON.stringify(demoUpdate);
			break;
		case 'player_update_apply':
			demoUpdate = { ...demoUpdate, state: 'ready', available: false, message: 'Update downloaded. Restart Theia to finish.' };
			result = JSON.stringify(demoUpdate);
			break;
		case 'player_discover':
			result = JSON.stringify([{ name: 'Demo library', url: demoServer.url, version: demoServer.health.version }]);
			break;
		case 'player_home':
			result = JSON.stringify(demoHome());
			break;
		case 'player_series_home':
			result = JSON.stringify(demoSeriesHome());
			break;
		case 'player_library':
			result = JSON.stringify(demoMovies);
			break;
		case 'player_series':
			result = JSON.stringify(demoSeries);
			break;
		case 'player_series_detail': {
			const series = demoSeries.find((entry) => entry.id === Number(args?.id)) ?? demoSeries[0];
			result = JSON.stringify(series);
			break;
		}
		case 'player_season':
			result = JSON.stringify(seasonFor(Number(args?.seriesId), Number(args?.seasonNumber)));
			break;
		case 'player_play': {
			const movie = demoMovies.find((entry) => entry.id === Number(args?.id)) ?? demoMovies[0];
			demoStatus = { ready: true, media: `demo://movie/${movie.id}`, title: movie.metadata?.title ?? movie.title, pause: false, mute: false, pos: movie.progress?.position_seconds || 73, duration: movie.progress?.duration_seconds || 7200 };
			emitStatus();
			result = demoStatus.media;
			break;
		}
		case 'player_play_episode': {
			const id = Number(args?.id);
			const seriesId = Math.floor(id / 10_000);
			const seasonNumber = Math.floor((id % 10_000) / 100);
			const episode = seasonFor(seriesId, seasonNumber).episodes?.find((entry) => entry.id === id);
			demoStatus = { ready: true, media: `demo://episode/${id}`, title: `${episode?.series_title ?? 'Series'} \u00b7 S${String(seasonNumber).padStart(2, '0')}E${String(episode?.episode_numbers?.[0] ?? 1).padStart(2, '0')}`, pause: false, mute: false, pos: episode?.progress?.position_seconds || 41, duration: episode?.progress?.duration_seconds || 2700 };
			emitStatus();
			result = demoStatus.media;
			break;
		}
		case 'player_stop':
			demoStatus = { ...demoStatus, media: null, title: null, pos: null, duration: null, pause: false };
			emitStatus();
			result = null;
			break;
		case 'player_toggle_pause':
			demoStatus = { ...demoStatus, pause: !demoStatus.pause };
			emitStatus();
			result = null;
			break;
		case 'player_seek':
			demoStatus = { ...demoStatus, pos: Math.max(0, Math.min(Number(demoStatus.duration) || 0, (Number(demoStatus.pos) || 0) + Number(args?.seconds || 0))) };
			emitStatus();
			result = null;
			break;
		case 'player_set_muted':
			demoStatus = { ...demoStatus, mute: Boolean(args?.muted) };
			emitStatus();
			result = null;
			break;
		case 'player_tracks':
			result = JSON.stringify(demoTracks);
			break;
		case 'player_set_track':
			result = null;
			break;
		default:
			throw new Error(`unsupported demo command: ${command}`);
	}
	return result as T;
}