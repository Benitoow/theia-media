// The player proof: a real capture of the Theia player with its interactive
// chrome rebuilt around it, explicitly labelled as a demonstration
// (docs/design-system.md §12.1). It is the page's one flourish.
import { useState, useEffect, useRef, useCallback } from 'react';

interface PlayerDemoCopy {
	status: string;
	title: string;
	alt: string;
	play: string;
	pause: string;
	back10: string;
	forward10: string;
	mute: string;
	unmute: string;
	tracksOpen: string;
	tracksTitle: string;
	audio: string;
	quality: string;
	subtitles: string;
	loading: string;
	position: string;
	volume: string;
	trackOptions: Record<'audio' | 'quality' | 'subtitles', [string, string][]>;
}

interface Props {
	t: PlayerDemoCopy;
	shot: string;
}

const Icons = {
	playPause: (
		<>
			<svg className="icon-play" viewBox="0 0 24 24" aria-hidden="true">
				<path fill="currentColor" d="M8 5v14l11-7z" />
			</svg>
			<svg className="icon-pause" viewBox="0 0 24 24" aria-hidden="true">
				<path fill="currentColor" d="M7 5h4v14H7zm6 0h4v14h-4z" />
			</svg>
		</>
	),
	settings: (
		<svg viewBox="0 0 24 24" aria-hidden="true">
			<path
				d="M12 8.6a3.4 3.4 0 1 0 0 6.8 3.4 3.4 0 0 0 0-6.8Z"
				fill="none"
				stroke="currentColor"
				strokeWidth="1.7"
			/>
			<path
				d="m19 13.2 1.2 1-.7 1.8-1.6-.1-1.2 1.2.1 1.6-1.8.7-1-1.2h-1.8l-1 1.2-1.8-.7.1-1.6-1.2-1.2-1.6.1-.7-1.8 1.2-1v-1.8L6 10.2l.7-1.8 1.6.1 1.2-1.2-.1-1.6 1.8-.7 1 1.2H14l1-1.2 1.8.7-.1 1.6 1.2 1.2 1.6-.1.7 1.8-1.2 1v1.8Z"
				fill="none"
				stroke="currentColor"
				strokeWidth="1.3"
				strokeLinejoin="round"
			/>
		</svg>
	),
	volume: (
		<svg viewBox="0 0 24 24" aria-hidden="true">
			<path
				d="M5 10v4h3l4 3V7l-4 3H5Zm10-1.5c1.3.8 2 2 2 3.5s-.7 2.7-2 3.5"
				fill="none"
				stroke="currentColor"
				strokeWidth="1.7"
				strokeLinecap="round"
			/>
		</svg>
	)
};

const DURATION = 7268;
const INITIAL = 2538;
const CLOCK_ELLAPSED = '42:18';
const CLOCK_TOTAL = '2:01:08';

