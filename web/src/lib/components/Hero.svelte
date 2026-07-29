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
<section class="relative isolate flex min-h-[82svh] items-end overflow-hidden">
	{#if backdrop}
		<img
			src={backdrop}
			alt=""
			fetchpriority="high"
			class="absolute inset-0 -z-20 h-full w-full scale-[1.015] object-cover object-center opacity-60"
		/>
	{/if}

	<!-- The picture is chrome, so it can carry treatment. Posters below stay
	     untouched. The three fades protect the copy without turning the frame
	     into the generic blue-black wash used by every streaming clone. -->
	<div
		class="absolute inset-0 -z-10"
		style="background:
			linear-gradient(to top, var(--color-ink) 0%, rgba(11,10,9,0.72) 34%, rgba(11,10,9,0.08) 76%),
			linear-gradient(to right, rgba(11,10,9,0.94) 0%, rgba(11,10,9,0.52) 43%, transparent 76%),
			radial-gradient(circle at 75% 28%, rgba(200,162,74,0.13), transparent 28rem)"
	></div>

	<div class="page-shell page-body--hero w-full">
		<div class="max-w-[52rem]">
			{#if movie.metadata?.genres?.length}
				<span class="label enter">{movie.metadata.genres.slice(0, 3).join(' · ')}</span>
			{/if}

			<h1 class="hero-title enter mt-4 mb-7">{title}</h1>

			<div class="enter enter-2 mb-7 flex flex-wrap items-center gap-x-6 gap-y-3">
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
				<p class="tv-copy enter enter-2 mb-9 max-w-[42rem] line-clamp-3">
					{movie.metadata.overview}
				</p>
			{/if}

			<a
				href="/film/{movie.id}"
				class="tv-action tv-action--primary enter enter-3"
				data-remote-default
			>
				<span>{t.hero.details}</span>
				<span aria-hidden="true">→</span>
			</a>
		</div>
	</div>
</section>
