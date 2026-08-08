<script>
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { getJSON, imageURL } from '$lib/api.js';
	import { profiles } from '$lib/profiles.svelte.js';
	import { remote } from '$lib/remote.svelte.js';
	import { strings as t } from '$lib/strings.js';
	import Hero from '$lib/components/Hero.svelte';
	import Row from '$lib/components/Row.svelte';
	import ChromeScene from '$lib/components/ChromeScene.svelte';
	import LoadingSkeleton from '$lib/components/LoadingSkeleton.svelte';

	/** @type {'loading' | 'ready' | 'offline'} */
	let state = $state('loading');
	let home = $state(null);
	let seriesHome = $state(null);

	// Series rows are additive and sit after the film rows: M3 kept its home
	// payload separate precisely so the released screen went on working
	// (decision 41). An episode brings its own still and heading; the card is
	// otherwise the same one the film rows use.
	const seriesRows = $derived.by(() => {
		const rows = [];
		const resuming = seriesHome?.continue_watching ?? [];
		if (resuming.length) {
			rows.push({
				kind: 'series_continue',
				cards: resuming.map((item) => ({
					movie: item,
					href: `/episode/${item.id}`,
					art: imageURL(item.episode_metadata?.[0]?.metadata?.still_path, 'w780'),
					title: item.series_title,
					legend: episodeLegend(item)
				}))
			});
		}
		const recent = seriesHome?.recent_series ?? [];
		if (recent.length) {
			rows.push({
				kind: 'series_recent',
				cards: recent.map((item) => ({
					movie: item,
					href: `/serie/${item.id}`,
					legend: item.metadata?.first_air_date?.slice(0, 4) ?? '—'
				}))
			});
		}
		return rows;
	});

	function episodeLegend(item) {
		const numbers = item.episode_numbers ?? [];
		if (numbers.length > 1) {
			return t.series.episodeRange(numbers[0], numbers[numbers.length - 1]);
		}
		return t.series.episodeLabel(numbers[0] ?? '?');
	}

	onMount(async () => {
		// Asked alongside the library rather than before it, so a normal launch
		// pays one extra request in parallel and never a round trip in series.
		// Onboarding is LAN-only. allSettled would swallow the 403, but asking for
		// something deliberately forbidden is still the wrong request to make:
		// the guard is a boundary, not a fallback (decision 44).
		await remote.load();
		// Who is watching decides what the rows contain, so it has to be settled
		// before the first request rather than alongside it.
		await profiles.ready();
		const [library, onboarding, series] = await Promise.allSettled([
			getJSON(profiles.url('/api/library/home')),
			remote.isRemote ? Promise.resolve({ needed: false }) : getJSON('/api/onboarding'),
			getJSON(profiles.url('/api/library/series/home'))
		]);

		if (onboarding.status === 'fulfilled' && onboarding.value.needed) {
			goto('/bienvenue', { replaceState: true });
			return;
		}

		if (library.status === 'rejected') {
			state = 'offline';
			return;
		}
		home = library.value;
		// A server without series is not a failure: the rows simply do not appear.
		if (series.status === 'fulfilled') seriesHome = series.value;
		state = 'ready';
	});
</script>

<svelte:head>
	<title>{t.appName}</title>
</svelte:head>

{#if state === 'loading'}
	<LoadingSkeleton variant="home" label={t.home.loading} />
{:else if state === 'offline'}
	<ChromeScene
		image="/chrome/theia-offline.webp"
		eyebrow={t.appName}
		title={t.home.unreachableTitle}
		body={t.home.unreachable}
		tone="error"
	>
		<button type="button" onclick={() => location.reload()} class="tv-action cursor-pointer" data-remote-default>
			{t.home.retry}
		</button>
	</ChromeScene>
{:else if home.total === 0}
	<ChromeScene
		image="/chrome/theia-empty.webp"
		eyebrow={t.tagline}
		title={t.home.emptyTitle}
		body={t.home.emptyBody}
	>
		<a href="/reglages" class="tv-action tv-action--primary" data-remote-default>
			<span>{t.nav.settings}</span>
			<span aria-hidden="true">→</span>
		</a>
	</ChromeScene>
{:else}
	{#if home.hero}
		<Hero movie={home.hero} kind={home.hero_kind} />
	{/if}

	<!-- Pulled up under the hero's fade, so the first row starts inside the
	     gradient rather than after a visible seam. -->
	<div class:home-rows={home.hero} class:page-body={!home.hero}>
		{#each home.rows as row (row.kind)}
			<Row {row} />
		{/each}

		{#each seriesRows as row (row.kind)}
			<Row {row} />
		{/each}
	</div>
{/if}
