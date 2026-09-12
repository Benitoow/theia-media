// A paused fragmented stream still owns a fetch, a SourceBuffer and usually an
// ffmpeg process. Keep a short pause instant to resume, then release the pipe:
// two minutes covers interruptions without letting an abandoned television
// transcode indefinitely or block an update all evening.
export const pausedStreamReleaseMilliseconds = 2 * 60 * 1000;

export class PlaybackLifecycle {
	constructor({
		heartbeat,
		release,
		releaseDelay = pausedStreamReleaseMilliseconds,
		setIntervalFn = globalThis.setInterval.bind(globalThis),
		clearIntervalFn = globalThis.clearInterval.bind(globalThis),
		setTimeoutFn = globalThis.setTimeout.bind(globalThis),
		clearTimeoutFn = globalThis.clearTimeout.bind(globalThis)
	}) {
		this.heartbeat = heartbeat;
		this.release = release;
		this.releaseDelay = releaseDelay;
		this.setInterval = setIntervalFn;
		this.clearInterval = clearIntervalFn;
		this.setTimeout = setTimeoutFn;
		this.clearTimeout = clearTimeoutFn;
		this.heartbeatTimer = null;
		this.releaseTimer = null;
	}

	update({ phase, paused, fragmented, started = true }) {
		const shouldHeartbeat = phase === 'playing' && !paused;
		if (shouldHeartbeat && this.heartbeatTimer === null) {
			this.heartbeat();
			this.heartbeatTimer = this.setInterval(this.heartbeat, 15_000);
		} else if (!shouldHeartbeat && this.heartbeatTimer !== null) {
			this.clearInterval(this.heartbeatTimer);
			this.heartbeatTimer = null;
		}

		// A transport can be ready while autoplay is still waiting for the first
		// frame. That is startup, not a pause: only arm the release after this
		// player has actually entered playback at least once.
		const shouldRelease = phase === 'playing' && paused && fragmented && started;
		if (shouldRelease && this.releaseTimer === null) {
			this.releaseTimer = this.setTimeout(() => {
				this.releaseTimer = null;
				this.release();
			}, this.releaseDelay);
		} else if (!shouldRelease && this.releaseTimer !== null) {
			this.clearTimeout(this.releaseTimer);
			this.releaseTimer = null;
		}
	}

	destroy() {
		if (this.heartbeatTimer !== null) this.clearInterval(this.heartbeatTimer);
		if (this.releaseTimer !== null) this.clearTimeout(this.releaseTimer);
		this.heartbeatTimer = null;
		this.releaseTimer = null;
	}
}
