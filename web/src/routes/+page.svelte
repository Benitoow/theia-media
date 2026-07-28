<script>
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { getJSON } from '$lib/api.js';
	import { strings as t } from '$lib/strings.js';
	import Hero from '$lib/components/Hero.svelte';
	import Row from '$lib/components/Row.svelte';

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
	<div class="page-shell flex min-h-screen items-center justify-center py-32">
		<div class="chrome-panel max-w-2xl p-8 sm:p-12">
			<span class="label text-error">{t.appName}</span>
			<p class="tv-copy mt-5 max-w-prose border-l border-error pl-6">
				{t.home.unreachable}
			</p>
		</div>
	</div>
{:else if home.total === 0}
	<section class="relative isolate flex min-h-screen items-center overflow-hidden">
		<img
			src="/chrome/theia-empty.webp"
			alt=""
			class="absolute inset-0 -z-20 h-full w-full object-cover object-[68%_center] opacity-65"
		/>
		<div
			class="absolute inset-0 -z-10"
			style="background:
				linear-gradient(to right, var(--color-ink) 8%, rgba(11,10,9,0.86) 46%, rgba(11,10,9,0.18) 100%),
				linear-gradient(to top, var(--color-ink), transparent 45%)"
		></div>
		<div class="page-shell py-36">
			<div class="max-w-2xl">
				<span class="label">{t.tagline}</span>
				<h1 class="hero-title mt-5 mb-7">{t.home.emptyTitle}</h1>
				<p class="tv-copy mb-9 max-w-prose">{t.home.emptyBody}</p>
				<a href="/reglages" class="tv-action tv-action--primary" data-remote-default>
					<span>{t.nav.settings}</span>
					<span aria-hidden="true">→</span>
				</a>
			</div>
		</div>
	</section>
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
