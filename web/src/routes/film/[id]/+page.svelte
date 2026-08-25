<script>
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { getJSON, imageURL, displayTitle, displayYear, formatRuntime } from '$lib/api.js';
	import { strings as t } from '$lib/strings.js';
	import Player from '$lib/components/Player.svelte';
	import Icon from '$lib/components/Icon.svelte';
	import LoadingSkeleton from '$lib/components/LoadingSkeleton.svelte';
	import FileChoice from '$lib/components/FileChoice.svelte';
	import MatchDialog from '$lib/components/MatchDialog.svelte';
	import Certificate from '$lib/components/Certificate.svelte';
	import Rating from '$lib/components/Rating.svelte';
	import MediaBadges from '$lib/components/MediaBadges.svelte';
	import CastList from '$lib/components/CastList.svelte';
	import Credits from '$lib/components/Credits.svelte';
	import Row from '$lib/components/Row.svelte';
	import { profiles } from '$lib/profiles.svelte.js';
	import { remote } from '$lib/remote.svelte.js';

	/** @type {'loading' | 'ready' | 'missing'} */
	let state = $state('loading');
	let movie = $state(null);
	let playing = $state(false);

	// Which file the player will open. Chosen by hand: M1-BE deliberately
	// refuses to rank quality, so the only default here is the primary file the
	// server already flagged. Audio and subtitles are the player's business.
	let fileId = $state(null);

	const meta = $derived(movie?.metadata ?? {});
	const backdrop = $derived(imageURL(meta.backdrop_path, 'w1280'));
	const poster = $derived(imageURL(meta.poster_path, 'w342'));
	const resumable = $derived(
		movie?.progress?.position_seconds > 0 && !movie?.progress?.finished
	);
	const resumeMinutes = $derived(
		resumable ? Math.max(1, Math.floor(movie.progress.position_seconds / 60)) : 0
	);
	const progressPercent = $derived(
		resumable && movie.progress.duration_seconds > 0
			? Math.min(100, (movie.progress.position_seconds / movie.progress.duration_seconds) * 100)
			: 0
	);

	// The record behind the film, beyond the synopsis. All of it came from the
	// same TMDB call the poster did; none of it costs a request.
	//
	// The original title is shown only when it differs from what the page is
	// already calling the film. "Heat" under a heading reading Heat is noise.
	const originalTitle = $derived(
		meta.original_title && meta.original_title !== displayTitle(movie) ? meta.original_title : ''
	);

	// One line per role, names joined. Roles arrive as codes -- writing, music,
	// cinematography -- and the words for them live in the catalogues.
	const creditRows = $derived([
		{ label: t.credits.originalTitle, value: originalTitle },
		...['writing', 'music', 'cinematography'].map((role) => ({
			label: t.credits[role],
			value: (meta.crew ?? [])
				.filter((person) => person.role === role)
				.map((person) => person.name)
				.join(' · ')
		}))
	]);

	// The other parts of the saga that are actually in the library. The server
	// only sends what can be played, so an empty list means a one-off film or a
	// household that owns a single part -- either way, no row.
	const collectionParts = $derived(movie?.collection_parts ?? []);

	const files = $derived(movie?.files ?? []);
	const selectedFile = $derived(files.find((file) => file.id === fileId) ?? null);

	const watched = $derived(!!movie?.progress?.finished);

	// Correcting a match changes what a file *is* for the whole household, so it
	// is administration and the remote listener refuses it. A button that always
	// fails is worse than no button (decision 44).
	let correcting = $state(false);
	let watchedBusy = $state(false);
	let watchedFailed = $state(false);

	// Marking watched, and taking it back.
	//
	// Unmarking is exactly forgetting the position, which is why it is a DELETE
	// on the same resource rather than a second verb: a film nobody has watched
	// and a film somebody un-watched are the same state.
	async function toggleWatched() {
		if (!movie) return;
		watchedBusy = true;
		watchedFailed = false;
		try {
			const path = profiles.url(`/api/library/movies/${movie.id}/watched`);
			if (watched) {
				const res = await fetch(path, { method: 'DELETE' });
				if (!res.ok) throw new Error(String(res.status));
				movie = { ...movie, progress: { position_seconds: 0, finished: false } };
			} else {
				syncProgress(await getJSON(path, { method: 'PUT' }));
			}
		} catch {
			watchedFailed = true;
		} finally {
			watchedBusy = false;
		}
	}

	// A correction replaces the record wholesale, so the page is re-read rather
	// than patched: the poster, the synopsis, the cast and the runtime all
	// belong to the film that was just chosen.
	async function onMatchApplied(updated) {
		correcting = false;
		if (updated) {
			movie = updated;
			return;
		}
		// Reverted to automatic. Nothing has been looked up yet, so the page
		// keeps what it has until the next pass fills it in.
		try {
			movie = await getJSON(profiles.url(`/api/library/movies/${$page.params.id}`));
		} catch {
			// Leaving the page as it stands is honest: the correction was
			// cleared on the server whether or not this re-read worked.
		}
	}

	onMount(async () => {
		try {
			await profiles.ready();
			movie = await getJSON(profiles.url(`/api/library/movies/${$page.params.id}`));
			// `files` only exists on the detail payload. The primary file is the
			// server's existing bookkeeping, not a quality judgement, so it is a
			// legitimate starting point for a choice the user still owns.
			fileId = (movie.files?.find((file) => file.is_primary) ?? movie.files?.[0])?.id ?? null;
			state = 'ready';
			// The home hero's "Reprendre" links here with this flag rather than
			// opening a player it does not own. The player still asks whether to
			// resume or start over; what this skips is the detour through a page
			// nobody wanted to read on the way back to a film already half seen.
			if ($page.url.searchParams.has('reprendre')) playing = true;
		} catch {
			state = 'missing';
		}
	});

	function syncProgress(progress) {
		if (!movie || !progress) return;
		movie = { ...movie, progress };
	}

	function onFileChoice({ fileId: nextFile }) {
		fileId = nextFile ?? fileId;
	}

	// A measurement replaces the page's copy of that one file. Keeping the single
	// copy here rather than a second one inside the chooser means the player and
	// the list can never disagree about what was actually measured.
	function onFileMeasured(measured) {
		if (!movie || !measured?.id) return;
		movie = {
			...movie,
			files: movie.files.map((file) => (file.id === measured.id ? measured : file))
		};
		// Track ids belong to a measurement. Re-measuring the selected file can
		// retire the chosen one, so the selection goes back to the file default
		// rather than pointing at a track that may no longer exist.
		if (measured.id === fileId) audioTrackId = null;
	}
