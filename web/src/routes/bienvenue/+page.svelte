<script>
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { getJSON } from '$lib/api.js';
	import { strings as t } from '$lib/strings.js';
	import ConnectPanel from '$lib/components/ConnectPanel.svelte';

	let info = $state(null);
	let failure = $state(null);

	onMount(async () => {
		try {
			info = await getJSON('/api/onboarding');
		} catch {
			// No network address means nothing to show and nothing to fix from
			// here. Let the library through rather than trapping the user on a
			// welcome screen.
			goto('/', { replaceState: true });
		}
	});

	async function enter() {
		try {
			await fetch('/api/onboarding/complete', { method: 'POST' });
		} catch {
			// Worst case the screen appears once more. Not worth blocking on.
		}
		goto('/', { replaceState: true });
	}
</script>

<svelte:head>
	<title>{t.welcome.title} — {t.appName}</title>
</svelte:head>

<main class="mx-auto min-h-screen max-w-4xl px-6 pt-32 pb-16 lg:px-16">
	{#if info}
		<header class="mb-16">
			<span class="label">{t.welcome.eyebrow}</span>
			<h1 class="mt-4 mb-6 font-display text-display font-normal">{t.welcome.title}</h1>
			<p class="max-w-prose text-body text-parchment">{t.welcome.body}</p>
		</header>

		<section class="border-y border-line py-12">
			<ConnectPanel {info} size="large" />
		</section>

		<div class="mt-12">
			<button
				type="button"
				onclick={enter}
				class="ease-cine cursor-pointer border border-accent px-7 py-3.5 text-label
				       uppercase text-accent transition-colors duration-160
				       hover:bg-accent hover:text-ink"
			>
				{t.welcome.enter}
			</button>
		</div>
	{:else if failure}
		<p class="border-l border-error py-1 pl-5 text-small text-parchment">{failure}</p>
	{/if}
</main>
