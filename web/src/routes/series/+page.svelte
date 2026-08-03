<script>
	// The series catalogue. Deliberately the same card as a film: a series is
	// another thing you look at and press, and §6 of the design system asks the
	// grid to be dense and fast rather than to invent a second visual language
	// for a second kind of row.
	import { onMount } from 'svelte';
	import { getJSON } from '$lib/api.js';
	import { profiles } from '$lib/profiles.svelte.js';
	import { strings as t } from '$lib/strings.js';
	import PosterCard from '$lib/components/PosterCard.svelte';
	import LoadingSkeleton from '$lib/components/LoadingSkeleton.svelte';

	/** @type {'loading' | 'ready' | 'failed'} */
	let state = $state('loading');
	let series = $state([]);

	onMount(async () => {
		try {
			const payload = await getJSON(profiles.url('/api/library/series?limit=500'));
			series = payload.series ?? [];
			state = 'ready';
		} catch {
			state = 'failed';
		}
	});
</script>

<svelte:head>
	<title>{t.series.title} — {t.appName}</title>
</svelte:head>

{#if state === 'loading'}
	<LoadingSkeleton variant="grid" label={t.series.loading} />
{:else}
	<div class="page-shell page-body">
		<header class="mb-10">
			<h1 class="page-title">{t.series.title}</h1>
			{#if state === 'ready' && series.length}
				<p class="label mt-3">{t.series.countAll(series.length)}</p>
			{/if}
		</header>

		{#if state === 'failed'}
			<div class="chrome-panel max-w-xl p-8">
				<p class="tv-copy border-l border-error py-2 pl-6">{t.home.unreachable}</p>
			</div>
		{:else if series.length === 0}
			<div class="chrome-panel max-w-2xl p-8 sm:p-12">
				<h2 class="font-display text-display font-normal">{t.series.emptyTitle}</h2>
				<p class="tv-copy mt-5">{t.series.emptyBody}</p>
			</div>
		{:else}
			<div class="library-grid">
				{#each series as item (item.id)}
					<PosterCard
						movie={item}
						href="/serie/{item.id}"
						fluid
						legend={item.metadata?.first_air_date?.slice(0, 4) ?? '—'}
					/>
				{/each}
			</div>
		{/if}
	</div>
{/if}
