<script>
	// A series: its identity, the seasons actually on disk, and the episodes of
	// the season being looked at.
	//
	// Seasons are a row of options rather than separate pages. A television
	// switching season should not be a navigation, and the season payload is
	// deliberately compact -- files live on the episode page, not here.
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { getJSON, imageURL, displayTitle } from '$lib/api.js';
	import { profiles } from '$lib/profiles.svelte.js';
	import { strings as t, formatDecimal } from '$lib/strings.js';
	import LoadingSkeleton from '$lib/components/LoadingSkeleton.svelte';
	import EpisodeRow from '$lib/components/EpisodeRow.svelte';
	import MatchDialog from '$lib/components/MatchDialog.svelte';
	import Certificate from '$lib/components/Certificate.svelte';
	import CastList from '$lib/components/CastList.svelte';
	import Credits from '$lib/components/Credits.svelte';
	import { remote } from '$lib/remote.svelte.js';

	/** @type {'loading' | 'ready' | 'missing'} */
	let state = $state('loading');
	let series = $state(null);
	let season = $state(null);
	let seasonNumber = $state(null);
	let loadingSeason = $state(false);

	const meta = $derived(series?.metadata ?? {});

	// A series has two dates, and the pair is more informative than either: the
	// year it started, and the year it stopped when it has stopped. TMDB knows
	// both; the page used to show only the first.
	const firstYear = $derived(meta.first_air_date?.slice(0, 4) ?? '');
	const lastYear = $derived(meta.last_air_date?.slice(0, 4) ?? '');
	const years = $derived(
		firstYear && lastYear && lastYear !== firstYear && meta.air_status === 'ended'
			? `${firstYear} – ${lastYear}`
			: firstYear
	);

	// Whether it is still running, as a word. The server sends a code precisely
	// so this line can be French or English without a second request.
	const airStatus = $derived(meta.air_status ? (t.credits.airStatus[meta.air_status] ?? '') : '');

	const originalName = $derived(
		meta.original_name && meta.original_name !== displayTitle(series) ? meta.original_name : ''
	);

	const creditRows = $derived([
		{ label: t.credits.originalTitle, value: originalName },
		{ label: t.credits.creators, value: (meta.creators ?? []).join(' · ') },
		{ label: t.credits.network, value: (meta.networks ?? []).join(' · ') }
	]);
	const backdrop = $derived(imageURL(meta.backdrop_path, 'w1280'));
	const poster = $derived(imageURL(meta.poster_path, 'w342'));
	// Season 0 is specials. It is a visible season and never part of the
	// automatic run (decision 41), so it is named rather than numbered.
	const seasonName = (number) => (number === 0 ? t.series.specials : t.series.season(number));

	onMount(async () => {
		try {
			await profiles.ready();
			series = await getJSON(profiles.url(`/api/library/series/${$page.params.id}`));
			state = 'ready';
			// Open on the first real season rather than on the specials, which are
			// almost never what somebody came for.
			const seasons = series.seasons ?? [];
			const first = seasons.find((s) => s.season_number > 0) ?? seasons[0];
			if (first) await selectSeason(first.season_number);
		} catch {
			state = 'missing';
		}
	});

	// Correcting a series is worth more than correcting a film: every season and
	// every episode title came from whichever show it was matched to, so getting
	// the show wrong gets the whole page wrong. The server re-reads the cascade.
	let correcting = $state(false);

	async function onMatchApplied(updated) {
		correcting = false;
		if (updated) series = updated;
		const seasons = series?.seasons ?? [];
		const current = seasons.find((s) => s.season_number === seasonNumber) ?? seasons[0];
		if (current) await selectSeason(current.season_number);
	}

	async function selectSeason(number) {
		if (number === seasonNumber) return;
		loadingSeason = true;
		try {
			season = await getJSON(
				profiles.url(`/api/library/series/${$page.params.id}/seasons/${number}`)
			);
			seasonNumber = number;
		} catch {
			season = null;
		} finally {
			loadingSeason = false;
		}
	}
</script>

<svelte:head>
	<title>{series ? displayTitle(series) : t.series.title}</title>
</svelte:head>

