import { Check } from 'lucide-react';
import { motion } from 'motion/react';
import { forwardRef, useImperativeHandle, useRef, useState } from 'react';

import type { QualityLadder, Track, TrackVocabulary } from '../types';
import { audioLabel, subtitleLabel, type TrackLabel } from '../lib/track-labels';

export type TrackMenuHandle = {
	moveFocus: (step: number) => void;
	moveTab: (step: number) => void;
};

type Props = {
	tracks: Track[];
	words: TrackVocabulary;
	/** What the server says this machine can produce for the film playing, or
	 * null when it offered no ladder at all. */
	qualities: QualityLadder | null;
	currentQuality: number | null;
	onPick: (kind: 'audio' | 'sub', id: number | null) => void;
	onPickQuality: (height: number | null) => void;
	t: (key: string) => string;
};

/** One row, written once: audio, subtitles and quality rungs differ in what
 * they list, not in how a choice looks, and writing the markup twice is how the
 * three drift apart.
 *
 * The tick's column is reserved whether or not a tick is drawn, so choosing a
 * track never shifts every other row sideways, and the chosen row carries a
 * plate as well as the tick - section 6b: "a 2px rule at 6% fill does not
 * survive the room". */
function option(label: TrackLabel, chosen: boolean, choose: () => void, disabled = false) {
	return (
		<button
			type="button"
			className={
				' track-option' +
				(chosen ? ' track-option--chosen' : '') +
				(disabled ? ' track-option--disabled' : '')
			}
			role="radio"
			aria-checked={chosen}
			aria-disabled={disabled || undefined}
			disabled={disabled}
			onClick={choose}
		>
			<span className="track-tick" aria-hidden="true">{chosen && <Check size={16} />}</span>
			<span className="track-lines">
				<span className="track-primary">{label.primary}</span>
				{label.detail ? <span className="track-detail">{label.detail}</span> : null}
			</span>
		</button>
	);
}

/** The tracks, the quality, and the tabs that keep them apart.
 *
 * Two tabs because the two questions are asked at different moments: which
 * language, mid-film and often; how much picture, once, when the machine or the
 * network cannot take the file as it is. The frequent one is the tab it opens
 * on. Neither tab repeats the other - the audio is listed where it is chosen,
 * with its own format under it, and the quality tab says nothing about sound -
 * which is the "no duplicates" rule the maintainer asked for on 24 September
 * 2026, and it holds because there is no audio ladder to draw: the server
 * converts the sound only to carry the track that was chosen.
 *
 * The strip only exists when there is something to choose: one rung is not a
 * question, and a tab that opens onto a single row is furniture. */