export default function PlayerDemo({ t, shot }: Props) {
	const [playing, setPlaying] = useState(false);
	const [muted, setMuted] = useState(false);
	const [tracksOpen, setTracksOpen] = useState(true);
	const [busy, setBusy] = useState(false);
	const [position, setPosition] = useState(INITIAL);
	const busyTimer = useRef<number | undefined>(undefined);
	const anchorRef = useRef<HTMLDivElement>(null);

	useEffect(() => () => window.clearTimeout(busyTimer.current), []);

	useEffect(() => {
		if (!tracksOpen) return;
		const onKey = (event: KeyboardEvent) => {
			if (event.key === 'Escape') setTracksOpen(false);
		};
		const onClick = (event: MouseEvent) => {
			if (!anchorRef.current?.contains(event.target as Node)) setTracksOpen(false);
		};
		document.addEventListener('keydown', onKey);
		document.addEventListener('click', onClick);
		return () => {
			document.removeEventListener('keydown', onKey);
			document.removeEventListener('click', onClick);
		};
	}, [tracksOpen]);

	const onScrub = useCallback((event: React.ChangeEvent<HTMLInputElement>) => {
		setPosition(Number(event.target.value));
	}, []);

	const seek = useCallback((delta: number) => {
		setPosition((current) => Math.max(0, Math.min(DURATION, current + delta)));
	}, []);

	const pickTrack = useCallback((type: 'audio' | 'quality' | 'subtitles', primary: string) => {
		// Single-select within a group. A quality change shows the preparing
		// state the real player shows while it re-negotiates the stream.
		setGroupSelection(type, primary);
		if (type === 'quality') {
			window.clearTimeout(busyTimer.current);
			setBusy(true);
			busyTimer.current = window.setTimeout(() => setBusy(false), 650);
		}
	}, []);

	const ratio = ((position) / DURATION) * 100;

	return (
		<div className="relative overflow-hidden border border-line rounded-[var(--radius-card)] bg-surface min-h-[clamp(27rem,38vw,32rem)] shadow-[0_2.5rem_6rem_rgb(11_10_9/0.76)]">
			<img
				className="absolute inset-0 w-full h-full object-cover"
				src={shot}
				alt={t.alt}
				width="1280"
				height="720"
				decoding="async"
				fetchPriority="high"
			/>
			<span className="absolute top-4 left-4 z-[5] max-w-[calc(100%-2rem)] rounded-full border border-line bg-surface/95 px-3 py-2 font-semibold text-[0.625rem] leading-[1.2] tracking-[var(--tracking-micro)] uppercase text-bone">
				{t.status}
			</span>

			<div className="absolute inset-x-0 top-0 z-[4] flex items-center justify-between pt-[4.2rem] px-5 pb-4 bg-gradient-to-b from-ink to-transparent">
				<span className="text-sm font-semibold">{t.title}</span>
			</div>

			{busy && (
				<div
					aria-live="polite"
					className="absolute inset-0 z-[12] grid place-content-center justify-items-center gap-3 bg-[rgb(11_10_9/0.84)] font-semibold text-[0.6875rem] tracking-[var(--tracking-label)] uppercase"
				>
					<span
						aria-hidden="true"
						className="w-[34px] h-[34px] rounded-full border-2 border-line border-t-bone animate-spin"
					/>
					<span>{t.loading}</span>
				</div>
			)}

			<div className="absolute inset-x-0 bottom-0 z-[6] px-5 pt-10 pb-4 bg-gradient-to-t from-ink via-ink/70 to-transparent">
				<input
					type="range"
					min={0}
					max={DURATION}
					value={position}
					onChange={onScrub}
					aria-label={t.position}
					className="scrub relative w-full h-[44px] m-0 appearance-none bg-transparent cursor-pointer"
					style={{ '--progress': `${ratio}%` } as React.CSSProperties}
				/>
				<div className="flex items-center gap-2 mt-1">
					<button
						type="button"
						aria-label={playing ? t.pause : t.play}
						aria-pressed={playing}
						onClick={() => setPlaying((value) => !value)}
						className="icon-button bg-bone text-ink hover:bg-parchment hover:border-transparent"
					>
						{Icons.playPause}
					</button>
					<button type="button" aria-label={t.back10} onClick={() => seek(-10)} className="icon-button">
						<span aria-hidden="true">−10</span>
					</button>
					<button type="button" aria-label={t.forward10} onClick={() => seek(10)} className="icon-button">
						<span aria-hidden="true">+10</span>
					</button>
					<span className="inline-flex items-center gap-2 ml-1.5 text-muted text-[0.6875rem] font-medium font-mono tabular-nums">
						{CLOCK_ELLAPSED}
						<i aria-hidden="true" className="w-px h-3.5 bg-line not-italic" />
						{CLOCK_TOTAL}
					</span>
					<span className="flex-1" />
					<button
						type="button"
						aria-label={muted ? t.unmute : t.mute}
						aria-pressed={muted}
						onClick={() => setMuted((value) => !value)}
						className="icon-button max-[43.75rem]:hidden"
					>
						{Icons.volume}
					</button>
					<div ref={anchorRef} className="relative">
						<button
							type="button"
							aria-label={t.tracksOpen}
							aria-expanded={tracksOpen}
							aria-controls="tracks-panel"
							onClick={() => setTracksOpen((value) => !value)}
							className={`icon-button ${tracksOpen ? 'border-line bg-raised' : ''}`}
						>
							{Icons.settings}
						</button>
						{tracksOpen && (
							<section
								id="tracks-panel"
								aria-label={t.tracksTitle}
								className="absolute right-0 bottom-[3.8rem] z-10 w-[min(20rem,70vw)] max-h-[18rem] overflow-auto rounded-2xl border border-line bg-surface p-3.5 shadow-[0_1.25rem_3rem_rgb(11_10_9/0.75)]"
							>
								{(['audio', 'quality', 'subtitles'] as const).map((type) => (
									<div key={type}>
										<p className="mt-2.5 mb-1.5 mx-1.5 text-muted font-semibold text-[0.6875rem] leading-[1.2] tracking-[var(--tracking-label)] uppercase">
											{type === 'audio' ? t.audio : type === 'quality' ? t.quality : t.subtitles}
										</p>
										{t.trackOptions[type].map(([primary, detail], index) => (
											<TrackButton
												key={primary}
												type={type}
												primary={primary}
												detail={detail}
												defaultPressed={index === 0}
												onPick={pickTrack}
											/>
										))}
									</div>
								))}
							</section>
						)}
					</div>
				</div>
			</div>
		</div>
	);
}

/* One selectable row inside the tracks panel. Selection state is per group and
   lives outside React on purpose: three tiny groups do not need a shared
   store, and the row keeps the aria-pressed contract the checker looks for. */
function setGroupSelection(type: string, primary: string) {
	document
		.querySelectorAll<HTMLButtonElement>(`button[data-track="${type}"]`)
		.forEach((button) => button.setAttribute('aria-pressed', String(button.dataset.primary === primary)));
}

function TrackButton({
	type,
	primary,
	detail,
	defaultPressed,
	onPick
}: {
	type: 'audio' | 'quality' | 'subtitles';
	primary: string;
	detail: string;
	defaultPressed: boolean;
	onPick: (type: 'audio' | 'quality' | 'subtitles', primary: string) => void;
}) {
	return (
		<button
			type="button"
			data-track={type}
			data-primary={primary}
			aria-pressed={defaultPressed}
			onClick={() => onPick(type, primary)}
			className="track-option flex w-full min-h-[50px] items-center justify-between gap-4 rounded-lg border-0 bg-transparent px-2.5 py-2 text-left text-bone cursor-pointer transition-colors hover:bg-raised aria-pressed:bg-raised active:translate-y-px"
		>
			<span>
				<strong className="block text-sm font-semibold">{primary}</strong>
				{detail ? <small className="block text-muted text-[0.6875rem]">{detail}</small> : null}
			</span>
			<span aria-hidden="true" className="track-tick text-bone">
				✓
			</span>
		</button>
	);
}
