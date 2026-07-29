<script>
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { getJSON } from '$lib/api.js';
	import { strings as t } from '$lib/strings.js';
	import Hero from '$lib/components/Hero.svelte';
	import Row from '$lib/components/Row.svelte';
	import ChromeScene from '$lib/components/ChromeScene.svelte';

	/** @type {'loading' | 'ready' | 'offline'} */
	let state = $state('loading');
	let home = $state(null);

	onMount(async () => {
		// Asked alongside the library rather than before it, so a normal launch
		// pays one extra request in parallel and never a round trip in series.
		const [library, onboarding] = await Promise.allSettled([
			getJSON('/api/library/home'),
			getJSON('/api/onboarding')
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
		state = 'ready';
	});
</script>

<svelte:head>
	<title>{t.appName}</title>
</svelte:head>

{#if state === 'loading'}
	<div class="flex min-h-screen flex-col items-center justify-center gap-5 px-6">
		<span class="brand-orbit" aria-hidden="true"></span>
		<span class="label">{t.home.loading}</span>
	</div>
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
		<Hero movie={home.hero} />
	{/if}

	<!-- Pulled up under the hero's fade, so the first row starts inside the
	     gradient rather than after a visible seam. -->
	<div class:home-rows={home.hero} class:pt-32={!home.hero}>
		{#each home.rows as row (row.title)}
			<Row {row} />
		{/each}
	</div>
{/if}
