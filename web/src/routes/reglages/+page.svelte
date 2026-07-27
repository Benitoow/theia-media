<script>
	import { onMount } from 'svelte';
	import { getJSON } from '$lib/api.js';
	import { strings as t, formatUptime } from '$lib/strings.js';

	let health = $state(null);
	let stats = $state(null);
	let settings = $state(null);
	let scanning = $state(false);
	let scanError = $state(null);

	async function refresh() {
		try {
			[health, stats, settings] = await Promise.all([
				getJSON('/api/health'),
				getJSON('/api/library/stats'),
				getJSON('/api/settings')
			]);
		} catch {
			// The layout already shows a dead page well enough; a settings screen
			// that cannot reach the server has nothing useful to add.
		}
	}

	async function scan() {
		scanning = true;
		scanError = null;
		try {
			await getJSON('/api/library/scan', { method: 'POST' });
			await refresh();
		} catch (e) {
			scanError = e.status === 409 ? t.errors.scanBusy : t.errors.scanFailed;
		} finally {
			scanning = false;
		}
	}

	onMount(() => {
		refresh();
		const timer = setInterval(refresh, 10_000);
		return () => clearInterval(timer);
	});

	const busy = $derived(scanning || stats?.scanning);
</script>

<svelte:head>
	<title>{t.settings.heading} — {t.appName}</title>
</svelte:head>

<main class="mx-auto max-w-3xl px-6 pt-32 pb-16 lg:px-16">
	<h1 class="mb-16 font-display text-display font-normal">{t.settings.heading}</h1>

	{#if settings && stats}
		<section class="mb-14">
			<h2 class="label mb-5">{t.settings.server}</h2>
			<dl class="grid gap-x-8 gap-y-3 sm:grid-cols-[11rem_1fr]">
				<dt class="label">{t.settings.version}</dt>
				<dd class="text-small text-parchment">{settings.version}</dd>
				<dt class="label">{t.settings.port}</dt>
				<dd class="text-small text-parchment">{settings.port}</dd>
				<dt class="label">{t.settings.hostname}</dt>
				<dd class="text-small text-parchment">{settings.hostname}.local</dd>
				<dt class="label">{t.settings.dataDir}</dt>
				<dd class="text-small break-all text-parchment">{settings.data_dir}</dd>
				{#if health}
					<dt class="label">{t.settings.lastScan}</dt>
					<dd class="text-small text-parchment">{formatUptime(health.uptime_seconds)}</dd>
				{/if}
			</dl>
		</section>

		<section class="mb-14">
			<div class="mb-5 flex flex-wrap items-baseline justify-between gap-4">
				<h2 class="label">{t.settings.library}</h2>
				<button
					type="button"
					onclick={scan}
					disabled={busy}
					class="ease-cine cursor-pointer border border-line px-6 py-3 text-label uppercase
					       transition-colors duration-160 hover:border-muted
					       disabled:cursor-not-allowed disabled:text-faint disabled:hover:border-line"
				>
					{busy ? t.settings.scanning : t.settings.scan}
				</button>
			</div>

			{#if scanError}
				<p class="mb-5 border-l border-error py-1 pl-5 text-small text-parchment">{scanError}</p>
			{/if}

			<dl class="mb-6 grid gap-x-8 gap-y-3 sm:grid-cols-[11rem_1fr]">
				<dt class="label">{t.settings.films}</dt>
				<dd class="text-small text-parchment">{stats.movies}</dd>
				<dt class="label">{t.settings.paths}</dt>
				<dd class="text-small text-parchment">
					{#if settings.library_paths?.length}
						{#each settings.library_paths as path (path)}
							<span class="block break-all">{path}</span>
						{/each}
					{:else}
						<span class="text-muted">{t.settings.noPaths}</span>
					{/if}
				</dd>
			</dl>

			{#if stats.last_scan}
				<dl class="flex flex-wrap gap-x-8 gap-y-2 border-t border-line pt-5">
					{#each [[t.settings.found, stats.last_scan.found], [t.settings.added, stats.last_scan.added], [t.settings.updated, stats.last_scan.updated], [t.settings.removed, stats.last_scan.removed], [t.settings.enriched, stats.last_scan.enriched], [t.settings.notFound, stats.last_scan.not_found]] as [label, value] (label)}
						<div class="flex gap-2">
							<dt class="label">{label}</dt>
							<dd class="text-label text-parchment">{value}</dd>
						</div>
					{/each}
				</dl>

				{#if stats.last_scan.problems?.length}
					<div class="mt-6 border-l border-warning py-1 pl-5">
						<p class="label mb-2">{t.settings.problems}</p>
						<ul class="space-y-1">
							{#each stats.last_scan.problems as problem (problem)}
								<li class="text-small break-words text-muted">{problem}</li>
							{/each}
						</ul>
					</div>
				{/if}
			{/if}
		</section>

		<section>
			<h2 class="label mb-5">{t.settings.metadata}</h2>
			{#if settings.tmdb.configured}
				<p class="text-small text-parchment">
					TMDB · <span class="text-muted">{t.settings.source} {settings.tmdb.source}</span>
				</p>
			{:else}
				<p class="border-l border-warning py-1 pl-5 text-small text-parchment">
					{settings.tmdb.advice}
				</p>
			{/if}
		</section>

		<p class="micro mt-16 border-t border-line pt-6">{t.settings.milestone}</p>
	{/if}
</main>
