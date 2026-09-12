// Fragmented MP4 is a sequential stream, not a byte-addressable file. Owning
// fetch prevents native Range retries from creating another HDR encoder.
const abortError = () => new DOMException('Playback replaced', 'AbortError');

function waitFor(target, event, signal) {
	return new Promise((resolve, reject) => {
		if (signal.aborted) return reject(abortError());
		const done = () => { cleanup(); resolve(); };
		const fail = () => { cleanup(); reject(signal.aborted ? abortError() : new Error('media_decode_failed')); };
		const cleanup = () => { target.removeEventListener(event, done); target.removeEventListener('error', fail); signal.removeEventListener('abort', fail); };
		target.addEventListener(event, done, { once: true });
		target.addEventListener('error', fail, { once: true });
		signal.addEventListener('abort', fail, { once: true });
	});
}

const sleep = (ms, signal) => new Promise((resolve, reject) => {
	if (signal.aborted) return reject(abortError());
	const cancel = () => { clearTimeout(timer); reject(abortError()); };
	const timer = setTimeout(() => { signal.removeEventListener('abort', cancel); resolve(); }, ms);
	signal.addEventListener('abort', cancel, { once: true });
});

function sourceBufferOperation(buffer, operation, signal) {
	return new Promise((resolve, reject) => {
		if (signal.aborted) return reject(abortError());
		const done = () => { cleanup(); resolve(); };
		const fail = () => { cleanup(); reject(signal.aborted ? abortError() : new Error('media_decode_failed')); };
		const cancel = () => { cleanup(); reject(abortError()); };
		const cleanup = () => {
			buffer.removeEventListener('updateend', done);
			buffer.removeEventListener('error', fail);
			signal.removeEventListener('abort', cancel);
		};
		buffer.addEventListener('updateend', done, { once: true });
		buffer.addEventListener('error', fail, { once: true });
		signal.addEventListener('abort', cancel, { once: true });
		try { operation(); } catch (error) { cleanup(); reject(error); }
	});
}

const appendBatchBytes = 1024 * 1024;
const initialBufferSeconds = 6;
const rollingBufferSeconds = 30;
const persistentQuotaMilliseconds = 15_000;

export function bufferTargetKey(bufferClass = 'source') {
	return `theia.playback.buffer.${String(bufferClass).replace(/[^a-z0-9_-]/gi, '') || 'source'}`;
}

export function rememberedRollingBufferSeconds(storage, key) {
	try {
		const value = Number(storage?.getItem(key));
		if (!Number.isFinite(value) || value < initialBufferSeconds) return rollingBufferSeconds;
		return Math.min(rollingBufferSeconds, value);
	} catch {
		return rollingBufferSeconds;
	}
}

function rememberRollingBufferSeconds(storage, key, value) {
	try { storage?.setItem(key, String(value)); } catch { /* private modes may refuse storage */ }
}

// MSE quotas are bytes, not seconds. Thirty seconds is small for an ordinary
// encode and over 200 MiB for the 55 Mb/s UHD remux that exposed this path in
// Edge. A quota hit halves the target until it fits, never below the six-second
// startup reserve. The pending batch is retained and retried after old media is
// removed; no part of the film is discarded.
export function reducedRollingBufferSeconds(currentTarget, bufferedAhead) {
	return Math.max(
		initialBufferSeconds,
		Math.min(currentTarget / 2, Math.max(initialBufferSeconds, bufferedAhead - 2))
	);
}

function join(chunks, length) {
	const joined = new Uint8Array(length);
	let offset = 0;
	for (const chunk of chunks) {
		joined.set(chunk, offset);
		offset += chunk.length;
	}
	return joined;
}

async function readBatch(reader, signal) {
	const chunks = [];
	let length = 0;
	let ended = false;
	while (length < appendBatchBytes && !ended) {
		if (signal.aborted) throw abortError();
		const { value, done } = await reader.read();
		ended = done;
		if (value?.length) {
			chunks.push(value);
			length += value.length;
		}
	}
	return { bytes: join(chunks, length), ended };
}

