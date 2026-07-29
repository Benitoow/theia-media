<script>
	import PosterCard from './PosterCard.svelte';
	import { strings as t } from '$lib/strings.js';

	let { row } = $props();

	// TV remotes generally report left/right as keyboard arrows. Keep native
	// Tab navigation, but make a row usable without asking for a hidden Tab key.
	function moveInRow(event) {
		if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return;

		const cards = [...event.currentTarget.querySelectorAll('[data-poster-card]')];
		const current = event.target.closest?.('[data-poster-card]');
		const index = cards.indexOf(current);
		if (index < 0) return;

		const nextIndex = event.key === 'ArrowRight' ? index + 1 : index - 1;
		const next = cards[nextIndex];
		if (!next) return;

		event.preventDefault();
		next.focus({ preventScroll: true });
		next.scrollIntoView({
			block: 'nearest',
			inline: 'center',
			behavior: window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth'
		});
	}
</script>

<section class="mb-10 lg:mb-14" aria-label={row.title}>
	<div class="page-shell mb-3 flex items-end justify-between gap-6">
		<h2 class="section-title">{row.title}</h2>
		{#if row.genre}
			<!-- A row shows twenty films. This is the way to the rest of them,
			     which until now simply did not exist. -->
			<a href="/films?genre={encodeURIComponent(row.genre)}" class="tv-link label shrink-0">
				{t.library.seeAll}
			</a>
		{/if}
	</div>

	<!--
		Horizontal scroll rather than a wrapping grid, so a row reads as one
		strip you skim. Snap points make a trackpad flick land on a card instead
		of half of two. Padding lives on the scroller, not a parent, otherwise
		the first and last cards sit flush against the viewport edge.
	-->
	<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
	<div
		class="scrollbar-thin flex snap-x snap-mandatory gap-4 overflow-x-auto px-[var(--page-gutter)]
		       pt-3 pb-5 lg:gap-5"
		style="scroll-padding-inline: var(--page-gutter)"
		role="group"
		aria-label={row.title}
		tabindex="-1"
		onkeydown={moveInRow}
	>
		{#each row.movies as movie (movie.id)}
			<div class="snap-start">
				<PosterCard {movie} />
			</div>
		{/each}
	</div>
</section>
