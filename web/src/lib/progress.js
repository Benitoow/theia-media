// When a playback position is worth writing.
//
// The rule exists because a position of zero is not a position, it is the
// absence of one: writing it erases the row, and the viewer loses the place
// they were at. Measured on 20 September 2026 - a web session that closed
// before it had played anything left `position 0, watched_at 0` where 7500
// seconds of a film used to be, and the whole row had to be restored by hand.
//
// The native player has carried this floor from its first version:
//
//   if position <= 0.5 { return; } // Nothing has been watched yet; a zero
//                                  // would erase a position.
//
// The web player is the fallback path and did not, which is the wrong way round:
// the fallback is the one more likely to be closed early, on a television, by
// somebody who opened a film and changed their mind. It calls this now.
//
// "Start from the beginning" is a different intent and has its own route -
// `DELETE .../progress` - so nothing here has to make a zero mean "reset".

/** The position below which a save is the absence of one. */
export const PROGRESS_FLOOR = 0.5;

/** How far the clock must move before an unforced save is worth a request. */
export const PROGRESS_INTERVAL = 5;

/**
 * The seconds to write, or null when this position should not be written.
 *
 * `force` is the "the player is closing" path: it bypasses the interval, never
 * the floor. It used to bypass everything, which is how a close at zero erased
 * a row.
 */
export function progressWrite(position, lastSaved, { force = false } = {}) {
	if (!Number.isFinite(position) || position <= PROGRESS_FLOOR) return null;
	if (!force && Number.isFinite(lastSaved) && Math.abs(position - lastSaved) < PROGRESS_INTERVAL) return null;
	return position;
}
