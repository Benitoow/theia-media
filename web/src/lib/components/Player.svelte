<script>
	import { onMount } from 'svelte';
	import { getJSON } from '$lib/api.js';
	import { strings as t } from '$lib/strings.js';

	let { movie, onclose } = $props();

	/** @type {'checking' | 'playing' | 'preparing' | 'failed'} */
	let state = $state('checking');
	let info = $state(null);
	let source = $state(null);
	let failure = $state(null);

	// Direct play is tried first for browser-friendly containers, and the
	// fallback below catches the case the server cannot know about without
	// running ffmpeg: an MP4 hiding a codec the browser will not decode.
	let triedRemux = $state(false);

	onMount(async () => {
		try {
			info = await getJSON(`/api/stream/${movie.id}/info`);
		} catch {
			state = 'failed';
			failure = t.player.unavailable;
			return;
		}

		if (info.mode === 'direct') {
			source = `/api/stream/${movie.id}`;
			state = 'playing';
			return;
		}
		startRemux();
	});

	function startRemux() {
		triedRemux = true;
		if (!info?.ffmpeg_supported) {
			state = 'failed';
			failure = t.player.noFfmpeg;
			return;
		}
		// The first remux on a fresh install pauses to fetch ~80 MB. Saying so
		// beats a spinner that looks like a hang.
		state = info?.ffmpeg_ready ? 'playing' : 'preparing';
		source = `/api/stream/${movie.id}/remux`;
	}

	function onError() {
		if (!triedRemux) {
			// The container looked fine but the browser could not decode it.
			startRemux();
			return;
		}
		state = 'failed';
		failure = t.player.failed;
	}
</script>

<div
	class="fixed inset-0 z-50 flex flex-col bg-ink"
	role="dialog"
	aria-modal="true"
	aria-label={movie.title}
>
	<div class="flex items-center justify-between px-6 py-4 lg:px-10">
		<span class="label truncate">{movie.metadata?.tmdb_title || movie.title}</span>
		<button
			type="button"
			onclick={onclose}
			class="ease-cine cursor-pointer text-label uppercase tracking-[0.18em] text-muted
			       transition-colors duration-160 hover:text-bone"
		>
			{t.player.close}
		</button>
	</div>

	<div class="flex flex-1 items-center justify-center px-6 pb-10 lg:px-10">
		{#if state === 'failed'}
			<p class="max-w-prose border-l border-error py-1 pl-5 text-small text-parchment">
				{failure}
			</p>
		{:else}
			<div class="w-full max-w-6xl">
				{#if state === 'preparing'}
					<p class="label mb-4">{t.player.preparing}</p>
				{/if}

				{#if source}
					<!-- svelte-ignore a11y_media_has_caption -->
					<video
						src={source}
						controls
						autoplay
						playsinline
						onerror={onError}
						onplaying={() => (state = 'playing')}
						class="max-h-[80vh] w-full bg-black"
					></video>
				{/if}

				{#if info && info.mode === 'remux' && state === 'playing'}
					<!-- Honest about the limitation rather than letting a dead seek
					     bar look like a bug. -->
					<p class="label mt-4">{t.player.remuxNotice}</p>
				{/if}
			</div>
		{/if}
	</div>
</div>