</script>

<svelte:head>
	<title>{movie ? displayTitle(movie) : t.appName}</title>
</svelte:head>

{#if state === 'loading'}
	<LoadingSkeleton variant="detail" label={t.home.loading} />
{:else if state === 'missing'}
	<div class="page-shell flex min-h-screen items-center justify-center py-32">
		<div class="chrome-panel max-w-xl p-8 text-center sm:p-12">
			<h1 class="font-display text-display font-normal">{t.film.notFound}</h1>
			<a href="/" class="tv-link label mt-7 justify-center">← {t.nav.back}</a>
		</div>
	</div>
{:else}
	<!-- Chrome again: the identity goes in full here, as on the hero. -->
	<article>
		<!-- The header exists to hold the backdrop. Without one it still claimed
		     68svh, which on an unmatched film left roughly 340px of empty black
		     between the navigation and the title. It keeps enough height for the
		     poster to overlap something, and no more. -->
		<header
			class="relative isolate overflow-hidden {backdrop ? 'min-h-[68svh]' : 'min-h-[34svh]'}"
		>
			{#if backdrop}
				<!-- Framed from the top for the reason the home hero is: the
				     navigation pill floats over the first 128px of this header and the
				     poster is pulled up over its bottom 208px, so the headroom is the
				     only part of the frame nothing else covers. Centring it put the
				     subject behind the bar. The rule holds for every backdrop the bar
				     floats over -- this header, the series one and Hero.svelte -- and is
				     written down in the design system section 5. -->
				<img
					src={backdrop}
					alt=""
					fetchpriority="high"
					class="absolute inset-0 -z-20 h-full w-full scale-[1.015] object-cover object-top opacity-[0.72]"
				/>
			{/if}
			<div class="picture-veil picture-veil--detail"></div>
		</header>

		<!-- The pull is what makes the poster overlap the backdrop. With no
		     backdrop there is nothing to overlap, and pulling the same 208px
		     anyway put the title behind the navigation bar. -->
		<div class="page-shell relative z-10 pb-20 {backdrop ? '-mt-52' : '-mt-20'}">
			<div class="flex flex-col gap-10 md:flex-row md:items-end md:gap-14">
				<!-- Poster keeps the grid's locked 2:3 and its plainness. -->
				<!-- Section 6.1 keeps a real 2:3 poster on this page because it "has
				     the room for it". A 390px phone does not: at w-48 the poster
				     was 53% of the screen width, and the title, the metadata and
				     the play button were all below the fold behind it. The
				     backdrop above is already showing the artwork, so here it
				     shrinks to a token of itself rather than competing. -->
				<div class="w-28 shrink-0 self-start sm:w-48 lg:w-56 2xl:w-64">
					<div
						class="aspect-[2/3] overflow-hidden rounded-[var(--radius-card)] border border-line bg-surface
						       shadow-[0_1.75rem_4rem_rgba(0,0,0,0.42)]"
					>
						{#if poster}
							<img src={poster} alt="" decoding="async" class="h-full w-full object-cover" />
						{:else}
							<!-- Same stand-in the grid uses, for the same reason: an empty
							     framed rectangle this size reads as a picture that failed
							     to load rather than as a film TMDB never recognised. -->
							<div class="poster-fallback" aria-hidden="true">
								<span class="poster-fallback-initial">{displayTitle(movie).trim()[0] ?? '?'}</span>
							</div>
						{/if}
					</div>

					{#if resumable && movie.progress.duration_seconds > 0}
						<div
							class="film-progress"
							role="progressbar"
							aria-label={t.film.progress}
							aria-valuemin="0"
							aria-valuemax="100"
							aria-valuenow={Math.round(progressPercent)}
						>
							<span style="width: {progressPercent}%"></span>
						</div>
					{/if}
				</div>

				<div class="min-w-0 flex-1 pb-2">
					{#if meta.genres?.length}
						<span class="label enter">{meta.genres.join(' · ')}</span>
					{/if}

					<h1
						class="page-title page-title--feature enter mt-4 {meta.tagline ? 'mb-4' : 'mb-7'}"
					>
						{displayTitle(movie)}
					</h1>

					{#if meta.tagline}
						<!-- The tagline is a sentence, so it is set in the reading face and
						     not in the display serif: section 4 keeps that register for
						     titles, and a marketing line in small capitals reads as a second
						     heading arguing with the first. Italic and quiet is what makes it
						     legible as a caption. -->
						<p class="film-tagline enter enter-2 mb-7">{meta.tagline}</p>
					{/if}

					<div class="enter enter-2 mb-5 flex flex-wrap items-center gap-x-6 gap-y-3">
						{#if displayYear(movie)}<span class="label">{displayYear(movie)}</span>{/if}
						{#if formatRuntime(meta.runtime_minutes)}
							<span class="label">{formatRuntime(meta.runtime_minutes)}</span>
						{/if}
						<Certificate value={meta.certification} country={meta.certification_country} />
						{#if meta.director}
							<a
								href="/films?q={encodeURIComponent(meta.director)}"
								class="film-director-link label"
								aria-label={t.film.moreByDirector(meta.director)}
							>
								<span>{meta.director}</span>
								<span aria-hidden="true">→</span>
							</a>
						{/if}
						<Rating value={meta.vote_average} />
					</div>

					<!-- Under the metadata line rather than in it. The year, the runtime
					     and the director describe a film; 4K and TrueHD describe one encode
					     of it, and a viewer who has two files of the same film needs to see
					     which of the two is on screen. It follows the chosen file, so
					     picking the other one relabels it. -->
					<MediaBadges media={selectedFile?.media} />

					<div class="film-actions mb-12">
						<button
							type="button"
							onclick={() => (playing = true)}
							class="tv-action tv-action--primary cursor-pointer"
							data-remote-default
						>
							<Icon name="play" size={18} />
							<span>{resumable ? t.player.resumeAtMinutes(resumeMinutes) : t.player.play}</span>
						</button>

						<button
							type="button"
							onclick={toggleWatched}
							disabled={watchedBusy}
							class="tv-action cursor-pointer"
							aria-pressed={watched}
						>
							<Icon name={watched ? 'check' : 'plus'} size={16} />
							<span>{watchedBusy ? t.watched.marking : watched ? t.watched.unmark : t.watched.mark}</span>
						</button>

						{#if !remote.isRemote}
							<button
								type="button"
								onclick={() => (correcting = true)}
								class="tv-action cursor-pointer"
							>
								<Icon name="search" size={16} />
								<span>{t.match.wrongFilm}</span>
							</button>
						{/if}
					</div>

					{#if watchedFailed}
						<p class="mb-8 text-small text-error" role="alert">{t.watched.failed}</p>
					{/if}

					<!-- The files behind the film. One card in the catalogue, the choice
					     of what actually plays made here, by hand.

					     It sits directly under the play button on purpose. Below the cast
					     list it was both editorially wrong -- choosing the file is part of
					     deciding to watch, not trivia after the credits -- and unreachable
					     by remote: the layout's geometric D-pad scored the distant "Retour"
					     link below the first file option and skipped the whole list. -->
					<FileChoice
						basePath={`/api/library/movies/${movie.id}`}
						{files}
						{fileId}
						onselect={onFileChoice}
						onmeasure={onFileMeasured}
					/>

					{#if meta.status === 'not_found'}
						<p class="mt-10 mb-8 border-l border-warning py-1 pl-5 text-small text-parchment">
							{t.film.unmatched}
						</p>
					{/if}

					<p class="tv-copy mt-10 mb-12 max-w-[46rem]">
						{meta.overview || t.film.noOverview}
					</p>

					<Credits rows={creditRows} heading={t.credits.heading} />

					<CastList cast={meta.cast ?? []} heading={t.film.cast} />

					<a href="/" class="tv-link label mt-10">
						← {t.nav.back}
					</a>
				</div>
			</div>
		</div>

		<!-- The saga, if the library holds more of it. Outside the page shell
		     because Row carries its own gutter and heading alignment: nested in the
		     detail column it would be padded twice and would not line up with any
		     other row in the interface. -->
		{#if collectionParts.length}
			<div class="film-collection">
				<Row
					row={{ kind: 'collection', movies: collectionParts }}
					title={t.credits.collection(meta.collection?.name ?? '')}
				/>
			</div>
		{/if}
	</article>

	{#if playing}
		<Player
			{movie}
			fileId={selectedFile?.id ?? null}
			subtitleBase={selectedFile
				? `/api/library/movies/${movie.id}/files/${selectedFile.id}`
				: null}
			onprogress={syncProgress}
			onclose={() => (playing = false)}
		/>
	{/if}

	{#if correcting}
		<MatchDialog
			kind="movies"
			id={movie.id}
			title={displayTitle(movie)}
			onapplied={onMatchApplied}
			onclose={() => (correcting = false)}
		/>
	{/if}
{/if}

<style>
	/* The play button keeps its weight; what sits beside it is deliberately
	   quieter. Wrapping rather than scrolling, because at 1280 on a television
	   these three do not fit on one line and a hidden action is a missing one. */
	.film-actions {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.75rem;
	}
</style>
