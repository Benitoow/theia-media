<script>
	import { imageURL, displayTitle, displayYear, formatRuntime } from '$lib/api.js';
	import { strings as t } from '$lib/strings.js';

	let { movie } = $props();

	const backdrop = $derived(imageURL(movie.metadata?.backdrop_path, 'w1280'));
	const title = $derived(displayTitle(movie));
	const year = $derived(displayYear(movie));
	const runtime = $derived(formatRuntime(movie.metadata?.runtime_minutes));
</script>

<!--
	The chrome, where the identity goes in full: display serif, room to breathe,
	a single accent. Everything below this section is the dense half.
-->
<section class="relative isolate flex min-h-[78vh] items-end overflow-hidden">
	{#if backdrop}
		<img
			src={backdrop}
			alt=""
			fetchpriority="high"
			class="absolute inset-0 -z-20 h-full w-full object-cover object-center opacity-45"
		/>
	{/if}

	<!-- Two gradients, not one. The vertical fade carries the text; the
	     horizontal one keeps the right of the frame from competing with it. -->
	<div
		class="absolute inset-0 -z-10"
		style="background:
			linear-gradient(to top, var(--color-ink) 4%, rgba(11,10,9,0.55) 45%, rgba(11,10,9,0.15) 100%),
			linear-gradient(to right, var(--color-ink) 0%, rgba(11,10,9,0.35) 55%, transparent 100%)"
	></div>

	<div class="w-full px-6 pt-32 pb-16 lg:px-16 lg:pb-24">
		<div class="max-w-2xl">
			{#if movie.metadata?.genres?.length}
				<span class="label">{movie.metadata.genres.slice(0, 3).join(' · ')}</span>
			{/if}

			<h1 class="mt-3 mb-5 font-display text-hero font-normal text-balance">{title}</h1>

			<div class="mb-6 flex flex-wrap items-center gap-x-5 gap-y-2">
				{#if year}<span class="label">{year}</span>{/if}
				{#if runtime}<span class="label">{runtime}</span>{/if}
				{#if movie.metadata?.director}<span class="label">{movie.metadata.director}</span>{/if}
				{#if movie.metadata?.vote_average}
					<span class="text-label tracking-[0.18em] text-accent uppercase">
						{movie.metadata.vote_average.toFixed(1)}
					</span>
				{/if}
			</div>

			{#if movie.metadata?.overview}
				<p class="mb-8 max-w-prose text-body text-parchment line-clamp-3">
					{movie.metadata.overview}
				</p>
			{/if}

			<a
				href="/film/{movie.id}"
				class="ease-cine inline-block border border-accent px-7 py-3.5 text-label
				       text-accent uppercase transition-colors duration-160
				       hover:bg-accent hover:text-ink"
			>
				{t.hero.details}
			</a>
		</div>
	</div>
</section>
