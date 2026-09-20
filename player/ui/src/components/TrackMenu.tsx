import { Check } from 'lucide-react';
import { motion } from 'motion/react';
import { forwardRef, useImperativeHandle, useRef } from 'react';

import type { Track } from '../types';

export type TrackMenuHandle = { moveFocus: (step: number) => void };

type Props = {
	tracks: Track[];
	onPick: (kind: 'audio' | 'sub', id: number | null) => void;
	t: (key: string) => string;
};

export const TrackMenu = forwardRef<TrackMenuHandle, Props>(({ tracks, onPick, t }, ref) => {
	const root = useRef<HTMLDivElement>(null);
	const audio = tracks.filter((track) => track.type === 'audio');
	const subtitles = tracks.filter((track) => track.type === 'sub');

	useImperativeHandle(ref, () => ({
		moveFocus(step) {
			const buttons = [...(root.current?.querySelectorAll<HTMLButtonElement>('button') ?? [])];
			if (!buttons.length) return;
			const current = buttons.indexOf(document.activeElement as HTMLButtonElement);
			buttons[(current + step + buttons.length) % buttons.length]?.focus();
		},
	}));

	const row = (track: Track, kind: 'audio' | 'sub') => (
		<button className="track" key={`${kind}-${track.id}`} role="menuitemradio" aria-checked={track.selected} onClick={() => onPick(kind, track.id)}>
			<span className="tick">{track.selected && <Check size={16} />}</span>
			<span className="track-text">
				<span className="track-primary">{label(track, t('trackNumber'))}</span>
				<span className="micro uppercase">{[track.codec?.toUpperCase(), track.external ? t('externalTrack') : ''].filter(Boolean).join(' · ')}</span>
			</span>
		</button>
	);

	return (
		<motion.div
			ref={root}
			className="track-menu"
			role="menu"
			initial={{ opacity: 0, y: 8, scale: 0.98 }}
			animate={{ opacity: 1, y: 0, scale: 1 }}
			exit={{ opacity: 0, y: 6, scale: 0.985 }}
			transition={{ duration: 0.18, ease: [0.16, 1, 0.3, 1] }}
		>
			<p className="label">{t('audioTracks')}</p>
			{audio.map((track) => row(track, 'audio'))}
			<p className="label">{t('subtitleTracks')}</p>
			<button className="track" role="menuitemradio" aria-checked={!subtitles.some((track) => track.selected)} onClick={() => onPick('sub', null)}>
				<span className="tick">{!subtitles.some((track) => track.selected) && <Check size={16} />}</span>
				<span className="track-primary">{t('subtitlesOff')}</span>
			</button>
			{subtitles.map((track) => row(track, 'sub'))}
		</motion.div>
	);
});
TrackMenu.displayName = 'TrackMenu';

function label(track: Track, fallback: string) {
	return track.title || track.lang?.toUpperCase() || `${fallback} ${track.id}`;
}
