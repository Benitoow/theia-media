<script>
	import { onMount } from 'svelte';
	import { strings as t, formatUptime } from '$lib/strings.js';

	/** @type {'checking' | 'online' | 'offline'} */
	let state = $state('checking');
	let health = $state(null);

	async function poll() {
		try {
			const res = await fetch('/api/health');
			if (!res.ok) throw new Error(`HTTP ${res.status}`);
			health = await res.json();
			state = 'online';
		} catch {
			state = 'offline';
		}
	}

	onMount(() => {
		poll();
		// Cheap enough to keep running: it is how you notice from the couch that
		// the server on the other side of the flat has stopped.
		const timer = setInterval(poll, 10_000);
		return () => clearInterval(timer);
	});
</script>

<svelte:head>
	<title>{t.appName} — {t.tagline}</title>
</svelte:head>

<main class="mx-auto flex min-h-screen max-w-2xl flex-col justify-center px-6 py-16">
	<header class="mb-10">
		<div class="mb-3 flex items-center gap-3">
			<img src="/favicon.svg" alt="" class="h-8 w-8" />
			<h1 class="text-3xl font-semibold tracking-tight">{t.appName}</h1>
		</div>
		<p class="text-muted">{t.tagline}</p>
	</header>

	<div class="mb-8 flex items-center gap-3 text-sm">
		<span
			class="h-2 w-2 rounded-full transition-colors"
			class:bg-helios={state === 'online'}
			class:bg-muted={state === 'checking'}
			class:bg-red-500={state === 'offline'}
			aria-hidden="true"
		></span>

		{#if state === 'checking'}
			<span class="text-muted">{t.status.checking}</span>
		{:else if state === 'online'}
			<span>{t.status.online}</span>
			<span class="text-muted">·</span>
			<span class="text-muted">{t.status.version} {health.version}</span>
			<span class="text-muted">·</span>
			<span class="text-muted">
				{t.status.uptime}
				{formatUptime(health.uptime_seconds)}
			</span>
		{:else}
			<span class="text-red-400">{t.status.offline}</span>
		{/if}
	</div>

	{#if state === 'offline'}
		<p class="mb-8 rounded-lg border border-red-900/50 bg-red-950/30 px-4 py-3 text-sm text-red-300">
			{t.errors.unreachable}
		</p>
	{/if}

	<section class="rounded-xl border border-edge bg-slate/60 px-6 py-8">
		<h2 class="mb-2 text-lg font-medium">{t.library.emptyTitle}</h2>
		<p class="mb-5 text-sm leading-relaxed text-muted">{t.library.emptyBody}</p>
		<p class="border-t border-edge pt-4 text-xs text-muted/70">{t.library.emptyHint}</p>
	</section>
</main>
