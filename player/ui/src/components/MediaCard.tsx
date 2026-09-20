import { Play, Tv } from 'lucide-react';
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
	const previewed = useRef(false);
	const ticks = useRef(0);
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
	 * Asked once per card and then remembered, asked again while the pointer
	 * stays because the server answers `building` while it makes one, and
	 * stopped the moment the pointer leaves - the same "ask again in a while is
	 * the whole protocol" the seek strip uses. A series is not asked at all: a
	 * series is not a file, so there is nothing to sample and its card keeps its
	 * still.
	 */
	const askForClip = () => {
		if (reducedMotion || kind === 'series' || previewed.current) return;
		previewed.current = true;
		const ask = async () => {
			ticks.current += 1;
			try {
				const answer = JSON.parse(await invoke<string>('player_preview', { kind, id: item.id })) as { state?: string; clip_url?: string };
				if (answer.state === 'ready' && answer.clip_url) {
					setClip(answer.clip_url);
					return;
				}
			} catch {
				// No server, no ffmpeg, or nothing to sample: the still is the
				// answer, and it is already on screen.
				return;
			}
			if (ticks.current < 5 && hovered) timer.current = window.setTimeout(ask, 3000);
		};
		void ask();
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
				onPointerEnter={() => {
					setHovered(true);
					askForClip();
				}}
				onPointerLeave={() => {
					setHovered(false);
					stopAsking();
				}}
				onFocus={() => {
					setHovered(true);
					askForClip();
				}}
				onBlur={() => {
					setHovered(false);
					stopAsking();
				}}
				aria-label={`${view.action} ${view.legend} ${view.title}`.trim()}
			>
				<span className={cn('film-art', artIsFallback && 'film-art--empty')}>
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
							// rather than a black rectangle where a film used to be.
							onError={() => setClip('')}
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
