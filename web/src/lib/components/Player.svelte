<script>
	import { onMount, onDestroy, tick } from 'svelte';
	import { getJSON, formatTime, displayTitle } from '$lib/api.js';
	import { strings as t } from '$lib/strings.js';
	import Icon from './Icon.svelte';

	let { movie, onclose, onprogress } = $props();

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
	/** @type {HTMLElement} */
	let scrubTrack = $state();

	// A remuxed stream is a pipe: it always starts at zero as far as the video
	// element is concerned, whatever timestamp ffmpeg actually began at. This
	// offset is what turns the element's clock back into the film's clock.
	let offset = $state(0);
	let elapsed = $state(0);
	let duration = $state(0);
	let paused = $state(true);
	let muted = $state(false);
	let volume = $state(1);
	let seeking = $state(false);
	let buffered = $state(0);
	let waiting = $state(false);
	let helpOpen = $state(false);
	/** @type {HTMLElement} */
	let helpPanel = $state();

	// While a scrub is in progress the bar follows the pointer rather than the
	// video, otherwise the handle fights the person dragging it.
	let scrubbing = $state(false);
	let scrubValue = $state(0);
	let hoverRatio = $state(null);

	// Controls fade out during playback and come back on any sign of life. A
	// player that keeps its furniture on screen over a film is a preview window,
	// not a player.
	let controlsVisible = $state(true);
	let idleTimer;

	const position = $derived(scrubbing ? scrubValue : offset + elapsed);
	const hidden = $derived(!controlsVisible && phase === 'playing');
	const isRemux = $derived(info?.mode === 'remux');
	const progressPercent = $derived(duration > 0 ? Math.min(100, (position / duration) * 100) : 0);
	const bufferedPercent = $derived(duration > 0 ? Math.min(100, (buffered / duration) * 100) : 0);
	const remaining = $derived(duration > 0 ? Math.max(0, duration - position) : 0);

	let saveTimer;
	let lastSaved = 0;
	let returnFocus;
	let helpReturnFocus;
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
		clearTimeout(idleTimer);
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
		readBuffered();
	}

	// How much is downloaded ahead, so the bar can show what is safe to seek
	// into. On a remuxed pipe the ranges are relative to the stream, hence the
	// offset.
	function readBuffered() {
		if (!video || video.buffered.length === 0) return;
		const end = video.buffered.end(video.buffered.length - 1);
		buffered = isRemux ? offset + end : end;
	}

	function seekTo(target) {
		const clamped = Math.max(0, Math.min(target, duration > 0 ? duration - 1 : target));
		if (isRemux) {
			// No byte ranges over a pipe, so seeking means asking ffmpeg to
			// start again somewhere else. This is what ?t= was built for.
			seeking = true;
			buffered = clamped;
			startRemux(clamped);
		} else {
			video.currentTime = clamped;
		}
		save(true, clamped);
		showControls();
	}

	function ratioFromPointer(event) {
		const rect = scrubTrack.getBoundingClientRect();
		return Math.max(0, Math.min(1, (event.clientX - rect.left) / rect.width));
	}

	function onScrubDown(event) {
		if (duration <= 0) return;
		scrubbing = true;
		scrubValue = ratioFromPointer(event) * duration;
		scrubTrack.setPointerCapture?.(event.pointerId);
	}

	function onScrubMove(event) {
		if (duration <= 0) return;
		hoverRatio = ratioFromPointer(event);
		if (scrubbing) scrubValue = hoverRatio * duration;
	}

	function onScrubUp(event) {
		if (!scrubbing) return;
		scrubbing = false;
		scrubTrack.releasePointerCapture?.(event.pointerId);
		seekTo(scrubValue);
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
			const response = await fetch(`/api/library/movies/${movie.id}/progress`, {
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ position_seconds: seconds, duration_seconds: duration }),
				keepalive: true
			});
			if (!response.ok) return;
			const saved = await response.json();
			onprogress?.(saved);
		} catch {
			// A lost position is not worth interrupting playback over.
		}
	}

	function togglePlay() {
		if (!video) return;
		if (video.paused) video.play();
		else video.pause();
		showControls();
	}

	function toggleFullscreen() {
		if (document.fullscreenElement) document.exitFullscreen();
		else shell.requestFullscreen?.();
	}

	function setVolume(value) {
		volume = value;
		video.volume = value;
		// Moving the slider off zero is an unmute; leaving it at zero is a mute.
		video.muted = value === 0;
	}

	function toggleMute() {
		video.muted = !video.muted;
		if (!video.muted && video.volume === 0) setVolume(0.5);
		showControls();
	}

	function isHelpKey(event) {
		return (
			!event.altKey &&
			!event.ctrlKey &&
			!event.metaKey &&
			(event.key === '?' || (event.code === 'Slash' && event.shiftKey))
		);
	}

	function openHelp(trigger = document.activeElement) {
		helpReturnFocus = trigger instanceof HTMLElement ? trigger : shell;
		helpOpen = true;
		controlsVisible = true;
		clearTimeout(idleTimer);

		tick().then(() => {
			const close = helpPanel?.querySelector('[data-shortcuts-close]');
			if (close instanceof HTMLElement) close.focus();
		});
	}

	function closeHelp() {
		helpOpen = false;
		showControls();

		tick().then(() => {
			if (helpReturnFocus instanceof HTMLElement && helpReturnFocus.isConnected) {
				helpReturnFocus.focus();
			} else {
				shell?.focus();
			}
		});
	}

	function scrollHelp(event) {
		if (!helpPanel) return false;
		const amount = Math.max(160, helpPanel.clientHeight * 0.72);
		const behavior = window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth';

		switch (event.key) {
			case 'ArrowDown':
				helpPanel.scrollBy({ top: 72, behavior });
				return true;
			case 'ArrowUp':
				helpPanel.scrollBy({ top: -72, behavior });
				return true;
			case 'PageDown':
				helpPanel.scrollBy({ top: amount, behavior });
				return true;
			case 'PageUp':
				helpPanel.scrollBy({ top: -amount, behavior });
				return true;
			case 'Home':
				helpPanel.scrollTo({ top: 0, behavior });
				return true;
			case 'End':
				helpPanel.scrollTo({ top: helpPanel.scrollHeight, behavior });
				return true;
			default:
				return false;
		}
	}

	function showControls() {
		controlsVisible = true;
		clearTimeout(idleTimer);
		// Paused, seeking or buffering means the person is looking at the
		// controls, so they stay.
		if (paused || seeking || waiting || helpOpen) return;

		idleTimer = setTimeout(() => {
			// Focus cannot be left sitting on a control that is about to become
			// invisible -- a focus ring floating over a film, or worse, an
			// invisible ring nobody can find. Move it to the dialog itself,
			// which is where the keyboard shortcuts are handled anyway.
			//
			// An earlier version refused to hide at all while focus was inside
			// the bar. That read as careful and was not: the play button is
			// focused automatically when the player opens, so the controls
			// never hid even once.
			const controls = shell?.querySelector('.player-controls');
			if (controls?.contains(document.activeElement)) shell.focus();
			controlsVisible = false;
		}, 3000);
	}

	function onKeydown(event) {
		// Any key is a sign of life. Somebody navigating the bar with a remote
		// must never have it disappear mid-press.
		showControls();

		if (isHelpKey(event) && (helpOpen || phase === 'playing')) {
			event.preventDefault();
			if (helpOpen) closeHelp();
			else openHelp();
			return;
		}

		if (helpOpen) {
			if (event.key === 'Escape') {
				event.preventDefault();
				closeHelp();
			} else if (scrollHelp(event)) {
				event.preventDefault();
			} else if (
				(event.key === ' ' || event.key === 'Enter') &&
				document.activeElement instanceof HTMLButtonElement &&
				helpPanel?.contains(document.activeElement)
			) {
				// Let the panel's own close button keep its native activation.
			} else if (event.key !== 'Tab') {
				event.preventDefault();
			}
			return;
		}

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
			return;
		}
		if (event.key === 'f') {
			event.preventDefault();
			toggleFullscreen();
			return;
		}
		if (event.key === 'm') {
			event.preventDefault();
			toggleMute();
			return;
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
		if (event.key === 'ArrowUp' || event.key === 'ArrowDown') {
			// Nothing above or below the video worth focusing, so the vertical
			// axis is volume -- which is what it does on a TV remote anyway.
			event.preventDefault();
			setVolume(Math.max(0, Math.min(1, volume + (event.key === 'ArrowUp' ? 0.1 : -0.1))));
			showControls();
		}
		if (event.key === 'Home') {
			event.preventDefault();
			seekTo(0);
		}
		if (event.key === 'End') {
			event.preventDefault();
			seekTo(duration - 5);
		}
	}

	function trapDialogFocus(event) {
		if (event.key !== 'Tab' || !shell) return;

		const scope = helpOpen ? helpPanel : shell;
		if (!scope) return;

		const focusable = [...scope.querySelectorAll(
			'a[href], button:not(:disabled), input:not(:disabled), select:not(:disabled), ' +
			'textarea:not(:disabled), summary, [role="slider"], [tabindex]:not([tabindex="-1"])'
		)].filter((element) => element instanceof HTMLElement && element.tabIndex >= 0
			&& element.offsetParent !== null);
		if (!focusable.length) return;

		// Tab is also a sign of life: hidden controls must reappear before the
		// focus ring lands on something invisible.
		showControls();

		const first = focusable[0];
		const last = focusable[focusable.length - 1];
		const active = document.activeElement;
		if (event.shiftKey && (active === first || !scope.contains(active))) {
			event.preventDefault();
			last.focus();
		} else if (!event.shiftKey && (active === last || !scope.contains(active))) {
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
		if (phase !== 'playing' && helpOpen) {
			// Preparation and error states replace the controls entirely. Do
			// not leave a hidden help panel consuming Escape behind them.
			helpOpen = false;
			helpReturnFocus = null;
		}
	});

	$effect(() => {
		const currentPhase = phase;
		if (!shell) return;

		tick().then(() => {
			if (!shell?.isConnected || phase !== currentPhase) return;
			const preferred =
				shell.querySelector('[data-remote-default]') ??
				shell.querySelector('button:not(:disabled), [href]');
			if (preferred instanceof HTMLElement) preferred.focus();
		});
	});
