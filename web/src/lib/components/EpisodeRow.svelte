<script>
	// One playable item in a season list.
	//
	// An item is not always one episode: a file holding S01E01E02 is a single
	// timeline with one resume position, so it appears once and says so. Showing
	// it as two cards would launch the same file twice from second zero and call
	// the lie a feature (decision 39).
	import { imageURL, formatTime } from '$lib/api.js';
	import { strings as t } from '$lib/strings.js';

	let { episode } = $props();

	const numbers = $derived(episode.episode_numbers ?? []);
	const combined = $derived(numbers.length > 1);
	const members = $derived(episode.episode_metadata ?? []);

	const numberLabel = $derived(
		combined
			? t.series.episodeRange(numbers[0], numbers[numbers.length - 1])
			: t.series.episodeLabel(numbers[0] ?? '?')
	);

	// Only names TMDB actually returned. A member it did not recognise
	// contributes nothing rather than a placeholder.
	const title = $derived(
		members
			.map((member) => member.metadata?.name)
			.filter(Boolean)
			.join(' · ')
	);

	const still = $derived(imageURL(members[0]?.metadata?.still_path, 'w342'));

	const watched = $derived(
		episode.progress?.position_seconds > 0 &&
			!episode.progress?.finished &&
			episode.progress?.duration_seconds > 0
	);
	const percent = $derived(
		watched
			? Math.min(
					100,
					(episode.progress.position_seconds / episode.progress.duration_seconds) * 100
				)
			: 0
	);
	const runtime = $derived(
		members.reduce((total, member) => total + (member.metadata?.runtime_minutes ?? 0), 0)
	);
</script>

<a href="/episode/{episode.id}" class="episode-row">
	<span class="episode-still">
		{#if still}
			<img src={still} alt="" loading="lazy" decoding="async" />
		{/if}
		{#if watched}
			<!-- The same exemption §6.2 grants a film card: a progress rule is
			     information, and it is the reason the row exists. -->
			<span class="episode-progress"><span style="width: {percent}%"></span></span>
		{/if}
	</span>

	<span class="episode-body">
		<span class="episode-head">
			<span class="label">{numberLabel}</span>
			{#if combined}
				<span class="episode-flag">{t.series.combined}</span>
			{/if}
			{#if episode.finished}
				<span class="episode-flag episode-flag--quiet">{t.library.finishedBadge}</span>
			{/if}
		</span>

		{#if title}
			<span class="episode-title">{title}</span>
		{/if}

		<span class="episode-facts">
			{#if runtime > 0}<span>{formatTime(runtime * 60)}</span>{/if}
			{#if episode.progress?.position_seconds > 0 && !episode.progress?.finished}
				<span class="episode-resume">
					{t.series.resumeAtMinutes(Math.max(1, Math.floor(episode.progress.position_seconds / 60)))}
				</span>
			{/if}
		</span>
	</span>
</a>
