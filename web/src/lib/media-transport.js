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

export function attachMedia(video, url, onFailure) {
	const controller = new AbortController(), { signal } = controller;
	let objectURL;
	const fragmented = url.includes('/remux?');
	if (!fragmented || typeof MediaSource === 'undefined') {
		video.src = url;
		return () => { video.removeAttribute('src'); video.load(); };
	}
	const run = async () => {
		let response;
		for (let attempt = 0; attempt < 6; attempt++) {
			response = await fetch(url, { signal });
			if (response.ok) break;
			const body = await response.json().catch(() => ({}));
			if (body.error !== 'transcode_busy' || attempt === 5) throw new Error(body.error || 'stream_unavailable');
			await sleep(250 * (attempt + 1), signal);
		}
		const reader = response.body.getReader();
		let init = new Uint8Array();
		while (!hasInitialization(init)) {
			const { value, done } = await reader.read();
			if (done || init.length > 4 * 1024 * 1024) throw new Error('stream_encode_failed');
			const next = new Uint8Array(init.length + value.length); next.set(init); next.set(value,init.length); init = next;
		}
		const mime = initializationMIME(init);
		if (!MediaSource.isTypeSupported(mime)) throw new Error('browser_cannot_decode_video');
		const media = new MediaSource();
		objectURL = URL.createObjectURL(media);
		const opened = waitFor(media,'sourceopen',signal);
		video.src = objectURL;
		await opened;
		const buffer = media.addSourceBuffer(mime);
		const append = async (bytes) => {
			const updated = waitFor(buffer,'updateend',signal);
			try { buffer.appendBuffer(bytes); } catch (error) { controller.abort(); await updated.catch(()=>{}); throw error; }
			await updated;
		};
		await append(init);
		void video.play().catch(() => {});
		while (!signal.aborted) {
			const ranges = buffer.buffered;
			if (ranges.length && ranges.end(ranges.length-1) - video.currentTime > 25) { await sleep(250,signal); continue; }
			if (ranges.length && video.currentTime - ranges.start(0) > 15) {
				const updated = waitFor(buffer,'updateend',signal);
				buffer.remove(0, video.currentTime - 5); await updated;
			}
			const { value, done } = await reader.read();
			if (done) { if (media.readyState === 'open') media.endOfStream(); break; }
			await append(value);
		}
	};
	void run().catch((error) => {
		if (error.name !== 'AbortError') onFailure(error.message || 'stream_unavailable');
		controller.abort();
	});
	return () => { controller.abort(); video.removeAttribute('src'); video.load(); if (objectURL) URL.revokeObjectURL(objectURL); };
}
