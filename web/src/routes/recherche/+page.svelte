<script>
	// One search, over both catalogues.
	//
	// /films already searches films and /series already searches series, each by
	// filtering a catalogue it had downloaded. Both are good pages; between them
	// they made you decide whether the thing you half-remembered was a film or a
	// series before you were allowed to look for it.
	//
	// This asks the server instead, which is also what makes it usable from
	// outside the house: a phone on the tunnel asks a question and gets twenty
	// rows back, rather than pulling the whole library down to filter locally.
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { replaceState } from '$app/navigation';
	import { getJSON } from '$lib/api.js';
	import { profiles } from '$lib/profiles.svelte.js';
	import { strings as t } from '$lib/strings.js';
	import Icon from '$lib/components/Icon.svelte';
	import PosterCard from '$lib/components/PosterCard.svelte';

	let query = $state('');
	/** @type {'idle' | 'searching' | 'ready' | 'failed'} */
	let state = $state('idle');
	let results = $state({ movies: [], series: [], truncated: false });

	// The query this result set answers, so the empty message can quote what was
	// actually searched rather than what has since been typed over it.
	let answered = $state('');

	// Every keystroke is not a question. A quarter of a second is long enough
	// that typing a title is one request and short enough that nobody waits.
	const debounceMs = 250;
	let timer = null;
	// Answers can overtake each other on a slow link; only the newest one counts.
	let sequence = 0;

	const total = $derived(results.movies.length + results.series.length);
	const nothing = $derived(state === 'ready' && total === 0 && answered !== '');

	onMount(async () => {
		await profiles.ready();
		// Arriving with ?q= — from a link, a bookmark, or the browser's back
		// button — searches straight away rather than showing an empty box.
		const initial = $page.url.searchParams.get('q') ?? '';
		if (initial.trim()) {
			query = initial;
			run(initial);
		}
	});

	function onInput() {
		clearTimeout(timer);
		const current = query;
		if (!current.trim()) {
			state = 'idle';
			answered = '';
			results = { movies: [], series: [], truncated: false };
			remember('');
			return;
		}
		timer = setTimeout(() => run(current), debounceMs);
	}

	// The query lives in the URL so that a result can be shared, reloaded and
	// come back to. replaceState rather than a navigation: typing should not
	// bury the page the user came from under thirty history entries.
	function remember(value) {
		const url = new URL($page.url);
		if (value) url.searchParams.set('q', value);
		else url.searchParams.delete('q');
		replaceState(url, {});
	}

	async function run(value) {
		const mine = ++sequence;
		state = 'searching';
		remember(value);
		try {
			const body = await getJSON(
				profiles.url(`/api/library/search?q=${encodeURIComponent(value)}`)
			);
			if (mine !== sequence) return;
			results = body;
			answered = value;
			state = 'ready';
		} catch {
			if (mine !== sequence) return;
			state = 'failed';
		}
	}

	function onSubmit(event) {
		event.preventDefault();
		clearTimeout(timer);
		if (query.trim()) run(query);
	}
</script>

<svelte:head>
	<title>{t.search.title} — {t.appName}</title>
</svelte:head>

<main class="page-shell py-16 md:py-24">
	<header class="mb-10">
		<h1 class="page-title">{t.search.title}</h1>
		<p class="tv-copy mt-4 max-w-prose">{t.search.prompt}</p>
	</header>

	<form class="search-field" onsubmit={onSubmit} role="search">
		<Icon name="search" size={20} class="shrink-0 text-muted" />
		<input
			type="search"
			bind:value={query}
			oninput={onInput}
			placeholder={t.search.placeholder}
			aria-label={t.search.label}
			class="search-input"
			data-remote-default
		/>
		{#if query}
			<button
				type="button"
				onclick={() => {
					query = '';
					onInput();
				}}
				class="player-icon-button !min-h-10 !min-w-10"
			>
				<Icon name="close" size={16} label={t.library.clear} />
			</button>
		{/if}
	</form>

	<div class="mt-6" aria-live="polite">
		{#if state === 'searching'}
			<p class="text-small text-muted">{t.search.searching}</p>
		{:else if state === 'failed'}
			<p class="text-small text-error" role="alert">{t.search.failed}</p>
		{:else if nothing}
			<p class="tv-copy">{t.search.empty(answered)}</p>
		{:else if state === 'ready' && total > 0}
			<p class="text-small text-muted">
				{t.search.results(total)}{results.truncated ? ` · ${t.search.truncated}` : ''}
			</p>
		{/if}
	</div>

	{#if results.movies.length}
		<section class="mt-10">
			<h2 class="label mb-5">{t.search.films}</h2>
			<div class="library-grid">
				{#each results.movies as movie (movie.id)}
					<PosterCard {movie} fluid />
				{/each}
			</div>
		</section>
	{/if}

	{#if results.series.length}
		<section class="mt-12">
			<h2 class="label mb-5">{t.search.series}</h2>
			<div class="library-grid">
				{#each results.series as item (item.id)}
					<PosterCard
						movie={item}
						href="/serie/{item.id}"
						fluid
						legend={item.metadata?.first_air_date?.slice(0, 4) ?? '—'}
					/>
				{/each}
			</div>
		</section>
	{/if}
</main>

<style>
	/* The same shape as the library toolbar's field, standing on its own: this
	   page is the search, so the box is the page's first object rather than one
	   control in a row of them. */
	.search-field {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		max-width: 42rem;
		border: 1px solid var(--color-line);
		background: var(--color-surface);
		padding: 0.35rem 0.9rem;
	}

	.search-field:focus-within {
		border-color: var(--color-muted);
	}

	.search-input {
		flex: 1 1 auto;
		min-width: 0;
		border: 0;
		background: transparent;
		padding: 0.65rem 0;
		color: var(--color-bone);
		font-size: 1rem;
	}

	.search-input:focus {
		outline: none;
	}

	.search-input::placeholder {
		color: var(--color-faint);
	}
</style>
