<script>
	import { onMount } from 'svelte';
	import { strings as t, formatUptime, formatSize } from '$lib/strings.js';

	/** @type {'checking' | 'online' | 'offline'} */
	let connection = $state('checking');
	let health = $state(null);
	let stats = $state(null);
	let movies = $state([]);
	let scanError = $state(null);
	let scanning = $state(false);

	async function json(path, options) {
		const res = await fetch(path, options);
		if (!res.ok) throw new Error(`HTTP ${res.status}`);
		return res.json();
	}

	async function refresh() {
		try {
			[health, stats] = await Promise.all([json('/api/health'), json('/api/library/stats')]);
			connection = 'online';
			// The grid arrives in M3; this is only enough to prove the scan
			// landed something in the database.
			movies = stats.movies > 0 ? (await json('/api/library/movies?limit=12')).movies : [];
		} catch {
			connection = 'offline';
		}
	}

	async function scan() {
		scanning = true;
		scanError = null;
		try {
			await json('/api/library/scan', { method: 'POST' });
			await refresh();
		} catch (e) {
			scanError = e.message.includes('409') ? t.errors.scanBusy : t.errors.scanFailed;
		} finally {
			scanning = false;
		}
	}

	onMount(() => {
		refresh();
		// Cheap enough to keep running: it is how you notice from the couch that
		// the server on the other side of the flat has stopped.
		const timer = setInterval(refresh, 10_000);
		return () => clearInterval(timer);
	});

	const busy = $derived(scanning || stats?.scanning);
</script>

<svelte:head>
	<title>{t.appName} — {t.tagline}</title>
</svelte:head>

<main class="mx-auto min-h-screen max-w-4xl px-6 py-24 lg:px-16">
	<!-- Hero. The one place the display serif appears, and the only screen with
	     this much room around it. -->
	<header class="mb-24">
		<span class="label">{t.tagline}</span>
		<h1 class="mt-4 mb-6 font-display text-hero font-normal">{t.appName}</h1>
		<p class="max-w-prose text-body text-parchment">{t.pitch}</p>
	</header>

	<!-- Status. Gold appears here once, on the dot. -->
	<section class="mb-16 flex flex-wrap items-center gap-x-8 gap-y-4 border-t border-line pt-6">
		<span class="flex items-center gap-3">
			<span
				class="h-1.5 w-1.5 rounded-full transition-colors"
				class:bg-accent={connection === 'online'}
				class:bg-muted={connection === 'checking'}
				class:bg-error={connection === 'offline'}
				aria-hidden="true"
			></span>
			<span class="text-small">
				{connection === 'online'
					? t.status.online
					: connection === 'checking'
						? t.status.checking
						: t.status.offline}
			</span>
		</span>

		{#if connection === 'online' && health}
			<span class="label">{t.status.version} {health.version}</span>
			<span class="label">{formatUptime(health.uptime_seconds)}</span>
			<span class="label">{stats?.movies ?? 0} {t.status.films}</span>
		{/if}
	</section>

	{#if connection === 'offline'}
		<p class="mb-16 border-l border-error py-1 pl-5 text-small text-parchment">
			{t.errors.unreachable}
		</p>
	{/if}

	<!-- Library -->
	<section>
		<div class="mb-8 flex flex-wrap items-baseline justify-between gap-4">
			<h2 class="label">{t.library.heading}</h2>
			<button
				type="button"
				onclick={scan}
				disabled={busy || connection !== 'online'}
				class="cursor-pointer border border-line px-6 py-3 text-label uppercase transition-colors
				       hover:border-muted disabled:cursor-not-allowed disabled:text-faint disabled:hover:border-line"
			>
				{busy ? t.library.scanning : t.library.scan}
			</button>
		</div>

		{#if scanError}
			<p class="mb-8 border-l border-error py-1 pl-5 text-small text-parchment">{scanError}</p>
		{/if}

		{#if stats && stats.movies === 0}
			<div class="border border-line bg-surface/60 px-8 py-16 text-center">
				<h3 class="mb-4 font-display text-display font-normal">
					{stats.library_paths === 0 ? t.library.emptyTitle : t.library.scannedTitle}
				</h3>
				<p class="mx-auto max-w-prose text-small text-muted">
					{stats.library_paths === 0 ? t.library.emptyBody : t.library.scannedBody}
				</p>
			</div>
		{:else if movies.length > 0}
			<ul class="divide-y divide-line border-y border-line">
				{#each movies as movie (movie.id)}
					<li class="flex items-baseline justify-between gap-6 py-4">
						<span class="truncate text-body">{movie.title}</span>
						<span class="label shrink-0">
							{movie.year ?? '—'} · {formatSize(movie.size_bytes)}
						</span>
					</li>
				{/each}
			</ul>
			{#if stats.movies > movies.length}
				<p class="mt-4 text-label">
					+ {stats.movies - movies.length}
				</p>
			{/if}
		{/if}

		{#if stats?.last_scan}
			<dl class="mt-8 flex flex-wrap gap-x-8 gap-y-2">
				<div class="flex gap-2">
					<dt class="label">{t.library.found}</dt>
					<dd class="text-label text-parchment">{stats.last_scan.found}</dd>
				</div>
				<div class="flex gap-2">
					<dt class="label">{t.library.added}</dt>
					<dd class="text-label text-parchment">{stats.last_scan.added}</dd>
				</div>
				<div class="flex gap-2">
					<dt class="label">{t.library.updated}</dt>
					<dd class="text-label text-parchment">{stats.last_scan.updated}</dd>
				</div>
				<div class="flex gap-2">
					<dt class="label">{t.library.removed}</dt>
					<dd class="text-label text-parchment">{stats.last_scan.removed}</dd>
				</div>
			</dl>

			{#if stats.last_scan.problems?.length}
				<div class="mt-6 border-l border-warning py-1 pl-5">
					<p class="label mb-2">{t.library.problems}</p>
					<ul class="space-y-1">
						{#each stats.last_scan.problems as problem (problem)}
							<li class="text-small break-words text-muted">{problem}</li>
						{/each}
					</ul>
				</div>
			{/if}
		{/if}
	</section>

	<footer class="mt-24 border-t border-line pt-6">
		<span class="micro">{t.library.milestone}</span>
	</footer>
</main>
