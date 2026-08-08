<script>
	import { onMount, onDestroy, tick } from 'svelte';
	import { apiFetch, getJSON, formatTime, displayTitle } from '$lib/api.js';
	import { strings as t } from '$lib/strings.js';
	import { profiles } from '$lib/profiles.svelte.js';
	import Icon from './Icon.svelte';

	// fileId and audioTrackId come from the chooser on the film page. When they
	// are absent the player falls back to the v1 routes, which the server still
	// binds to the primary file -- that is the compatibility net for the home
	// hero's "reprendre", not a path M1 should build on.
	// streamBase and progressPath let an episode reuse this player: its routes
	// live under /api/library/episodes/{id}/... rather than /api/stream/{id}/...,
	// but the shapes below them are identical -- info, the bare path, /remux.
	let {
		movie,
		fileId = null,
		streamBase = null,
		progressPath = null,
		subtitleBase = null,
		title = null,
		onclose,
		onprogress
	} = $props();

	const base = $derived(
		streamBase ?? (fileId ? `/api/stream/${movie.id}/files/${fileId}` : `/api/stream/${movie.id}`)
	);
	const progressRoute = $derived(progressPath ?? `/api/library/movies/${movie.id}/progress`);
	const heading = $derived(title ?? displayTitle(movie));

	// Which audio and which subtitles, owned here rather than by the page.
	//
	// It used to be the film page's business, which was wrong twice over. A
	// choice made while watching belongs where the watching happens -- nobody
	// leaves a film to change the language -- and the page's copy of the tracks
	// was a snapshot taken before playback, so a file measured for the first
	// time by this very playback showed an empty menu until somebody reloaded.
	// The player asks /info for itself and gets both from the same answer.
	let audioTrackId = $state(null);
	let subtitleTrackId = $state(null);

	// V2-M6. `null` is the file as it is; a number is a height to re-encode to.
	let qualityHeight = $state(null);
	// Set when the browser has proved it cannot decode the picture. The server
	// cannot know this -- HEVC plays in Safari and not in Chrome -- so this is
	// the one fact only the client holds, sent back as a request to re-encode.
	let forceTranscode = $state(false);

	const audioQuery = $derived(
		[
			audioTrackId ? `audio=${audioTrackId}` : '',
			qualityHeight ? `h=${qualityHeight}` : '',
			forceTranscode ? 'video=transcode' : ''
		]
			.filter(Boolean)
			.join('&')
	);

	/** @type {'checking' | 'resume' | 'playing' | 'preparing' | 'failed'} */
	let phase = $state('checking');
	let info = $state(null);
	let source = $state(null);
	let failureCode = $state(null);
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
	let tracksOpen = $state(false);
	/** @type {HTMLElement} */
	let tracksPanel = $state();
	/** @type {HTMLTrackElement} */
	let trackElement = $state();
	// A file nobody has inspected only reveals its tracks when playback probes
	// it. One refresh after the picture arrives fills the menu; a flag keeps it
	// to one, because /info must never become a poll.
	let refreshedAfterProbe = $state(false);

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
	// Three sources, one message: the player's own states (`unavailable`,
	// `noFfmpeg`, `failed`) and the server's stable codes, which live in their
	// own table so a code is never mistaken for a UI string.
	const failure = $derived(
		failureCode
			? (t.player.codes[failureCode] ?? t.player[failureCode] ?? t.player.failed)
			: null
	);
	const progressPercent = $derived(duration > 0 ? Math.min(100, (position / duration) * 100) : 0);
	const bufferedPercent = $derived(duration > 0 ? Math.min(100, (buffered / duration) * 100) : 0);
	const remaining = $derived(duration > 0 ? Math.max(0, duration - position) : 0);

	const audioTracks = $derived(info?.audio_tracks ?? []);
	const subtitleTracks = $derived(info?.subtitle_tracks ?? []);
	// Only rungs this machine can actually produce. The server sends none when
	// it has no encoder that runs, and the menu then has no quality section
	// rather than a button that fails.
	const qualities = $derived(info?.transcode?.available ? (info?.qualities ?? []) : []);
	const transcodeKind = $derived(info?.transcode?.kind ?? null);
	// A single audio track is not a choice, and offering it would only force a
	// remux for nothing. Subtitles always are: "none" is one of the answers.
	const hasTrackMenu = $derived(
		audioTracks.length > 1 || subtitleTracks.length > 0 || qualities.length > 1
	);
	const subtitleTrack = $derived(
		subtitleTracks.find((track) => track.id === subtitleTrackId && track.kind === 'text') ?? null
	);
	// The cues are on the film's clock; a remuxed pipe restarts at zero on every
	// seek. The same number that moved ffmpeg moves the text, so they cannot
	// drift apart. Direct play needs no shift: its clock is the film's.
	const subtitleSrc = $derived(
		subtitleTrack && subtitleBase
			? profiles.url(
					`${subtitleBase}/subtitles/${subtitleTrack.id}` +
						(isRemux && offset > 0 ? `?t=${Math.floor(offset)}` : '')
				)
			: null
	);

	let saveTimer;
	let lastSaved = 0;
	let returnFocus;
	let helpReturnFocus;
	let tracksReturnFocus;
	let previousBodyOverflow;

	onMount(async () => {
		returnFocus = document.activeElement;
		previousBodyOverflow = document.body.style.overflow;
		document.body.style.overflow = 'hidden';

		if (!(await loadInfo())) return;

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

	// One place asks the server how this file will be delivered, and with it
	// what can be chosen while watching. Returns false when it has already put
	// the player into a failed state.
	async function loadInfo() {
		try {
			const query = audioQuery ? `?${audioQuery}` : '';
			info = await getJSON(profiles.url(`${base}/info${query}`));
		} catch (error) {
			phase = 'failed';
			failureCode = error?.code ?? 'unavailable';
			return false;
		}
		// The server decides; it does not merely advise. A refusal is reported
		// with its own reason rather than letting the element fail on its own
		// and blaming "an unsupported format" for a codec already named.
		if (info.mode === 'unsupported') {
			phase = 'failed';
			failureCode = info.reason_code ?? 'failed';
			return false;
		}
		// A subtitle chosen before a re-measurement may no longer exist; track
		// ids belong to one inspection.
		if (subtitleTrackId && !subtitleTracks.some((track) => track.id === subtitleTrackId)) {
			subtitleTrackId = null;
		}
		return true;
	}

	function start(from) {
		offset = from;
		elapsed = 0;
		triedRemux = false;

		// Direct play hands over the file untouched, so it cannot honour a
		// height or a re-encode. Either of those takes the ffmpeg route even
		// when the container would otherwise have played as it is.
		if (info.mode === 'direct' && !qualityHeight && !forceTranscode) {
			source = base;
			phase = 'playing';
			return;
		}
		startRemux(from);
	}

	function startRemux(from) {
		triedRemux = true;
		if (!info?.ffmpeg_supported) {
			phase = 'failed';
			failureCode = 'noFfmpeg';
			return;
		}
		// The offset belongs here rather than only in start(), because seeking
		// also comes through this function. Setting the source without moving
		// the offset leaves the displayed clock reading the old position while
		// ffmpeg streams from the new one -- and saves that wrong number.
		offset = from;
		elapsed = 0;

		phase = info?.ffmpeg_ready ? 'playing' : 'preparing';
		// `audio` has to survive every seek: dropping it would silently restart
		// the film on the file's default track halfway through.
		const query = [`t=${Math.floor(from)}`, audioQuery].filter(Boolean).join('&');
		source = `${base}/remux?${query}`;
	}

	// Changing the audio track is a new stream, not a setting: ffmpeg maps one
	// track and cannot be told to swap it mid-pipe. The position is kept so the
	// film carries on where it was, which is the only part the viewer sees.
	async function chooseAudio(id) {
		if (id === audioTrackId) return;
		const at = position;
		audioTrackId = id;
		phase = 'preparing';
		if (!(await loadInfo())) return;
		duration = info.duration_seconds || duration;
		start(at);
	}

	function chooseSubtitle(id) {
		subtitleTrackId = id;
		showSubtitleTrack();
	}

	// Changing quality is the same move as changing audio: ffmpeg is restarted
	// with different arguments, and the only part the viewer should notice is
	// that the film carries on where it was.
	async function chooseQuality(height) {
		if (height === qualityHeight) return;
		const at = position;
		qualityHeight = height;
		phase = 'preparing';
		if (!(await loadInfo())) return;
		duration = info.duration_seconds || duration;
		start(at);
	}

	// `default` on a <track> only counts while the document is parsed, and
	// changing the video's src resets every text track to disabled. So the mode
	// is asserted rather than declared, after each of those moments.
	function showSubtitleTrack() {
		tick().then(() => {
			const tracks = video?.textTracks;
			if (!tracks) return;
			for (const track of tracks) {
				track.mode = trackElement && track === trackElement.track ? 'showing' : 'disabled';
			}
			liftCues();
		});
	}

	// Subtitles sit on the last line of the picture, which is exactly where the
	// control bar appears. Measured in a real browser: the second line of a cue
	// was behind the scrub bar for as long as the controls were up.
	//
	// `line` counts from the bottom when negative, so this is the standard way
	// to say "four lines higher" without a vendor-prefixed pseudo-element. The
	// text lifts while the controls are up and drops back when they fade, which
	// is the behaviour every other player has trained people to expect.
	function liftCues() {
		const cues = trackElement?.track?.cues;
		if (!cues || !video) return;

		// Measured rather than guessed, in three steps that each cost something
		// to get wrong.
		//
		// `line` as a count of lines was the obvious answer and does not work:
		// it snaps to a line height the browser picks, and at this type size a
		// two-line cue still sat under the scrub bar at -4. `lineAlign: 'end'`
		// would anchor the bottom of the cue and is the right idea, but Chrome
		// ignores it -- verified, the box stayed anchored by its top. So the
		// position is computed here: the top of the cue, as a share of the
		// picture, from its own line count and the real height of the controls.
		const area = video.getBoundingClientRect().height;
		if (!area) return;
		const bar = hidden
			? 0
			: (shell?.querySelector('.player-controls')?.getBoundingClientRect().height ?? 0);

		// The height of one cue line, which no API will tell you: the cue box
		// lives in a closed shadow root. The font size mirrors
		// `.player-video::cue` in app.css -- changing one means changing the
		// other -- and the multiplier is measured, not assumed, because Chrome
		// does not honour `line-height` on ::cue.
		//
		// What `line` buys is worth being precise about. Measured at 1080p on a
		// one-line cue, it goes from 79.06% with the bar up to 90.76% with it
		// down: an 11.7-point lift, which is the bar's own height. The engine
		// then maps that request through a safe area of its own, so the absolute
		// resting position is Chrome's to decide and lands in the lower third,
		// where a subtitle belongs. What this controls reliably -- and all it
		// needs to control -- is that the text moves out of the way of the
		// furniture and comes back when the furniture goes.
		const root = parseFloat(getComputedStyle(document.documentElement).fontSize) || 16;
		const cueFont = Math.min(Math.max(root * 1.0625, window.innerWidth * 0.026), root * 2.375);
		const lineHeight = cueFont * 2.2;
		const gap = area * 0.015;

		for (const cue of cues) {
			const lines = cue.text.split('\n').length;
			const top = area - bar - lines * lineHeight - gap;
			cue.snapToLines = false;
			cue.line = Math.max(0, Math.min(95, (top / area) * 100));
		}
	}

	$effect(() => {
		// Reading `hidden` is what subscribes this to the controls fading.
		void hidden;
		liftCues();
	});

	function onLoadedMetadata() {
		// Direct play knows its own length. A remux does not, so the value from
		// the server stands.
		if (!isRemux && Number.isFinite(video.duration) && video.duration > 0) {
			duration = video.duration;
			if (offset > 0) video.currentTime = offset;
		}
		seeking = false;

		// A browser that cannot decode the video codec still loads the file, still
		// reports readyState 4 and still plays the sound -- it simply never
		// produces a picture. Measured on an HEVC Main 10 remux: canPlayType
		// answers with an empty string, videoWidth stays 0 and not one frame is
		// decoded, while currentTime advances. Left alone that reads as "the sound
		// is badly out of sync", which is the wrong bug to go looking for. The
		// server flags the codec as risky; this is the browser answering.
		if (video.videoWidth === 0 && video.videoHeight === 0) {
			// The browser has just proved what no server could ask it: it loaded
			// the file, it will play the sound, and it will never produce a
			// picture. Before M6 that was the end of the road and the honest
			// thing to do was name the codec. Now there may be an encoder on
			// this machine, so the film is asked for again, re-encoded --
			// once, guarded by the flag, because a transcode that also fails
			// must not loop.
			if (info?.transcode?.available && !forceTranscode) {
				forceTranscode = true;
				start(position);
				return;
			}
			phase = 'failed';
			failureCode = 'browser_cannot_decode_video';
			return;
		}
		showSubtitleTrack();

		// The picture proves the file was probed. A file that had never been
		// inspected has only now revealed its tracks, so the menu is refilled
		// once -- the exact bug where a chooser stayed empty until a reload.
		if (!refreshedAfterProbe && info?.media_status !== 'ok') {
			refreshedAfterProbe = true;
			refreshTracks();
		}
	}

	// Deliberately silent, unlike loadInfo: the film is playing. A refresh that
	// fails costs a menu entry, and must not end a working playback.
	async function refreshTracks() {
		try {
			const query = audioQuery ? `?${audioQuery}` : '';
			const fresh = await getJSON(profiles.url(`${base}/info${query}`));
			if (fresh?.mode && fresh.mode !== 'unsupported') info = fresh;
		} catch {
			// The menu simply keeps what it had.
		}
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
		failureCode = 'failed';
	}

	async function save(force = false, at = null) {
		const seconds = at ?? position;
		if (!Number.isFinite(seconds)) return;
		// Every few seconds is plenty; the row only needs to know roughly where
		// somebody stopped.
		if (!force && Math.abs(seconds - lastSaved) < 5) return;
		lastSaved = seconds;

		try {
			const response = await apiFetch(profiles.url(progressRoute), {
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

	function openTracks(trigger = document.activeElement) {
		tracksReturnFocus = trigger instanceof HTMLElement ? trigger : shell;
		tracksOpen = true;
		controlsVisible = true;
		clearTimeout(idleTimer);

		tick().then(() => {
			const first = tracksPanel?.querySelector('[data-remote-default], button:not(:disabled)');
			if (first instanceof HTMLElement) first.focus();
		});
	}

	// Anywhere else dismisses it, which is what removed the "Fermer" button that
	// used to sit at the bottom of the panel. §6b: the furniture is icons, and a
	// popover that already answers Escape, its own toggle and a press outside
	// does not need a full row spelling out the word.
	function onShellPointerDown(event) {
		showControls();
		if (!tracksOpen) return;
		if (event.target instanceof Node && tracksPanel?.closest('.player-tracks-anchor')?.contains(event.target)) {
			return;
		}
		closeTracks();
	}

	function closeTracks() {
		tracksOpen = false;
		showControls();

		tick().then(() => {
			if (tracksReturnFocus instanceof HTMLElement && tracksReturnFocus.isConnected) {
				tracksReturnFocus.focus();
			} else {
				shell?.focus();
			}
		});
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
		if (paused || seeking || waiting || helpOpen || tracksOpen) return;

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

		// The panel is the innermost thing open, so it is what Escape closes.
		if (tracksOpen) {
			if (event.key === 'Escape') {
				event.preventDefault();
				closeTracks();
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

		// The track panel is deliberately absent here. It is a popover over the
		// control bar, not a second dialog: the button that opened it must stay
		// reachable to close it, and trapping focus would put that button out of
		// the tab order at exactly the wrong moment.
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
		if (phase !== 'playing' && tracksOpen) {
			tracksOpen = false;
			tracksReturnFocus = null;
		}
	});

	// The panel owns its vertical axis, in the spirit of decision 27 and of the
	// rule §9 of the design system took from it: inside a list, down means the
	// next line, not whatever geometry ranks nearest. An edge press is left
	// alone so leaving the list still falls through.
	function onTracksKeydown(event) {
		if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return;
		if (event.altKey || event.ctrlKey || event.metaKey || event.shiftKey) return;

		const options = [...(tracksPanel?.querySelectorAll('.track-option') ?? [])];
		const index = options.indexOf(document.activeElement);
		if (index < 0) return;

		const next = index + (event.key === 'ArrowDown' ? 1 : -1);
		if (next < 0 || next >= options.length) return;

		event.preventDefault();
		event.stopPropagation();
		options[next].focus({ preventScroll: true });
		options[next].scrollIntoView({
			block: 'nearest',
			behavior: window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth'
		});
	}

	// A row is two lines, not one string.
	//
	// Joining everything with middots gave "Anglais · Original 5.1 · AC3 ·
	// 5.1(side)" -- four facts at one weight, which at three metres is a wall.
	// People choose by language; the codec only confirms the choice. So the
	// language leads at reading size and the rest sits under it as metadata,
	// which is the same hierarchy the card grid uses for a title and its year.
	function audioLabel(track, index) {
		const primary = track.language
			? languageName(track.language)
			: track.title || t.film.audio.unnamed(index + 1);
		return { primary, detail: detailOf([track.title, track.codec?.toUpperCase(), track.channels], primary) };
	}

	function subtitleLabel(track, index) {
		const primary = track.language
			? languageName(track.language)
			: track.title || t.player.tracks.unnamedSubtitle(index + 1);
		return {
			primary,
			detail: detailOf(
				[
					track.title,
					track.is_forced ? t.player.tracks.forced : null,
					track.is_external ? t.player.tracks.external : null
				],
				primary
			)
		};
	}

	// A detail that merely repeats the line above it is noise: a French track
	// titled "Français" was rendering as "Français · Français".
	function detailOf(parts, primary) {
		const kept = parts
			.filter(Boolean)
			.filter((part) => part.toLowerCase() !== primary.toLowerCase());
		return kept.join(' · ');
	}

	// The catalogue owns the names; the server only ever sends the ISO code.
	function languageName(code) {
		return t.languages[code] ?? code.toUpperCase();
	}

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
		<!-- A tick, not a tinted row. The chosen state has to survive three
		     metres, and a 2px rule at 6% opacity did not. -->
		<span class="track-tick" aria-hidden="true">
			{#if chosen}<Icon name="check" size={18} />{/if}
		</span>
	</button>
{/snippet}

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<div
	bind:this={shell}
	class="player"
	class:player--idle={hidden}
	role="dialog"
	aria-modal="true"
	aria-label={heading}
	tabindex="-1"
	onkeydown={trapDialogFocus}
	onpointermove={showControls}
	onpointerdown={onShellPointerDown}
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
			<h2 class="page-title mx-auto mt-4 mb-3">{heading}</h2>
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
						await apiFetch(profiles.url(progressRoute), { method: 'DELETE' });
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
			>
				<!--
					One track element at a time, rather than all of them with modes
					juggled. The browser is then never holding a track nobody chose,
					and the src carries the seek, so the text arrives already on the
					same clock as the picture.
				-->
				{#if subtitleSrc}
					{#key subtitleSrc}
						<track
							bind:this={trackElement}
							kind="subtitles"
							src={subtitleSrc}
							srclang={subtitleTrack.language || 'und'}
							label={subtitleLabel(subtitleTrack, 0).primary}
							onload={showSubtitleTrack}
							default
						/>
					{/key}
				{/if}
			</video>
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
			<div class="player-heading">
				<p class="player-title">{heading}</p>
				{#if isRemux}
					<!-- Was `.micro`, which is --faint at 2.64:1 -- a ratio measured
					     against --ink, and this sits over whatever the film happens to
					     be showing. §3 reserves faint for decoration and forbids it for
					     anything read. A bordered pill in --muted instead: it reads as
					     chrome rather than as a caption trying to disappear. -->
					<span class="player-badge">{t.player.remuxBadge}</span>
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

				<!-- Two numbers, not three. The remaining time was the same fact as
				     the other two subtracted, printed in --faint, which §3 reserves
				     for decoration and forbids for anything read. -->
				<div class="player-time">
					<span class="player-time-now">{formatTime(position)}</span>
					<span class="player-time-total">{duration > 0 ? formatTime(duration) : '—'}</span>
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

				{#if hasTrackMenu}
					<!--
						The panel lives inside this wrapper rather than floating in the
						player, so it is anchored by construction and cannot drift.

						It used to be positioned against the frame: measured on a 900px
						viewport, the panel's right edge sat 117px away from the right
						edge of the button that opened it, which reads as a slab that
						happened to appear rather than a menu belonging to a control.
						Anchoring in CSS also means it follows the button at every
						viewport without a line of measuring code.
					-->
					<div class="player-tracks-anchor">
						<button
							type="button"
							onclick={(event) => (tracksOpen ? closeTracks() : openTracks(event.currentTarget))}
							class="player-icon-button"
							class:player-icon-button--lit={subtitleTrack !== null}
							aria-expanded={tracksOpen}
							aria-haspopup="true"
						>
							<Icon name="captions" label={t.player.tracks.open} />
						</button>

						{#if tracksOpen}
							<!-- svelte-ignore a11y_no_static_element_interactions -->
							<section
								bind:this={tracksPanel}
								class="player-tracks"
								aria-label={t.player.tracks.title}
								onkeydown={onTracksKeydown}
							>
								{#if audioTracks.length > 1}
									<h2 class="track-heading">{t.film.audio.title}</h2>
									<ul class="track-list">
										<li>
											{@render trackOption(
												{ primary: t.film.audio.auto, detail: '' },
												!audioTrackId,
												() => chooseAudio(null),
												true
											)}
										</li>
										{#each audioTracks as track, index (track.id)}
											<li>
												{@render trackOption(
													audioLabel(track, index),
													audioTrackId === track.id,
													() => chooseAudio(track.id),
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
													() => chooseQuality(quality.height || null),
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
												() => chooseSubtitle(null),
												audioTracks.length <= 1
											)}
										</li>
										{#each subtitleTracks as track, index (track.id)}
											<li>
												{#if track.kind === 'text'}
													{@render trackOption(
														subtitleLabel(track, index),
														subtitleTrackId === track.id,
														() => chooseSubtitle(track.id),
														false
													)}
												{:else}
													<!--
														Listed, not hidden. Decision 3 refuses to render a
														bitmap track because showing one means burning it
														into the picture; a rip whose only subtitles are
														PGS would otherwise look like a film that has
														none, and somebody would go hunting for a setting
														that does not exist.
													-->
													<p class="track-option track-option--refused">
														<span class="track-primary">{subtitleLabel(track, index).primary}</span>
														<span class="track-detail">{t.player.tracks.imageBased}</span>
													</p>
												{/if}
											</li>
										{/each}
									</ul>
								{/if}
							</section>
						{/if}
					</div>
				{/if}

				{#if phase === 'playing'}
					<button
						type="button"
						onclick={(event) => openHelp(event.currentTarget)}
						class="player-icon-button player-icon-button--desktop"
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
