<script>
	import { onMount } from 'svelte';
	import { getJSON } from '$lib/api.js';
	import { strings as t } from '$lib/strings.js';
	import Hero from '$lib/components/Hero.svelte';
	import Row from '$lib/components/Row.svelte';

	/** @type {'loading' | 'ready' | 'offline'} */
	let state = $state('loading');
	let home = $state(null);

	onMount(async () => {
		try {
			home = await getJSON('/api/library/home');
			state = 'ready';
		} catch {
			state = 'offline';
		}
	});
</script>

<svelte:head>
	<title>{t.appName}</title>
</svelte:head>

{#if state === 'loading'}
	<div class="flex min-h-screen items-center justify-center px-6">
		<span class="label">{t.home.loading}</span>
	</div>
{:else if state === 'offline'}
	<div class="flex min-h-screen items-center justify-center px-6">
		<p class="max-w-prose border-l border-error py-1 pl-5 text-small text-parchment">
			{t.home.unreachable}
		</p>
	</div>
{:else if home.total === 0}
	<div class="flex min-h-screen flex-col items-center justify-center px-6 text-center">
		<h1 class="mb-5 font-display text-display font-normal">{t.home.emptyTitle}</h1>
		<p class="mb-8 max-w-prose text-small text-muted">{t.home.emptyBody}</p>
		<a
			href="/reglages"
			class="ease-cine border border-accent px-7 py-3.5 text-label text-accent uppercase
			       transition-colors duration-160 hover:bg-accent hover:text-ink"
		>
			{t.nav.settings}
		</a>
	</div>
{:else}
	{#if home.hero}
		<Hero movie={home.hero} />
	{/if}

	<!-- Pulled up under the hero's fade, so the first row starts inside the
	     gradient rather than after a visible seam. -->
	<div class="relative z-10 -mt-8">
		{#each home.rows as row (row.title)}
			<Row {row} />
		{/each}
	</div>
{/if}
