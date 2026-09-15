<script>
	// The OSD. It owns every sentence the viewer reads and nothing else: the
	// engine, the session and the lifecycle belong to Rust, and the Rust side
	// sends codes rather than prose (decision 25).
	//
	// The furniture follows design system section 6b: one filled control, the
	// gold accent spent on the timeline, targets of at least 44px, and a bar
	// that hides itself after three seconds of nothing while never hiding while
	// the film is paused.

	import Icon from '../../../web/src/lib/components/Icon.svelte';
	import FilmCard from './components/FilmCard.svelte';
	import TrackMenu from './components/TrackMenu.svelte';
	import { catalogues, initialLanguage } from './lib/catalogues.js';

	let lang = $state(initialLanguage());
	const t = (key) => catalogues[lang][key] ?? key;

	let status = $state({ ready: false });
	/** @type {string | null} */
	let noticeKey = $state(null);
	let idle = $state(false);
	let overControls = $state(false);

	// The library panel. It is shown while nothing is playing, which is the
	// only moment a viewer is choosing rather than watching.
	/** @type {{url: string, health: {version: string}, profiles: unknown[]} | null} */
	let server = $state(null);
	/** @type {Array<{id: number, title: string, year?: number, progress?: {position_seconds: number, finished: boolean}}>} */
	let movies = $state([]);
	/** @type {Array<{name: string, url: string, version?: string}>} */
	let discovered = $state([]);
	let address = $state('http://');
	let busy = $state(false);
	/** @type {string | null} */
	let errorKey = $state(null);

	// The track menu. Its list is fetched when it opens rather than carried in
	// every status frame: tracks change rarely, and a status that shipped the
	// whole list twice a second would be a status nobody reads.
	/** @type {Array<Record<string, unknown>>} */
	let tracks = $state([]);
	let trackMenuOpen = $state(false);

	async function refreshTracks() {
		try {
			tracks = JSON.parse(await invoke('player_tracks'));
		} catch {
			tracks = [];
		}
	}

	async function toggleTrackMenu() {
		wake();
		trackMenuOpen = !trackMenuOpen;
		if (trackMenuOpen) await refreshTracks();
	}

	async function pickTrack(kind, id) {
		try {
			await invoke('player_set_track', { kind, id });
		} catch {
			errorKey = 'trackFailed';
		}
		// Re-read rather than assume: mpv is the one that knows which track is
		// now playing, and a menu that ticks the wrong line is worse than none.
		await refreshTracks();
	}

	const invoke = window.__TAURI__?.core?.invoke ?? (async () => {});
	const listen = window.__TAURI__?.event?.listen ?? (async () => () => {});
	const currentWindow = window.__TAURI__?.window?.getCurrentWindow?.();

	let idleTimer;
	const IDLE_MS = 3000;

	function wake() {
		idle = false;
		clearTimeout(idleTimer);
		if (status.pause) return;
		idleTimer = setTimeout(() => {
			if (!overControls) idle = true;
		}, IDLE_MS);
	}

	// Re-armed on every state change, and never armed while paused.
	$effect(() => {
		if (status.pause) {
			idle = false;
			clearTimeout(idleTimer);
		} else if (status.ready) {
			wake();
		}
	});

	$effect(() => {
		const pending = [];
		listen('player-status', (event) => {
			try {
				status = JSON.parse(event.payload);
			} catch {
				// A malformed frame is not worth taking the interface down for.
			}
		}).then((un) => pending.push(un));
		listen('player-event', (event) => {
			try {
				const ev = JSON.parse(event.payload);
				if (ev.kind === 'audio' && ev.mode === 'pcm') noticeKey = 'audioFallback';
				if (ev.kind === 'engine' && ev.state === 'unavailable') noticeKey = 'engineUnavailable';
			} catch {
				// Same.
			}
		}).then((un) => pending.push(un));
		return () => pending.forEach((un) => un());
	});

	const seconds = $derived(Number(status.pos) || 0);
	const duration = $derived(Number(status.duration) || 0);
	const progress = $derived(duration > 0 ? Math.min(1, seconds / duration) : 0);

	function clock(value) {
		if (!Number.isFinite(value) || value <= 0) return t('unknownDuration');
		const total = Math.floor(value);
		const h = Math.floor(total / 3600);
		const m = Math.floor((total % 3600) / 60);
		const s = total % 60;
		const mm = String(m).padStart(h ? 2 : 1, '0');
		const ss = String(s).padStart(2, '0');
		return h ? `${h}:${mm}:${ss}` : `${mm}:${ss}`;
	}

	function toggle() {
		wake();
		invoke('player_toggle_pause');
	}

	function toggleMute() {
		wake();
		invoke('player_set_muted', { muted: !status.mute });
	}

	// --- the library -------------------------------------------------------

	async function connect(url) {
		if (!url || busy) return;
		busy = true;
		errorKey = null;
		try {
			server = JSON.parse(await invoke('player_connect', { url }));
			await loadLibrary();
		} catch {
			// The Rust side answers with a diagnostic in English; the sentence a
			// person reads is chosen here, from the catalogue.
			server = null;
			errorKey = 'connectionFailed';
		} finally {
			busy = false;
		}
	}

	async function loadLibrary() {
		try {
			movies = JSON.parse(await invoke('player_library', { limit: 60 }));
		} catch {
			movies = [];
			errorKey = 'libraryFailed';
		}
	}

	async function findServers() {
		if (busy) return;
		busy = true;
		errorKey = null;
		try {
			discovered = JSON.parse(await invoke('player_discover'));
			// One server and nothing else to choose between: connect to it, the
			// same way one profile is not a question.
			if (discovered.length === 1) await connect(discovered[0].url);
		} catch {
			discovered = [];
		} finally {
			busy = false;
		}
	}

	async function play(id) {
		try {
			await invoke('player_play', { id });
		} catch {
			errorKey = 'playFailed';
		}
	}

	function seek(offset) {
		wake();
		invoke('player_seek', { seconds: offset, mode: 'relative' });
	}

	function seekTo(event) {
		wake();
		if (duration <= 0) return;
		const rect = event.currentTarget.getBoundingClientRect();
		const ratio = Math.min(1, Math.max(0, (event.clientX - rect.left) / rect.width));
		invoke('player_seek', { seconds: ratio * duration, mode: 'absolute' });
	}

	// A slider that only answers a mouse is not a slider. The arrows move by ten
	// seconds like the skip buttons, Home and End go to the ends, and every key
	// is stopped here so the window handler below does not seek a second time.
	function onScrubKey(event) {
		const go = (fn) => {
			event.preventDefault();
			event.stopPropagation();
			fn();
		};
		if (event.key === 'ArrowLeft' || event.key === 'ArrowDown') go(() => seek(-10));
		else if (event.key === 'ArrowRight' || event.key === 'ArrowUp') go(() => seek(10));
		else if (event.key === 'Home') go(() => invoke('player_seek', { seconds: 0, mode: 'absolute' }));
		else if (event.key === 'End' && duration > 0)
			go(() => invoke('player_seek', { seconds: duration, mode: 'absolute' }));
	}

	// A click that is not on a control is a click on the picture. The OSD layer
	// carries pointer-events: none, so those clicks arrive here; the guard keeps
	// a button press from also toggling playback.
	function onBackgroundClick(event) {
		if (event.target.closest('button, .scrub, .title-bar, .notice, .track-menu')) return;
		// A click away from an open menu closes it. Toggling playback as well
		// would be two answers to one gesture.
		if (trackMenuOpen) {
			trackMenuOpen = false;
			return;
		}
		toggle();
	}

	// Where the pointer is decides whether the furniture may hide. Tracked from
	// the window rather than from a handler on the overlay, which would make a
	// static element interactive and is what the a11y linter rightly objects to.
	function onPointerMove(event) {
		overControls = !!event.target.closest?.('.controls, .title-bar, .notice');
		wake();
	}

	async function toggleFullscreen() {
		wake();
		if (!currentWindow) return;
		const isFull = await currentWindow.isFullscreen();
		await currentWindow.setFullscreen(!isFull);
	}

	function close() {
		currentWindow?.close();
	}

	function switchLanguage() {
		lang = lang === 'fr' ? 'en' : 'fr';
		try {
			localStorage.setItem('theia.player.language', lang);
		} catch {
			// Storage disabled: the choice lasts for the session, which is fine.
		}
		document.documentElement.lang = lang;
	}

	function onKey(event) {
		const key = event.key;
		if (key === ' ' || key === 'k') {
			event.preventDefault();
			toggle();
		} else if (key === 'ArrowLeft') {
			seek(-10);
		} else if (key === 'ArrowRight') {
			seek(10);
		} else if (key === 'f') {
			toggleFullscreen();
		} else if (key === 'm') {
			toggleMute();
		} else if (key === 'c') {
			toggleTrackMenu();
		} else if (key === 'Escape') {
			// The menu closes before the player does, so a viewer who opened it
			// by accident does not lose the film with it.
			if (trackMenuOpen) {
				trackMenuOpen = false;
			} else {
				close();
			}
		} else if (key === 'l') {
			switchLanguage();
		} else {
			wake();
		}
	}
