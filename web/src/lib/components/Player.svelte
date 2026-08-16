<script>
	import { onMount, onDestroy, tick } from 'svelte';
	import { apiFetch, getJSON, formatTime, displayTitle } from '$lib/api.js';
	import { strings as t } from '$lib/strings.js';
	import { profiles } from '$lib/profiles.svelte.js';
	import { codecPlayback } from '$lib/codec-playback.svelte.js';
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
	/** The lines currently on screen, painted by this component. */
	let subtitleLines = $state([]);

	// How far the subtitles are pushed against the picture, in seconds.
	//
	// A rip whose subtitle track was muxed from a different cut runs a second or
	// two out, and nothing else in the interface can rescue it: the file is the
	// file, and decision 3 refuses to burn anything into the image. This is the
	// one control that makes such a file watchable.
	//
	// Deliberately not persisted. It belongs to this viewing, not to the film:
	// a stored offset would be one more piece of state to be wrong later, and
	// the setting is two presses away whenever it is wanted.
	let subtitleOffset = $state(0);
	const subtitleOffsetStep = 0.5;
	const subtitleOffsetLimit = 30;

	// The cue times as the file wrote them. Shifting has to be computed against
	// these rather than against the last shift, or the rounding walks.
	let originalCueTimes = new WeakMap();
	/** Distance from the bottom of the video element, in pixels. */
	let subtitleBottom = $state(0);
	// A file nobody has inspected only reveals its tracks when playback probes
	// it. One refresh after the picture arrives fills the menu; a flag keeps it
	// to one, because /info must never become a poll.
	let refreshedAfterProbe = $state(false);

	// While a scrub is in progress the bar follows the pointer rather than the
	// video, otherwise the handle fights the person dragging it.
	let scrubbing = $state(false);
	let scrubValue = $state(0);
	let hoverRatio = $state(null);

	// The strip of frames under the cursor.
	//
	// A comfort, and treated as one: it is asked for once, it is never waited
	// for, and every failure -- no ffmpeg, a file too short, a build still
	// running -- ends with the timestamp alone, which is what the bar showed
	// before this existed.
	let preview = $state(null);
	let previewWidth = $state(0);

	const previewTile = $derived.by(() => {
		if (!preview || hoverRatio === null || duration <= 0 || !previewWidth) return null;
		const index = Math.min(
			preview.manifest.count - 1,
			Math.max(0, Math.floor((hoverRatio * duration) / preview.manifest.interval_seconds))
		);
		const column = index % preview.manifest.columns;
		const row = Math.floor(index / preview.manifest.columns);
		const height = preview.manifest.tile_height;
		return {
			width: previewWidth,
			height,
			// Background offsets are negative: the sheet is moved behind a
			// window the size of one tile, not cropped.
			x: -column * previewWidth,
			y: -row * height,
			sheetWidth: previewWidth * preview.manifest.columns,
			sheetHeight: height * preview.manifest.rows
		};
	});

	// The tile width is measured off the loaded sheet rather than sent: the
	// pinned ffmpeg build ships no ffprobe, so the server cannot say what the
	// source's aspect made it without guessing one.
	function onSheetLoaded(event) {
		const image = event.currentTarget;
		if (!preview || !image.naturalWidth) return;
		previewWidth = Math.round(image.naturalWidth / preview.manifest.columns);
	}

	async function loadPreview() {
		try {
			const body = await getJSON(`${base}/preview`);
			if (body.state !== 'ready' || !body.manifest) return;
			preview = body;
		} catch {
			// No preview for this file, and nothing to say about it.
		}
	}

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
	// What can actually be turned on, and what merely explains itself.
	const textSubtitles = $derived(subtitleTracks.filter((track) => track.kind === 'text'));
	// Decision 3 refuses to render a bitmap track, and listing one was the way
	// to stop a PGS-only rip looking like a film with no subtitles at all. But
	// on a disc rip that also carries an SRT it is a dead row under a live one,
	// which reads as a choice that does not work. So it is shown only when it is
	// the only thing there is.
	const refusedSubtitles = $derived(
		textSubtitles.length ? [] : subtitleTracks.filter((track) => track.kind !== 'text')
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

		// Asked once, never awaited. A sheet that does not exist yet is being
		// built for the next time this film is opened; nothing here waits for it
		// and nothing here fails without it.
		loadPreview();

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
		clearTimeout(paceTimer);
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
			// Belt and braces: the pages that mount the player already wait for it,
			// but a position written against the wrong viewer is not a mistake that
			// announces itself, so the player does not assume.
			await profiles.ready();
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
		// A verdict this browser already reached, on an earlier film. Without it
		// every HEVC file would open with the same few seconds of sound running
		// ahead of the picture before the measurement below caught up; with it,
		// only the first one ever does. The re-encode starts at the source size,
		// so nothing but the codec changes.
		if (
			!forceTranscode &&
			info.video_risky &&
			info.transcode?.available &&
			codecPlayback.strugglesWith(info.video_codec)
		) {
			forceTranscode = true;
			return loadInfo();
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
	// `hidden`, not `showing`: the cues still fire, and nothing is painted by the
	// browser. What the viewer reads is drawn by this component instead, because
	// the engine would not put it where it belongs -- see .player-subtitles.
	function showSubtitleTrack() {
		tick().then(() => {
			const tracks = video?.textTracks;
			if (!tracks) return;
			for (const track of tracks) {
				track.mode = trackElement && track === trackElement.track ? 'hidden' : 'disabled';
			}
			const active = trackElement?.track;
			if (active) {
				active.removeEventListener('cuechange', readActiveCues);
				active.addEventListener('cuechange', readActiveCues);
			}
			// A newly loaded track arrives at the file's own times, so whatever
			// the viewer had already dialled in is re-applied here rather than
			// silently forgotten when they switch track.
			applySubtitleOffset();
			readActiveCues();
			liftCues();
		});
	}

	// Moves every cue of the active track to where the viewer says it belongs.
	//
	// The times are rewritten rather than the clock being read with an offset,
	// so the browser's own cuechange keeps firing and everything downstream --
	// active cues, the lift over the control bar -- carries on unchanged.
	function applySubtitleOffset() {
		const track = trackElement?.track;
		const cues = track?.cues;
		if (!cues) return;

		for (const cue of cues) {
			let original = originalCueTimes.get(cue);
			if (!original) {
				original = { start: cue.startTime, end: cue.endTime };
				originalCueTimes.set(cue, original);
			}
			// A cue may not start before the film does, and may not end before it
			// starts, which a large negative offset would otherwise ask for.
			const start = Math.max(0, original.start + subtitleOffset);
			cue.startTime = start;
			cue.endTime = Math.max(start + 0.05, original.end + subtitleOffset);
		}
		readActiveCues();
	}

	function nudgeSubtitles(direction) {
		const next = subtitleOffset + direction * subtitleOffsetStep;
		subtitleOffset = Math.max(-subtitleOffsetLimit, Math.min(subtitleOffsetLimit, next));
		// Rounded, because repeated halves in binary drift into 1.4999999.
		subtitleOffset = Math.round(subtitleOffset * 100) / 100;
		applySubtitleOffset();
	}

	function resetSubtitleOffset() {
		subtitleOffset = 0;
		applySubtitleOffset();
	}

	// One string per line, so a two-line cue stays two lines and the shadow is
	// drawn per line rather than around a block.
	function readActiveCues() {
		const cues = trackElement?.track?.activeCues;
		subtitleLines = cues?.length
			? [...cues].flatMap((cue) => cue.text.split('\n')).filter((line) => line.trim())
			: [];
	}

	// Subtitles sit on the last line of the picture, which is exactly where the
	// control bar appears. Measured in a real browser: the second line of a cue
	// was behind the scrub bar for as long as the controls were up.
	//
	// `line` counts from the bottom when negative, so this is the standard way
	// to say "four lines higher" without a vendor-prefixed pseudo-element. The
	// text lifts while the controls are up and drops back when they fade, which
	// is the behaviour every other player has trained people to expect.
	// How far the subtitle layer sits above the bottom of the video element, in
	// pixels. Recomputed whenever the picture, the furniture or the window moves.
	function placeSubtitles() {
		if (!video) return;
		const box = video.getBoundingClientRect();
		if (!box.height) return;

		// object-fit: contain means the element is not the picture. A 2.39:1 film
		// paints 1280x536 inside a 1280x720 element, so anchoring to the element
		// would float the text a sixth of the way up the frame.
		const ratio = video.videoWidth && video.videoHeight ? video.videoWidth / video.videoHeight : 0;
		const picture = ratio ? Math.min(box.height, box.width / ratio) : box.height;
		const letterbox = (box.height - picture) / 2;

		const bar = hidden
			? 0
			: (shell?.querySelector('.player-controls')?.getBoundingClientRect().height ?? 0);

		// Sit just inside the bottom of the picture, and step above the control
		// bar when it covers that. The gap is a share of the picture so it holds
		// at every size.
		subtitleBottom = Math.max(letterbox + picture * 0.04, bar + picture * 0.04);
	}

	function liftCues() {
		placeSubtitles();
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

		// `line` is a share of the *element*, and object-fit: contain means the
		// element is not the picture. A 2.39:1 film in a 16/9 window paints
		// 1280x536 inside 1280x720 and puts 92px of black above and below it,
		// measured on a real scope rip. Anchoring to the element floated the text
		// a sixth of the way up the frame, over the actors rather than under
		// them.
		//
		// So the resting place is the bottom of the picture, wherever letterboxing
		// puts it. A film with no bars is the same calculation with a zero bar,
		// which is why there is no special case for it.
		const ratio = video.videoWidth && video.videoHeight ? video.videoWidth / video.videoHeight : 0;
		const width = video.getBoundingClientRect().width;
		const picture = ratio && width ? Math.min(area, width / ratio) : area;
		const pictureBottom = area - (area - picture) / 2;

		// Whichever comes first: the bottom of the picture, or the top of the
		// control bar when it is up.
		const floor = Math.min(pictureBottom, area - bar);

		for (const cue of cues) {
			const lines = cue.text.split('\n').length;
			const top = floor - lines * lineHeight - gap;
			cue.snapToLines = false;
			cue.line = Math.max(0, Math.min(95, (top / area) * 100));
		}
	}

	$effect(() => {
		// Reading `hidden` is what subscribes this to the controls fading, and
		// reading the line count re-places the layer when a one-line cue is
		// replaced by a two-line one.
		void hidden;
		void subtitleLines.length;
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
		watchDecodePace();

		// The picture proves the file was probed. A file that had never been
		// inspected has only now revealed its tracks, so the menu is refilled
		// once -- the exact bug where a chooser stayed empty until a reload.
		if (!refreshedAfterProbe && info?.media_status !== 'ok') {
			refreshedAfterProbe = true;
			refreshTracks();
		}
	}

	// The half of the problem the check above cannot see.
	//
	// videoWidth === 0 catches a browser that decodes nothing. The worse case is
	// a browser that decodes *something*: on the maintainer's machine Chrome
	// reports 1920x804, readyState 4 and a picture, and then renders it far
	// slower than the film runs while the sound keeps perfect time. The symptom
	// people report is "the sound is way ahead of the picture"; the cause is that
	// the remux handed the browser a codec it cannot keep up with.
	//
	// No API will say so in advance. canPlayType answers "probably" for HEVC and
	// mediaCapabilities.decodingInfo answers smooth and power-efficient, both on
	// the machine where it does not work. So it is measured: presented frames per
	// second *of film*, which a stall in the network cannot flatter because both
	// halves of the ratio stop together.
	//
	// Ten is chosen to sit nowhere near either answer. Every real film runs at
	// 23.976 or more and a healthy decoder matches it; the broken case measures
	// close to zero. Deliberately not the source frame rate, which the server
	// does not store: a fixed floor needs no schema change to be right.
	const decodeFloorFPS = 10;
	let paceTimer;

	function watchDecodePace() {
		clearTimeout(paceTimer);
		// Only a risky remux is worth measuring. H.264 that already plays does not
		// need watching, and a transcode is this check's own answer.
		if (!isRemux || !info?.video_risky || forceTranscode) return;
		if (!video?.getVideoPlaybackQuality) return;

		const sample = () => ({
			frames: video.getVideoPlaybackQuality().totalVideoFrames,
			at: video.currentTime
		});

		const first = sample();
		paceTimer = setTimeout(() => {
			if (!video || paused || seeking || forceTranscode) return;
			const second = sample();
			const played = second.at - first.at;
			// Too little film went by to judge: buffering, or a pause the flags
			// above did not catch. Say nothing rather than guess.
			if (played < 1.5) return;

			const fps = (second.frames - first.frames) / played;
			if (fps >= decodeFloorFPS) return;

			codecPlayback.recordStruggle(info?.video_codec);
			if (info?.transcode?.available) {
				forceTranscode = true;
				start(position);
			}
		}, 2500);
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
	// A release title is written for a forum post, not for a menu: one real file
	// carried "DTS 5.1  70mm Theatrical v3 by hairy_hen", which pushed the two
	// facts that separate the tracks -- how many channels, which codec -- off the
	// end of the line. So the detail is built from the measurements, and the
	// title only joins it when it is short enough to be a name rather than a
	// paragraph.
	const titleIsShortEnough = (title) => !!title && title.length <= 24;

	// ffmpeg's channel layout is precise and unreadable: "5.1(side)" is 5.1.
	// The parenthetical says where the surrounds sit, which changes nothing for
	// somebody picking a track.
	function channelLabel(channels) {
		if (!channels) return null;
		const base = channels.replace(/\(.*\)/, '').trim();
		return t.player.tracks.channels[base] ?? base;
	}

	// The commentary track is the one people most need to tell apart, and the
	// container does not flag it in what the server sends. The title is the only
	// signal there is, so it is read for the word -- a heuristic, deliberately,
	// and it only changes a label.
	const looksLikeCommentary = (title) => /commentary|commentaire/i.test(title || '');

	function audioLabel(track, index) {
		const primary = track.language
			? languageName(track.language)
			: track.title || t.film.audio.unnamed(index + 1);
		const parts = looksLikeCommentary(track.title)
			? [t.player.tracks.commentary, channelLabel(track.channels)]
			: [
					channelLabel(track.channels),
					track.codec?.toUpperCase(),
					titleIsShortEnough(track.title) ? track.title : null
				];
		return { primary, detail: detailOf(parts, primary) };
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

<!-- The layer is placed in pixels against the picture, so a resize or a rotation
     has to re-measure it; going full screen is the case that matters most. -->
<svelte:window onkeydown={onKeydown} onresize={placeSubtitles} />

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

			<!--
				The subtitles, drawn here rather than by the browser. The track runs
				in `hidden` mode so the cues still fire and nothing is painted twice;
				placeSubtitles puts this layer against the bottom of the picture the
				video element actually paints, which is not the same as the bottom of
				the element on anything wider than 16/9.
			-->
			{#if subtitleLines.length}
				<div class="player-subtitles" style="bottom: {subtitleBottom}px" aria-hidden="true">
					{#each subtitleLines as line, index (index + line)}
						<span class="player-subtitle-line">{line}</span>
					{/each}
				</div>
			{/if}
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
						{#if previewTile}
							<span
								class="scrub-frame"
								style="width: {previewTile.width}px; height: {previewTile.height}px;
								       background-image: url({preview.sheet_url});
								       background-size: {previewTile.sheetWidth}px {previewTile.sheetHeight}px;
								       background-position: {previewTile.x}px {previewTile.y}px"
							></span>
						{/if}
						{formatTime(hoverRatio * duration)}
					</span>
				{/if}

				<!-- Loaded off-screen purely to measure it. The tiles themselves are
				     drawn as a background offset, which is one decode for the whole
				     strip instead of one image element per frame. -->
				{#if preview}
					<img
						src={preview.sheet_url}
						alt=""
						aria-hidden="true"
						class="scrub-sheet-probe"
						onload={onSheetLoaded}
					/>
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
							aria-expanded={tracksOpen}
							aria-haspopup="true"
						>
							<Icon name="settings" label={t.player.tracks.open} />
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
										{#each textSubtitles as track, index (track.id)}
											<li>
												{@render trackOption(
													subtitleLabel(track, index),
													subtitleTrackId === track.id,
													() => chooseSubtitle(track.id),
													false
												)}
											</li>
										{/each}
										{#if subtitleTrackId}
										<!--
											Only offered while a subtitle is actually on. A sync
											control under a film showing none is a setting for
											nothing, and the panel is already three sections long.
										-->
										<div class="track-offset">
											<span class="track-offset-label">{t.player.tracks.offset}</span>
											<div class="track-offset-controls">
												<button
													type="button"
													class="track-offset-step"
													onclick={() => nudgeSubtitles(-1)}
													aria-label={t.player.tracks.offsetEarlier}
												>
													−
												</button>
												<button
													type="button"
													class="track-offset-value"
													onclick={resetSubtitleOffset}
													disabled={subtitleOffset === 0}
													aria-label={t.player.tracks.offsetReset}
												>
													{t.player.tracks.offsetValue(subtitleOffset)}
												</button>
												<button
													type="button"
													class="track-offset-step"
													onclick={() => nudgeSubtitles(1)}
													aria-label={t.player.tracks.offsetLater}
												>
													+
												</button>
											</div>
										</div>
									{/if}

									{#each refusedSubtitles as track, index (track.id)}
											<li>
												<!--
													Only reached when the file has no text track at all.
													Decision 3 refuses to render a bitmap subtitle, and
													saying so beats a film that silently appears to have
													no subtitles.
												-->
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
