import { Play, Tv } from 'lucide-react';
import type { CSSProperties } from 'react';
import { useEffect, useMemo, useRef, useState } from 'react';

import { cn } from '../lib/utils';
import { artworkCandidates, displayTitle, displayYear, imageURL } from '../lib/tmdb';
import notFoundArt from '../assets/media-not-found.png';
import type { Episode, Movie, Series } from '../types';

// The same bridge App.tsx uses, and the same shape: a missing one is a thrown
// error rather than a silent undefined, because every caller here is inside a
// catch that decides what the card shows instead.
const invoke = async <T,>(command: string, args?: Record<string, unknown>): Promise<T> => {
	const call = window.__TAURI__?.core?.invoke;
	if (!call) throw new Error('no tauri bridge');
	return call<T>(command, args);
};

type CommonProps = {
	onOpen: (id: number) => void;
	resumeLabel: string;
	actionLabel: string;
	kindLabel: string;
	reducedMotion: boolean;
	/**
	 * Overrides the card's own heading. The home screen's episode rows use it
	 * so a card reads as the series it belongs to rather than as episode 4.
	 */
	heading?: string;
};

type Props =
	| (CommonProps & { kind: 'movie'; item: Movie })
	| (CommonProps & { kind: 'series'; item: Series })
	| (CommonProps & { kind: 'episode'; item: Episode });

export function MediaCard({ kind, item, onOpen, resumeLabel, actionLabel, kindLabel, reducedMotion, heading }: Props) {
	const [failed, setFailed] = useState<string[]>([]);
	const [clip, setClip] = useState('');
	const [hovered, setHovered] = useState(false);
	// Two names for one fact on purpose. The state drives what is drawn - only a
	// card under the pointer plays - and the ref is what the retry reads, because
	// the closure a timer was armed in still holds the value from the moment it
	// was armed. That stale `false` is what made the first version never retry,
	// and it is the whole reason a two-hour film's clip never appeared.
	const hovering = useRef(false);
	const asking = useRef(false);
	const timer = useRef<number | null>(null);
	const view = useMemo(() => describe(kind, item, actionLabel, kindLabel, heading), [kind, item, actionLabel, kindLabel, heading]);
	const artwork = view.art.find((url) => !failed.includes(url));
	// The not-found plate, never the demo art: demo items always carry their
	// own artwork, so this only answers when TMDB/IMDb provided nothing or
	// every candidate failed to load.
	const artSrc = artwork ?? notFoundArt;
	const artIsFallback = !artwork;
	const progress = view.finished || !view.duration || !view.position ? 0 : Math.min(100, (view.position / view.duration) * 100);

	const stopAsking = () => {
		if (timer.current !== null) window.clearTimeout(timer.current);
		timer.current = null;
	};

	/**
	 * Asks the server for six seconds of this film.
	 *
	 * One ask per hover, and the asks are repeated **while the pointer stays**
	 * because the server answers `building` while it builds - six seconds for a
	 * two-hour remux, measured. The moment the pointer leaves, the timer stops
	 * and the card is free to ask again on the next hover; only a clip already
	 * in hand stops a new question. A series is asked like everything else: it is
	 * not a file itself, so the server samples the file playback would reach
	 * first, and the answer is cached against that file.
	 */
	const askForClip = () => {
		if (reducedMotion || clip || asking.current) return;
		asking.current = true;
		let ticks = 0;
		const ask = async () => {
			ticks += 1;
			try {
				const answer = JSON.parse(await invoke<string>('player_preview', { kind, id: item.id })) as { state?: string; data_url?: string };
				if (answer.state === 'ready' && answer.data_url) {
					setClip(answer.data_url);
					return;
				}
			} catch (error) {
				// No server, no ffmpeg, or nothing to sample: the still is the
				// answer, and it is already on screen. The reason goes to the
				// player's own output, because a card that fails silently is
				// indistinguishable from a card that was never asked - which is
				// exactly how "the preview does nothing" arrived, twice.
				void invoke('player_log', { message: `preview ${kind} ${item.id}: ${String(error)}` }).catch(() => {});
				asking.current = false;
				return;
			}
			if (ticks < 8 && hovering.current) {
				timer.current = window.setTimeout(ask, 2000);
				return;
			}
			// The pointer left before the server was done. The next hover asks
			// again, and finds it built.
			asking.current = false;
		};
		void ask();
	};

	const enter = () => {
		hovering.current = true;
		setHovered(true);
		askForClip();
	};

	const leave = () => {
		hovering.current = false;
		setHovered(false);
		stopAsking();
	};

	useEffect(() => stopAsking, []);

	const activate = () => {
		stopAsking();
		onOpen(item.id);
	};

	return (
		<li className="media-card">
			<button
				className="film"
				onClick={activate}
				onPointerEnter={enter}
				onPointerLeave={leave}
				onFocus={enter}
				onBlur={leave}
				aria-label={`${view.action} ${view.legend} ${view.title}`.trim()}
			>
				<span
					className={cn('film-art', artIsFallback && 'film-art--empty')}
					// The frame's own picture, blurred behind whatever is drawn in
					// it: a poster is contained rather than cropped, and containing a
					// 2:3 image in a 16/9 frame is what made the black bands.
					style={{ '--card-art': `url("${artSrc}")` } as CSSProperties}
				>
					<img
						src={artSrc}
						alt=""
						crossOrigin="anonymous"
						loading="lazy"
						decoding="async"
						onError={artwork ? () => setFailed((current) => [...current, artwork]) : undefined}
						className={cn(artwork != null && artwork === view.poster && 'film-art--poster')}
					/>
					{hovered && clip && (
						<video
							className="film-clip"
							src={clip}
							poster={artSrc}
							muted
							loop
							autoPlay
							playsInline
							// A clip that will not decode leaves the artwork showing
							// rather than a black rectangle where a film used to be,
							// and says so in the player's own output: the card
							// swallows a failed preview by design, so without this
							// line a clip that never loads and a clip that was never
							// asked for look identical from outside - which is
							// exactly how "the preview does nothing" arrived.
							onError={(event) => {
								const media = (event.currentTarget as HTMLVideoElement).error;
								void invoke('player_log', {
									message: `a card preview would not play: code=${media?.code} ${media?.message ?? ''}`,
								}).catch(() => {});
								setClip('');
							}}
						/>
					)}
					<span className="film-mark" aria-hidden="true">
						{kind === 'series' ? <Tv size={20} /> : <Play size={18} fill="currentColor" />}
					</span>
					{progress > 0 && <span className="film-watched" style={{ width: `${progress}%` }} />}
				</span>
				<span className="film-name">{view.title}</span>
				<span className="film-legend label">
					{view.legend}
					{view.position >= 30 && !view.finished ? ` · ${resumeLabel} ${Math.max(1, Math.floor(view.position / 60))} min` : ''}
				</span>
			</button>
		</li>
	);
}

