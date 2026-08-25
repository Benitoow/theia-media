// The geometry that decides where a subtitle sits. Every number it produces was
// measured in a real browser before it was written down; these tests pin the
// arithmetic so a refactor cannot quietly move the text back under the scrub bar.
import { test } from 'node:test';
import assert from 'node:assert/strict';

import {
	cueFloor,
	cueFontSize,
	cueLine,
	cueLineHeight,
	layerBottom,
	pictureBox
} from '../../src/lib/subtitle-layout.js';

test('the picture is not the element when the film is scope', () => {
	// Measured on a real 2.39:1 rip: 1280x536 painted inside a 1280x720 element,
	// with 92px of black above and below.
	const frame = pictureBox({ width: 1280, height: 720, videoWidth: 2390, videoHeight: 1000 });
	assert.equal(Math.round(frame.picture), 536);
	assert.equal(Math.round(frame.letterbox), 92);
});

test('a film with no bars is the same calculation with a zero letterbox', () => {
	const frame = pictureBox({ width: 1280, height: 720, videoWidth: 1920, videoHeight: 1080 });
	assert.equal(Math.round(frame.picture), 720);
	assert.equal(frame.letterbox, 0);
});

test('a stream that has not reported its size falls back to the element', () => {
	const frame = pictureBox({ width: 1280, height: 720, videoWidth: 0, videoHeight: 0 });
	assert.equal(frame.picture, 720);
	assert.equal(frame.letterbox, 0);
	assert.equal(pictureBox({ width: 1280, height: 0 }), null);
});

test('the layer sits inside the picture, and steps above the bar when there is one', () => {
	const noBar = layerBottom({ letterbox: 92, picture: 536, bar: 0 });
	const withBar = layerBottom({ letterbox: 92, picture: 536, bar: 120 });
	assert.ok(withBar > noBar, 'the controls must push the text up');
	assert.equal(Math.round(noBar), Math.round(92 + 536 * 0.04));
	assert.equal(Math.round(withBar), Math.round(120 + 536 * 0.04));
});

test('the floor is the bottom of the picture, or the top of the bar', () => {
	// Bar down: the picture decides.
	assert.equal(cueFloor({ area: 720, picture: 536, bar: 0 }), 720 - 92);
	// Bar up and taller than the letterbox: the bar decides.
	assert.equal(cueFloor({ area: 720, picture: 536, bar: 200 }), 520);
});

test('the cue font follows the viewport between two bounds', () => {
	assert.equal(cueFontSize(16, 400), 17); // the floor: 16 * 1.0625
	assert.equal(cueFontSize(16, 1280), 1280 * 0.026);
	assert.equal(cueFontSize(16, 4000), 38); // the ceiling: 16 * 2.375
	assert.equal(cueLineHeight(16, 1280), cueFontSize(16, 1280) * 2.2);
});

test('a two-line cue is lifted further than a one-line cue', () => {
	const shared = { floor: 600, area: 720, lineHeight: 40, gap: 10 };
	const one = cueLine({ ...shared, lines: 1 });
	const two = cueLine({ ...shared, lines: 2 });
	assert.ok(two < one, 'more lines means a higher top');
	assert.equal(one, ((600 - 40 - 10) / 720) * 100);
});

test('a cue is never asked for a position the engine will refuse', () => {
	assert.equal(cueLine({ floor: 10, area: 720, lines: 4, lineHeight: 40, gap: 10 }), 0);
	assert.equal(cueLine({ floor: 100000, area: 720, lines: 1, lineHeight: 40, gap: 10 }), 95);
	assert.equal(cueLine({ floor: 600, area: 0, lines: 1, lineHeight: 40, gap: 10 }), 0);
});
