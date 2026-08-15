<script>
	// One playable episode item: what it is, which file plays it, and what comes
	// next. The file chooser and the player are the same components the film page
	// uses -- an episode is a different resource, not a different interaction.
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { getJSON, imageURL, formatTime } from '$lib/api.js';
	import { profiles } from '$lib/profiles.svelte.js';
	import { strings as t } from '$lib/strings.js';
	import Icon from '$lib/components/Icon.svelte';
	import Player from '$lib/components/Player.svelte';
	import FileChoice from '$lib/components/FileChoice.svelte';
	import LoadingSkeleton from '$lib/components/LoadingSkeleton.svelte';

	/** @type {'loading' | 'ready' | 'missing'} */
	let state = $state('loading');
	let episode = $state(null);
	let playing = $state(false);
	let fileId = $state(null);

	const basePath = $derived(`/api/library/episodes/${episode?.id}`);
	const files = $derived(episode?.files ?? []);
	const selectedFile = $derived(files.find((file) => file.id === fileId) ?? null);
	const members = $derived(episode?.episode_metadata ?? []);
	const numbers = $derived(episode?.episode_numbers ?? []);
	const combined = $derived(numbers.length > 1);

	const numberLabel = $derived(
		combined
			? t.series.episodeRange(numbers[0], numbers[numbers.length - 1])
			: t.series.episodeLabel(numbers[0] ?? '?')
	);
	const title = $derived(
		members
			.map((member) => member.metadata?.name)
			.filter(Boolean)
			.join(' · ')
	);
	const still = $derived(imageURL(members[0]?.metadata?.still_path, 'w780'));
	const resumable = $derived(
		episode?.progress?.position_seconds > 0 && !episode?.progress?.finished
	);
	const resumeMinutes = $derived(
		resumable ? Math.max(1, Math.floor(episode.progress.position_seconds / 60)) : 0
	);
	// The heading the player announces: the series, then which episode.
	const playerTitle = $derived(
		`${episode?.series_title ?? ''} — ${numberLabel}${title ? ` · ${title}` : ''}`
	);

	async function load(id) {
		state = 'loading';
		try {
			await profiles.ready();
			episode = await getJSON(profiles.url(`/api/library/episodes/${id}`));
			fileId =
				(episode.files?.find((file) => file.is_primary) ?? episode.files?.[0])?.id ?? null;
			state = 'ready';
		} catch {
			state = 'missing';
		}
	}

	onMount(() => load($page.params.id));

	function syncProgress(progress) {
		if (!episode || !progress) return;
		episode = { ...episode, progress };
	}

	function onFileChoice({ fileId: nextFile }) {
		fileId = nextFile ?? fileId;
	}

	const watched = $derived(!!episode?.progress?.finished);
	let watchedBusy = $state(false);
	let watchedFailed = $state(false);

	// The common case for a series: watched up to episode six somewhere else,
	// and nothing on this server knows it. Same contract as the film page.
	async function toggleWatched() {
		if (!episode) return;
		watchedBusy = true;
		watchedFailed = false;
		try {
			const path = profiles.url(`/api/library/episodes/${episode.id}/watched`);
			if (watched) {
				const res = await fetch(path, { method: 'DELETE' });
				if (!res.ok) throw new Error(String(res.status));
				episode = { ...episode, progress: { position_seconds: 0, finished: false } };
			} else {
				syncProgress(await getJSON(path, { method: 'PUT' }));
			}
		} catch {
			watchedFailed = true;
		} finally {
			watchedBusy = false;
		}
	}

	function onFileMeasured(measured) {
		if (!episode || !measured?.id) return;
		episode = {
			...episode,
			files: episode.files.map((file) => (file.id === measured.id ? measured : file))
		};
	}
</script>

<svelte:head>
	<title>{episode ? playerTitle : t.series.title}</title>
</svelte:head>

{#if state === 'loading'}
	<LoadingSkeleton variant="detail" label={t.series.loading} />
{:else if state === 'missing'}
	<div class="page-shell flex min-h-screen items-center justify-center py-32">
		<div class="chrome-panel max-w-xl p-8 text-center sm:p-12">
			<h1 class="font-display text-display font-normal">{t.series.episodeNotFound}</h1>
			<a href="/series" class="tv-link label mt-7 justify-center">← {t.nav.back}</a>
		</div>
	</div>
{:else}
	<article class="page-shell page-body">
		<a href="/serie/{episode.series_id}" class="tv-link label mb-6">← {episode.series_title}</a>

		<div class="episode-header">
			{#if still}
				<img src={still} alt="" class="episode-header-still" decoding="async" />
			{/if}

			<div class="min-w-0 flex-1">
				<span class="label">
					{episode.season_number === 0 ? t.series.specials : t.series.season(episode.season_number)}
					· {numberLabel}
				</span>

				<h1 class="page-title mt-3 mb-5">{title || numberLabel}</h1>

				{#if combined}
					<p class="episode-note">
						<Icon name="info" size={16} />
						<span>{t.series.combinedHint}</span>
					</p>
				{/if}

				<div class="episode-actions mt-2 mb-8">
					<button
						type="button"
						onclick={() => (playing = true)}
						class="tv-action tv-action--primary cursor-pointer"
						data-remote-default
					>
						<Icon name="play" size={18} />
						<span>{resumable ? t.series.resumeAtMinutes(resumeMinutes) : t.series.play}</span>
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
				</div>

				{#if watchedFailed}
					<p class="mb-6 text-small text-error" role="alert">{t.watched.failed}</p>
				{/if}
			</div>
		</div>

		<FileChoice
			{basePath}
			{files}
			{fileId}
			onselect={onFileChoice}
			onmeasure={onFileMeasured}
		/>

		{#each members as member (member.id)}
			{#if member.metadata?.overview}
				<section class="mt-10">
					{#if combined}
						<h2 class="label mb-3">{t.series.episodeLabel(member.episode_number)}</h2>
					{/if}
					<p class="tv-copy max-w-[46rem]">{member.metadata.overview}</p>
				</section>
			{/if}
		{/each}

		<!-- The next item owned locally. A missing number is stated and does not
		     block anything (decision 41); specials never lead anywhere. -->
		<section class="episode-next">
			{#if episode.next_episode_id}
				<a href="/episode/{episode.next_episode_id}" class="tv-action cursor-pointer">
					<span>{t.series.next}</span>
					<Icon name="chevronRight" size={16} />
				</a>
				{#if episode.next_has_gap}
					<p class="episode-gap">{t.series.gap}</p>
				{/if}
			{:else}
				<p class="label">{t.series.lastOwned}</p>
			{/if}
		</section>
	</article>

	{#if playing}
		<Player
			movie={episode}
			title={playerTitle}
			fileId={selectedFile?.id ?? null}
			streamBase={selectedFile
				? `/api/library/episodes/${episode.id}/files/${selectedFile.id}/stream`
				: null}
			subtitleBase={selectedFile
				? `/api/library/episodes/${episode.id}/files/${selectedFile.id}`
				: null}
			progressPath={`/api/library/episodes/${episode.id}/progress`}
			onprogress={syncProgress}
			onclose={() => (playing = false)}
		/>
	{/if}
{/if}

<style>
	.episode-actions {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.75rem;
	}
</style>
