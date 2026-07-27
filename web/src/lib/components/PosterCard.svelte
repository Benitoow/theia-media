<script>
	import { imageURL, displayTitle, displayYear } from '$lib/api.js';

	let { movie } = $props();

	const poster = $derived(imageURL(movie.metadata?.poster_path, 'w342'));
	const title = $derived(displayTitle(movie));
	const year = $derived(displayYear(movie));

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
	Deliberately plain. Section 6 of the design system exempts the grid from the
	rest of the identity: no display serif, no negative space, no gold at rest.
	TMDB posters already carry a hundred competing art directions, and framing
	them dramatically makes a grid that is slow to scan. Gold appears on hover
	and focus only, as one pixel of border.
-->
<a
	href="/film/{movie.id}"
	class="group block w-34 shrink-0 focus:outline-none sm:w-38"
	title={title}
>
	<div
		class="ease-cine relative aspect-[2/3] overflow-hidden rounded-xs border border-transparent
		       bg-surface transition-colors duration-320
		       group-hover:border-accent group-focus-visible:border-accent"
	>
		{#if poster}
			<img src={poster} alt="" loading="lazy" class="h-full w-full object-cover" />
		{:else}
			<!-- No poster is a normal state: TMDB did not recognise the file, or
			     has no artwork. The title still has to be readable. -->
			<div class="flex h-full items-center justify-center p-3">
				<span class="line-clamp-4 text-center text-small text-muted">{title}</span>
			</div>
		{/if}

		{#if watched}
			<!-- The one accent the grid carries at rest, allowed by section 6 of
			     the design system because it is information rather than
			     decoration: it says how far in you are, which is the whole point
			     of the row it appears in. -->
			<div class="absolute inset-x-0 bottom-0 h-[3px] bg-ink/70">
				<div class="h-full bg-accent" style="width: {percent}%"></div>
			</div>
		{/if}
	</div>

	<p class="mt-2 truncate text-small">{title}</p>
	<p class="label mt-0.5">{year ?? '—'}</p>
</a>
