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

	// What was looked for before.
	//
	// This screen used to be a field alone in an empty page: on a 1512px window
	// its content stopped 340px down and everything below was nothing at all. A
	// search screen with nothing yet to search is the one place a little memory
	// earns its keep, and the answer is already local -- this is the browser's
	// own storage, not a server-side record of what the household looks for.
	const recentKey = 'theia.recent-searches';
	const recentMax = 8;
	let recent = $state([]);

	// Every keystroke is not a question. A quarter of a second is long enough
	// that typing a title is one request and short enough that nobody waits.
	const debounceMs = 250;
	let timer = null;
	// Answers can overtake each other on a slow link; only the newest one counts.
	let sequence = 0;

	const total = $derived(results.movies.length + results.series.length);
	const nothing = $derived(state === 'ready' && total === 0 && answered !== '');
	// The recents stand in for results, so they show only when there is no
	// result set on screen to replace them.
	const showRecent = $derived(state === 'idle' && recent.length > 0);

	onMount(async () => {
		recent = readRecent();
		await profiles.ready();
		// Arriving with ?q= — from a link, a bookmark, or the browser's back
		// button — searches straight away rather than showing an empty box.
		const initial = $page.url.searchParams.get('q') ?? '';
		if (initial.trim()) {
			query = initial;
			run(initial);
		}
	});

	function readRecent() {
		try {
			const stored = JSON.parse(localStorage.getItem(recentKey) ?? '[]');
			return Array.isArray(stored)
				? stored.filter((v) => typeof v === 'string').slice(0, recentMax)
				: [];
		} catch {
			// Private modes deny storage, and a corrupted value is not worth a
			// broken page. No memory is a perfectly good state for this screen.
			return [];
		}
	}

	function remember(value) {
		const trimmed = value.trim();
		if (!trimmed) return;
		// Case-insensitive dedup, newest first: searching the same title twice
		// should move it up the list rather than sit in it twice.
		const without = recent.filter((entry) => entry.toLowerCase() !== trimmed.toLowerCase());
		recent = [trimmed, ...without].slice(0, recentMax);
		persist();
	}

	function forget(value) {
		recent = recent.filter((entry) => entry !== value);
		persist();
	}

	function persist() {
		try {
			localStorage.setItem(recentKey, JSON.stringify(recent));
		} catch {
			// Nothing to do and nothing to say: the list simply does not survive
			// this browser, which is the same as never having had one.
		}
	}

	function onInput() {
		clearTimeout(timer);
		const current = query;
		if (!current.trim()) {
			state = 'idle';
			answered = '';
			results = { movies: [], series: [], truncated: false };
			rememberInURL('');
			return;
		}
		timer = setTimeout(() => run(current), debounceMs);
	}

	// The query lives in the URL so that a result can be shared, reloaded and
	// come back to. replaceState rather than a navigation: typing should not
	// bury the page the user came from under thirty history entries.
	function rememberInURL(value) {
		const url = new URL($page.url);
		if (value) url.searchParams.set('q', value);
		else url.searchParams.delete('q');
		replaceState(url, {});
	}

	async function run(value) {
		const mine = ++sequence;
		state = 'searching';
		rememberInURL(value);
		try {
			const body = await getJSON(
				profiles.url(`/api/library/search?q=${encodeURIComponent(value)}`)
			);
			if (mine !== sequence) return;
			results = body;
			answered = value;
			state = 'ready';
			// Only a search that found something is worth keeping: a list of
			// things this library does not contain helps nobody.
			if (body.movies.length + body.series.length > 0) remember(value);
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

	function repeat(value) {
		query = value;
		clearTimeout(timer);
		run(value);
	}

	function clear() {
		query = '';
		onInput();
	}
</script>

<svelte:head>
	<title>{t.search.title} — {t.appName}</title>
</svelte:head>

<main class="page-shell page-body">
	<h1 class="page-title">{t.search.title}</h1>

	<!-- The field is the page, so it carries the page's weight: full measure up
	     to a readable maximum, and the same pill the library toolbar uses rather
	     than a second kind of box invented for this screen. -->
	<form class="search-field" onsubmit={onSubmit} role="search">
		<Icon name="search" size={22} class="shrink-0 text-muted" />
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
			<button type="button" onclick={clear} class="player-icon-button player-icon-button--compact">
				<Icon name="close" size={16} label={t.library.clear} />
			</button>
		{/if}
	</form>

	<p class="search-hint text-small text-muted">{t.search.prompt}</p>

	<div class="search-status" aria-live="polite">{#if state === 'searching'}
			<p class="text-small text-muted">{t.search.searching}</p>
		{:else if state === 'failed'}
			<p class="text-small text-error" role="alert">{t.search.failed}</p>
		{:else if nothing}
			<p class="tv-copy">{t.search.empty(answered)}</p>
		{:else if state === 'ready' && total > 0}
			<p class="text-small text-muted">
				{t.search.results(total)}{results.truncated ? ` · ${t.search.truncated}` : ''}
			</p>
		{/if}</div>

	{#if showRecent}
		<section class="search-recent">
			<h2 class="label">{t.search.recent}</h2>
			<ul class="search-chips">
				{#each recent as entry (entry)}
					<li class="search-chip">
						<button type="button" class="search-chip-go" onclick={() => repeat(entry)}>
							{entry}
						</button>
						<button
							type="button"
							class="search-chip-forget"
							onclick={() => forget(entry)}
							aria-label="{t.search.forget} — {entry}"
						>
							<Icon name="close" size={12} />
						</button>
					</li>
				{/each}
			</ul>
		</section>
	{/if}

	{#if results.movies.length}
		<section class="search-results">
			<h2 class="label">{t.search.films}</h2>
			<div class="library-grid">
				{#each results.movies as movie (movie.id)}
					<PosterCard {movie} fluid />
				{/each}
			</div>
		</section>
	{/if}

	{#if results.series.length}
		<section class="search-results">
			<h2 class="label">{t.search.series}</h2>
			<div class="library-grid">
				{#each results.series as item (item.id)}
					<PosterCard
						movie={item}
						href="/serie/{item.id}"
						fluid
						playable={false}
						legend={item.metadata?.first_air_date?.slice(0, 4) ?? ''}
					/>
				{/each}
			</div>
		</section>
	{/if}
</main>
