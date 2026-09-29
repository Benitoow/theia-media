import { PLAYBACK_DEFAULTS } from '../types';
import type { PlaybackPreferences } from '../types';
export type Preferences = { reducedMotion: boolean; autoHideControls: boolean; playback: PlaybackPreferences };

/// The two sliders' own ranges, which the stored-value reader and the controls
/// both use: a number outside them is not a preference, it is a bad file.
export const SUBTITLE_SIZE = { min: 24, max: 72, step: 2 };
export const SUBTITLE_HEIGHT = { min: 24, max: 200, step: 4 };

/// The colours the sheet offers, as values rather than names: the name is the
/// catalogue key beside it, so the two languages cannot disagree about which
/// swatch is which. Six, not sixteen - the accent is one of them, which is
/// deliberate, and a picker would be a second screen for a personal choice.
export const SUBTITLE_COLOURS: Array<{ value: string; key: string }> = [
	{ value: '#EDE7DC', key: 'colourBone' },
	{ value: '#FFFFFF', key: 'colourWhite' },
	{ value: '#F2D46B', key: 'colourYellow' },
	{ value: '#8ED0E8', key: 'colourCyan' },
	{ value: '#C8A24A', key: 'colourGold' },
	{ value: '#0E0D0C', key: 'colourBlack' },
];

/// The band behind the text: none, or four greys. A band is not a designer's
/// palette, it is there to be read through, so the choice is dark, light or
/// nothing - and the engine draws it at 70 % alpha, which is what makes it a
/// band rather than a bar.
export const SUBTITLE_BANDS: Array<{ value: string; key: string }> = [
	{ value: 'none', key: 'bandNone' },
	{ value: '#000000', key: 'colourBlack' },
	{ value: '#3A3632', key: 'bandGrey' },
	{ value: '#EDE7DC', key: 'colourWhite' },
];

export const AUDIO_LANGUAGES = ['auto', 'vf', 'vo'] as const;
export const SUBTITLE_LANGUAGES = ['auto', 'none', 'fr', 'en'] as const;
export const SUBTITLE_OUTLINES = ['shadow', 'outline', 'none'] as const;
export const SUBTITLE_FONTS = ['standard', 'serif', 'mono'] as const;

/// A stored value when it is one this build offers, the default otherwise.
///
/// A preferences file outlives the build that wrote it: a colour dropped from
/// the palette, or a number typed into the devtools, would otherwise reach the
/// engine as a value it refuses - and the sheet would draw a swatch as chosen
/// while the film showed something else.
export function oneOf<T extends string>(value: unknown, allowed: readonly T[], fallback: T): T {
	return allowed.includes(value as T) ? (value as T) : fallback;
}

export function storedPlayback(value: unknown): PlaybackPreferences {
	const raw = (value ?? {}) as Record<string, unknown>;
	const style = (raw.subtitleStyle ?? {}) as Record<string, unknown>;
	const fallback = PLAYBACK_DEFAULTS.subtitleStyle;
	// A number inside its control's own range, or the default: the engine
	// refuses a size of nine thousand rather than clamping it, and a sheet that
	// showed one would be showing a value the film never had.
	const slider = (stored: unknown, range: { min: number; max: number }, or: number) =>
		typeof stored === 'number' && Number.isFinite(stored)
			? Math.min(range.max, Math.max(range.min, Math.round(stored)))
			: or;
	return {
		autoPlayNext: raw.autoPlayNext !== false,
		audioLanguage: oneOf(raw.audioLanguage, AUDIO_LANGUAGES, PLAYBACK_DEFAULTS.audioLanguage),
		subtitleLanguage: oneOf(raw.subtitleLanguage, SUBTITLE_LANGUAGES, PLAYBACK_DEFAULTS.subtitleLanguage),
		subtitleStyle: {
			sizePx: slider(style.sizePx, SUBTITLE_SIZE, fallback.sizePx),
			heightPx: slider(style.heightPx, SUBTITLE_HEIGHT, fallback.heightPx),
			colour: oneOf(style.colour, SUBTITLE_COLOURS.map((one) => one.value), fallback.colour),
			outline: oneOf(style.outline, SUBTITLE_OUTLINES, fallback.outline),
			background: oneOf(style.background, SUBTITLE_BANDS.map((one) => one.value), fallback.background),
			font: oneOf(style.font, SUBTITLE_FONTS, fallback.font),
			bold: style.bold === true,
		},
	};
}

/// The colour the outline takes for a given text colour.
///
/// The engine inverts it over dark text - a black outline around black letters
/// is not an outline - and the preview has to obey the same rule or it would
/// show a look the film never takes.
export function outlineColourFor(colour: string): string {
	const channels = [1, 3, 5].map((at) => parseInt(colour.slice(at, at + 2), 16) / 255);
	const [r, g, b] = channels.map((one) => (Number.isFinite(one) ? one : 1));
	const luminance = 0.2126 * r + 0.7152 * g + 0.0722 * b;
	return luminance < 0.5 ? '#EDE7DC' : '#000000';
}

/// What the OSD is saying, and for how long.
///
/// `label` is a catalogue key rather than a word, because the two notices are
/// not about the same thing: a refused raw stream is "Son", an engine that will
/// not start is not. A null `dwell` means the notice must not leave on its own -
/// an engine that could not start is still not started.
export type Notice = { key: string; label: string; dwell: number | null };

export function initialPreferences(): Preferences {
	try {
		const value = JSON.parse(localStorage.getItem('theia.player.preferences') ?? '{}');
		return {
			reducedMotion: Boolean(value.reducedMotion),
			autoHideControls: value.autoHideControls !== false,
			playback: storedPlayback(value.playback),
		};
	} catch {
		return { reducedMotion: false, autoHideControls: true, playback: PLAYBACK_DEFAULTS };
	}
}
