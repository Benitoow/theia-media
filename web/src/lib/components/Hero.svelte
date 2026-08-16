<script>
	import { imageURL, displayTitle, displayYear, formatRuntime } from '$lib/api.js';
	import { strings as t, formatDecimal } from '$lib/strings.js';

	// kind comes from the server: 'resume' when this film is already under way,
	// 'featured' when nothing is and it is simply worth starting. The hero is
	// not a fixed slot, and which of the two it is changes what it offers.
	let { movie, kind = 'featured' } = $props();

	const backdrop = $derived(imageURL(movie.metadata?.backdrop_path, 'w1280'));
	const title = $derived(displayTitle(movie));
	const year = $derived(displayYear(movie));
	const runtime = $derived(formatRuntime(movie.metadata?.runtime_minutes));

	const resuming = $derived(kind === 'resume' && movie.progress?.position_seconds > 0);
	const percent = $derived(
		resuming && movie.progress?.duration_seconds > 0
			? Math.min(100, (movie.progress.position_seconds / movie.progress.duration_seconds) * 100)
			: 0
	);

	// Remaining rather than elapsed: what is left is the thing that decides
	// whether you put it back on tonight.
	const remaining = $derived(
		resuming && movie.progress?.duration_seconds > 0
			? formatRuntime(
					Math.round((movie.progress.duration_seconds - movie.progress.position_seconds) / 60)
				)
			: null
	);
</script>

<!--
	The chrome, where the identity goes in full: display serif, room to breathe,
	a single accent. Everything below this section is the dense half.
-->
<section class="relative isolate flex min-h-[78svh] items-end overflow-hidden">
	{#if backdrop}
		<img
			src={backdrop}
			alt=""
			fetchpriority="high"
			class="absolute inset-0 -z-20 h-full w-full scale-[1.015] object-cover object-center opacity-[0.78]"
		/>
	{/if}

	<!-- The picture is chrome, so it can carry treatment. Cards below stay
	     untouched. See .picture-veil--hero. -->
	<div class="picture-veil picture-veil--hero"></div>

	<div class="page-shell page-body--hero w-full">
		<div class="max-w-[52rem]">
			{#if resuming}
				<span class="label enter text-accent">{t.hero.resumeEyebrow}</span>
			{:else if movie.metadata?.genres?.length}
				<span class="label enter">{movie.metadata.genres.slice(0, 3).join(' · ')}</span>
			{/if}

			<h1 class="hero-title enter mt-4 mb-7">{title}</h1>

			<div class="enter enter-2 mb-7 flex flex-wrap items-center gap-x-6 gap-y-3">
				{#if year}<span class="label">{year}</span>{/if}
				{#if runtime}<span class="label">{runtime}</span>{/if}
				{#if movie.metadata?.director}<span class="label">{movie.metadata.director}</span>{/if}
				{#if movie.metadata?.vote_average}
					<span class="text-label tracking-[0.18em] text-accent uppercase">
						{formatDecimal(movie.metadata.vote_average)}
					</span>
				{/if}
			</div>

			{#if resuming}
				<!-- Section 6 exempts the poster grid, not the chrome, so the hero
				     may state this properly rather than as a 3px rule. -->
				<div class="hero-progress enter enter-2 mb-8">
					<div class="hero-progress-track">
						<div class="hero-progress-played" style="width: {percent}%"></div>
					</div>
					{#if remaining}
						<span class="label shrink-0">{t.hero.remaining(remaining)}</span>
					{/if}
				</div>
			{:else if movie.metadata?.overview}
				<p class="tv-copy enter enter-2 mb-9 max-w-[42rem] line-clamp-3">
					{movie.metadata.overview}
				</p>
			{/if}

			<div class="enter enter-3 flex flex-wrap items-center gap-4">
				{#if resuming}
					<!-- Straight into the film. The player still offers to start from
					     the beginning; this saves the trip through the detail page,
					     which was the whole complaint about the old home screen. -->
					<a
						href="/film/{movie.id}?reprendre=1"
						class="tv-action tv-action--primary"
						data-remote-default
					>
						<span>{t.hero.resume}</span>
						<span aria-hidden="true">→</span>
					</a>
					<a href="/film/{movie.id}" class="tv-action">{t.hero.details}</a>
				{:else}
					<a href="/film/{movie.id}" class="tv-action tv-action--primary" data-remote-default>
						<span>{t.hero.details}</span>
						<span aria-hidden="true">→</span>
					</a>
				{/if}
			</div>
		</div>
	</div>
</section>
