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
	import ChromeScene from '$lib/components/ChromeScene.svelte';

	/** @type {'loading' | 'ready' | 'failed'} */
	let state = $state('loading');
	let series = $state([]);

	onMount(async () => {
		try {
			await profiles.ready();
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

<!--
	The two message states are full-bleed, so they sit outside the page shell.
	They were a pair of hand-built panels here while /films and the home screen
	used ChromeScene for the identical states -- the same drift the library page
	records having already fixed on its own side. A panel floating half-width in
	a 1512px window, with a display title inside a box too small to carry one,
	was the visible cost of it.
-->
{#if state === 'loading'}
	<LoadingSkeleton variant="grid" label={t.series.loading} />
{:else if state === 'failed'}
	<ChromeScene
		image="/chrome/theia-offline.webp"
		eyebrow={t.appName}
		title={t.home.unreachableTitle}
		body={t.home.unreachable}
		tone="error"
	>
		<button
			type="button"
			onclick={() => location.reload()}
			class="tv-action cursor-pointer"
			data-remote-default
		>
			{t.home.retry}
		</button>
	</ChromeScene>
{:else if series.length === 0}
	<ChromeScene
		image="/chrome/theia-empty.webp"
		eyebrow={t.series.title}
		title={t.series.emptyTitle}
		body={t.series.emptyBody}
	>
		<a href="/reglages" class="tv-action cursor-pointer" data-remote-default>
			{t.nav.settings}
		</a>
	</ChromeScene>
{:else}
	<div class="page-shell page-body">
		<header class="mb-10">
			<h1 class="page-title">{t.series.title}</h1>
			<p class="label mt-3">{t.series.countAll(series.length)}</p>
		</header>

		<div class="library-grid">
			{#each series as item, index (item.id)}
				<PosterCard
					movie={item}
					href="/serie/{item.id}"
					fluid
					playable={false}
					legend={item.metadata?.first_air_date?.slice(0, 4) ?? ''}
					priority={index < 6}
				/>
			{/each}
		</div>
	</div>
{/if}
