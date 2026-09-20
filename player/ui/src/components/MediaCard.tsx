import { AnimatePresence, motion } from 'motion/react';
import { Play, Tv } from 'lucide-react';
import { useEffect, useMemo, useRef, useState } from 'react';
import { createPortal } from 'react-dom';

import { cn } from '../lib/utils';
import { artworkCandidates, displayTitle, displayYear, imageURL } from '../lib/tmdb';
import notFoundArt from '../assets/media-not-found.png';
import type { Episode, Movie, Series } from '../types';
import { Button } from './ui/button';

type CommonProps = {
	onOpen: (id: number) => void;
	resumeLabel: string;
	actionLabel: string;
	kindLabel: string;
	reducedMotion: boolean;
	/**
	 * Overrides the card's own heading. The home screen's episode rows use it
	 * to name the series: a row of cards titled "S01E03" would make the
	 * viewer open one to learn what they are, and the series is what
	 * identifies the card there.
	 */
	heading?: string;
};

type Props =
	| (CommonProps & { kind: 'movie'; item: Movie })
	| (CommonProps & { kind: 'series'; item: Series })
	| (CommonProps & { kind: 'episode'; item: Episode });

type PreviewPosition = { left: number; top: number; width: number };

export function MediaCard({ kind, item, onOpen, resumeLabel, actionLabel, kindLabel, reducedMotion, heading }: Props) {
	const [failed, setFailed] = useState<string[]>([]);
	const [open, setOpen] = useState(false);
	const [position, setPosition] = useState<PreviewPosition | null>(null);
	const trigger = useRef<HTMLButtonElement>(null);
	const preview = useRef<HTMLDivElement>(null);
	const openTimer = useRef<number | null>(null);
	const closeTimer = useRef<number | null>(null);
	const view = useMemo(() => describe(kind, item, actionLabel, kindLabel, heading), [kind, item, actionLabel, kindLabel, heading]);
	const artwork = view.art.find((url) => !failed.includes(url));
	// The not-found plate, never the demo art: demo items always carry their
	// own artwork, so this only answers when TMDB/IMDb provided nothing or
	// every candidate failed to load.
	const artSrc = artwork ?? notFoundArt;
	const artIsFallback = !artwork;
	const progress = view.finished || !view.duration || !view.position ? 0 : Math.min(100, (view.position / view.duration) * 100);
	const previewId = `media-preview-${kind}-${item.id}`;

	const clearTimers = () => {
		if (openTimer.current !== null) window.clearTimeout(openTimer.current);
		if (closeTimer.current !== null) window.clearTimeout(closeTimer.current);
		openTimer.current = null;
		closeTimer.current = null;
	};
	const measure = () => {
		const rect = trigger.current?.getBoundingClientRect();
		if (!rect) return;
		const margin = 14;
		const width = Math.min(520, Math.max(400, rect.width * 1.8), window.innerWidth - margin * 2);
		const height = Math.min(width * 0.56, 292);
		const left = Math.max(margin, Math.min(window.innerWidth - width - margin, rect.left + rect.width / 2 - width / 2));
		const top = Math.max(64, Math.min(window.innerHeight - height - margin, rect.top - Math.max(10, (height - rect.height) / 2)));
		setPosition({ left, top, width });
	};
	const reveal = (immediate = false) => {
		clearTimers();
		measure();
		openTimer.current = window.setTimeout(() => setOpen(true), immediate || reducedMotion ? 0 : 190);
	};
	const conceal = (immediate = false) => {
		if (openTimer.current !== null) window.clearTimeout(openTimer.current);
		openTimer.current = null;
		closeTimer.current = window.setTimeout(() => setOpen(false), immediate ? 0 : 110);
	};

	useEffect(() => () => clearTimers(), []);
	useEffect(() => {
		if (!open) return;
		const onResize = () => measure();
		const onKey = (event: KeyboardEvent) => {
			if (event.key !== 'Escape') return;
			event.preventDefault();
			setOpen(false);
			trigger.current?.focus();
		};
		window.addEventListener('resize', onResize);
		window.addEventListener('keydown', onKey);
		return () => {
			window.removeEventListener('resize', onResize);
			window.removeEventListener('keydown', onKey);
		};
	}, [open]);

	const activate = () => {
		setOpen(false);
		onOpen(item.id);
	};

	return (
		<li className="media-card">
			<button
				ref={trigger}
				className="film"
				onClick={activate}
				onPointerEnter={() => reveal(false)}
				onPointerLeave={() => conceal(false)}
				onFocus={() => reveal(true)}
				onBlur={(event) => {
					if (preview.current?.contains(event.relatedTarget as Node | null)) return;
					conceal(false);
				}}
				aria-label={`${view.action} ${view.legend} ${view.title}`.trim()}
				aria-expanded={open}
				aria-controls={previewId}
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

			{createPortal(
				<AnimatePresence>
					{open && position && (
						<motion.div
							ref={preview}
							id={previewId}
							className="media-preview"
							style={{ left: position.left, top: position.top, width: position.width }}
							role="group"
							aria-label={`${view.title} — ${view.legend}`}
							initial={{ opacity: 0, y: reducedMotion ? 0 : 10 }}
							animate={{ opacity: 1, y: 0 }}
							exit={{ opacity: 0, y: reducedMotion ? 0 : 6 }}
							transition={{ duration: reducedMotion ? 0 : 0.2, ease: [0.16, 1, 0.3, 1] }}
							onPointerEnter={clearTimers}
							onPointerLeave={() => conceal(false)}
							onBlur={(event) => {
								if (preview.current?.contains(event.relatedTarget as Node | null) || event.relatedTarget === trigger.current) return;
								conceal(false);
							}}
						>
							<div className="media-preview-art" aria-hidden="true">
								<img src={artSrc} alt="" crossOrigin="anonymous" className={cn(artwork != null && artwork === view.poster && 'media-preview-art--poster')} />
							</div>
							<div className="media-preview-shade" aria-hidden="true" />
							<div className="media-preview-content">
								<span className="media-preview-kind label">{view.kind}</span>
								<h2>{view.title}</h2>
								<p>{view.legend}{view.position >= 30 && !view.finished ? ` · ${resumeLabel} ${Math.max(1, Math.floor(view.position / 60))} min` : ''}</p>
								<Button size="sm" onClick={activate}>
									{kind === 'series' ? <Tv size={16} /> : <Play size={15} fill="currentColor" />}
									{view.action}
								</Button>
							</div>
							{progress > 0 && <span className="media-preview-progress"><span style={{ width: `${progress}%` }} /></span>}
						</motion.div>
					)}
				</AnimatePresence>,
				document.body
			)}
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
		return {
			title,
			legend: `${kindLabel}${year ? ` · ${year}` : ''}`,
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