function aheadOf(video, buffer) {
	const ranges = buffer.buffered;
	if (!ranges.length) return 0;
	for (let index = 0; index < ranges.length; index++) {
		if (video.currentTime >= ranges.start(index) && video.currentTime <= ranges.end(index)) {
			return ranges.end(index) - video.currentTime;
		}
	}
	return 0;
}

// Read the actual AVC profile from the initialization segment, rather than
// assuming the source and hardware encoder produce the same profile/level.
export function initializationMIME(bytes) {
	let video = null, audio = null, hevcEntry = 'hvc1';
	const hex = (v) => v.toString(16).padStart(2, '0');
	for (let i = 4; i + 9 < bytes.length; i++) {
		const type = String.fromCharCode(...bytes.subarray(i, i + 4));
		if (type === 'avcC') video = `avc1.${hex(bytes[i+5])}${hex(bytes[i+6])}${hex(bytes[i+7])}`;
		if (type === 'hev1' || type === 'hvc1') hevcEntry = type;
		if (type === 'hvcC' && i + 17 <= bytes.length) {
			const profile = bytes[i + 5];
			let compatibility = 0;
			for (let bit = 0; bit < 32; bit++) {
				compatibility = (compatibility * 2) + ((bytes[i + 9 - Math.floor(bit / 8)] >> (bit % 8)) & 1);
			}
			const constraints = [...bytes.subarray(i + 10, i + 16)];
			while (constraints.length && constraints.at(-1) === 0) constraints.pop();
			video = `${hevcEntry}.${['', 'A', 'B', 'C'][profile >> 6]}${profile & 31}.${compatibility.toString(16).toUpperCase()}.${profile & 32 ? 'H' : 'L'}${bytes[i + 16]}`;
			if (constraints.length) video += '.' + constraints.map(v => hex(v).toUpperCase()).join('.');
		}
		if (type === 'mp4a') audio = 'mp4a.40.2';
		if (type === 'Opus') audio = 'opus';
	}
	if (!video) throw new Error('browser_cannot_decode_video');
	return `video/mp4; codecs="${[video,audio].filter(Boolean).join(', ')}"`;
}

function hasInitialization(bytes) {
	const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
	for (let offset = 0; offset + 8 <= bytes.length;) {
		const size = view.getUint32(offset);
		if (size < 8) throw new Error('stream_encode_failed');
		if (offset + size > bytes.length) return false;
		if (String.fromCharCode(...bytes.subarray(offset+4, offset+8)) === 'moov') return true;
		offset += size;
	}
	return false;
}

/**
 * @param {HTMLVideoElement} video
 * @param {string} url
 * @param {(code: string) => void} onFailure
 * @param {(status: 'preparing' | 'ready') => void} [onStatus]
 * @param {{bufferClass?: string, storage?: Storage, onBufferPressure?: (detail: object) => void}} [options]
 */
