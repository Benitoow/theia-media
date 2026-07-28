<script>
	import { onMount, onDestroy, tick } from 'svelte';
	import { getJSON, formatTime, displayTitle } from '$lib/api.js';
	import { strings as t } from '$lib/strings.js';

	let { movie, onclose } = $props();

	/** @type {'checking' | 'resume' | 'playing' | 'preparing' | 'failed'} */
	let phase = $state('checking');
	let info = $state(null);
	let source = $state(null);
	let failure = $state(null);
	let triedRemux = $state(false);

	/** @type {HTMLVideoElement} */
	let video = $state();
	/** @type {HTMLElement} */
	let shell = $state();

	// A remuxed stream is a pipe: it always starts at zero as far as the video
	// element is concerned, whatever timestamp ffmpeg actually began at. This
	// offset is what turns the element's clock back into the film's clock.
	let offset = $state(0);
	let elapsed = $state(0);
	let duration = $state(0);
	let paused = $state(true);
	let muted = $state(false);
	let seeking = $state(false);

	const position = $derived(offset + elapsed);
	const isRemux = $derived(info?.mode === 'remux');
	const progressPercent = $derived(duration > 0 ? Math.min(100, (position / duration) * 100) : 0);

	let saveTimer;
	let lastSaved = 0;
	let returnFocus;
	let previousBodyOverflow;

	onMount(async () => {
		returnFocus = document.activeElement;
		previousBodyOverflow = document.body.style.overflow;
		document.body.style.overflow = 'hidden';

		try {
			info = await getJSON(`/api/stream/${movie.id}/info`);
		} catch {
			phase = 'failed';
			failure = t.player.unavailable;
			return;
		}

		duration = info.duration_seconds || 0;

		// A film finished last time starts over rather than resuming two minutes
		// from the credits.
		const saved = info.progress?.finished ? 0 : (info.progress?.position_seconds ?? 0);
		if (saved > 0) {
			offset = saved;
			phase = 'resume';
			return;
		}
		start(0);
	});

	onDestroy(() => {
		clearInterval(saveTimer);
		save(true);
		document.body.style.overflow = previousBodyOverflow;
		queueMicrotask(() => {
			if (returnFocus instanceof HTMLElement && returnFocus.isConnected) {
				returnFocus.focus();
			}
		});
	});

	function start(from) {
		offset = from;
		elapsed = 0;
		triedRemux = false;

		if (info.mode === 'direct') {
			source = `/api/stream/${movie.id}`;
			phase = 'playing';
			return;
		}
		startRemux(from);
	}

	function startRemux(from) {
		triedRemux = true;
		if (!info?.ffmpeg_supported) {
			phase = 'failed';
			failure = t.player.noFfmpeg;
			return;
		}
		// The offset belongs here rather than only in start(), because seeking
		// also comes through this function. Setting the source without moving
		// the offset leaves the displayed clock reading the old position while
		// ffmpeg streams from the new one -- and saves that wrong number.
		offset = from;
		elapsed = 0;

		phase = info?.ffmpeg_ready ? 'playing' : 'preparing';
		source = `/api/stream/${movie.id}/remux?t=${Math.floor(from)}`;
	}

	function onLoadedMetadata() {
		// Direct play knows its own length. A remux does not, so the value from
		// the server stands.
		if (!isRemux && Number.isFinite(video.duration) && video.duration > 0) {
			duration = video.duration;
			if (offset > 0) video.currentTime = offset;
		}
		seeking = false;
	}

	function onTimeUpdate() {
		elapsed = video.currentTime;
		if (!isRemux) {
			// currentTime already is the film's clock in direct mode.
			elapsed = 0;
			offset = video.currentTime;
		}
	}

	function seekTo(target) {
		const clamped = Math.max(0, Math.min(target, duration > 0 ? duration - 1 : target));
		if (isRemux) {
			// No byte ranges over a pipe, so seeking means asking ffmpeg to
			// start again somewhere else. This is what ?t= was built for.
			seeking = true;
			startRemux(clamped);
		} else {
			video.currentTime = clamped;
		}
		save(true, clamped);
	}

	function onScrub(event) {
		if (duration <= 0) return;
		const bar = event.currentTarget;
		const rect = bar.getBoundingClientRect();
		const ratio = (event.clientX - rect.left) / rect.width;
		seekTo(ratio * duration);
	}

	function onError() {
		// The container looked playable but the browser could not decode it.
		// The server cannot know this without running ffmpeg, so the fallback
		// lives here.
		if (!triedRemux) {
			startRemux(position);
			return;
		}
		phase = 'failed';
		failure = t.player.failed;
	}

	async function save(force = false, at = null) {
		const seconds = at ?? position;
		if (!Number.isFinite(seconds)) return;
		// Every few seconds is plenty; the row only needs to know roughly where
		// somebody stopped.
		if (!force && Math.abs(seconds - lastSaved) < 5) return;
		lastSaved = seconds;

		try {
			await fetch(`/api/library/movies/${movie.id}/progress`, {
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ position_seconds: seconds, duration_seconds: duration }),
				keepalive: true
			});
		} catch {
			// A lost position is not worth interrupting playback over.
		}
	}

	function togglePlay() {
		if (video.paused) video.play();
		else video.pause();
	}

	function toggleFullscreen() {
		if (document.fullscreenElement) document.exitFullscreen();
		else shell.requestFullscreen?.();
	}

	function onKeydown(event) {
		if (event.key === 'Escape' && !document.fullscreenElement) {
			event.preventDefault();
			onclose();
			return;
		}
		if (event.defaultPrevented || phase !== 'playing') return;

		const active = document.activeElement;
		const isDiscreteControl =
			active instanceof HTMLButtonElement ||
			active instanceof HTMLAnchorElement ||
			active instanceof HTMLInputElement ||
			active instanceof HTMLSelectElement ||
			active instanceof HTMLTextAreaElement;

		if ((event.key === ' ' || event.key === 'k') && !isDiscreteControl) {
			event.preventDefault();
			togglePlay();
		}
		if (isDiscreteControl) return;
		if (event.key === 'ArrowRight') {
			event.preventDefault();
			seekTo(position + 10);
		}
		if (event.key === 'ArrowLeft') {
			event.preventDefault();
			seekTo(position - 10);
		}
	}

	function trapDialogFocus(event) {
		if (event.key !== 'Tab' || !shell) return;

		const focusable = [...shell.querySelectorAll(
			'a[href], button:not(:disabled), input:not(:disabled), select:not(:disabled), ' +
			'textarea:not(:disabled), summary, video[controls], [role="slider"], ' +
			'[tabindex]:not([tabindex="-1"])'
		)].filter((element) => element instanceof HTMLElement && element.tabIndex >= 0);
		if (!focusable.length) return;

		const first = focusable[0];
		const last = focusable[focusable.length - 1];
		const active = document.activeElement;
		if (event.shiftKey && (active === first || !shell.contains(active))) {
			event.preventDefault();
			last.focus();
		} else if (!event.shiftKey && (active === last || !shell.contains(active))) {
			event.preventDefault();
			first.focus();
		}
	}

	$effect(() => {
		if (phase !== 'playing') return;
		saveTimer = setInterval(() => save(), 5000);
		return () => clearInterval(saveTimer);
	});

	$effect(() => {
		const currentPhase = phase;
		if (!shell) return;

		tick().then(() => {
			if (!shell?.isConnected || phase !== currentPhase) return;
			const preferred =
				shell.querySelector('[data-remote-default]') ??
				shell.querySelector('button:not(:disabled), [href], [role="slider"]');
			if (preferred instanceof HTMLElement) preferred.focus();
		});
	});
