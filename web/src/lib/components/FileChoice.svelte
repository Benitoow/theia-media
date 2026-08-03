<script>
	// The file and audio-track chooser on a film's page.
	//
	// Two rules govern everything here and both come from M1-BE: the server never
	// picks a "best quality", and it never sends a characteristic it has not
	// measured. So this component only ever renders what `media.status === "ok"`
	// actually contains, and a file that has not been inspected says so instead of
	// guessing a resolution from its filename.
	import { apiFetch, formatTime } from '$lib/api.js';
	import { strings as t, formatSize } from '$lib/strings.js';
	import Icon from './Icon.svelte';

	// basePath is the resource these files hang off:
	// /api/library/movies/{id} or /api/library/episodes/{id}. Inspection sits at
	// the same place under both, so one component serves films and episodes.
	let { basePath, files = [], fileId, audioTrackId, onselect, onmeasure } = $props();

	/** file id -> 'idle' | 'running', so two files can be inspected independently. */
	let inspecting = $state({});
	/** file id -> stable server error code from a failed inspection. */
	let inspectError = $state({});

	const many = $derived(files.length > 1);
	const selected = $derived(files.find((file) => file.id === fileId) ?? files[0] ?? null);
	const tracks = $derived(
		selected?.media?.status === 'ok' ? (selected.media.audio_tracks ?? []) : []
	);
	// One track is not a choice. Offering it would only force a remux for nothing.
	const choosableTracks = $derived(tracks.length > 1 ? tracks : []);

	function selectFile(file) {
		if (file.id === fileId) return;
		// The audio track belongs to the file that was measured, so it cannot
		// survive a change of file: track ids are per-file in M1-BE.
		onselect?.({ fileId: file.id, audioTrackId: null });
	}

	function selectTrack(id) {
		onselect?.({ fileId: selected.id, audioTrackId: id });
	}

	async function inspect(file) {
		if (inspecting[file.id] === 'running') return;
		inspecting = { ...inspecting, [file.id]: 'running' };
		inspectError = { ...inspectError, [file.id]: null };

		try {
			const response = await apiFetch(`${basePath}/files/${file.id}/inspect`, {
				method: 'POST'
			});
			if (!response.ok) {
				// The body carries a code, never a sentence. Anything unparseable is
				// still a failure, so fall back to a code this catalogue knows.
				let code = 'media_unreadable';
				try {
					const body = await response.json();
					if (body?.error) code = body.error;
				} catch {
					// An empty or non-JSON body keeps the fallback above.
				}
				inspectError = { ...inspectError, [file.id]: code };

				// Only `media_unreadable` is cached against the file by the server.
				// A local ffmpeg preparation failure is deliberately not (decision
				// 38), so mirroring it here would invent a fault in the media.
				if (code === 'media_unreadable') {
					onmeasure?.({ ...file, media: { ...(file.media ?? {}), status: 'error', audio_tracks: [] } });
				}
				return;
			}
			onmeasure?.(await response.json());
		} catch {
			inspectError = { ...inspectError, [file.id]: 'ffmpeg_unavailable' };
		} finally {
			inspecting = { ...inspecting, [file.id]: 'idle' };
		}
	}

	/** Only measured facts, in a fixed order, joined by the usual middot. */
	function facts(file) {
		const parts = [file.extension?.toUpperCase(), formatSize(file.size_bytes)];
		const media = file.media ?? {};
		if (media.status !== 'ok') return parts.filter(Boolean);

		const video = media.video;
		if (video?.width > 0 && video?.height > 0) {
			parts.push(t.film.files.resolution(video.width, video.height));
		}
		if (video?.codec) parts.push(video.codec.toUpperCase());
		if (media.duration_seconds > 0) parts.push(formatTime(media.duration_seconds));
		if (media.audio_tracks?.length) {
			parts.push(t.film.files.trackCount(media.audio_tracks.length));
		}
		return parts.filter(Boolean);
	}

	/** @type {HTMLElement} */
	let root = $state();

	// Deterministic vertical movement inside the chooser, in the spirit of
	// decision 27: a row owns its own axis rather than leaving it to geometry.
	// The layout's spatial navigation ranks candidates by centre distance and
	// weights the horizontal axis 2.25x, which makes a list of wide rows lose to
	// any narrow link elsewhere on the page and skips the narrowest option in the
	// list. Inside this section the order is the reading order, always.
	//
	// An edge press is deliberately not consumed, so leaving the list at the top
	// or bottom still falls through to the page's own navigation.
	function onKeydown(event) {
		if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return;
		if (event.altKey || event.ctrlKey || event.metaKey || event.shiftKey) return;

		const options = [...(root?.querySelectorAll('.file-option') ?? [])];
		const index = options.indexOf(document.activeElement);
		if (index < 0) return;

		const next = index + (event.key === 'ArrowDown' ? 1 : -1);
		if (next < 0 || next >= options.length) return;

		event.preventDefault();
		options[next].focus({ preventScroll: true });
		options[next].scrollIntoView({
			block: 'nearest',
			behavior: window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth'
		});
	}

	/** A track's name is built only from the fields the probe actually returned. */
	function trackLabel(track, index) {
		const parts = [];
		if (track.language) parts.push(track.language.toUpperCase());
		if (track.title) parts.push(track.title);
		if (!parts.length) parts.push(t.film.audio.unnamed(index + 1));
		if (track.codec) parts.push(track.codec.toUpperCase());
		if (track.channels) parts.push(track.channels);
		if (track.is_default) parts.push(t.film.audio.isDefault);
		return parts.join(' · ');
	}
