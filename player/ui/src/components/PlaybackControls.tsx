import { AnimatePresence } from 'motion/react';
import { Languages, Maximize2, Minimize2, Pause, Play, RotateCcw, RotateCw, Settings2, Volume1, Volume2, VolumeX } from 'lucide-react';
import { KeyboardEvent as ReactKeyboardEvent, MouseEvent as ReactMouseEvent } from 'react';

import { TrackMenu, type TrackMenuHandle } from './TrackMenu';
import { Button } from './ui/button';

import type { PlayerStatus, QualityLadder, Track, TrackVocabulary } from '../types';

type PlaybackProps = {
	visible: boolean; status: PlayerStatus; seconds: number; duration: number; progress: number; fullscreen: boolean;
	language: string; tracks: Track[]; words: TrackVocabulary; trackMenuOpen: boolean; trackButton: React.RefObject<HTMLButtonElement | null>;
	qualities: QualityLadder | null; currentQuality: number | null; onPickQuality: (height: number | null) => void;
	volume: number; onVolume: (value: number) => void;
	trackMenu: React.RefObject<TrackMenuHandle | null>; t: (key: string) => string; onToggle: () => void;
	onSeek: (seconds: number) => void; onSeekAbsolute: (seconds: number) => void; onMute: () => void;
	onTracks: () => void; onPickTrack: (kind: 'audio' | 'sub', id: number | null) => void;
	onLanguage: () => void; onFullscreen: () => void;
};

export function PlaybackControls(props: PlaybackProps) {
	const scrub = (event: ReactMouseEvent<HTMLDivElement>) => {
		if (!props.duration) return;
		const rect = event.currentTarget.getBoundingClientRect();
		props.onSeekAbsolute(Math.max(0, Math.min(1, (event.clientX - rect.left) / rect.width)) * props.duration);
	};
	const scrubKey = (event: ReactKeyboardEvent<HTMLDivElement>) => {
		if (event.key === 'ArrowLeft' || event.key === 'ArrowDown') { event.preventDefault(); props.onSeek(-10); }
		else if (event.key === 'ArrowRight' || event.key === 'ArrowUp') { event.preventDefault(); props.onSeek(10); }
		else if (event.key === 'Home') { event.preventDefault(); props.onSeekAbsolute(0); }
		else if (event.key === 'End') { event.preventDefault(); props.onSeekAbsolute(props.duration); }
	};
	return (
		<div className={`controls ${props.visible ? '' : 'controls--hidden'}`}>
			<div className="scrub" role="slider" tabIndex={0} aria-label={props.t('play')} aria-valuemin={0} aria-valuemax={Math.round(props.duration)} aria-valuenow={Math.round(props.seconds)} onClick={scrub} onKeyDown={scrubKey}>
				<div className="scrub-track"><div className="scrub-played" style={{ width: `${props.progress * 100}%` }} /></div>
				<div className="scrub-thumb" style={{ left: `${props.progress * 100}%` }} />
			</div>
			<div className="row">
				<Button className="control control--skip" variant="ghost" size="icon" onClick={() => props.onSeek(-10)} aria-label={props.t('back10')}><RotateCcw size={21} /></Button>
				<Button className="control control--primary" size="icon" onClick={props.onToggle} aria-label={props.status.pause ? props.t('play') : props.t('pause')}>{props.status.pause ? <Play size={20} fill="currentColor" /> : <Pause size={20} />}</Button>
				<Button className="control control--skip" variant="ghost" size="icon" onClick={() => props.onSeek(10)} aria-label={props.t('forward10')}><RotateCw size={21} /></Button>
				<span className="clock"><span className="elapsed">{clock(props.seconds)}</span><span className="rule" /><span className="total">{clock(props.duration)}</span></span>
				<span className="spacer" />
				<div className="menu-anchor">
					<Button ref={props.trackButton} className="control" variant="ghost" size="icon" onClick={props.onTracks} aria-label={props.t('tracks')} aria-haspopup="dialog" aria-expanded={props.trackMenuOpen}><Settings2 size={21} /></Button>
					<AnimatePresence>{props.trackMenuOpen && <TrackMenu ref={props.trackMenu} tracks={props.tracks} words={props.words} qualities={props.qualities} currentQuality={props.currentQuality} onPick={props.onPickTrack} onPickQuality={props.onPickQuality} t={props.t} />}</AnimatePresence>
				</div>
				{/* The slider is the volume; the button is the mute. The engine
				    keeps the two apart, so the thumb shows the level a press on
				    the icon would bring back rather than reporting zero - the
				    struck-through icon already says the sound is off. */}
				<div className="player-volume">
					<Button className="control control--mute" variant="ghost" size="icon" onClick={props.onMute} aria-label={props.status.mute ? props.t('unmute') : props.t('mute')}>{props.status.mute || props.volume === 0 ? <VolumeX size={21} /> : props.volume < 0.5 ? <Volume1 size={21} /> : <Volume2 size={21} />}</Button>
					<input
						type="range"
						className="volume-slider"
						min={0}
						max={1}
						step={0.02}
						value={props.volume}
						onChange={(event) => props.onVolume(Number(event.currentTarget.value))}
						aria-label={props.t('volume')}
					/>
				</div>
				{props.status.audioMode && <span className="label audio-mode">{props.status.audioMode === 'passthrough' ? 'BITSTREAM' : props.status.audioMode.toUpperCase()}</span>}
				<Button className="control control--desktop" variant="ghost" size="icon" onClick={props.onLanguage} aria-label="Français / English"><Languages size={20} /><span className="sr-only">{props.language}</span></Button>
				<Button className="control" variant="ghost" size="icon" onClick={props.onFullscreen} aria-label={props.fullscreen ? props.t('exitFullscreen') : props.t('fullscreen')} aria-pressed={props.fullscreen}>{props.fullscreen ? <Minimize2 size={21} /> : <Maximize2 size={21} />}</Button>
			</div>
		</div>
	);
}

function clock(value: number) {
	if (!Number.isFinite(value) || value <= 0) return '--:--';
	const total = Math.floor(value);
	const h = Math.floor(total / 3600);
	const m = Math.floor((total % 3600) / 60);
	const s = total % 60;
	return h ? `${h}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}` : `${m}:${String(s).padStart(2, '0')}`;
}
