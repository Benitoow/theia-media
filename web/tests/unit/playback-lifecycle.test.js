import { test } from 'node:test';
import assert from 'node:assert/strict';

import {
	PlaybackLifecycle,
	pausedStreamReleaseMilliseconds
} from '../../src/lib/playback-lifecycle.js';

function fixture() {
	const intervals = new Map();
	const timeouts = new Map();
	let sequence = 0;
	let heartbeats = 0;
	let releases = 0;
	const lifecycle = new PlaybackLifecycle({
		heartbeat: () => { heartbeats++; },
		release: () => { releases++; },
		setIntervalFn: (fn, delay) => { const id = ++sequence; intervals.set(id, { fn, delay }); return id; },
		clearIntervalFn: (id) => intervals.delete(id),
		setTimeoutFn: (fn, delay) => { const id = ++sequence; timeouts.set(id, { fn, delay }); return id; },
		clearTimeoutFn: (id) => timeouts.delete(id)
	});
	return { lifecycle, intervals, timeouts, heartbeats: () => heartbeats, releases: () => releases };
}

test('heartbeats describe active viewing, not a paused television', () => {
	const f = fixture();
	f.lifecycle.update({ phase: 'playing', paused: false, fragmented: true });
	assert.equal(f.heartbeats(), 1);
	assert.equal([...f.intervals.values()][0].delay, 15_000);

	f.lifecycle.update({ phase: 'playing', paused: true, fragmented: true });
	assert.equal(f.intervals.size, 0);
	assert.equal([...f.timeouts.values()][0].delay, pausedStreamReleaseMilliseconds);
});

test('only a fragmented stream is released after a long pause', () => {
	const f = fixture();
	f.lifecycle.update({ phase: 'playing', paused: true, fragmented: false });
	assert.equal(f.timeouts.size, 0);

	f.lifecycle.update({ phase: 'playing', paused: true, fragmented: true });
	const pending = [...f.timeouts.values()][0];
	pending.fn();
	assert.equal(f.releases(), 1);
	assert.equal(f.timeouts.size, 1); // the fake scheduler keeps fired jobs for inspection
});

test('transport startup is not mistaken for a long pause', () => {
	const f = fixture();
	f.lifecycle.update({ phase: 'playing', paused: true, fragmented: true, started: false });
	assert.equal(f.timeouts.size, 0);

	f.lifecycle.update({ phase: 'playing', paused: false, fragmented: true, started: true });
	f.lifecycle.update({ phase: 'playing', paused: true, fragmented: true, started: true });
	assert.equal(f.timeouts.size, 1);
});

test('resuming or closing cancels every pending lifecycle task', () => {
	const f = fixture();
	f.lifecycle.update({ phase: 'playing', paused: true, fragmented: true });
	f.lifecycle.update({ phase: 'playing', paused: false, fragmented: true });
	assert.equal(f.timeouts.size, 0);
	assert.equal(f.intervals.size, 1);
	f.lifecycle.destroy();
	assert.equal(f.intervals.size, 0);
});
