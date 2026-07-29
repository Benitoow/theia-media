<script>
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { getJSON, imageURL, displayTitle, displayYear, formatRuntime } from '$lib/api.js';
	import { strings as t, formatSize } from '$lib/strings.js';
	import Player from '$lib/components/Player.svelte';
	import Icon from '$lib/components/Icon.svelte';
	import LoadingSkeleton from '$lib/components/LoadingSkeleton.svelte';

	/** @type {'loading' | 'ready' | 'missing'} */
	let state = $state('loading');
	let movie = $state(null);
	let playing = $state(false);

	const meta = $derived(movie?.metadata ?? {});
	const backdrop = $derived(imageURL(meta.backdrop_path, 'w1280'));
	const poster = $derived(imageURL(meta.poster_path, 'w342'));
	const resumable = $derived(
		movie?.progress?.position_seconds > 0 && !movie?.progress?.finished
	);
	const resumeMinutes = $derived(
		resumable ? Math.max(1, Math.floor(movie.progress.position_seconds / 60)) : 0
	);
	const progressPercent = $derived(
		resumable && movie.progress.duration_seconds > 0
			? Math.min(100, (movie.progress.position_seconds / movie.progress.duration_seconds) * 100)
			: 0
	);

	onMount(async () => {
		try {
			movie = await getJSON(`/api/library/movies/${$page.params.id}`);
			state = 'ready';
			// The home hero's "Reprendre" links here with this flag rather than
			// opening a player it does not own. The player still asks whether to
			// resume or start over; what this skips is the detour through a page
			// nobody wanted to read on the way back to a film already half seen.
			if ($page.url.searchParams.has('reprendre')) playing = true;
		} catch {
			state = 'missing';
		}
	});

	function syncProgress(progress) {
		if (!movie || !progress) return;
		movie = { ...movie, progress };
	}
</script>

<svelte:head>
	<title>{movie ? displayTitle(movie) : t.appName}</title>
</svelte:head>

{#if state === 'loading'}
	<LoadingSkeleton variant="detail" label={t.home.loading} />
{:else if state === 'missing'}
	<div class="page-shell flex min-h-screen items-center justify-center py-32">
		<div class="chrome-panel max-w-xl p-8 text-center sm:p-12">
			<h1 class="font-display text-display font-normal">{t.film.notFound}</h1>
			<a href="/" class="tv-link label mt-7 justify-center">← {t.nav.back}</a>
		</div>
	</div>
{:else}
	<!-- Chrome again: the identity goes in full here, as on the hero. -->
	<article>
		<header class="relative isolate min-h-[68svh] overflow-hidden">
			{#if backdrop}
				<img
					src={backdrop}
					alt=""
					fetchpriority="high"
					class="absolute inset-0 -z-20 h-full w-full scale-[1.015] object-cover opacity-[0.58]"
				/>
			{/if}
			<div
				class="absolute inset-0 -z-10"
				style="background:
					linear-gradient(to top, var(--color-ink) 2%, rgba(11,10,9,0.58) 48%, rgba(11,10,9,0.08) 100%),
					linear-gradient(to right, rgba(11,10,9,0.7), transparent 70%)"
			></div>
		</header>

		<div class="page-shell relative z-10 -mt-52 pb-20">
			<div class="flex flex-col gap-10 md:flex-row md:items-end md:gap-14">
				<!-- Poster keeps the grid's locked 2:3 and its plainness. -->
				<div class="w-48 shrink-0 self-start lg:w-56 2xl:w-64">
					<div
						class="aspect-[2/3] overflow-hidden rounded-sm border border-line bg-surface
						       shadow-[0_1.75rem_4rem_rgba(0,0,0,0.42)]"
					>
						{#if poster}
							<img src={poster} alt="" decoding="async" class="h-full w-full object-cover" />
						{/if}
					</div>

					{#if resumable && movie.progress.duration_seconds > 0}
						<div
							class="film-progress"
							role="progressbar"
							aria-label={t.film.progress}
							aria-valuemin="0"
							aria-valuemax="100"
							aria-valuenow={Math.round(progressPercent)}
						>
							<span style="width: {progressPercent}%"></span>
						</div>
					{/if}
				</div>

				<div class="min-w-0 flex-1 pb-2">
					{#if meta.genres?.length}
						<span class="label enter">{meta.genres.join(' · ')}</span>
					{/if}

					<h1 class="page-title page-title--feature enter mt-4 mb-7">
						{displayTitle(movie)}
					</h1>

					<div class="enter enter-2 mb-8 flex flex-wrap items-center gap-x-6 gap-y-3">
						{#if displayYear(movie)}<span class="label">{displayYear(movie)}</span>{/if}
						{#if formatRuntime(meta.runtime_minutes)}
							<span class="label">{formatRuntime(meta.runtime_minutes)}</span>
						{/if}
						{#if meta.director}
							<a
								href="/films?q={encodeURIComponent(meta.director)}"
								class="film-director-link label"
								aria-label={t.film.moreByDirector(meta.director)}
							>
								<span>{meta.director}</span>
								<span aria-hidden="true">→</span>
							</a>
						{/if}
						{#if meta.vote_average}
							<span class="text-label tracking-[0.18em] text-accent uppercase">
								{meta.vote_average.toFixed(1)}
							</span>
						{/if}
					</div>

					<button
						type="button"
						onclick={() => (playing = true)}
						class="tv-action tv-action--primary mb-12 cursor-pointer"
						data-remote-default
					>
						<Icon name="play" size={18} />
						<span>{resumable ? t.player.resumeAtMinutes(resumeMinutes) : t.player.play}</span>
					</button>

					{#if meta.status === 'not_found'}
						<p class="mb-8 border-l border-warning py-1 pl-5 text-small text-parchment">
							{t.film.unmatched}
						</p>
					{/if}

					<p class="tv-copy mb-12 max-w-[46rem]">
						{meta.overview || t.film.noOverview}
					</p>

					{#if meta.cast?.length}
						<section class="mb-10">
							<h2 class="label mb-4">{t.film.cast}</h2>
							<ul class="grid gap-x-8 gap-y-3 sm:grid-cols-2">
								{#each meta.cast as person (person.name + person.character)}
									<li class="flex flex-col">
										<span class="text-base">{person.name}</span>
										{#if person.character}
											<span class="label mt-0.5">{person.character}</span>
										{/if}
									</li>
								{/each}
							</ul>
						</section>
					{/if}

					<!-- The file behind the film. Useful when two copies of the same
					     title are in the library and you need to know which is which. -->
					<section class="border-t border-line pt-6">
						<dl class="grid gap-x-8 gap-y-3 sm:grid-cols-[8rem_1fr]">
							<dt class="label">{t.film.file}</dt>
							<dd class="text-small break-all text-parchment">{movie.file_name}</dd>
							<dt class="label">{t.film.size}</dt>
							<dd class="text-small text-parchment">{formatSize(movie.size_bytes)}</dd>
						</dl>
					</section>

					<a href="/" class="tv-link label mt-10">
						← {t.nav.back}
					</a>
				</div>
			</div>
		</div>
	</article>

	{#if playing}
		<Player {movie} onprogress={syncProgress} onclose={() => (playing = false)} />
	{/if}
{/if}
