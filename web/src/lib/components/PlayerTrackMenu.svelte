<script>
	import { strings as t } from '$lib/strings.js';
	import {
		audioLabel as labelAudio,
		subtitleLabel as labelSubtitle
	} from '$lib/track-labels.js';
	import Icon from './Icon.svelte';

	let {
		open,
		panel = $bindable(),
		phase,
		audioTracks,
		audioTrackId,
		qualities,
		qualityHeight,
		transcodeKind,
		subtitleTracks,
		textSubtitles,
		refusedSubtitles,
		subtitleTrackId,
		subtitleOffset,
		onopen,
		onclose,
		onkeydown,
		onaudio,
		onquality,
		onsubtitle,
		onoffset,
		onoffsetreset
	} = $props();

	const audioLabel = (track, index) => labelAudio(track, index, t);
	const subtitleLabel = (track, index) => labelSubtitle(track, index, t);
</script>

<!--
	One row, defined once. Audio and subtitles differ in what they list, not in
	how a choice looks, and writing the markup twice is how the two drift apart.
-->
{#snippet trackOption(label, chosen, choose, first)}
	<button
		type="button"
		class="track-option"
		class:track-option--chosen={chosen}
		aria-pressed={chosen}
		onclick={choose}
		{...first ? { 'data-remote-default': '' } : {}}
	>
		<span class="track-lines">
			<span class="track-primary">{label.primary}</span>
			{#if label.detail}
				<span class="track-detail">{label.detail}</span>
			{/if}
		</span>
		<span class="track-tick" aria-hidden="true">
			{#if chosen}<Icon name="check" size={18} />{/if}
		</span>
	</button>
{/snippet}

<!--
	The panel lives inside the button's wrapper, so CSS anchors both without a
	viewport measurement. Focus ownership stays in Player: opening, dismissing
	and D-pad navigation are lifecycle decisions, not presentation decisions.
-->
<div class="player-tracks-anchor">
	<button
		type="button"
		onclick={(event) => (open ? onclose() : onopen(event.currentTarget))}
		class="player-icon-button"
		disabled={phase !== 'playing'}
		aria-expanded={open}
		aria-haspopup="true"
	>
		<Icon name="settings" label={t.player.tracks.open} />
	</button>

	{#if open}
		<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
		<section
			bind:this={panel}
			class="player-tracks"
			aria-label={t.player.tracks.title}
			onkeydown={onkeydown}
		>
			{#if audioTracks.length > 1}
				<h2 class="track-heading">{t.film.audio.title}</h2>
				<ul class="track-list">
					<li>
						{@render trackOption(
							{ primary: t.film.audio.auto, detail: '' },
							!audioTrackId,
							() => onaudio(null),
							true
						)}
					</li>
					{#each audioTracks as track, index (track.id)}
						<li>
							{@render trackOption(
								audioLabel(track, index),
								audioTrackId === track.id,
								() => onaudio(track.id),
								false
							)}
						</li>
					{/each}
				</ul>
			{/if}

			{#if qualities.length > 1}
				<h2 class="track-heading">
					{t.player.tracks.quality}
					{#if transcodeKind}
						<span class="track-heading-note">
							{t.player.tracks.kinds[transcodeKind] ?? ''}
						</span>
					{/if}
				</h2>
				<ul class="track-list">
					{#each qualities as quality (quality.height)}
						<li>
							{@render trackOption(
								{
									primary: quality.height
										? t.player.tracks.height(quality.height)
										: t.player.tracks.original,
									detail:
										quality.mode === 'transcode' && quality.height
											? t.player.tracks.reencoded
											: ''
								},
								qualityHeight === (quality.height || null),
								() => onquality(quality.height || null),
								audioTracks.length <= 1 && !subtitleTracks.length
							)}
						</li>
					{/each}
				</ul>
			{/if}

			{#if subtitleTracks.length}
				<h2 class="track-heading">{t.player.tracks.subtitles}</h2>
				<ul class="track-list">
					<li>
						{@render trackOption(
							{ primary: t.player.tracks.noSubtitles, detail: '' },
							!subtitleTrackId,
							() => onsubtitle(null),
							audioTracks.length <= 1
						)}
					</li>
					{#each textSubtitles as track, index (track.id)}
						<li>
							{@render trackOption(
								subtitleLabel(track, index),
								subtitleTrackId === track.id,
								() => onsubtitle(track.id),
								false
							)}
						</li>
					{/each}

					{#if subtitleTrackId}
						<li class="track-offset">
							<span class="track-offset-label">{t.player.tracks.offset}</span>
							<div class="track-offset-controls">
								<button
									type="button"
									class="track-offset-step"
									onclick={() => onoffset(-1)}
									aria-label={t.player.tracks.offsetEarlier}
								>
									−
								</button>
								<button
									type="button"
									class="track-offset-value"
									onclick={onoffsetreset}
									disabled={subtitleOffset === 0}
									aria-label={t.player.tracks.offsetReset}
								>
									{t.player.tracks.offsetValue(subtitleOffset)}
								</button>
								<button
									type="button"
									class="track-offset-step"
									onclick={() => onoffset(1)}
									aria-label={t.player.tracks.offsetLater}
								>
									+
								</button>
							</div>
						</li>
					{/if}

					{#each refusedSubtitles as track, index (track.id)}
						<li>
							<p class="track-option track-option--refused">
								<span class="track-primary">{subtitleLabel(track, index).primary}</span>
								<span class="track-detail">{t.player.tracks.imageBased}</span>
							</p>
						</li>
					{/each}
				</ul>
			{/if}
		</section>
	{/if}
</div>
