<script>
	import { imageURL, displayTitle, displayYear } from '$lib/api.js';

	let { movie } = $props();

	const poster = $derived(imageURL(movie.metadata?.poster_path, 'w342'));
	const title = $derived(displayTitle(movie));
	const year = $derived(displayYear(movie));
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
		class="ease-cine aspect-[2/3] overflow-hidden rounded-xs border border-transparent
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
	</div>

	<p class="mt-2 truncate text-small">{title}</p>
	<p class="label mt-0.5">{year ?? '—'}</p>
</a>
