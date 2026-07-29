<script>
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { getAllMovies, searchKey, displayTitle, displayYear } from '$lib/api.js';
	import { strings as t } from '$lib/strings.js';
	import PosterCard from '$lib/components/PosterCard.svelte';
	import Icon from '$lib/components/Icon.svelte';

	/** @type {'loading' | 'ready' | 'offline'} */
	let state = $state('loading');
	let movies = $state([]);
	let loaded = $state(0);

	let query = $state('');
	let sort = $state('title');
	let genre = $state('');
	let status = $state('all');

	onMount(async () => {
		// A genre row on the home screen links here pre-filtered, so the deep
		// link has to survive a reload rather than being lost on mount.
		genre = $page.url.searchParams.get('genre') ?? '';
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

	const genres = $derived(
		[...new Set(movies.flatMap((m) => m.metadata?.genres ?? []))].sort((a, b) =>
			a.localeCompare(b, 'fr')
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

			const byTitle = (a, b) => displayTitle(a).localeCompare(displayTitle(b), 'fr');
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

	const sortOptions = [
		{ value: 'title', label: t.library.sortTitle },
		{ value: 'year', label: t.library.sortYear },
		{ value: 'rating', label: t.library.sortRating },
		{ value: 'added', label: t.library.sortAdded },
		{ value: 'runtime', label: t.library.sortRuntime }
	];

	const statusOptions = [
		{ value: 'all', label: t.library.statusAll },
		{ value: 'unseen', label: t.library.statusUnseen },
		{ value: 'progress', label: t.library.statusInProgress },
		{ value: 'finished', label: t.library.statusFinished }
	];
</script>

<svelte:head>
	<title>{t.library.title} — {t.appName}</title>
</svelte:head>

<main class="page-shell pt-32 pb-16 lg:pt-40">
	{#if state === 'loading'}
		<div class="flex min-h-[50vh] flex-col items-center justify-center gap-5">
			<span class="brand-orbit" aria-hidden="true"></span>
			<span class="label">{t.library.loading}{loaded ? ` (${loaded})` : ''}</span>
		</div>
	{:else if state === 'offline'}
		<div class="chrome-panel max-w-2xl p-8 sm:p-12">
			<span class="label text-error">{t.home.unreachableTitle}</span>
			<p class="tv-copy mt-5 border-l border-error pl-6">{t.home.unreachable}</p>
			<button type="button" onclick={() => location.reload()} class="tv-action mt-8 cursor-pointer">
				{t.home.retry}
			</button>
		</div>
	{:else}
		<header class="mb-10">
			<h1 class="hero-title !text-[clamp(2.5rem,5.5vw,4.5rem)]">{t.library.title}</h1>
			<p class="label mt-4">
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
					<button type="button" onclick={() => (query = '')} class="player-icon-button !min-h-10 !min-w-10">
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
				{#each filtered as movie (movie.id)}
					<PosterCard {movie} fluid />
				{/each}
			</div>
		{/if}
	{/if}
</main>
