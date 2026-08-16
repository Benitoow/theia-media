<script>
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { getAllMovies, searchKey, displayTitle, displayYear, formatRuntime } from '$lib/api.js';
	import { i18n } from '$lib/i18n/index.svelte.js';
	import { strings as t } from '$lib/strings.js';
	import PosterCard from '$lib/components/PosterCard.svelte';
	import Icon from '$lib/components/Icon.svelte';
	import LoadingSkeleton from '$lib/components/LoadingSkeleton.svelte';
	import ChromeScene from '$lib/components/ChromeScene.svelte';

	/** @type {'loading' | 'ready' | 'offline'} */
	let state = $state('loading');
	let movies = $state([]);
	let loaded = $state(0);

	let query = $state('');
	let sort = $state('title');
	let genre = $state('');
	let status = $state('all');

	onMount(async () => {
		// Every home row links here pre-filtered to match what it was showing, so
		// the deep link has to survive a reload rather than being lost on mount.
		// Film detail pages use the same contract for director searches.
		//
		// Unknown values are dropped rather than accepted: a hand-edited
		// ?sort=banana should leave the page in a state its own controls can
		// describe, not a state no dropdown can show.
		const params = $page.url.searchParams;
		const pick = (name, allowed, fallback) => {
			const raw = params.get(name);
			return allowed.includes(raw) ? raw : fallback;
		};

		genre = params.get('genre') ?? '';
		query = params.get('q') ?? '';
		sort = pick('sort', ['title', 'year', 'rating', 'added', 'runtime'], 'title');
		status = pick('status', ['all', 'unseen', 'progress', 'finished'], 'all');
		try {
			movies = await getAllMovies((n) => (loaded = n));
			state = 'ready';
		} catch {
			state = 'offline';
		}
	});

	// Built once per library rather than per keystroke: 274 titles normalised on
	// every character typed is work nobody needs to do twice.
	const indexed = $derived(
		movies.map((movie) => ({
			movie,
			haystack: searchKey(
				[
					displayTitle(movie),
					movie.title,
					movie.metadata?.director,
					movie.metadata?.genres?.join(' '),
					displayYear(movie)
				]
					.filter(Boolean)
					.join(' ')
			)
		}))
	);

	const collator = $derived(
		new Intl.Collator(i18n.localeTag, { sensitivity: 'base', numeric: true })
	);

	const genres = $derived(
		[...new Set(movies.flatMap((m) => m.metadata?.genres ?? []))].sort((a, b) =>
			collator.compare(a, b)
		)
	);

	function matchesStatus(movie) {
		const p = movie.progress;
		switch (status) {
			case 'unseen':
				return !p?.watched_at && !p?.finished;
			case 'progress':
				return p?.position_seconds > 0 && !p?.finished;
			case 'finished':
				return !!p?.finished;
			default:
				return true;
		}
	}

	const filtered = $derived(
		(() => {
			const needle = searchKey(query.trim());
			const result = indexed
				.filter(({ movie, haystack }) => {
					if (needle && !haystack.includes(needle)) return false;
					if (genre && !(movie.metadata?.genres ?? []).includes(genre)) return false;
					return matchesStatus(movie);
				})
				.map(({ movie }) => movie);

			const byTitle = (a, b) => collator.compare(displayTitle(a), displayTitle(b));
			switch (sort) {
				case 'year':
					// Films with no year sink rather than leading the list.
					return result.sort((a, b) => (displayYear(b) ?? 0) - (displayYear(a) ?? 0) || byTitle(a, b));
				case 'rating':
					return result.sort(
						(a, b) => (b.metadata?.vote_average ?? 0) - (a.metadata?.vote_average ?? 0) || byTitle(a, b)
					);
				case 'added':
					return result.sort((a, b) => (b.added_at ?? '').localeCompare(a.added_at ?? '') || byTitle(a, b));
				case 'runtime':
					return result.sort(
						(a, b) => (b.metadata?.runtime_minutes ?? 0) - (a.metadata?.runtime_minutes ?? 0) || byTitle(a, b)
					);
				default:
					return result.sort(byTitle);
			}
		})()
	);

	const filtering = $derived(Boolean(query.trim() || genre || status !== 'all'));

	function reset() {
		query = '';
		genre = '';
		status = 'all';
	}

	const sortOptions = $derived([
		{ value: 'title', label: t.library.sortTitle },
		{ value: 'year', label: t.library.sortYear },
		{ value: 'rating', label: t.library.sortRating },
		{ value: 'added', label: t.library.sortAdded },
		{ value: 'runtime', label: t.library.sortRuntime }
	]);

	const statusOptions = $derived([
		{ value: 'all', label: t.library.statusAll },
		{ value: 'unseen', label: t.library.statusUnseen },
		{ value: 'progress', label: t.library.statusInProgress },
		{ value: 'finished', label: t.library.statusFinished }
	]);

	const addedDate = $derived(
		new Intl.DateTimeFormat(i18n.localeTag, {
			day: 'numeric',
			month: 'short',
			year: 'numeric'
		})
	);

	// An em dash for every value the library does not have read as a placeholder
	// somebody had forgotten to fill in -- and on a library of unmatched files it
	// was most of the grid. The card keeps the line's height either way, so
	// saying nothing is both quieter and more honest than saying "—".
	function cardLegend(movie) {
		switch (sort) {
			case 'rating':
				return movie.metadata?.vote_average
					? t.library.ratingLegend(movie.metadata.vote_average)
					: '';
			case 'added': {
				const date = new Date(movie.added_at);
				return Number.isNaN(date.getTime()) ? '' : addedDate.format(date);
			}
			case 'runtime':
				return formatRuntime(movie.metadata?.runtime_minutes) ?? '';
			default:
				return displayYear(movie) ?? '';
		}
	}
