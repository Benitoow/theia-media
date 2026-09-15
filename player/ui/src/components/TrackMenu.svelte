<script>
	// The audio and subtitle menu.
	//
	// Design system section 6b: a popover, not a panel - it is a child of the
	// button that opens it, so it is anchored by construction rather than by a
	// measured offset. A track is two lines, because the language is what the
	// choice is made on and the codec sits under it as provenance. The chosen
	// track carries a tick in a column reserved whether or not it is drawn.
	//
	// The shape of a track here is mpv's own, passed through untouched by the
	// Rust side: id, type, title, lang, codec, demux-channels, external,
	// selected. Nothing is renamed on the way, so a field mpv adds tomorrow is
	// available without a change here.

	import Icon from '../../../../web/src/lib/components/Icon.svelte';

	/** @type {{tracks: Array<Record<string, unknown>>, onpick: (kind: string, id: number) => void, t: (key: string) => string}} */
	let { tracks = [], onpick, t } = $props();

	const audio = $derived(tracks.filter((track) => track.type === 'audio'));
	const subtitles = $derived(tracks.filter((track) => track.type === 'sub'));
	const subtitlesOff = $derived(subtitles.every((track) => !track.selected));

	/** Language first: that is what the choice is made on. */
	function primary(track, index) {
		if (track.title) return track.title;
		if (track.lang) return String(track.lang).toUpperCase();
		return `${t('trackNumber')} ${index + 1}`;
	}

	/** Codec, channels and provenance. A detail that repeats the line above is
	    dropped, which is why the title is not in here. */
	function detail(track) {
		const channels = track['demux-channels'];
		const parts = [
			track.codec ? String(track.codec).toUpperCase() : null,
			channels && channels !== 'stereo' ? channels : null,
			track.external ? t('externalTrack') : null,
		];
		return parts.filter(Boolean).join(' · ');
	}
</script>

<div class="track-menu">
	<p class="label">{t('audioTracks')}</p>
	{#each audio as track, index (track.id)}
		<button
			class="track"
			role="menuitemradio"
			aria-checked={track.selected ? 'true' : 'false'}
			onclick={() => onpick('audio', track.id)}
		>
			<span class="tick">{#if track.selected}<Icon name="check" size={16} />{/if}</span>
			<span class="track-text">
				<span class="track-primary">{primary(track, index)}</span>
				<span class="label">{detail(track)}</span>
			</span>
		</button>
	{/each}

	<p class="label">{t('subtitleTracks')}</p>
	<button
		class="track"
		role="menuitemradio"
		aria-checked={subtitlesOff ? 'true' : 'false'}
		onclick={() => onpick('subtitle', 0)}
	>
		<span class="tick">{#if subtitlesOff}<Icon name="check" size={16} />{/if}</span>
		<span class="track-text"><span class="track-primary">{t('subtitlesOff')}</span></span>
	</button>
	{#each subtitles as track, index (track.id)}
		<button
			class="track"
			role="menuitemradio"
			aria-checked={track.selected ? 'true' : 'false'}
			onclick={() => onpick('subtitle', track.id)}
		>
			<span class="tick">{#if track.selected}<Icon name="check" size={16} />{/if}</span>
			<span class="track-text">
				<span class="track-primary">{primary(track, index)}</span>
				<span class="label">{detail(track)}</span>
			</span>
		</button>
	{/each}
</div>