</script>

<svelte:window onkeydown={onKey} onmousemove={onPointerMove} onclick={onBackgroundClick} />

<div class="osd" data-idle={idle}>
	<div class="scrim-top"></div>
	<div class="scrim-bottom"></div>

	<header class="title-bar" data-tauri-drag-region>
		<span class="label">Theia</span>
		<span class="film-title">{status.title ?? ''}</span>
	</header>

	{#if noticeKey}
		<div class="notice" role="status">
			<span class="label">{t('audioFallbackLabel')}</span>
			{t(noticeKey)}
		</div>
	{/if}

	{#if !status.ready}
		<div class="notice" role="status"><span class="label">{t('loading')}</span></div>
	{/if}

	{#if !status.media}
		<!-- The library is shown while nothing plays, which is the only moment
		     a viewer is choosing rather than watching. Once a film starts it
		     gets out of the way entirely: the picture is the interface. -->
		<section class="library">
			<h1 class="library-title">{server ? t('library') : t('chooseFilm')}</h1>

			{#if !server}
				<form
					class="connect"
					onsubmit={(event) => {
						event.preventDefault();
						connect(address);
					}}
				>
					<label class="label" for="theia-address">{t('address')}</label>
					<div class="connect-row">
						<input
							id="theia-address"
							bind:value={address}
							placeholder="http://192.168.1.20:8395"
							autocomplete="off"
							spellcheck="false"
						/>
						<button class="action" type="submit" disabled={busy}>
							{busy ? t('searching') : t('connect')}
						</button>
						<button class="action action--quiet" type="button" onclick={findServers} disabled={busy}>
							{t('findServers')}
						</button>
					</div>
				</form>

				{#if discovered.length}
					<ul class="servers">
						{#each discovered as entry (entry.url)}
							<li>
								<button class="action action--quiet" onclick={() => connect(entry.url)}>
									{entry.name} — {entry.url}
								</button>
							</li>
						{/each}
					</ul>
				{/if}

				<p class="hint" class:hint--error={errorKey}>
					{errorKey ? t(errorKey) : t('noServer')}
				</p>
			{:else if movies.length === 0}
				<p class="hint">{t('emptyLibrary')}</p>
			{:else}
				<!-- The library is a card grid, not a list of names: section 6 is
				     the authority, and a second way of drawing a library would be
				     a second identity. -->
				<ul class="films">
					{#each movies as movie (movie.id)}
						<FilmCard {movie} onplay={play} {t} />
					{/each}
				</ul>
				{#if errorKey}<p class="hint hint--error">{t(errorKey)}</p>{/if}
			{/if}
		</section>
	{/if}

	<div class="controls">
		<!-- Three things on one rule: played in gold, and the rest quiet. The
		     painted bar is 4px, the hit area 24px, because a thumb is not a
		     mouse (design system 6b). -->
		<div
			class="scrub"
			role="slider"
			tabindex="0"
			aria-label={t('play')}
			aria-valuemin="0"
			aria-valuemax={Math.round(duration)}
			aria-valuenow={Math.round(seconds)}
			onclick={seekTo}
			onkeydown={onScrubKey}
		>
			<div class="scrub-track">
				<div class="scrub-played" style="width: {progress * 100}%"></div>
			</div>
			<div class="scrub-thumb" style="left: {progress * 100}%"></div>
		</div>

		<div class="row">
			<button
				class="control control--skip"
				onclick={() => seek(-10)}
				aria-label={t('back10')}
				title={t('back10')}
			>
				<Icon name="back10" />
			</button>
			<button
				class="control control--primary"
				onclick={toggle}
				aria-label={status.pause ? t('play') : t('pause')}
				title={status.pause ? t('play') : t('pause')}
			>
				<Icon name={status.pause ? 'play' : 'pause'} />
			</button>
			<button
				class="control control--skip"
				onclick={() => seek(10)}
				aria-label={t('forward10')}
				title={t('forward10')}
			>
				<Icon name="forward10" />
			</button>

			<span class="clock">
				<span class="elapsed">{clock(seconds)}</span>
				<span class="rule"></span>
				<span class="total">{clock(duration)}</span>
			</span>

			<span class="spacer"></span>

			{#if status.media}
				<!-- Anchored to its own button, as 6b requires: a popover
				     positioned against the frame reads as a slab that happened
				     to appear. -->
				<div class="menu-anchor">
					<button
						class="control"
						onclick={toggleTrackMenu}
						aria-label={t('tracks')}
						title={t('tracks')}
						aria-haspopup="menu"
						aria-expanded={trackMenuOpen ? 'true' : 'false'}
					>
						<Icon name="settings" />
					</button>
					{#if trackMenuOpen}
						<TrackMenu {tracks} onpick={pickTrack} {t} />
					{/if}
				</div>
			{/if}

			<button
				class="control control--mute"
				onclick={toggleMute}
				aria-label={status.mute ? t('unmute') : t('mute')}
				title={status.mute ? t('unmute') : t('mute')}
				aria-pressed={status.mute ? 'true' : 'false'}
			>
				<Icon name={status.mute ? 'volumeMuted' : 'volumeHigh'} />
			</button>

			{#if status.audioMode === 'pcm'}
				<span class="label audio-mode">PCM</span>
			{:else if status.audioMode === 'passthrough'}
				<span class="label audio-mode">BITSTREAM</span>
			{/if}

			<button
				class="control control--desktop"
				onclick={switchLanguage}
				aria-label="Français / English"
				title="Français / English"
			>
				<span class="label">{lang.toUpperCase()}</span>
			</button>
			<button
				class="control"
				onclick={toggleFullscreen}
				aria-label={t('fullscreen')}
				title={t('fullscreen')}
			>
				<Icon name="fullscreen" />
			</button>
			<button class="control" onclick={close} aria-label={t('close')} title={t('close')}>
				<Icon name="close" />
			</button>
		</div>
	</div>
</div>
