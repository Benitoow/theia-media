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

	/** Language first: that is what the choice is made on, and it is the rule the
	    web player already applies (web/src/lib/track-labels.js). A title only
	    leads when there is no language to lead with - which is also what keeps a
	    sidecar from reading "fra" beside an embedded track's "FRA". */
	function primary(track, index) {
		if (track.lang) return String(track.lang).toUpperCase();
		if (track.title) return track.title;
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

	// The menu owns its own axis.
	//
	// Design system 9: "a vertical list of options owns its own axis, in reading
	// order, and does not consume the press at either edge". Until 15 September
	// 2026 this list had no keyboard handling at all, so the arrows fell through
	// to the player and scrubbed the film while a viewer was reading the tracks -
	// measured, then asserted in the render check before it was fixed here.
	//
	// The first press enters the list rather than moving inside it: the menu is
	// opened by a button that keeps focus, so a viewer pressing Down expects to
	// arrive on the first row, not to jump to the second.
	let rows = $state([]);

	export function moveFocus(direction) {
		const items = rows.filter(Boolean);
		if (!items.length) return false;
		const current = items.indexOf(document.activeElement);
		const next =
			direction === 'first'
				? 0
				: current === -1
					? direction > 0
						? 0
						: items.length - 1
					: Math.min(items.length - 1, Math.max(0, current + direction));
		items[next].focus();
		return true;
	}
</script>

<div class="track-menu">
	<p class="label">{t('audioTracks')}</p>
	{#each audio as track, index (track.id)}
		<button
			class="track"
			bind:this={rows[index]}
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
		bind:this={rows[audio.length]}
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
			bind:this={rows[audio.length + 1 + index]}
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
