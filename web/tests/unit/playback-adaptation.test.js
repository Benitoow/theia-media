import { test } from 'node:test';
import assert from 'node:assert/strict';

import {
	initialCompatibilityHeight,
	nextLowerHeight
} from '../../src/lib/playback-adaptation.js';

const info = {
	height: 1604,
	tone_map: true,
	qualities: [
		{ height: 0, mode: 'remux' },
		{ height: 1080, mode: 'transcode' },
		{ height: 720, mode: 'transcode' },
		{ height: 480, mode: 'transcode' }
	]
};

test('a 4K HDR compatibility conversion starts at the highest rung up to 1080p', () => {
	assert.equal(initialCompatibilityHeight(info), 1080);
});

test('SDR and already-small sources keep their original resolution', () => {
	assert.equal(initialCompatibilityHeight({ ...info, tone_map: false }), null);
	assert.equal(initialCompatibilityHeight({ ...info, height: 804 }), null);
});

test('repeated stalls step down exactly one available rung', () => {
	assert.equal(nextLowerHeight(info, null), 1080);
	assert.equal(nextLowerHeight(info, 1080), 720);
	assert.equal(nextLowerHeight(info, 720), 480);
	assert.equal(nextLowerHeight(info, 480), null);
});