export function attachMedia(video, url, onFailure, onStatus = (_status) => {}, options = {}) {
	const controller = new AbortController(), { signal } = controller;
	let objectURL;
	const fragmented = url.includes('/remux?');
	const MediaSourceAPI = globalThis.MediaSource ?? globalThis.ManagedMediaSource;
	if (!fragmented || !MediaSourceAPI) {
		video.src = url;
		return () => { video.removeAttribute('src'); video.load(); };
	}
	let storage = options.storage;
	if (storage === undefined) {
		try { storage = globalThis.localStorage; } catch { storage = null; }
	}
	const targetKey = bufferTargetKey(options.bufferClass);
	const shouldAutoplay = video.autoplay;
	const run = async () => {
		onStatus('preparing');
		let response;
		for (let attempt = 0; attempt < 6; attempt++) {
			response = await fetch(url, { signal });
			if (response.ok) break;
			const body = await response.json().catch(() => ({}));
			if (body.error !== 'transcode_busy' || attempt === 5) throw new Error(body.error || 'stream_unavailable');
			await sleep(250 * (attempt + 1), signal);
		}
		if (!response?.body?.getReader) throw new Error('stream_unavailable');
		const reader = response.body.getReader();
		let init = new Uint8Array();
		while (!hasInitialization(init)) {
			const { value, done } = await reader.read();
			if (done || init.length > 4 * 1024 * 1024) throw new Error('stream_encode_failed');
			const next = new Uint8Array(init.length + value.length); next.set(init); next.set(value,init.length); init = next;
		}
		const mime = initializationMIME(init);
		if (!MediaSourceAPI.isTypeSupported(mime)) throw new Error('browser_cannot_decode_video');
		const media = new MediaSourceAPI();
		objectURL = URL.createObjectURL(media);
		video.autoplay = false;
		const opened = waitFor(media,'sourceopen',signal);
		video.src = objectURL;
		await opened;
		const buffer = media.addSourceBuffer(mime);
		const append = (bytes) => sourceBufferOperation(buffer, () => buffer.appendBuffer(bytes), signal);
		const evictBehind = async (keepSeconds) => {
			const ranges = buffer.buffered;
			if (!ranges.length) return false;
			const start = ranges.start(0);
			const end = video.currentTime - keepSeconds;
			if (end <= start + 0.25) return false;
			await sourceBufferOperation(buffer, () => buffer.remove(start, end), signal);
			return true;
		};
		await append(init);

		// A real-time encoder produces one playable fragment at a time. Starting on
		// the first two-second fragment made the decoder race the encoder forever;
		// six seconds is enough to absorb ordinary jitter without hoarding the film.
		let ended = false;
		let pending = null;
		let rollingTarget = rememberedRollingBufferSeconds(storage, targetKey);
		let quotaAtFloorSince = null;
		const recordQuotaPressure = (error) => {
			const previousTarget = rollingTarget;
			const bufferedAhead = aheadOf(video, buffer);
			rollingTarget = reducedRollingBufferSeconds(rollingTarget, aheadOf(video, buffer));
			if (rollingTarget < previousTarget) {
				rememberRollingBufferSeconds(storage, targetKey, rollingTarget);
				options.onBufferPressure?.({
					previous_buffer_target_seconds: previousTarget,
					buffer_target_seconds: rollingTarget,
					buffered_seconds: bufferedAhead
				});
			}
			if (rollingTarget > initialBufferSeconds) {
				quotaAtFloorSince = null;
				return;
			}
			quotaAtFloorSince ??= performance.now();
			if (performance.now() - quotaAtFloorSince >= persistentQuotaMilliseconds) throw error;
		};
		while (!signal.aborted && !ended && aheadOf(video, buffer) < initialBufferSeconds) {
			const batch = await readBatch(reader, signal);
			ended = batch.ended;
			if (!batch.bytes.length) continue;
			try {
				await append(batch.bytes);
			} catch (error) {
				if (error?.name !== 'QuotaExceededError') throw error;
				recordQuotaPressure(error);
				pending = batch;
				if (aheadOf(video, buffer) <= 0) throw error;
				break;
			}
		}
		video.autoplay = shouldAutoplay;
		onStatus('ready');
		if (shouldAutoplay) void video.play().catch(() => {});

		while (!signal.aborted) {
			if (ended && !pending) {
				if (media.readyState === 'open') media.endOfStream();
				break;
			}
			if (aheadOf(video, buffer) > rollingTarget) { await sleep(250,signal); continue; }
			const ranges = buffer.buffered;
			if (ranges.length && video.currentTime - ranges.start(0) > 15) {
				await evictBehind(5);
			}
			const batch = pending ?? await readBatch(reader, signal);
			ended = batch.ended;
			if (!batch.bytes.length) { pending = null; continue; }
			try {
				await append(batch.bytes);
				pending = null;
				quotaAtFloorSince = null;
			} catch (error) {
				if (error?.name !== 'QuotaExceededError') throw error;
				pending = batch;
				recordQuotaPressure(error);
				if (!(await evictBehind(2))) await sleep(250, signal);
			}
		}
	};
	void run().catch((error) => {
		if (error.name !== 'AbortError') {
			onFailure(error.name === 'QuotaExceededError' ? 'stream_buffer_full' : (error.message || 'stream_unavailable'));
		}
		controller.abort();
	});
	return () => { controller.abort(); video.autoplay = shouldAutoplay; video.removeAttribute('src'); video.load(); if (objectURL) URL.revokeObjectURL(objectURL); };
}
