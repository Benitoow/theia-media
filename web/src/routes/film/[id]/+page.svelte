<script>
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { getJSON, imageURL, displayTitle, displayYear, formatRuntime } from '$lib/api.js';
	import { strings as t, formatSize } from '$lib/strings.js';

	/** @type {'loading' | 'ready' | 'missing'} */
	let state = $state('loading');
	let movie = $state(null);

	const meta = $derived(movie?.metadata ?? {});
	const backdrop = $derived(imageURL(meta.backdrop_path, 'w1280'));
	const poster = $derived(imageURL(meta.poster_path, 'w342'));

	onMount(async () => {
		try {
			movie = await getJSON(`/api/library/movies/${$page.params.id}`);
			state = 'ready';
		} catch {
			state = 'missing';
		}
	});
</script>

<svelte:head>
	<title>{movie ? displayTitle(movie) : t.appName}</title>
</svelte:head>

{#if state === 'loading'}
	<div class="flex min-h-screen items-center justify-center">
		<span class="label">{t.home.loading}</span>
	</div>
{:else if state === 'missing'}
	<div class="flex min-h-screen flex-col items-center justify-center gap-6 px-6">
		<p class="text-small text-parchment">{t.film.notFound}</p>
		<a href="/" class="label hover:text-bone">{t.nav.back}</a>
	</div>
{:else}
	<!-- Chrome again: the identity goes in full here, as on the hero. -->
	<article>
		<header class="relative isolate min-h-[52vh] overflow-hidden">
			{#if backdrop}
				<img
					src={backdrop}
					alt=""
					fetchpriority="high"
					class="absolute inset-0 -z-20 h-full w-full object-cover opacity-40"
				/>
			{/if}
			<div
				class="absolute inset-0 -z-10"
				style="background: linear-gradient(to top, var(--color-ink) 6%, rgba(11,10,9,0.6) 55%, rgba(11,10,9,0.2) 100%)"
			></div>
		</header>

		<div class="mx-auto -mt-40 max-w-5xl px-6 pb-16 lg:px-16">
			<div class="flex flex-col gap-10 sm:flex-row sm:gap-12">
				<!-- Poster keeps the grid's locked 2:3 and its plainness. -->
				<div class="w-44 shrink-0 self-start overflow-hidden rounded-xs border border-line bg-surface">
					<div class="aspect-[2/3]">
						{#if poster}
							<img src={poster} alt="" class="h-full w-full object-cover" />
						{/if}
					</div>
				</div>

				<div class="min-w-0 flex-1">
					{#if meta.genres?.length}
						<span class="label">{meta.genres.join(' · ')}</span>
					{/if}

					<h1 class="mt-3 mb-5 font-display text-display font-normal text-balance">
						{displayTitle(movie)}
					</h1>

					<div class="mb-8 flex flex-wrap items-center gap-x-5 gap-y-2">
						{#if displayYear(movie)}<span class="label">{displayYear(movie)}</span>{/if}
						{#if formatRuntime(meta.runtime_minutes)}
							<span class="label">{formatRuntime(meta.runtime_minutes)}</span>
						{/if}
						{#if meta.director}<span class="label">{meta.director}</span>{/if}
						{#if meta.vote_average}
							<span class="text-label tracking-[0.18em] text-accent uppercase">
								{meta.vote_average.toFixed(1)}
							</span>
						{/if}
					</div>

					{#if meta.status === 'not_found'}
						<p class="mb-8 border-l border-warning py-1 pl-5 text-small text-parchment">
							{t.film.unmatched}
						</p>
					{/if}

					<p class="mb-10 max-w-prose text-body text-parchment">
						{meta.overview || t.film.noOverview}
					</p>

					{#if meta.cast?.length}
						<section class="mb-10">
							<h2 class="label mb-4">{t.film.cast}</h2>
							<ul class="grid gap-x-8 gap-y-3 sm:grid-cols-2">
								{#each meta.cast as person (person.name + person.character)}
									<li class="flex flex-col">
										<span class="text-small">{person.name}</span>
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

					<a href="/" class="label ease-cine mt-10 inline-block transition-colors duration-160 hover:text-bone">
						← {t.nav.back}
					</a>
				</div>
			</div>
		</div>
	</article>
{/if}
