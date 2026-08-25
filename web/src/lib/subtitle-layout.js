// Where a subtitle sits, in numbers.
//
// Every value here was measured in a real browser rather than reasoned about,
// and the comments say which — this is the one part of the player whose
// correctness cannot be read off the source, because it depends on what Chrome
// does with a cue box that lives in a closed shadow root.
//
// It is a module rather than a handful of functions inside the player so that it
// can be tested at all. None of it touches the DOM: callers measure, these
// functions decide.

/**
 * The height of one line of subtitle text, which no API will tell you: the cue
 * box is inside a closed shadow root.
 *
 * The font size mirrors `.player-video::cue` in app.css — changing one means
 * changing the other — and the 2.2 multiplier is measured, not assumed, because
 * Chrome does not honour `line-height` on `::cue`.
 */
export function cueFontSize(rootFontSize, viewportWidth) {
	const root = rootFontSize || 16;
	return Math.min(Math.max(root * 1.0625, viewportWidth * 0.026), root * 2.375);
}

export function cueLineHeight(rootFontSize, viewportWidth) {
	return cueFontSize(rootFontSize, viewportWidth) * 2.2;
}

/**
 * The picture inside the element, which are not the same thing.
 *
 * `object-fit: contain` means a 2.39:1 film paints 1280x536 inside a 1280x720
 * element and puts 92px of black above and below it — measured on a real scope
 * rip. Anchoring anything to the element floats it a sixth of the way up the
 * frame, over the actors rather than under them.
 *
 * A film with no bars is the same calculation with a zero letterbox, which is
 * why there is no special case for one.
 */
export function pictureBox({ width, height, videoWidth, videoHeight }) {
	if (!height) return null;
	const ratio = videoWidth && videoHeight ? videoWidth / videoHeight : 0;
	const picture = ratio && width ? Math.min(height, width / ratio) : height;
	return { picture, letterbox: (height - picture) / 2 };
}

/**
 * How far the subtitle layer sits above the bottom of the video element.
 *
 * Subtitles belong on the last line of the picture, which is exactly where the
 * control bar appears: measured in a real browser, the second line of a cue sat
 * behind the scrub bar for as long as the controls were up. So it sits just
 * inside the bottom of the picture, and steps above the bar when the bar covers
 * that. The gap is a share of the picture so it holds at every size.
 */
export function layerBottom({ letterbox, picture, bar }) {
	return Math.max(letterbox + picture * 0.04, bar + picture * 0.04);
}

/**
 * The `line` value for a cue, as a percentage of the element.
 *
 * `line` as a count of lines was the obvious answer and does not work: it snaps
 * to a line height the browser picks, and at this type size a two-line cue still
 * sat under the scrub bar at -4. `lineAlign: 'end'` would anchor the bottom of
 * the cue and is the right idea, but Chrome ignores it — verified, the box
 * stayed anchored by its top. So the position is computed instead.
 *
 * What it buys is worth being precise about. Measured at 1080p on a one-line
 * cue, it goes from 79.06% with the bar up to 90.76% with it down: an 11.7-point
 * lift, which is the bar's own height. The engine then maps that request through
 * a safe area of its own, so the absolute resting position is Chrome's to decide
 * and lands in the lower third, where a subtitle belongs. What this controls
 * reliably — and all it needs to control — is that the text moves out of the way
 * of the furniture and comes back when the furniture goes.
 */
export function cueLine({ floor, area, lines, lineHeight, gap }) {
	if (!area) return 0;
	const top = floor - lines * lineHeight - gap;
	return Math.max(0, Math.min(95, (top / area) * 100));
}

/**
 * Whichever comes first: the bottom of the picture, or the top of the control
 * bar when it is up.
 */
export function cueFloor({ area, picture, bar }) {
	const pictureBottom = area - (area - picture) / 2;
	return Math.min(pictureBottom, area - bar);
}
