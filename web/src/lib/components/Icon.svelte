<script>
	// Every icon the interface uses, drawn here rather than pulled from a pack.
	//
	// Two reasons. An icon library ships far more than the dozen glyphs actually
	// needed, and this project contacts no CDN, so it would have to be vendored
	// anyway. And one hand-kept set stays on a single grid with a single stroke
	// weight, which is what makes a control bar look deliberate rather than
	// assembled from three sources.
	//
	// 24-unit grid, 1.7 stroke, round caps and joins. Anything added later
	// follows that, or the bar stops looking like one family.

	let { name, size = 22, filled = false, label = null, class: extra = '' } = $props();

	/** @type {Record<string, string | string[]>} */
	const glyphs = {
		// Filled, because a play triangle drawn as an outline reads as a hollow
		// shape rather than a button.
		play: 'M8 5.14v13.72a.6.6 0 0 0 .92.5l10.78-6.86a.6.6 0 0 0 0-1L8.92 4.64a.6.6 0 0 0-.92.5Z',
		pause: ['M9.5 4.75v14.5', 'M14.5 4.75v14.5'],

		// Feather's rotate-ccw / rotate-cw. The seconds are written beside the
		// icon rather than inside it: legible at 22px, which "10" in a circle
		// is not.
		back10: ['M1.6 4.4v6h6', 'M4.1 15a9 9 0 1 0 2.1-9.4L1.6 10'],
		forward10: ['M22.4 4.4v6h-6', 'M19.9 15a9 9 0 1 1-2.1-9.4l4.6 4.4'],

		volumeHigh: [
			'M11 5.2 6.4 9.3H3.2v5.4h3.2L11 18.8V5.2Z',
			'M15.6 8.7a4.7 4.7 0 0 1 0 6.6',
			'M18.6 5.7a8.9 8.9 0 0 1 0 12.6',
		],
		volumeLow: ['M11 5.2 6.4 9.3H3.2v5.4h3.2L11 18.8V5.2Z', 'M15.6 8.7a4.7 4.7 0 0 1 0 6.6'],
		volumeMuted: ['M11 5.2 6.4 9.3H3.2v5.4h3.2L11 18.8V5.2Z', 'M16.2 9.6 21 14.4', 'M21 9.6l-4.8 4.8'],

		fullscreen: ['M3.6 8.8V3.6h5.2', 'M20.4 8.8V3.6h-5.2', 'M3.6 15.2v5.2h5.2', 'M20.4 15.2v5.2h-5.2'],
		fullscreenExit: ['M8.8 3.6v5.2H3.6', 'M15.2 3.6v5.2h5.2', 'M8.8 20.4v-5.2H3.6', 'M15.2 20.4v-5.2h5.2'],

		close: ['M5.8 5.8 18.2 18.2', 'M18.2 5.8 5.8 18.2'],
		back: 'M15 4.6 7.6 12 15 19.4',
		chevronLeft: 'M15 4.6 7.6 12 15 19.4',
		chevronRight: 'M9 4.6 16.4 12 9 19.4',
		chevronDown: 'M4.6 9 12 16.4 19.4 9',

		search: ['M10.9 3.6a7.3 7.3 0 1 0 0 14.6 7.3 7.3 0 0 0 0-14.6Z', 'M16.2 16.2 20.4 20.4'],
		sort: ['M3.6 6.4h16.8', 'M3.6 12h10.8', 'M3.6 17.6h5.6'],
		grid: ['M3.6 3.6h7v7h-7Z', 'M13.4 3.6h7v7h-7Z', 'M3.6 13.4h7v7h-7Z', 'M13.4 13.4h7v7h-7Z'],
		rows: ['M3.6 5.4h16.8', 'M3.6 12h16.8', 'M3.6 18.6h16.8'],
		check: 'M4.6 12.4 9.6 17.4 19.4 6.6',
		plus: ['M12 4.6v14.8', 'M4.6 12h14.8'],
		filter: 'M3.6 4.6h16.8l-6.5 7.7v6.1l-3.8 2.4v-8.5Z',
		info: ['M12 3.6a8.4 8.4 0 1 0 0 16.8 8.4 8.4 0 0 0 0-16.8Z', 'M12 11v5.2', 'M12 7.8h.01'],

		// A cog, for the panel that holds audio, subtitles and quality together.
		// The button used to wear the closed-caption mark, which says
		// "subtitles" and nothing about the other two, so the audio and quality
		// sections went unfound. Drawn here rather than lifted from a pack, like
		// everything else in this file: a ring, eight teeth on the 45s, and a
		// hub. Teeth reach r=8.7, which keeps the whole mark inside the 24 grid.
		settings: [
			'M12 5.4a6.6 6.6 0 1 0 0 13.2 6.6 6.6 0 0 0 0-13.2Z',
			'M12 9.2a2.8 2.8 0 1 0 0 5.6 2.8 2.8 0 0 0 0-5.6Z',
			'M18.6 12h2.1M5.4 12H3.3M12 5.4V3.3M12 18.6v2.1',
			'M16.67 7.33l1.48-1.48M7.33 16.67l-1.48 1.48M16.67 16.67l1.48 1.48M7.33 7.33 5.85 5.85'
		],

		// The three destinations a phone's tab bar needs. Same 24-unit grid and
		// stroke as everything above: a bar of glyphs from three sources is the
		// thing this file exists to prevent.
		//
		// A house, a film strip and a television. Each is the most literal shape
		// for its destination, because a tab bar is read at a glance from the
		// corner of the eye and is not the place to be inventive.
		home: ['M3.4 10.6 12 3.8l8.6 6.8', 'M5.9 9.4v9.8a1 1 0 0 0 1 1h10.2a1 1 0 0 0 1-1V9.4'],

		film: [
			'M3.6 5.2h16.8a1 1 0 0 1 1 1v11.6a1 1 0 0 1-1 1H3.6a1 1 0 0 1-1-1V6.2a1 1 0 0 1 1-1Z',
			'M8.2 5.2v13.6M15.8 5.2v13.6'
		],

		series: [
			'M4.2 7.8h15.6a1 1 0 0 1 1 1v9.4a1 1 0 0 1-1 1H4.2a1 1 0 0 1-1-1V8.8a1 1 0 0 1 1-1Z',
			'M8.4 3.4 12 7.8l3.6-4.4'
		],

		// The closed-caption mark: the one glyph everybody already reads as
		// "subtitles", which matters more here than drawing something prettier.
		captions: [
			'M3.4 5.4h17.2a1 1 0 0 1 1 1v11.2a1 1 0 0 1-1 1H3.4a1 1 0 0 1-1-1V6.4a1 1 0 0 1 1-1Z',
			'M10.1 10.2a2.7 2.7 0 1 0 0 3.6',
			'M17.3 10.2a2.7 2.7 0 1 0 0 3.6'
		],
		help: [
			'M12 3.6a8.4 8.4 0 1 0 0 16.8 8.4 8.4 0 0 0 0-16.8Z',
			'M9.7 9.2a2.45 2.45 0 1 1 3.45 2.24c-.72.38-1.15.92-1.15 1.76v.35',
			'M12 17.15h.01'
		],
	};

	const strokes = $derived(
		(() => {
			const value = glyphs[name];
			if (!value) return [];
			return Array.isArray(value) ? value : [value];
		})(),
	);

	// A play triangle is the one glyph that has to be solid.
	const solid = $derived(filled || name === 'play');
</script>

{#if strokes.length}
	<svg
		width={size}
		height={size}
		viewBox="0 0 24 24"
		fill={solid ? 'currentColor' : 'none'}
		stroke={solid ? 'none' : 'currentColor'}
		stroke-width="1.7"
		stroke-linecap="round"
		stroke-linejoin="round"
		aria-hidden={label ? undefined : 'true'}
		aria-label={label}
		role={label ? 'img' : undefined}
		class={extra}
	>
		{#each strokes as d (d)}
			<path {d} />
		{/each}
	</svg>
{/if}
