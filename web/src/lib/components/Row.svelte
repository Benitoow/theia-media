<script>
	import { onMount } from 'svelte';
	import PosterCard from './PosterCard.svelte';
	import Icon from './Icon.svelte';
	import { strings as t } from '$lib/strings.js';

	// priority marks the row that is on screen before anything is scrolled.
	// Only the home page's first row sets it, and only its leading cards act on
	// it: eager-loading every row would fetch the whole page at once, which is
	// the fault this is meant to fix, in the other direction.
	// title names a row the catalogue does not: a film's own saga, whose heading
	// is the collection's name and which leads nowhere -- every part of it is
	// already on screen.
	let { row, priority = false, title = null } = $props();
	const eagerCards = 4;

	// The server says what a row is; this is where it gets its name and its way
	// through to the library. An unknown kind falls back to the code itself
	// rather than rendering an empty heading -- a new row kind should look
	// unfinished, not invisible.
	const copy = $derived(
		title ? { title, href: null } : (t.rows[row.kind] ?? { title: row.kind, href: null })
	);

	let scroller = $state();
	let canScrollLeft = $state(false);
	let canScrollRight = $state(false);

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

	function updateScrollEdges() {
		if (!scroller) return;
		const max = Math.max(0, scroller.scrollWidth - scroller.clientWidth);
		canScrollLeft = scroller.scrollLeft > 2;
		canScrollRight = scroller.scrollLeft < max - 2;
	}

	function scrollRow(direction) {
		if (!scroller) return;
		scroller.scrollBy({
			left: direction * Math.max(320, scroller.clientWidth * 0.78),
			behavior: window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth'
		});
	}

	onMount(() => {
		updateScrollEdges();
		const observer =
			typeof ResizeObserver === 'undefined' ? null : new ResizeObserver(updateScrollEdges);
		observer?.observe(scroller);
		window.addEventListener('resize', updateScrollEdges);

		return () => {
			observer?.disconnect();
			window.removeEventListener('resize', updateScrollEdges);
		};
	});
</script>

<section class="mb-10 lg:mb-14" aria-label={copy.title}>
	<!-- Baselines, not box edges. "Tout voir" is a .tv-link, which carries the
	     52px hit target section 9 requires and centres its text inside it; lining
	     the boxes up by their bottoms therefore left the link's text sitting 19px
	     above the row heading it belongs to. Aligning baselines puts the two
	     pieces of text on one line, which is what the eye reads. -->
	<div class="page-shell mb-3 flex items-baseline justify-between gap-6">
		<div class="min-w-0">
			<h2 class="section-title">{copy.title}</h2>
			{#if copy.hint}
				<p class="label mt-1.5">{copy.hint}</p>
			{/if}
		</div>
		{#if copy.href}
			<!-- A row is a suggestion, not an inventory. This is the way to the
			     rest, pre-filtered to match what the row was showing. -->
			<a href={copy.href} class="tv-link label shrink-0">
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
	<div class="row-scroll-frame">
		{#if canScrollLeft}
			<button
				type="button"
				class="row-scroll-button row-scroll-button--left"
				tabindex="-1"
				aria-label={t.library.scrollPrevious}
				onpointerdown={(event) => event.preventDefault()}
				onclick={() => scrollRow(-1)}
			>
				<Icon name="chevronLeft" size={28} />
			</button>
		{/if}

		<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
		<div
			bind:this={scroller}
			class="scrollbar-hidden flex snap-x snap-mandatory gap-4 overflow-x-auto px-[var(--page-gutter)]
			       pt-3 pb-5 lg:gap-5"
			style="scroll-padding-inline: var(--page-gutter)"
			role="group"
			aria-label={copy.title}
			tabindex="-1"
			onkeydown={moveInRow}
			onscroll={updateScrollEdges}
		>
			<!-- A row carries either films or prepared cards. Episodes need their
			     own artwork and heading; everything else about the card is the
			     same, so this stays one component rather than two. -->
			{#each row.cards ?? row.movies.map((movie) => ({ movie })) as card, index (card.movie.id)}
				<div class="snap-start">
					<PosterCard
						movie={card.movie}
						href={card.href ?? null}
						art={card.art ?? null}
						title={card.title ?? null}
						legend={card.legend ?? null}
						playable={card.playable ?? true}
						priority={priority && index < eagerCards}
					/>
				</div>
			{/each}
		</div>

		{#if canScrollRight}
			<button
				type="button"
				class="row-scroll-button row-scroll-button--right"
				tabindex="-1"
				aria-label={t.library.scrollNext}
				onpointerdown={(event) => event.preventDefault()}
				onclick={() => scrollRow(1)}
			>
				<Icon name="chevronRight" size={28} />
			</button>
		{/if}
	</div>
</section>
