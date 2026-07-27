<script>
	import PosterCard from './PosterCard.svelte';

	let { row } = $props();
</script>

<section class="mb-12">
	<h2 class="label mb-4 px-6 lg:px-16">{row.title}</h2>

	<!--
		Horizontal scroll rather than a wrapping grid, so a row reads as one
		strip you skim. Snap points make a trackpad flick land on a card instead
		of half of two. Padding lives on the scroller, not a parent, otherwise
		the first and last cards sit flush against the viewport edge.
	-->
	<div
		class="scrollbar-thin flex snap-x snap-mandatory gap-3 overflow-x-auto px-6 pb-3 lg:px-16"
	>
		{#each row.movies as movie (movie.id)}
			<div class="snap-start">
				<PosterCard {movie} />
			</div>
		{/each}
	</div>
</section>
