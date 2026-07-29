<script>
	import { imageURL, displayTitle, displayYear } from '$lib/api.js';

	// fluid lets the card fill a grid cell instead of carrying its own width.
	// Rows want a fixed size so a strip reads evenly; the library grid wants the
	// column to decide.
	let { movie, fluid = false, legend = null } = $props();

	// Landscape artwork, which is what a 16/9 card is for. TMDB's backdrop is
	// already that shape, so nothing is cropped to fit.
	const backdrop = $derived(imageURL(movie.metadata?.backdrop_path, 'w780'));

	// Only reached by a film that has a poster and no backdrop. On the 274-film
	// library that is nobody -- artwork arrives in pairs or not at all -- but a
	// portrait poster stretched to fill a landscape box would lose its title to
	// the crop, so it is contained rather than covered when it does happen.
	const poster = $derived(backdrop ? null : imageURL(movie.metadata?.poster_path, 'w342'));

	const title = $derived(displayTitle(movie));
	const year = $derived(displayYear(movie));
	const secondary = $derived(legend ?? year ?? '—');

	// Only drawn for a film actually part-watched. A bar at zero on every card
	// would be noise, and one on a finished film would be a lie.
	const watched = $derived(
		movie.progress?.position_seconds > 0 &&
			!movie.progress?.finished &&
			movie.progress?.duration_seconds > 0
	);
	const percent = $derived(
		watched
			? Math.min(100, (movie.progress.position_seconds / movie.progress.duration_seconds) * 100)
			: 0
	);
</script>

<!--
	Wide landscape cards, per §6 of the design system as rewritten. The card is
	still restrained -- no display serif, no gold at rest beyond the progress
	rule -- but it is no longer a bare 2:3 poster: it shows the film's own
	backdrop at 16/9, with corners from the same family as every other panel.
-->
<a
	href="/film/{movie.id}"
	class="poster-card group"
	class:poster-card--fluid={fluid}
	title={title}
	data-poster-card
>
	<div class="poster-frame">
		{#if backdrop}
			<img src={backdrop} alt="" loading="lazy" decoding="async" class="poster-art" />
		{:else if poster}
			<img src={poster} alt="" loading="lazy" decoding="async" class="poster-art poster-art--fit" />
		{:else}
			<!-- No artwork is a normal state: TMDB did not recognise the file, or
			     has none. The title still has to be readable. -->
			<div class="poster-fallback">
				<span class="line-clamp-3 text-center text-small text-muted">{title}</span>
			</div>
		{/if}

		{#if watched}
			<!-- The one accent the grid carries at rest, allowed by §6 because it
			     is information rather than decoration: it says how far in you are,
			     which is the whole point of the row it appears in. -->
			<div class="poster-progress">
				<div class="poster-progress-played" style="width: {percent}%"></div>
			</div>
		{/if}
	</div>

	<p class="poster-title">{title}</p>
	<p class="poster-legend label">{secondary}</p>
</a>
