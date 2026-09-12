import { test } from 'node:test';
import assert from 'node:assert/strict';

import {
	diagnosticClientContext,
	playbackFrameCounters,
	playbackSnapshot,
	reportDiagnostic
} from '../../src/lib/diagnostic-events.js';

test('diagnostic context records useful browser facts without storage or URLs', () => {
	const context = diagnosticClientContext({
		navigator: {
			userAgent: 'Fixture Browser',
			platform: 'Fixture OS',
			language: 'fr-FR',
			hardwareConcurrency: 16,
			deviceMemory: 8,
			onLine: true,
			connection: { effectiveType: '4g', downlink: 42, saveData: false }
		},
		screen: { width: 1920, height: 1080 },
		devicePixelRatio: 1.25
	});
	assert.deepEqual(context, {
		user_agent: 'Fixture Browser', platform: 'Fixture OS', language: 'fr-FR',
		hardware_concurrency: 16, device_memory_gb: 8,
		screen_width: 1920, screen_height: 1080, pixel_ratio: 1.25,
		online: true, connection_type: '4g', downlink_mbps: 42, save_data: false
	});
	assert.equal('href' in context, false);
});

test('Safari frame counters remain observable without getVideoPlaybackQuality', () => {
	assert.deepEqual(
		playbackFrameCounters({ webkitDecodedFrameCount: 240, webkitDroppedFrameCount: 3 }),
		{ totalFrames: 240, droppedFrames: 3 }
	);
});

test('playback snapshot measures only the range containing the current frame', () => {
	const snapshot = playbackSnapshot({
		currentTime: 12,
		readyState: 4,
		buffered: { length: 2, start: (i) => [0, 10][i], end: (i) => [5, 20][i] },
		getVideoPlaybackQuality: () => ({ totalVideoFrames: 300, droppedVideoFrames: 2 })
	});
	assert.equal(snapshot.buffered_seconds, 8);
	assert.equal(snapshot.total_frames, 300);
	assert.equal(snapshot.dropped_frames, 2);
});

test('reporting a diagnostic is local, structured and non-blocking', async () => {
	let request;
	const surface = {
		navigator: {}, screen: {},
		fetch: async (url, options) => { request = { url, options }; }
	};
	reportDiagnostic('playback_waiting', { playback: { item_id: 7 } }, surface);
	await new Promise((resolve) => setImmediate(resolve));
	assert.equal(request.url, '/api/diagnostics/events');
	assert.equal(request.options.keepalive, true);
	assert.equal(JSON.parse(request.options.body).playback.item_id, 7);
});
