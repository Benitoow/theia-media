import { test } from 'node:test';
import assert from 'node:assert/strict';

import {
	compatibilityRows,
	defaultAudioTrack,
	probePlaybackEnvironment
} from '../../src/lib/playback-compatibility.js';

const media = (video = { codec: 'h264' }, audio_tracks = []) => ({
	status: 'ok',
	video,
	audio_tracks
});

test('an uninspected file is never presented as compatible', () => {
	assert.deepEqual(
		compatibilityRows({ media: { status: 'pending' }, plan: null }),
		[{ feature: 'analysis', status: 'unknown', detail: 'analysisRequired' }]
	);
});

test('direct play keeps a browser codec claim distinct from playback proof', () => {
	const [video] = compatibilityRows({
		media: media(),
		plan: { mode: 'direct', video_codec: 'h264' },
		environment: { codecSupport: 'probably' }
	});
	assert.equal(video.status, 'reported');
	assert.equal(video.detail, 'directReported');
});

test('a measured risky-codec failure reports the automatic conversion', () => {
	const [video] = compatibilityRows({
		media: media({ codec: 'hevc' }),
		plan: {
			mode: 'remux',
			video_codec: 'hevc',
			video_risky: true,
			transcode: { available: true, kind: 'hardware' }
		},
		browserStruggles: true
	});
	assert.equal(video.status, 'adapted');
	assert.equal(video.detail, 'measuredFallback');
	assert.equal(video.kind, 'hardware');
});

test('HDR display reporting remains a report rather than a verified claim', () => {
	const rows = compatibilityRows({
		media: media({ codec: 'hevc', color_transfer: 'smpte2084' }),
		plan: { mode: 'remux', video_codec: 'hevc', video_risky: true },
		environment: { codecSupport: 'probably', hdrDisplay: true }
	});
	const hdr = rows.find((row) => row.feature === 'hdr');
	assert.equal(hdr.status, 'reported');
	assert.equal(hdr.detail, 'hdrDisplayReported');
});

test('video conversion states exactly what happens to HDR and Dolby Vision', () => {
	const rows = compatibilityRows({
		media: media({
			codec: 'hevc',
			color_transfer: 'smpte2084',
			dolby_vision: true
		}),
		plan: { mode: 'transcode', video_codec: 'hevc', transcode: { kind: 'software' } }
	});
	assert.equal(rows.find((row) => row.feature === 'hdr').detail, 'hdrToneMapped');
	assert.equal(
		rows.find((row) => row.feature === 'dolbyVision').detail,
		'dolbyVisionConverted'
	);
});

test('Atmos names the unknown speaker chain unless audio conversion is certain', () => {
	const source = media(
		{ codec: 'h264' },
		[
			{ codec: 'aac', profile: '', is_default: false },
			{ codec: 'eac3', profile: 'Dolby Atmos', is_default: true }
		]
	);
	assert.equal(defaultAudioTrack(source).codec, 'eac3');

	const direct = compatibilityRows({ media: source, plan: { mode: 'direct' } });
	assert.equal(direct.find((row) => row.feature === 'atmos').detail, 'atmosUnknown');

	const converted = compatibilityRows({
		media: source,
		plan: { mode: 'remux', reason_code: 'audio_transcode' }
	});
	assert.equal(converted.find((row) => row.feature === 'atmos').detail, 'atmosConverted');
});

test('the environment probe reports only the APIs the browser exposed', () => {
	const surface = {
		matchMedia: () => ({ matches: true }),
		document: { createElement: () => ({ canPlayType: () => 'maybe' }) }
	};
	assert.deepEqual(probePlaybackEnvironment('hevc', surface), {
		hdrDisplay: true,
		codecSupport: 'maybe'
	});
});