export const TrackMenu = forwardRef<TrackMenuHandle, Props>(
	({ tracks, words, qualities, currentQuality, onPick, onPickQuality, t }, ref) => {
		const root = useRef<HTMLDivElement>(null);
		const [tab, setTab] = useState<'languages' | 'quality'>('languages');
		const audio = tracks.filter((track) => track.type === 'audio');
		const subtitles = tracks.filter((track) => track.type === 'sub');
		const ladder = qualities?.qualities ?? [];
		// The file itself is always a rung; a ladder of one is the answer "there
		// is nothing else", which is a statement rather than a choice.
		const hasQuality = ladder.length > 1;
		const active = hasQuality ? tab : 'languages';

		useImperativeHandle(ref, () => ({
			moveFocus(step) {
				const buttons = [...(root.current?.querySelectorAll<HTMLButtonElement>('.track-option') ?? [])];
				if (!buttons.length) return;
				const current = buttons.indexOf(document.activeElement as HTMLButtonElement);
				buttons[(current + step + buttons.length) % buttons.length]?.focus();
			},
			moveTab(step) {
				if (!hasQuality) return;
				const next = step > 0 ? 'quality' : 'languages';
				setTab(next);
				// The ring follows the switch: a tab changed by keyboard that left
				// focus behind on a row of the panel it just hid is a menu that
				// looks broken to anybody navigating with arrows.
				window.requestAnimationFrame(() => {
					root.current?.querySelector<HTMLButtonElement>(`[data-tab="${next}"]`)?.focus();
				});
			},
		}));

		const kind = qualities?.transcode?.kind ?? '';
		const busy = Boolean(qualities?.transcode?.busy);

		return (
			<motion.div
				ref={root}
				className="track-menu"
				role="dialog"
				aria-label={t('tracks')}
				initial={{ opacity: 0, y: 8, scale: 0.98 }}
				animate={{ opacity: 1, y: 0, scale: 1 }}
				exit={{ opacity: 0, y: 6, scale: 0.985 }}
				transition={{ duration: 0.18, ease: [0.16, 1, 0.3, 1] }}
			>
				{hasQuality && (
					<div className="track-tabs" role="tablist" aria-label={t('tracks')}>
						{(['languages', 'quality'] as const).map((name) => (
							<button
								key={name}
								type="button"
								role="tab"
								data-tab={name}
								id={`track-tab-${name}`}
								aria-selected={active === name}
								aria-controls={`track-panel-${name}`}
								className={active === name ? 'track-tab track-tab--chosen' : 'track-tab'}
								onClick={() => setTab(name)}
							>
								{t(name === 'quality' ? 'qualityTab' : 'languagesTab')}
							</button>
						))}
					</div>
				)}

				{active === 'languages' ? (
					<div id="track-panel-languages" role="tabpanel" aria-labelledby={hasQuality ? 'track-tab-languages' : undefined}>
						<h2 className="track-heading">{t('audioTracks')}</h2>
						<ul className="track-list" role="radiogroup" aria-label={t('audioTracks')}>
							{audio.map((track, index) => (
								<li key={`audio-${track.id}`} role="none">
									{option(audioLabel(track, index, words), Boolean(track.selected), () => onPick('audio', track.id))}
								</li>
							))}
						</ul>
						<h2 className="track-heading">{t('subtitleTracks')}</h2>
						<ul className="track-list" role="radiogroup" aria-label={t('subtitleTracks')}>
							<li role="none">
								{option(
									{ primary: t('subtitlesOff'), detail: '' },
									!subtitles.some((track) => track.selected),
									() => onPick('sub', null)
								)}
							</li>
							{subtitles.map((track, index) => (
								<li key={`sub-${track.id}`} role="none">
									{option(subtitleLabel(track, index, words), Boolean(track.selected), () => onPick('sub', track.id))}
								</li>
							))}
						</ul>
					</div>
				) : (
					<div id="track-panel-quality" role="tabpanel" aria-labelledby="track-tab-quality">
						<h2 className="track-heading">
							{t('qualityTab')}
							{kind === 'hardware' || kind === 'software' ? (
								<span className="track-heading-note">
									{kind === 'hardware' ? words.qualityHardware : words.qualitySoftware}
								</span>
							) : null}
						</h2>
						<ul className="track-list" role="radiogroup" aria-label={t('qualityTab')}>
							{ladder.map((rung) => {
								// Height 0 is the file itself, and it is the only rung this
								// player serves from the container: mpv reads it directly, so
								// TrueHD and Atmos reach the amplifier untouched.
								const label: TrackLabel = rung.height
									? {
											primary: words.qualityHeight.replace('{n}', String(rung.height)),
											detail: rung.mode === 'transcode' ? words.qualityReencoded : ''
										}
									: { primary: t('qualityOriginal'), detail: '' };
								return (
									<li key={`quality-${rung.height}`} role="none">
										{option(
											label,
											(currentQuality ?? 0) === (rung.height || 0),
											() => onPickQuality(rung.height || null),
											// Every rung but the file's own needs an encoder, and a
											// press that would stall somebody else's film is greyed
											// rather than refused.
											busy && rung.mode === 'transcode'
										)}
									</li>
								);
							})}
						</ul>
					</div>
				)}
			</motion.div>
		);
	}
);
TrackMenu.displayName = 'TrackMenu';
