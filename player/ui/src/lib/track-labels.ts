// How an audio or subtitle track is named in the player's menu.
//
// The same rule as the web application's `web/src/lib/track-labels.js`, with the
// same function names on purpose: a reader who knows one knows the other, and a
// track is called the same thing in both players. It is written a second time
// because the two are handed different shapes - the web reads the server's
// tracks, this reads mpv's `track-list`, where a layout is `demux-channels` and
// a sidecar is `external` - and a translation layer between them would be a
// third thing to keep right.
//
// Section 6b owns the shape of the answer. The language leads at reading size,
// because that is what the choice is made on; codec, channels and provenance sit
// under it as a tracked label; and a detail that merely repeats the line above
// it is dropped. The menu the maintainer photographed on 23 September 2026 did
// none of that: it led with the container's title ("TrueHD 7.1 Atmos") and put
// the raw codec under it ("TRUEHD"), so seven tracks read as fourteen lines of
// abbreviations.

import type { Track, TrackVocabulary } from '../types';

export type TrackLabel = { primary: string; detail: string };

/** What the catalogue calls a language. The container sends an ISO code, and a
 * code nobody has a word for stays itself rather than disappearing. */
export function languageName(code: string | undefined, words: TrackVocabulary): string | null {
	if (!code) return null;
	return words.languages[code.toLowerCase()] ?? code.toUpperCase();
}

/** A channel layout as somebody says it out loud.
 *
 * mpv reports the container's own layout - "7.1", "5.1(side)", "stereo" - and
 * `unknown6` when the container wrote none. The parenthesis is for a mixing
 * desk, so it goes; and when the layout really is unknown, the number of
 * channels is the one thing left to say, which is what `channelCount` is for.
 * Calling six unknown channels "5.1" would be the player naming something it
 * was not told. */
export function channelLabel(track: Track, words: TrackVocabulary): string | null {
	const layout = (track['demux-channels'] ?? '').replace(/\(.*\)/, '').trim();
	if (layout && !layout.startsWith('unknown')) return words.channels[layout] ?? layout;
	const count = track['demux-channel-count'] ?? track['audio-channels'];
	if (!count) return null;
	if (count === 1) return words.channels.mono;
	if (count === 2) return words.channels.stereo;
	return words.channelCount.replace('{n}', String(count));
}

/** mpv's codec id, said the way a person reads it: `ac3` is AC-3, and every PCM
 * variant is PCM.
 *
 * The engine's own `codec-desc` is deliberately not used: it is libavcodec's
 * English ("SubRip subtitle", "Dolby TrueHD + Dolby Atmos"), and decision 25
 * keeps the engine's words out of the interface - the catalogue owns them. An id
 * with no word of its own is shown upper-cased, which is what the menu did for
 * all of them before. */
export function codecName(track: Track, words: TrackVocabulary): string | null {
	const codec = track.codec?.toLowerCase();
	if (!codec) return null;
	if (codec.startsWith('pcm_')) return words.codecs.pcm;
	return words.codecs[codec] ?? codec.toUpperCase();
}

/** Comparison form: case, accents and punctuation are not what a person reads.
 * "Francais" and "Français" are one word, and a track titled "DD 5.1" already
 * says its own "5.1". */
function comparable(text: string): string {
	return text
		.toLowerCase()
		.normalize('NFD')
		.replace(/[\u0300-\u036f]/g, '')
		.replace(/[^a-z0-9.]+/g, ' ')
		.trim();
}

/** A detail that merely repeats the line above it is noise, and so is one that
 * repeats another part of its own line: a track titled "TrueHD 7.1 Atmos" with a
 * codec of `truehd` was reading "TRUEHD · TrueHD 7.1 Atmos".
 *
 * A part contained in the primary is dropped; a part contained in another part
 * is dropped; and when a longer part arrives after a shorter one it already
 * contains, the shorter one goes. What is left is what the line has not said. */
export function detailOf(parts: Array<string | null | undefined>, primary: string): string {
	const kept: string[] = [];
	for (const part of parts) {
		if (!part) continue;
		const value = part.trim();
		if (!value) continue;
		const form = comparable(value);
		if (!form) continue;
		if (comparable(primary).includes(form)) continue;
		// A kept part that already says this one wins: it came first and it is
		// the shorter fact. The other way round, this part says what a kept one
		// said and more - "TrueHD 7.1 Atmos" against a codec of "TrueHD" - and
		// the shorter one goes, because the longer one is the whole story.
		if (kept.some((other) => comparable(other).includes(form))) continue;
		const superseded = kept.filter((other) => form.includes(comparable(other)));
		for (const other of superseded) kept.splice(kept.indexOf(other), 1);
		kept.push(value);
	}
	return kept.join(' · ');
}

export function audioLabel(track: Track, index: number, words: TrackVocabulary): TrackLabel {
	const primary =
		languageName(track.lang, words) ?? track.title ?? words.unnamedAudio.replace('{n}', String(index + 1));
	// The commentary track is the one people most need to tell apart, and the
	// container does not flag it in what mpv sends: the title is the only signal
	// there is, read for the word. A heuristic, deliberately, and it only ever
	// changes a label. A title long enough to wrap belongs in the menu, not on
	// one line of it.
	const commentary = /commentary|commentaire/i.test(track.title || '');
	const short = !!track.title && track.title.length <= 24;
	const parts = commentary
		? [words.commentary, channelLabel(track, words)]
		: [channelLabel(track, words), codecName(track, words), short ? track.title : null];
	return { primary, detail: detailOf(parts, primary) };
}

/** Subtitles differ from audio in one way that matters here: a film can carry
 * the same language twice in two formats, and a text track and a bitmap one look
 * identical without the codec said out loud - "Anglais" twice, one of which is
 * PGS. The web application never has to say it, because it refuses to draw
 * bitmap subtitles at all; the native player draws them, so the row says which
 * is which. */
export function subtitleLabel(track: Track, index: number, words: TrackVocabulary): TrackLabel {
	const primary =
		languageName(track.lang, words) ?? track.title ?? words.unnamedSubtitle.replace('{n}', String(index + 1));
	return {
		primary,
		detail: detailOf(
			[
				track.title,
				track.forced ? words.forced : null,
				track.external ? words.external : null,
				codecName(track, words),
			],
			primary
		),
	};
}
