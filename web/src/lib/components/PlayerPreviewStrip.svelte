<script>
	// The frames shown under the cursor while somebody drags the seek bar.
	//
	// Its own component because it is genuinely its own thing: it fetches its
	// sheet, measures it, and draws a window onto it, and none of that is
	// anybody else's business. The player hands it where to ask and where the
	// cursor is.
	//
	// A comfort, and it behaves like one (decision 71): asked for once, never
	// awaited, and silent when there is nothing to show.
	import { onMount } from 'svelte';
	import { getJSON, formatTime } from '$lib/api.js';

	let { base, hoverRatio = null, duration = 0 } = $props();

	let sheet = $state(null);
	let tileWidth = $state(0);

	const tile = $derived.by(() => {
		if (!sheet || hoverRatio === null || duration <= 0 || !tileWidth) return null;
		const index = Math.min(
			sheet.manifest.count - 1,
			Math.max(0, Math.floor((hoverRatio * duration) / sheet.manifest.interval_seconds))
		);
		const column = index % sheet.manifest.columns;
		const row = Math.floor(index / sheet.manifest.columns);
		const height = sheet.manifest.tile_height;
		return {
			width: tileWidth,
			height,
			// Background offsets are negative: the sheet is moved behind a window
			// the size of one tile, not cropped.
			x: -column * tileWidth,
			y: -row * height,
			sheetWidth: tileWidth * sheet.manifest.columns,
			sheetHeight: height * sheet.manifest.rows
		};
	});

	// The tile width is measured off the loaded sheet rather than sent: the
	// pinned ffmpeg build ships no ffprobe, so the server cannot say what the
	// source's aspect made it without guessing one.
	function onSheetLoaded(event) {
		const image = event.currentTarget;
		if (!sheet || !image.naturalWidth) return;
		tileWidth = Math.round(image.naturalWidth / sheet.manifest.columns);
	}

	onMount(async () => {
		try {
			const body = await getJSON(`${base}/preview`);
			if (body.state !== 'ready' || !body.manifest) return;
			sheet = body;
		} catch {
			// No preview for this file, and nothing to say about it.
		}
	});
</script>

{#if hoverRatio !== null && duration > 0}
	<span class="scrub-preview" style="left: {hoverRatio * 100}%">
		{#if tile}
			<span
				class="scrub-frame"
				style="width: {tile.width}px; height: {tile.height}px;
				       background-image: url({sheet.sheet_url});
				       background-size: {tile.sheetWidth}px {tile.sheetHeight}px;
				       background-position: {tile.x}px {tile.y}px"
			></span>
		{/if}
		{formatTime(hoverRatio * duration)}
	</span>
{/if}

<!-- Loaded off-screen purely to measure it. The tiles themselves are drawn as a
     background offset, which is one decode for the whole strip instead of one
     image element per frame. -->
{#if sheet}
	<img src={sheet.sheet_url} alt="" aria-hidden="true" class="scrub-sheet-probe" onload={onSheetLoaded} />
{/if}
