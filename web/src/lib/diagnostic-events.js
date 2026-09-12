export function diagnosticClientContext(surface = globalThis) {
	const nav = /** @type {any} */ (surface.navigator ?? {});
	const screen = /** @type {any} */ (surface.screen ?? {});
	const connection = /** @type {any} */ (
		nav.connection ?? nav.mozConnection ?? nav.webkitConnection ?? {}
	);
	return {
		user_agent: nav.userAgent ?? '',
		platform: nav.userAgentData?.platform ?? nav.platform ?? '',
		language: nav.language ?? '',
		hardware_concurrency: finite(nav.hardwareConcurrency),
		device_memory_gb: finite(nav.deviceMemory),
		screen_width: finite(screen.width),
		screen_height: finite(screen.height),
		pixel_ratio: finite(surface.devicePixelRatio),
		online: typeof nav.onLine === 'boolean' ? nav.onLine : undefined,
		connection_type: connection.effectiveType ?? connection.type ?? '',
		downlink_mbps: finite(connection.downlink),
		save_data: typeof connection.saveData === 'boolean' ? connection.saveData : undefined
	};
}

export function diagnosticSessionID(surface = globalThis) {
	if (typeof surface.crypto?.randomUUID === 'function') return surface.crypto.randomUUID();
	return `playback-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`;
}

export function playbackSnapshot(video) {
	if (!video) return {};
	let bufferedSeconds = 0;
	for (let index = 0; index < video.buffered.length; index++) {
		if (video.currentTime >= video.buffered.start(index) && video.currentTime <= video.buffered.end(index)) {
			bufferedSeconds = Math.max(0, video.buffered.end(index) - video.currentTime);
			break;
		}
	}
	const frames = playbackFrameCounters(video);
	return {
		position_seconds: finite(video.currentTime),
		buffered_seconds: finite(bufferedSeconds),
		ready_state: finite(video.readyState),
		total_frames: finite(frames.totalFrames),
		dropped_frames: finite(frames.droppedFrames)
	};
}

// Safari exposed frame counters before it implemented Chromium's
// getVideoPlaybackQuality shape. Keep the playback verdict measurable on both
// APIs; a missing counter remains missing rather than being invented as zero.
export function playbackFrameCounters(video) {
	const quality = video?.getVideoPlaybackQuality?.();
	return {
		totalFrames: quality?.totalVideoFrames ?? video?.webkitDecodedFrameCount,
		droppedFrames: quality?.droppedVideoFrames ?? video?.webkitDroppedFrameCount
	};
}

// Diagnostics stay on the Theia server. A reporting failure must never become
// a playback failure, so this is intentionally fire-and-forget.
export function reportDiagnostic(event, payload = {}, surface = globalThis) {
	if (typeof surface.fetch !== 'function') return;
	const body = JSON.stringify({
		event,
		client_time: new Date().toISOString(),
		client: diagnosticClientContext(surface),
		...payload
	});
	void surface.fetch('/api/diagnostics/events', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body,
		keepalive: true
	}).catch(() => {});
}

function finite(value) {
	const number = Number(value);
	return Number.isFinite(number) && number >= 0 ? number : undefined;
}