</script>

<svelte:window onkeydown={onKeydown} />

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<div
	bind:this={shell}
	class="player"
	class:player--idle={hidden}
	role="dialog"
	aria-modal="true"
	aria-label={displayTitle(movie)}
	tabindex="-1"
	onkeydown={trapDialogFocus}
	onpointermove={showControls}
	onpointerdown={showControls}
>
	{#if phase === 'failed'}
		<div class="player-message">
			<p class="tv-copy max-w-prose border-l border-error py-2 pl-6">{failure}</p>
			<button type="button" onclick={onclose} class="tv-action mt-8 cursor-pointer" data-remote-default>
				{t.player.close}
			</button>
		</div>
	{:else if phase === 'resume'}
		<!-- Resuming has to be the obvious choice without hiding the other one. -->
		<div class="player-message">
			<span class="label">{t.player.continueWatching}</span>
			<h2 class="hero-title mt-4 mb-3 !text-[clamp(2.25rem,5vw,4rem)]">{displayTitle(movie)}</h2>
			<p class="tv-copy mb-10">{t.player.resumeAt} {formatTime(offset)}</p>
			<div class="flex flex-wrap items-center justify-center gap-5">
				<button
					type="button"
					onclick={() => start(offset)}
					class="tv-action tv-action--primary cursor-pointer"
					data-remote-default
				>
					<Icon name="play" size={18} />
					<span>{t.player.resume}</span>
				</button>
				<button
					type="button"
					onclick={async () => {
						await fetch(`/api/library/movies/${movie.id}/progress`, { method: 'DELETE' });
						start(0);
					}}
					class="tv-action cursor-pointer"
				>
					<Icon name="back10" size={18} />
					<span>{t.player.fromStart}</span>
				</button>
			</div>
		</div>
	{:else}
		<!-- The picture fills the frame. Everything else floats over it and gets
		     out of the way, which is the whole difference between a player and a
		     video in a box. -->
		{#if source}
			<!-- svelte-ignore a11y_media_has_caption -->
			<video
				bind:this={video}
				inert={helpOpen}
				src={source}
				autoplay
				playsinline
				class="player-video"
				onerror={onError}
				onloadedmetadata={onLoadedMetadata}
				ontimeupdate={onTimeUpdate}
				onprogress={readBuffered}
				onwaiting={() => (waiting = true)}
				onplaying={() => { waiting = false; showControls(); }}
				onplay={() => { paused = false; phase = 'playing'; showControls(); }}
				onpause={() => { paused = true; save(true); showControls(); }}
				onended={() => save(true, duration || position)}
				onvolumechange={() => { muted = video.muted; volume = video.volume; }}
				onclick={togglePlay}
			></video>
		{/if}

		{#if waiting || seeking || phase === 'preparing'}
			<div class="player-busy" aria-live="polite">
				<span class="player-spinner" aria-hidden="true"></span>
				<span class="label">{phase === 'preparing' ? t.player.preparing : t.player.buffering}</span>
			</div>
		{/if}

		<div class="player-scrim player-scrim--top" class:player-hidden={hidden}></div>
		<div class="player-scrim player-scrim--bottom" class:player-hidden={hidden}></div>

		<header class="player-top" class:player-hidden={hidden} inert={helpOpen}>
			<button type="button" onclick={() => { save(true); onclose(); }} class="player-icon-button">
				<Icon name="back" label={t.player.close} />
			</button>
			<div class="min-w-0">
				<p class="player-title truncate">{displayTitle(movie)}</p>
				{#if isRemux}
					<p class="micro mt-1">{t.player.remuxBadge}</p>
				{/if}
			</div>
		</header>

		<div class="player-controls" class:player-hidden={hidden} inert={helpOpen}>
			<!--
				Custom controls rather than the browser's, for one reason that
				matters: a remuxed stream is a pipe with no length, so the native
				scrub bar has nothing to draw and seeking does nothing. This bar
				knows the duration from the server and seeks by restarting ffmpeg
				at a timestamp.
			-->
			<div
				bind:this={scrubTrack}
				class="scrub"
				class:scrub--disabled={duration <= 0}
				role="slider"
				tabindex="0"
				aria-label={t.player.position}
				aria-valuemin="0"
				aria-valuemax={Math.round(duration)}
				aria-valuenow={Math.round(position)}
				aria-valuetext={formatTime(position)}
				onpointerdown={onScrubDown}
				onpointermove={onScrubMove}
				onpointerup={onScrubUp}
				onpointercancel={onScrubUp}
				onpointerleave={() => (hoverRatio = null)}
				onkeydown={(e) => {
					const step = e.shiftKey ? 60 : 10;
					if (e.key === 'ArrowRight') { e.preventDefault(); seekTo(position + step); }
					if (e.key === 'ArrowLeft') { e.preventDefault(); seekTo(position - step); }
					if (e.key === 'Home') { e.preventDefault(); seekTo(0); }
					if (e.key === 'End') { e.preventDefault(); seekTo(duration - 5); }
				}}
			>
				<div class="scrub-track">
					<div class="scrub-buffered" style="width: {bufferedPercent}%"></div>
					<div class="scrub-played" style="width: {progressPercent}%"></div>
				</div>
				<div class="scrub-handle" style="left: {progressPercent}%"></div>

				{#if hoverRatio !== null && duration > 0}
					<span class="scrub-preview" style="left: {hoverRatio * 100}%">
						{formatTime(hoverRatio * duration)}
					</span>
				{/if}
			</div>

			<div class="player-buttons">
				<button type="button" onclick={togglePlay} class="player-icon-button player-icon-button--primary" data-remote-default>
					<Icon name={paused ? 'play' : 'pause'} size={26} label={paused ? t.player.play : t.player.pause} />
				</button>

				<button type="button" onclick={() => seekTo(position - 10)} class="player-icon-button">
					<Icon name="back10" label={t.player.back10} />
				</button>
				<button type="button" onclick={() => seekTo(position + 10)} class="player-icon-button">
					<Icon name="forward10" label={t.player.forward10} />
				</button>

				<div class="player-time tabular-nums">
					<span>{formatTime(position)}</span>
					<span class="text-faint"> / </span>
					<span class="text-muted">{duration > 0 ? formatTime(duration) : '—'}</span>
					{#if remaining > 0}
						<span class="player-remaining">−{formatTime(remaining)}</span>
					{/if}
				</div>

				<span class="flex-1"></span>

				<div class="player-volume">
					<button type="button" onclick={toggleMute} class="player-icon-button">
						<Icon
							name={muted || volume === 0 ? 'volumeMuted' : volume < 0.5 ? 'volumeLow' : 'volumeHigh'}
							label={muted ? t.player.unmute : t.player.mute}
						/>
					</button>
					<input
						type="range"
						min="0"
						max="1"
						step="0.02"
						value={muted ? 0 : volume}
						oninput={(e) => setVolume(Number(e.currentTarget.value))}
						class="volume-slider"
						aria-label={t.player.volume}
					/>
				</div>

				{#if phase === 'playing'}
					<button
						type="button"
						onclick={(event) => openHelp(event.currentTarget)}
						class="player-icon-button"
					>
						<Icon name="help" label={t.player.shortcuts.open} />
					</button>
				{/if}

				<button type="button" onclick={toggleFullscreen} class="player-icon-button">
					<Icon name="fullscreen" label={t.player.fullscreen} />
				</button>
			</div>
		</div>

		{#if helpOpen && phase === 'playing'}
			<section
				bind:this={helpPanel}
				class="player-shortcuts"
				aria-labelledby="player-shortcuts-title"
			>
				<div class="player-shortcuts-header">
					<div>
						<span class="label">{t.appName}</span>
						<h2 id="player-shortcuts-title">{t.player.shortcuts.title}</h2>
					</div>
					<button
						type="button"
						onclick={closeHelp}
						class="player-icon-button"
						data-shortcuts-close
					>
						<Icon name="close" label={t.player.shortcuts.close} />
					</button>
				</div>

				<p class="tv-copy player-shortcuts-intro">{t.player.shortcuts.intro}</p>

				<dl class="player-shortcuts-list">
					{#each t.player.shortcuts.items as shortcut (shortcut.action)}
						<div>
							<dt>
								{#each shortcut.keys as key (key)}
									<kbd>{key}</kbd>
								{/each}
							</dt>
							<dd>{shortcut.action}</dd>
						</div>
					{/each}
				</dl>

				<p class="player-shortcuts-hint">{t.player.shortcuts.scrubHint}</p>
			</section>
		{/if}
	{/if}
</div>
