import { browser } from '$app/environment';

// What this browser has proved it cannot play smoothly.
//
// The server cannot answer this and neither can any browser API. Both
// canPlayType and mediaCapabilities.decodingInfo report HEVC Main 10 as
// supported, smooth and power-efficient on a machine where the picture then
// runs at a fraction of real time while the sound keeps perfect pace. Measured;
// see decision 36. The only honest signal is the playback itself, so the player
// measures it and stores the verdict here.
//
// It belongs to the browser rather than to the server or the profile, for the
// same reason the interface language does (decision 32): the television and the
// laptop have different GPUs and different builds of Chrome, and a verdict
// reached on one would be a lie on the other.

const storageKey = 'theia.codec-playback';

function read() {
	if (!browser) return {};
	try {
		const parsed = JSON.parse(localStorage.getItem(storageKey) ?? '{}');
		return parsed && typeof parsed === 'object' ? parsed : {};
	} catch {
		// Corrupt or denied. An unreadable verdict is the same as no verdict:
		// try the lossless path and measure again.
		return {};
	}
}

class CodecPlayback {
	/** @type {Record<string, { smooth: boolean, at: string }>} */
	verdicts = $state({});

	constructor() {
		this.verdicts = read();
	}

	/** True once this browser has failed to keep up with the codec. */
	strugglesWith(codec) {
		if (!codec) return false;
		return this.verdicts[codec.toLowerCase()]?.smooth === false;
	}

	/** Recorded once, by the player, after it has measured a real playback. */
	recordStruggle(codec) {
		if (!codec) return;
		const key = codec.toLowerCase();
		if (this.verdicts[key]?.smooth === false) return;
		this.verdicts = { ...this.verdicts, [key]: { smooth: false, at: new Date().toISOString() } };
		this.persist();
	}

	get count() {
		return Object.keys(this.verdicts).length;
	}

	/** The way back, for the day a driver or a browser update fixes it. */
	forget() {
		this.verdicts = {};
		if (!browser) return;
		try {
			localStorage.removeItem(storageKey);
		} catch {
			// Private mode denies storage. Nothing was persisted to remove.
		}
	}

	persist() {
		if (!browser) return;
		try {
			localStorage.setItem(storageKey, JSON.stringify(this.verdicts));
		} catch {
			// The verdict still holds for this page; only its memory is lost.
		}
	}
}

export const codecPlayback = new CodecPlayback();