</script>

<svelte:window onkeydown={onKeydown} />

<div
	bind:this={shell}
	class="fixed inset-0 z-50 flex flex-col overflow-hidden bg-ink"
	role="dialog"
	aria-modal="true"
	aria-label={displayTitle(movie)}
	tabindex="-1"
	onkeydown={trapDialogFocus}
>
	<div
		class="flex min-h-20 items-center justify-between gap-6 border-b border-line
		       px-[var(--page-gutter)] py-3"
	>
		<span class="section-title truncate">{displayTitle(movie)}</span>
		<button
			type="button"
			onclick={() => { save(true); onclose(); }}
			class="tv-link label cursor-pointer border-0 bg-transparent px-4"
		>
			{t.player.close}
		</button>
	</div>

	<div
		class="flex min-h-0 flex-1 items-center justify-center overflow-y-auto
		       px-[var(--page-gutter)] py-6"
	>
		{#if phase === 'failed'}
			<p class="tv-copy max-w-prose border-l border-error py-2 pl-6">
				{failure}
			</p>
		{:else if phase === 'resume'}
			<!-- Resuming has to be the obvious choice without hiding the other one. -->
			<div class="text-center">
				<p class="label mb-7">{t.player.continueWatching}</p>
				<h2 class="mb-10 font-display text-display font-normal">{displayTitle(movie)}</h2>
				<div class="flex flex-wrap items-center justify-center gap-5">
					<button
						type="button"
						onclick={() => start(offset)}
						class="tv-action tv-action--primary cursor-pointer"
						data-remote-default
					>
						{t.player.resume} {formatTime(offset)}
					</button>
					<button
						type="button"
						onclick={async () => {
							await fetch(`/api/library/movies/${movie.id}/progress`, { method: 'DELETE' });
							start(0);
						}}
						class="tv-action cursor-pointer"
					>
						{t.player.fromStart}
					</button>
				</div>
			</div>
		{:else}
			<div class="w-full max-w-6xl">
				{#if phase === 'preparing'}
					<p class="tv-copy mb-5">{t.player.preparing}</p>
				{/if}

				{#if source}
					<!-- svelte-ignore a11y_media_has_caption -->
					<video
						bind:this={video}
						src={source}
						autoplay
						playsinline
						onerror={onError}
						onloadedmetadata={onLoadedMetadata}
						ontimeupdate={onTimeUpdate}
						onplay={() => { paused = false; phase = 'playing'; }}
						onpause={() => { paused = true; save(true); }}
						onended={() => save(true, duration || position)}
						onvolumechange={() => (muted = video.muted)}
						class="max-h-[min(70vh,calc(100svh-16rem))] w-full rounded-sm bg-black
						       shadow-[0_2rem_6rem_rgba(0,0,0,0.4)]"
					></video>
				{/if}

				<!--
					Custom controls rather than the browser's, for one reason that
					matters: a remuxed stream is a pipe with no length, so the
					native scrub bar has nothing to draw and seeking does nothing.
					This bar knows the duration from the server and seeks by
					restarting ffmpeg at a timestamp.
				-->
				<div class="mt-4">
					<div
						class="group -my-3 cursor-pointer rounded-sm py-3"
						role="slider"
						tabindex="0"
						aria-label="Position"
						aria-valuemin="0"
						aria-valuemax={Math.round(duration)}
						aria-valuenow={Math.round(position)}
						onclick={onScrub}
						onkeydown={(e) => {
							if (e.key === 'ArrowRight') {
								e.preventDefault();
								seekTo(position + 10);
							}
							if (e.key === 'ArrowLeft') {
								e.preventDefault();
								seekTo(position - 10);
							}
						}}
					>
						<div class="h-1 w-full bg-raised">
							<div
								class="ease-cine h-full bg-accent transition-[width] duration-160"
								style="width: {progressPercent}%"
							></div>
						</div>
					</div>

					<div class="mt-4 flex flex-wrap items-center gap-3 sm:gap-5">
						<button
							type="button"
							onclick={togglePlay}
							class="tv-link label cursor-pointer border-0 bg-transparent px-3"
							data-remote-default
						>
							{paused ? t.player.play : t.player.pause}
						</button>

						<span class="text-sm font-medium tabular-nums text-parchment sm:text-base">
							{formatTime(position)}{#if duration > 0} / {formatTime(duration)}{/if}
						</span>

						{#if seeking}
							<span class="label text-accent">{t.player.seeking}</span>
						{/if}

						<span class="flex-1"></span>

						<button
							type="button"
							onclick={() => (video.muted = !video.muted)}
							class="tv-link label cursor-pointer border-0 bg-transparent px-3"
						>
							{muted ? t.player.unmute : t.player.mute}
						</button>
						<button
							type="button"
							onclick={toggleFullscreen}
							class="tv-link label cursor-pointer border-0 bg-transparent px-3"
						>
							{t.player.fullscreen}
						</button>
					</div>

					{#if isRemux && duration <= 0}
						<p class="label mt-3">{t.player.remuxNotice}</p>
					{/if}
				</div>
			</div>
		{/if}
	</div>
</div>