function describe(kind: Props['kind'], item: Movie | Series | Episode, actionLabel: string, kindLabel: string, heading?: string) {
	if (kind === 'episode') {
		const episode = item as Episode;
		const record = episode.episode_metadata?.[0];
		const numbers = episode.episode_numbers ?? [];
		const code = `S${String(episode.season_number ?? 0).padStart(2, '0')}${numbers.map((number) => `E${String(number).padStart(2, '0')}`).join('')}`;
		const runtime = record?.metadata?.runtime_minutes ?? 0;
		return {
			title: heading || record?.metadata?.name || record?.local_title || code,
			legend: `${code}${runtime ? ` · ${runtime} min` : ''}`,
			kind: kindLabel,
			art: [episode.still_url, imageURL(record?.metadata?.still_path, 'w780')].filter((url): url is string => Boolean(url)),
			poster: undefined as string | undefined,
			fallback: code,
			action: actionLabel,
			position: episode.progress?.position_seconds ?? 0,
			duration: episode.progress?.duration_seconds || runtime * 60,
			finished: episode.progress?.finished ?? false,
		};
	}
	if (kind === 'series') {
		const series = item as Series;
		const title = displayTitle(series);
		const year = displayYear(series);
		// Seasons and episodes only when the server sent them: the grid's series
		// records carry no season list, and a count nobody measured is the fake
		// metadata section 6.2.1 refuses.
		const seasons = series.seasons?.length ?? 0;
		const episodes = (series.seasons ?? []).reduce((total, season) => total + (season.metadata?.episode_count ?? 0), 0);
		const counts = seasons ? ` · ${seasons} ${seasons === 1 ? 'saison' : 'saisons'}${episodes ? ` · ${episodes} épisode${episodes === 1 ? '' : 's'}` : ''}` : '';
		return {
			title,
			legend: `${kindLabel}${year ? ` · ${year}` : ''}${counts}`,
			kind: kindLabel,
			art: artworkCandidates(series),
			poster: series.poster_url ?? imageURL(series.metadata?.poster_path, 'w342') ?? undefined,
			fallback: title.slice(0, 1).toUpperCase(),
			action: actionLabel,
			position: 0,
			duration: 0,
			finished: false,
		};
	}
	const movie = item as Movie;
	const title = displayTitle(movie);
	const year = displayYear(movie);
	return {
		title,
		legend: year ? String(year) : '',
		kind: kindLabel,
		art: artworkCandidates(movie),
		poster: movie.poster_url ?? imageURL(movie.metadata?.poster_path, 'w342') ?? undefined,
		fallback: title.slice(0, 1).toUpperCase(),
		action: actionLabel,
		position: movie.progress?.position_seconds ?? 0,
		duration: movie.progress?.duration_seconds ?? 0,
		finished: movie.progress?.finished ?? false,
	};
}
