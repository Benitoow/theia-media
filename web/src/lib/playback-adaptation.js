function transcodeHeights(info) {
	return (info?.qualities ?? [])
		.filter((quality) => quality.mode === 'transcode' && quality.height > 0)
		.map((quality) => quality.height)
		.sort((a, b) => b - a);
}

// A 4K HDR compatibility conversion is not a free 4K path just because the
// final H.264 encode runs on a GPU. The current colour conversion runs on the
// CPU; scaling before it is the measured way to create useful headroom.
export function initialCompatibilityHeight(info) {
	if (!info?.tone_map || !(info.height > 1080)) return null;
	return transcodeHeights(info).find((height) => height <= 1080) ?? null;
}

// Three stalls are evidence that the current rung is too ambitious. Move one
// rung only: a transient problem must not throw a 4K film straight to 480p.
export function nextLowerHeight(info, currentHeight = null) {
	const current = currentHeight || info?.height || Infinity;
	return transcodeHeights(info).find((height) => height < current) ?? null;
}