</script>

<svelte:head>
	<title>{t.library.title} — {t.appName}</title>
</svelte:head>

<!--
	The unreachable-server screen is full-bleed, so it sits outside the page
	shell rather than inside it. It used to be a hand-built panel here while the
	home screen used ChromeScene for the identical state; one state, two screens,
	already drifting apart.
-->
{#if state === 'offline'}
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
{:else}
	<main class="page-shell page-body">
		{#if state === 'loading'}
			<LoadingSkeleton variant="library" label={t.library.loadingProgress(loaded)} />
		{:else}
			<header class="mb-10">
				<h1 class="page-title enter">{t.library.title}</h1>
				<p class="label enter enter-2 mt-4">
					{filtering
						? t.library.countFiltered(filtered.length, movies.length)
						: t.library.countAll(movies.length)}
				</p>
			</header>

			<!-- The toolbar is chrome, so it may carry treatment; the grid below it
			     stays plain, as section 6 of the design system requires. -->
			<div class="library-toolbar">
				<div class="library-search">
					<Icon name="search" size={18} class="shrink-0 text-muted" />
					<input
						type="search"
						bind:value={query}
						placeholder={t.library.searchPlaceholder}
						aria-label={t.library.search}
						class="library-search-input"
					/>
					{#if query}
						<button type="button" onclick={() => (query = '')} class="player-icon-button player-icon-button--compact">
							<Icon name="close" size={16} label={t.library.clear} />
						</button>
					{/if}
				</div>

				<div class="library-selects">
					<label class="library-select">
						<span class="label">{t.library.sort}</span>
						<select bind:value={sort}>
							{#each sortOptions as option (option.value)}
								<option value={option.value}>{option.label}</option>
							{/each}
						</select>
					</label>

					<label class="library-select">
						<span class="label">{t.library.genre}</span>
						<select bind:value={genre}>
							<option value="">{t.library.allGenres}</option>
							{#each genres as name (name)}
								<option value={name}>{name}</option>
							{/each}
						</select>
					</label>

					<label class="library-select">
						<span class="label">{t.library.status}</span>
						<select bind:value={status}>
							{#each statusOptions as option (option.value)}
								<option value={option.value}>{option.label}</option>
							{/each}
						</select>
					</label>
				</div>
			</div>

			{#if filtered.length === 0}
				<div class="py-24 text-center">
					<p class="section-title mb-3">{t.library.noResults}</p>
					<p class="tv-copy mb-8 text-muted">{t.library.noResultsBody}</p>
					<button type="button" onclick={reset} class="tv-action cursor-pointer">
						{t.library.reset}
					</button>
				</div>
			{:else}
				<!--
					A wrapping grid rather than the home screen's scrolling rows: this
					is the page for finding one film among hundreds, and a row you
					have to drag through is the wrong shape for that.
				-->
				<div class="library-grid">
					<!-- Six covers the first row on a television and the first three
					     on a phone: the artwork that is already on screen when the
					     page arrives. -->
					{#each filtered as movie, index (movie.id)}
						<PosterCard {movie} fluid legend={cardLegend(movie)} priority={index < 6} />
					{/each}
				</div>
			{/if}
		{/if}
	</main>
{/if}
