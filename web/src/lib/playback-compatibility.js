// Compatibility is assembled from separate sources instead of flattened into
// one optimistic yes/no. File inspection knows the encode, the server knows
// how it will deliver it, the browser can report a capability, and only actual
// playback can prove that the report was true on this device.

const codecMIMEs = {
	h264: ['video/mp4; codecs="avc1.42E01E"'],
	avc: ['video/mp4; codecs="avc1.42E01E"'],
	hevc: ['video/mp4; codecs="hvc1"', 'video/mp4; codecs="hev1"'],
	h265: ['video/mp4; codecs="hvc1"', 'video/mp4; codecs="hev1"'],
	av1: ['video/mp4; codecs="av01.0.05M.08"'],
	vp9: ['video/webm; codecs="vp9"'],
	vp8: ['video/webm; codecs="vp8"']
};

function normaliseCodec(codec) {
	return (codec ?? '').trim().toLowerCase();
}

export function defaultAudioTrack(media) {
	const tracks = media?.audio_tracks ?? [];
	return tracks.find((track) => track.is_default) ?? tracks[0] ?? null;
}

// canPlayType is deliberately reported as a browser claim. It has returned
// "probably" for HEVC on hardware that then decoded far below real time, which
// is why the player still measures risky codecs during actual playback.
export function probePlaybackEnvironment(codec, surface = globalThis) {
	let hdrDisplay = null;
	try {
		if (typeof surface?.matchMedia === 'function') {
			hdrDisplay = Boolean(surface.matchMedia('(dynamic-range: high)').matches);
		}
	} catch {
		// An absent or denied media query is unknown, never SDR by deduction.
	}

	let codecSupport = 'unknown';
	const probes = codecMIMEs[normaliseCodec(codec)] ?? [];
	try {
		const video = surface?.document?.createElement?.('video');
		if (video?.canPlayType && probes.length) {
			const answers = probes.map((mime) => video.canPlayType(mime));
			codecSupport = answers.includes('probably')
				? 'probably'
				: answers.includes('maybe')
					? 'maybe'
					: 'unsupported';
		}
	} catch {
		// Capability APIs can be missing or blocked on older television browsers.
	}

	return { hdrDisplay, codecSupport };
}

function videoRow(plan, codec, codecSupport, browserStruggles) {
	if (!plan) return { feature: 'video', status: 'unknown', detail: 'planUnavailable', codec };

	if (browserStruggles && plan.video_risky) {
		if (plan.transcode?.available) {
			return {
				feature: 'video',
				status: 'adapted',
				detail: 'measuredFallback',
				codec,
				kind: plan.transcode.kind ?? ''
			};
		}
		return { feature: 'video', status: 'measured', detail: 'measuredNoFallback', codec };
	}

	switch (plan.mode) {
		case 'direct':
			return codecSupport === 'probably' || codecSupport === 'maybe'
				? { feature: 'video', status: 'reported', detail: 'directReported', codec }
				: { feature: 'video', status: 'planned', detail: 'directPlanned', codec };
		case 'remux':
			if (plan.video_risky) {
				return codecSupport === 'probably' || codecSupport === 'maybe'
					? { feature: 'video', status: 'reported', detail: 'riskyReported', codec }
					: { feature: 'video', status: 'unknown', detail: 'riskyUnknown', codec };
			}
			return {
				feature: 'video',
				status: 'adapted',
				detail: plan.reason_code === 'audio_transcode' ? 'audioContainerAdapted' : 'containerAdapted',
				codec
			};
		case 'transcode':
			return {
				feature: 'video',
				status: 'adapted',
				detail: 'videoTranscoded',
				codec,
				kind: plan.transcode?.kind ?? ''
			};
		case 'unsupported':
			return { feature: 'video', status: 'unavailable', detail: 'videoUnavailable', codec };
		default:
			return { feature: 'video', status: 'unknown', detail: 'planUnavailable', codec };
	}
}

/**
 * @param {{
 *   media: any,
 *   plan: any,
 *   environment?: { hdrDisplay?: boolean | null, codecSupport?: string },
 *   browserStruggles?: boolean,
 *   loading?: boolean
 * }} input
 */
export function compatibilityRows({
	media,
	plan,
	environment = {},
	browserStruggles = false,
	loading = false
}) {
	if (!media || media.status !== 'ok') {
		return [
			{
				feature: 'analysis',
				status: 'unknown',
				detail: media?.status === 'error' ? 'analysisFailed' : 'analysisRequired'
			}
		];
	}

	if (loading) {
		return [{ feature: 'video', status: 'checking', detail: 'checking' }];
	}

	const codec = normaliseCodec(plan?.video_codec || media.video?.codec);
	/** @type {Array<{ feature: string, status: string, detail: string, codec?: string, kind?: string }>} */
	const rows = [
		videoRow(plan, codec, environment.codecSupport ?? 'unknown', browserStruggles)
	];
	const transfer = (media.video?.color_transfer ?? '').toLowerCase();
	const hasHDR = transfer === 'smpte2084' || transfer === 'arib-std-b67';
	const willTranscode =
		plan?.mode === 'transcode' ||
		(Boolean(browserStruggles) && Boolean(plan?.video_risky) && Boolean(plan?.transcode?.available));

	if (hasHDR) {
		rows.push(
			willTranscode
				? { feature: 'hdr', status: 'adapted', detail: 'hdrToneMapped' }
				: environment.hdrDisplay === true
					? { feature: 'hdr', status: 'reported', detail: 'hdrDisplayReported' }
					: environment.hdrDisplay === false
						? { feature: 'hdr', status: 'unknown', detail: 'hdrDisplayNotReported' }
						: { feature: 'hdr', status: 'unknown', detail: 'hdrDisplayUnknown' }
		);
	}

	if (media.video?.dolby_vision) {
		rows.push(
			willTranscode
				? { feature: 'dolbyVision', status: 'adapted', detail: 'dolbyVisionConverted' }
				: { feature: 'dolbyVision', status: 'unknown', detail: 'dolbyVisionUnknown' }
		);
	}

	const audio = defaultAudioTrack(media);
	if (/atmos/i.test(audio?.profile ?? '')) {
		rows.push(
			plan?.reason_code === 'audio_transcode'
				? { feature: 'atmos', status: 'adapted', detail: 'atmosConverted' }
				: { feature: 'atmos', status: 'unknown', detail: 'atmosUnknown' }
		);
	}

	return rows;
}