</script>

<!--
	The handler sits on the section because it coordinates its focusable
	children; the section itself is never focused and adds no stop. Silencing the
	rule rather than giving it a role: an interactive role here would announce a
	control that does not exist.
-->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<section class="file-choice" bind:this={root} onkeydown={onKeydown}>
	<h2 class="label">{many ? t.film.files.many : t.film.files.one}</h2>

	{#if many}
		<p class="file-choice-hint">{t.film.files.hint}</p>
	{/if}

	<!--
		Buttons rather than a roving-tabindex radiogroup, deliberately. The layout's
		D-pad navigation walks focusable elements geometrically and skips anything
		with tabindex="-1", so a proper radiogroup would leave every unselected file
		unreachable from a remote. aria-pressed carries the same state to assistive
		technology without costing a focus stop.
	-->
	<ul class="file-choice-list" aria-label={t.film.files.choose}>
		{#each files as file (file.id)}
			{@const isChosen = selected?.id === file.id}
			{@const status = file.media?.status ?? 'pending'}
			<li class="file-row">
				<button
					type="button"
					class="file-option"
					class:file-option--chosen={isChosen}
					aria-pressed={many ? isChosen : undefined}
					onclick={() => selectFile(file)}
				>
					<span class="file-option-head">
						<span class="file-option-name">{file.file_name}</span>
						{#if isChosen && many}
							<span class="file-option-flag">{t.film.files.chosen}</span>
						{:else if file.is_primary && many}
							<span class="file-option-flag file-option-flag--quiet">
								{t.film.files.primary}
							</span>
						{/if}
					</span>

					<!-- The separator is glued to the fact that follows it, inside one
					     inline box. As separate children the middot could end a wrapped
					     line, leaving "2:30 ·" hanging at the right edge. -->
					<span class="file-option-facts">
						{#each facts(file) as fact, index (fact + index)}
							<span class="file-option-fact">
								{#if index > 0}<span class="file-option-sep" aria-hidden="true">·&nbsp;</span>{/if}{fact}
							</span>
						{/each}
					</span>

					{#if status !== 'ok'}
						<span class="file-option-state">
							{status === 'error' ? t.film.files.errored : t.film.files.pending}
						</span>
					{/if}
				</button>

				<!--
					The action sits beside its own file rather than under it. The
					layout's D-pad ranks candidates by distance and weights horizontal
					offset 2.25x, so a narrow button placed between two full-width rows
					wins every downward press and the file options themselves become
					unreachable by remote. Measured on this page: from "Lire", the first
					option scored 1705 against the inspect button's 1095. Side by side,
					vertical presses move between files and a horizontal one reaches the
					action -- which is also the better mental model on a remote.
				-->
				{#if status !== 'ok'}
					<button
						type="button"
						class="tv-action file-option-inspect cursor-pointer"
						onclick={() => inspect(file)}
						aria-busy={inspecting[file.id] === 'running'}
					>
						{#if inspecting[file.id] === 'running'}
							<span class="player-spinner file-option-spinner" aria-hidden="true"></span>
							<span>{t.film.files.inspecting}</span>
						{:else}
							<Icon name="search" size={16} />
							<span>{status === 'error' ? t.film.files.retry : t.film.files.inspect}</span>
						{/if}
					</button>
				{/if}

				{#if inspectError[file.id]}
					<p class="file-option-error" role="status">
						{t.player.codes[inspectError[file.id]] ?? t.player.failed}
					</p>
				{/if}
			</li>
		{/each}
	</ul>

	{#if choosableTracks.length}
		<div class="file-choice-audio">
			<h3 class="label">{t.film.audio.title}</h3>
			<ul class="file-choice-list" aria-label={t.film.audio.choose}>
				<li class="file-row">
					<button
						type="button"
						class="file-option file-option--compact"
						class:file-option--chosen={!audioTrackId}
						aria-pressed={!audioTrackId}
						onclick={() => selectTrack(null)}
					>
						<span class="file-option-name">{t.film.audio.auto}</span>
					</button>
				</li>
				{#each choosableTracks as track, index (track.id)}
					<li class="file-row">
						<button
							type="button"
							class="file-option file-option--compact"
							class:file-option--chosen={audioTrackId === track.id}
							aria-pressed={audioTrackId === track.id}
							onclick={() => selectTrack(track.id)}
						>
							<span class="file-option-name">{trackLabel(track, index)}</span>
						</button>
					</li>
				{/each}
			</ul>

			{#if audioTrackId}
				<p class="file-choice-note">{t.film.audio.remuxNote}</p>
			{/if}
		</div>
	{/if}
</section>