{#if state === 'loading'}
	<LoadingSkeleton variant="detail" label={t.series.loading} />
{:else if state === 'missing'}
	<div class="page-shell flex min-h-screen items-center justify-center py-32">
		<div class="chrome-panel max-w-xl p-8 text-center sm:p-12">
			<h1 class="font-display text-display font-normal">{t.series.notFound}</h1>
			<a href="/series" class="tv-link label mt-7 justify-center">← {t.nav.back}</a>
		</div>
	</div>
{:else}
	<article>
		<header class="relative isolate min-h-[58svh] overflow-hidden">
			{#if backdrop}
				<img
					src={backdrop}
					alt=""
					fetchpriority="high"
					class="absolute inset-0 -z-20 h-full w-full scale-[1.015] object-cover opacity-[0.58]"
				/>
			{/if}
			<div class="picture-veil picture-veil--detail"></div>
		</header>

		<div class="page-shell relative z-10 -mt-44 pb-20">
			<div class="flex flex-col gap-10 md:flex-row md:items-end md:gap-14">
				<div class="w-40 shrink-0 self-start lg:w-48 2xl:w-56">
					<div
						class="aspect-[2/3] overflow-hidden rounded-[var(--radius-card)] border border-line bg-surface
						       shadow-[0_1.75rem_4rem_rgba(0,0,0,0.42)]"
					>
						{#if poster}
							<img src={poster} alt="" decoding="async" class="h-full w-full object-cover" />
						{/if}
					</div>
				</div>

				<div class="min-w-0 flex-1 pb-2">
					{#if meta.genres?.length}
						<span class="label enter">{meta.genres.join(' · ')}</span>
					{/if}

					<h1 class="page-title page-title--feature enter mt-4 {meta.tagline ? 'mb-4' : 'mb-6'}">
						{displayTitle(series)}
					</h1>

					{#if meta.tagline}
						<p class="film-tagline enter enter-2 mb-6">{meta.tagline}</p>
					{/if}

					<div class="enter enter-2 mb-8 flex flex-wrap items-center gap-x-6 gap-y-3">
						{#if years}
							<span class="label">{years}</span>
						{/if}
						<Certificate value={meta.certification} country={meta.certification_country} />
						{#if airStatus}
							<span class="label">{airStatus}</span>
						{/if}
						{#if meta.vote_average}
							<span class="text-label tracking-[0.18em] text-accent uppercase">
								{formatDecimal(meta.vote_average)}
							</span>
						{/if}
					</div>

					{#if meta.status === 'not_found'}
						<p class="mb-8 border-l border-warning py-1 pl-5 text-small text-parchment">
							{t.series.unmatched}
						</p>
					{/if}

					<p class="tv-copy mb-8 max-w-[46rem]">{meta.overview || t.series.noOverview}</p>

					<Credits rows={creditRows} heading={t.credits.heading} />

					<!-- The series page showed no cast at all, while the film page showed
					     names with no faces. Both now show the same thing, from the same
					     credits call. -->
					<CastList cast={meta.cast ?? []} heading={t.film.cast} />

					{#if !remote.isRemote}
						<button
							type="button"
							onclick={() => (correcting = true)}
							class="tv-action mb-10 cursor-pointer"
						>
							<span>{t.match.wrongSeries}</span>
						</button>
					{/if}
				</div>
			</div>

			<!-- Seasons, then the episodes of the one chosen. Same list rules as the
			     file chooser: shared width, own vertical axis, edges not consumed. -->
			<section class="season-picker">
				<h2 class="label">{t.series.seasons}</h2>
				<ul class="season-list" aria-label={t.series.seasons}>
					{#each series.seasons ?? [] as entry (entry.id)}
						<li>
							<button
								type="button"
								class="season-option"
								class:season-option--active={entry.season_number === seasonNumber}
								aria-pressed={entry.season_number === seasonNumber}
								onclick={() => selectSeason(entry.season_number)}
							>
								{seasonName(entry.season_number)}
							</button>
						</li>
					{/each}
				</ul>
			</section>

			{#if season && !loadingSeason}
				<p class="label mt-4">{t.series.ownedEpisodes(season.episodes?.length ?? 0)}</p>
				<!-- "Épisodes spéciaux, c'est quoi ?" was a fair question to have to
				     ask. Season zero is where TMDB puts everything outside the
				     numbering -- making-of, Christmas episodes, recaps -- and it only
				     ever appears because a folder on disk is named Specials or
				     Saison 00. Explaining it here is cheaper than being asked. -->
				{#if seasonNumber === 0}
					<p class="tv-copy mt-3 max-w-[46rem] text-small text-muted">{t.series.specialsHelp}</p>
				{/if}
			{/if}

			{#if loadingSeason}
				<p class="label mt-8" role="status">{t.series.loading}</p>
			{:else if season}
				<ul class="episode-list">
					{#each season.episodes ?? [] as item (item.id)}
						<li><EpisodeRow episode={item} /></li>
					{/each}
				</ul>
			{/if}

			<a href="/series" class="tv-link label mt-10">← {t.nav.back}</a>
		</div>
	</article>

	{#if correcting}
		<MatchDialog
			kind="series"
			id={series.id}
			title={displayTitle(series)}
			onapplied={onMatchApplied}
			onclose={() => (correcting = false)}
		/>
	{/if}
{/if}
