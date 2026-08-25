// How an audio or subtitle track is named in the player's menu.
//
// Pure, and the catalogue is handed in rather than imported, for two reasons.
// The interface owns every word shown (decision 25), so these must never invent
// one; and taking `t` as an argument is what lets them be tested at all, with
// `node --test` and no browser.

/**
 * A channel layout as somebody says it out loud. "5.1(side)" is a layout and
 * the parenthesis is for a mixing desk.
 */
export function channelLabel(channels, t) {
	if (!channels) return null;
	const base = channels.replace(/\(.*\)/, '').trim();
	return t.player.tracks.channels[base] ?? base;
}

/**
 * The commentary track is the one people most need to tell apart, and the
 * container does not flag it in what the server sends. The title is the only
 * signal there is, so it is read for the word -- a heuristic, deliberately, and
 * it only ever changes a label.
 */
export function looksLikeCommentary(title) {
	return /commentary|commentaire/i.test(title || '');
}

/** A title long enough to wrap belongs in the menu, not on one line of it. */
export function titleIsShortEnough(title) {
	return !!title && title.length <= 24;
}

/** The catalogue owns the names; the server only ever sends the ISO code. */
export function languageName(code, t) {
	return t.languages[code] ?? code.toUpperCase();
}

/**
 * A detail that merely repeats the line above it is noise: a French track titled
 * "Français" was rendering as "Français · Français".
 */
export function detailOf(parts, primary) {
	return parts
		.filter(Boolean)
		.filter((part) => part.toLowerCase() !== primary.toLowerCase())
		.join(' · ');
}

export function audioLabel(track, index, t) {
	const primary = track.language
		? languageName(track.language, t)
		: track.title || t.film.audio.unnamed(index + 1);
	const parts = looksLikeCommentary(track.title)
		? [t.player.tracks.commentary, channelLabel(track.channels, t)]
		: [
				channelLabel(track.channels, t),
				track.codec?.toUpperCase(),
				titleIsShortEnough(track.title) ? track.title : null
			];
	return { primary, detail: detailOf(parts, primary) };
}

export function subtitleLabel(track, index, t) {
	const primary = track.language
		? languageName(track.language, t)
		: track.title || t.player.tracks.unnamedSubtitle(index + 1);
	return {
		primary,
		detail: detailOf(
			[
				track.title,
				track.is_forced ? t.player.tracks.forced : null,
				track.is_external ? t.player.tracks.external : null
			],
			primary
		)
	};
}
