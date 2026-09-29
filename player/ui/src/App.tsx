import { MotionConfig } from 'motion/react';

import { FormEvent, MouseEvent as ReactMouseEvent, lazy, Suspense, useCallback, useEffect, useRef, useState } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';

import { TitleBar } from './components/TitleBar';
import { type TrackMenuHandle } from './components/TrackMenu';

import { catalogues, initialLanguage, storedLanguage, trackVocabulary } from './lib/catalogues.js';

import type { DiscoveredServer, Movie, PlayerStatus, Profile, QualityLadder, Season, Series, Server, Track, UpdateStatus } from './types';

import { clearPreviews } from './lib/previewCache';
import { useCatalogue } from './lib/useCatalogue';
import { useServerHealth } from './lib/useServerHealth';
import { invoke, listen, getAppWindow } from './lib/bridge';
const IDLE_MS = 3000;

/// How long the audio fallback explains itself.
///
/// The sentence is about 150 characters, which is read in six seconds; eight
/// leaves the margin. It used to stay until the player was closed, and the
/// maintainer met it sitting over the home screen's hero after a film had been
/// left - a notice outliving the state that raised it.
const AUDIO_NOTICE_MS = 8000;

/// How long the interface's own volume outranks the engine's.
///
/// Status frames arrive twice a second, so a frame read while a thumb is being
/// dragged is always older than the drag: without this window the thumb jumps
/// back to where it was and forward again, twice a second, for as long as
/// somebody holds it. Longer than one frame period, and short enough that a
/// volume changed from anywhere else lands promptly.
const VOLUME_SETTLE_MS = 900;

import type { Section } from './types';
import { Button } from './components/ui/button';
import { Library } from './components/Library';
import { PlaybackControls } from './components/PlaybackControls';
const SettingsModal = lazy(() => import('./components/SettingsModal'));
const ProfileDialog = lazy(() => import('./components/ProfileDialog'));
const PlayerHelp = lazy(() => import('./components/PlayerHelp'));

import { initialPreferences, type Preferences, type Notice } from './lib/preferences';
/// The sentence a refused connection earns.
///
/// The player answers a code rather than prose (decision 25, read the other way
/// round), and the two codes are not the same advice: an address the player
/// could not read is somebody's typo, while an address it read and got nothing
/// from is the case the old sentence described. Sending a typo to "check that
/// Theia is running" points at a machine that was never the problem.
///
/// Anything unrecognised - a code from a newer player, or a failure that is not
/// a refusal at all - keeps the sentence this screen always had.
function connectErrorKey(error: unknown) {
	const code = typeof error === 'string' ? error : '';
	if (code === 'address_unreadable') return 'connectionUnreadable';
	return 'connectionFailed';
}

export default function App() {
	const location = useLocation();
	const navigate = useNavigate();
	const [language, setLanguage] = useState(initialLanguage());
	const [preferences, setPreferences] = useState(initialPreferences);
	const catalogue = catalogues[language as keyof typeof catalogues];
	const words = trackVocabulary(language);
	const t = useCallback((key: string) => (catalogue as Record<string, string>)[key] ?? key, [catalogue]);
	const lastSection = useRef<Section>('home');
	const routeSection: Section | null = location.pathname.startsWith('/home')
		? 'home'
		: location.pathname.startsWith('/series')
			? 'series'
			: location.pathname.startsWith('/search')
				? 'search'
				: location.pathname.startsWith('/films')
					? 'films'
					: null;
	const section = routeSection ?? lastSection.current;
	const settingsOpen = location.pathname === '/settings';
	const profilesOpen = location.pathname === '/profiles';

	const [status, setStatus] = useState<PlayerStatus>({ ready: false });
	// The engine owns the volume, but its answer arrives twice a second - too
	// slow to drive a thumb that is being dragged, and always one frame behind
	// it. The interface keeps its own copy and reconciles: a frame is adopted
	// once the slider has not been touched for longer than a frame takes to
	// arrive, and ignored while it is under the pointer.
	const [volume, setVolume] = useState(1);
	const volumeTouched = useRef(0);
	const [notice, setNotice] = useState<Notice | null>(null);
	const [idle, setIdle] = useState(false);
	const [fullscreen, setFullscreen] = useState(false);
	const [maximized, setMaximized] = useState(false);
	const [server, setServer] = useState<Server | null>(null);
	const [updateStatus, setUpdateStatus] = useState<UpdateStatus | null>(null);
	const [updateBusy, setUpdateBusy] = useState(false);
	const [profileBusy, setProfileBusy] = useState(false);
	const [searchQuery, setSearchQuery] = useState('');
	const [selectedSeries, setSelectedSeries] = useState<Series | null>(null);
	const [selectedMovie, setSelectedMovie] = useState<Movie | null>(null);
	const [selectedSeason, setSelectedSeason] = useState<Season | null>(null);
	const [discovered, setDiscovered] = useState<DiscoveredServer[]>([]);
	const [address, setAddress] = useState('http://');
	const [busy, setBusy] = useState(false);
	const [booting, setBooting] = useState(true);
	const [returning, setReturning] = useState(false);
	const [errorKey, setErrorKey] = useState<string | null>(null);
	const [tracks, setTracks] = useState<Track[]>([]);
	// What the server says this machine can produce for the film playing. Asked
	// for when the menu opens, like the tracks: the ladder belongs to the file
	// and not to the moment, and whether an encoder is free changes while
	// somebody else's film is being converted.
	const [qualities, setQualities] = useState<QualityLadder | null>(null);
	const [trackMenuOpen, setTrackMenuOpen] = useState(false);
	const [focusInFurniture, setFocusInFurniture] = useState(false);
	const idleTimer = useRef<number | null>(null);
	const noticeTimer = useRef<number | null>(null);
	// The resize listener's debounce: one drag is one answer, asked when the
	// size stops moving.
	const resizeTimer = useRef<number | null>(null);
	const lastPointerDown = useRef(0);
	const trackButton = useRef<HTMLButtonElement>(null);
	const trackMenu = useRef<TrackMenuHandle>(null);

	useEffect(() => {
		if (routeSection) lastSection.current = routeSection;
	}, [routeSection]);

	useEffect(() => {
		document.documentElement.lang = language;
		document.documentElement.dataset.reducedMotion = preferences.reducedMotion ? 'true' : 'false';
	}, [language, preferences.reducedMotion]);

	const persistLanguage = useCallback((next: string) => {
		setLanguage(next);
		try {
			localStorage.setItem('theia.player.language', next);
		} catch {
			// The choice still lasts for this session.
		}
	}, []);

	const persistPreferences = useCallback((next: Preferences) => {
		setPreferences(next);
		try {
			localStorage.setItem('theia.player.preferences', JSON.stringify(next));
		} catch {
			// The choices still last for this session.
		}
	}, []);

	const switchLanguage = useCallback(() => {
		persistLanguage(language === 'fr' ? 'en' : 'fr');
	}, [language, persistLanguage]);

	const { movies, series, home, seriesHome, homeError, loadLibrary, loadHome, invalidate: invalidateCatalogue, clear: clearCatalogue } = useCatalogue(setErrorKey);
    const [savedChoice, setSavedChoice] = useState<string | null>(null);
    const connectionGeneration = useRef(0);
    const detailGeneration = useRef(0);
    const startupCancelled = useRef(false);
    const [helpOpen, setHelpOpen] = useState(false);
    const [historyBusy, setHistoryBusy] = useState(false);
    const cancelConnect = () => { startupCancelled.current = true; connectionGeneration.current++; detailGeneration.current++; invalidateCatalogue(); setBusy(false); setBooting(false); void invoke('player_cancel_connect'); };
	const refreshUpdateStatus = useCallback(async () => {
		try {
			setUpdateStatus(JSON.parse(await invoke<string>('player_update_status')) as UpdateStatus);
		} catch {
			setUpdateStatus(null);
		}
	}, []);

	const connect = useCallback(
		async (url: string, quiet = false) => {
			if (!url || (quiet && startupCancelled.current)) return false;
            const generation = ++connectionGeneration.current;
            clearCatalogue(); clearPreviews(); detailGeneration.current++;
			setBusy(true);
			if (!quiet) setErrorKey(null);
			try {
				const connected = JSON.parse(await invoke<string>('player_connect', { url })) as Server;
                if (generation !== connectionGeneration.current) return false;
				try {
					const rememberedProfile = Number(localStorage.getItem('theia.player.profile'));
					if (rememberedProfile && connected.profiles?.some((profile) => profile.id === rememberedProfile)) {
						await invoke('player_set_profile', { id: rememberedProfile });
						connected.profile = rememberedProfile;
					}
				} catch {
					// The server-selected profile remains authoritative when storage is unavailable.
				}
                if (generation !== connectionGeneration.current) return false;
				setServer(connected);
                setSavedChoice(connected.url);
				// The language this installation was set up in, which travels with
				// the server's identity. It is a starting point and never a
				// decision: a viewer who has chosen here keeps their choice.
				const served = connected.health?.language;
				if (!storedLanguage() && served && (catalogues as Record<string, unknown>)[served]) {
					setLanguage(served);
				}
				setAddress(connected.url);
				if (!quiet) {
					try {
						await invoke('player_remember_server', { url: connected.url });
						localStorage.removeItem('theia.player.server');
					} catch {
						setErrorKey('saveServerFailed');
					}
				}
				await Promise.all([loadLibrary(), loadHome(), refreshUpdateStatus()]);
				return true;
			} catch (error) {
                if (generation !== connectionGeneration.current) return false;
				setServer(null);
				if (!quiet) if (generation === connectionGeneration.current) setErrorKey(connectErrorKey(error));
				return false;
			} finally {
				if (generation === connectionGeneration.current) setBusy(false);
			}
		},
		[clearCatalogue, loadHome, loadLibrary, refreshUpdateStatus]
	);

    const waitingForConnection = !server && Boolean(savedChoice) && !startupCancelled.current;
    const recovered = useCallback(async () => {
        if (!server && savedChoice && !startupCancelled.current) { await connect(savedChoice, true); }
        else { await Promise.all([loadLibrary(), loadHome()]); void invoke('player_retry_progress'); }
    }, [connect, loadLibrary, loadHome, savedChoice, server]);
    const offline = useServerHealth(server?.url ?? (waitingForConnection ? savedChoice ?? undefined : undefined), recovered, waitingForConnection);

	const switchProfile = useCallback(async (id: number) => {
		if (!server || server.profile === id) {
			navigate(`/${lastSection.current}`);
			return;
		}
		setBusy(true);
		setErrorKey(null);
		try {
			detailGeneration.current++; clearCatalogue(); clearPreviews();
            await invoke('player_set_profile', { id });
			setServer((current) => current ? { ...current, profile: id } : current);
			setSelectedMovie(null);
			setSelectedSeries(null);
			setSelectedSeason(null);
			localStorage.setItem('theia.player.profile', String(id));
			await loadLibrary();
			await loadHome();
			navigate(`/${lastSection.current}`);
		} catch {
			setErrorKey('profileSwitchFailed');
		} finally {
			setBusy(false);
		}
	}, [clearCatalogue, loadHome, loadLibrary, navigate, server]);

	const applyProfile = useCallback((profile: Profile) => {
		setServer((current) => current ? { ...current, profiles: current.profiles.map((entry) => entry.id === profile.id ? profile : entry) } : current);
		return profile;
	}, []);

	const renameProfile = useCallback(async (id: number, name: string) => {
		setProfileBusy(true);
		try {
			return applyProfile(JSON.parse(await invoke<string>('player_profile_rename', { id, name })) as Profile);
		} finally {
			setProfileBusy(false);
		}
	}, [applyProfile]);

	const setProfileAvatar = useCallback(async (id: number, file: File) => {
		setProfileBusy(true);
		try {
			const data = Array.from(new Uint8Array(await file.arrayBuffer()));
			return applyProfile(JSON.parse(await invoke<string>('player_profile_set_avatar', { id, contentType: file.type || 'application/octet-stream', data })) as Profile);
		} finally {
			setProfileBusy(false);
		}
	}, [applyProfile]);

	const clearProfileAvatar = useCallback(async (id: number) => {
		setProfileBusy(true);
		try {
			return applyProfile(JSON.parse(await invoke<string>('player_profile_clear_avatar', { id })) as Profile);
		} finally {
			setProfileBusy(false);
		}
	}, [applyProfile]);

	const checkUpdate = useCallback(async () => {
		setUpdateBusy(true);
		try {
			setUpdateStatus(JSON.parse(await invoke<string>('player_update_check')) as UpdateStatus);
		} finally {
			setUpdateBusy(false);
		}
	}, []);

	const applyUpdate = useCallback(async () => {
		setUpdateBusy(true);
		try {
			setUpdateStatus(JSON.parse(await invoke<string>('player_update_apply')) as UpdateStatus);
		} finally {
			setUpdateBusy(false);
		}
	}, []);

	const findServers = useCallback(
		async (quiet = false) => {
            const generation = ++connectionGeneration.current;
			setBusy(true);
			if (!quiet) setErrorKey(null);
			try {
				const found = JSON.parse(await invoke<string>('player_discover')) as DiscoveredServer[];
                if (generation !== connectionGeneration.current || (quiet && startupCancelled.current)) return false;
				setDiscovered(found);
				if (found.length === 1) return await connect(found[0].url, quiet);
				return false;
			} catch {
                if (generation !== connectionGeneration.current) return false;
				setDiscovered([]);
				return false;
			} finally {
                if (generation === connectionGeneration.current) setBusy(false);
			}
		},
		[connect]
	);

	useEffect(() => {
		let cancelled = false;
		(async () => {
			setBooting(true);
			setErrorKey(null);
			try {
				// A connection the player already holds wins. The command line named
				// it - `--server` exists so the client can be exercised without a
				// click - and adopting the machine's own install over it made the
				// flag a lie: the window talked to a server nobody asked for, and
				// nothing on screen said so.
				try {
					const current = await invoke<string | null>('player_current_server');
					if (current) {
						const url = (JSON.parse(current) as { url?: string }).url;
						if (url && (await connect(url, true))) return;
					}
				} catch {
					// Nothing connected yet, which is the ordinary start.
				}
				let remembered: string | null = null;
				try { remembered = await invoke<string | null>('player_saved_server'); } catch { /* First launch. */ }
				if (!remembered) {
					try { remembered = localStorage.getItem('theia.player.server'); } catch { /* Legacy storage unavailable. */ }
				}
				if (cancelled || startupCancelled.current) return;
				if (remembered) {
                    setSavedChoice(remembered);
					setAddress(remembered);
					if (await connect(remembered, true)) {
						// Migrate a legacy choice to the app's durable configuration.
						try { await invoke('player_remember_server', { url: remembered }); localStorage.removeItem('theia.player.server'); } catch { /* Retry next launch. */ }
						return;
					}
					// A chosen server stays chosen even while it is offline. The viewer
					// can change it from the connect screen; discovery cannot replace it.
                    if (cancelled || startupCancelled.current) return;
					setErrorKey('connectionFailed');
					return;
				}
				let local: string | null = null;
				try {
					local = await invoke<string | null>('player_local_server');
				} catch {
					// Player-only installs have no local server to prepare.
				}
				if (cancelled || startupCancelled.current) return;
				if (local && (await connect(local, true))) return;
				await findServers(true);
			} finally {
				if (!cancelled) setBooting(false);
			}
		})();
		return () => {
			cancelled = true;
		};
	}, [connect, findServers]);

	/// A notice replaces the one before it, and only one of them is ever on the
	/// clock: two timers would fight over the same screen.
	const clearNotice = useCallback(() => {
		if (noticeTimer.current !== null) window.clearTimeout(noticeTimer.current);
		noticeTimer.current = null;
		setNotice(null);
	}, []);

	const showNotice = useCallback((next: Notice) => {
		if (noticeTimer.current !== null) window.clearTimeout(noticeTimer.current);
		noticeTimer.current = null;
		setNotice(next);
		if (next.dwell === null) return;
		noticeTimer.current = window.setTimeout(() => {
			noticeTimer.current = null;
			setNotice(null);
		}, next.dwell);
	}, []);

	// What the interface measures about itself, so that "fluid" can be a number
	// rather than an opinion.
	//
	// Three numbers, and the first two are the ones that matter:
	//
	// - the worst frame interval seen while somebody was *doing* something. That
	//   is what a viewer feels, and it is measured by drawing frames rather than
	//   by asking a platform whether it felt slow: the first version of this
	//   watched `longtask` entries, which stayed at zero through a two-hundred-
	//   and-fifty-card render and through scrolling the grid. An instrument that
	//   cannot fail is not an instrument.
	// - how many frames in the window took longer than 32 ms, which is two
	//   frames at 60 Hz: one slow frame is a hiccup, a run of them is a stutter.
	// A third number used to live here - the longest gap between status frames -
	// and it was removed on the same day it was written, because the emission
	// became event-driven and a gap then measures the heartbeat rather than the
	// thread. A number whose meaning depends on another decision is a trap.
	//
	// The sampling is armed by a gesture and stops by itself. A permanent
	// requestAnimationFrame loop on a transparent page above a film would keep
	// the compositor producing frames nobody asked for, which is an optimisation
	// that costs - so the loop only runs for three seconds after a pointer, a
	// key or a scroll, which is exactly when a stutter would be felt.
	const osdMeasure = useRef({ worstFrame: 0, slowFrames: 0, frames: 0, resizeEvents: 0, lastPaintAt: 0, samplingUntil: 0, sampling: false, arm: () => {} });
	useEffect(() => {
		const measure = osdMeasure.current;
		const sample = (now: number) => {
			if (measure.lastPaintAt > 0) {
				const interval = now - measure.lastPaintAt;
				measure.worstFrame = Math.max(measure.worstFrame, interval);
				if (interval > 32) measure.slowFrames += 1;
			}
			measure.lastPaintAt = now;
			measure.frames += 1;
			if (now < measure.samplingUntil) {
				requestAnimationFrame(sample);
			} else {
				measure.sampling = false;
				measure.lastPaintAt = 0;
			}
		};
		const arm = () => {
			measure.samplingUntil = performance.now() + 3000;
			if (!measure.sampling) {
				measure.sampling = true;
				measure.lastPaintAt = 0;
				requestAnimationFrame(sample);
			}
		};
		const events: Array<keyof WindowEventMap> = ['pointerdown', 'pointermove', 'keydown', 'wheel', 'scroll'];
		for (const name of events) window.addEventListener(name, arm, { passive: true, capture: true });
		// A window resize is not a page gesture, so the listener above never sees
		// one - and a resize is exactly when a stutter is felt. The resize
		// listener calls this.
		measure.arm = arm;
		const tick = window.setInterval(() => {
			// Nothing measured, nothing sent. A player nobody is touching draws
			// no frames of its own to measure, and reporting zeroes once a
			// second would be a command per second for a number that says
			// "idle" - which is what the frame's own gap already says.
			if (measure.frames === 0) {
				measure.worstFrame = 0;
				measure.slowFrames = 0;
				return;
			}
			// `frames` is what keeps this honest: a report of zero milliseconds
			// is a measurement of a smooth window, and a report of zero frames
			// is an instrument that never ran. The two are not the same answer
			// and the first version of this could not tell them apart.
			void invoke('player_osd_stats', {
				worstFrameMs: Math.round(measure.worstFrame * 10) / 10,
				slowFrames: measure.slowFrames,
				frames: measure.frames,
				resizeEvents: measure.resizeEvents
			}).catch(() => {});
			measure.worstFrame = 0;
			measure.slowFrames = 0;
			measure.frames = 0;
			// `resizeEvents` is deliberately not reset: it is the count since the
			// page loaded, because the first version counted per report and I
			// read it a second too late and concluded the listener never fired.
		}, 1000);
		return () => {
			for (const name of events) window.removeEventListener(name, arm, { capture: true });
			window.clearInterval(tick);
		};
	}, []);

	useEffect(() => {
		const cleanups: Array<() => void> = [];
		let disposed = false;
		listen('player-status', (event) => {
			try {
				const frame = JSON.parse(String(event.payload)) as PlayerStatus;
				// While a film plays the interface must keep up, and the sampler
				// has no gesture to arm it: a viewer watching a film touches
				// nothing. The compositor is awake anyway then, so the loop
				// costs nothing it was not already costing.
				if (frame.media && !frame.pause) osdMeasure.current.arm();
				setStatus(frame);
				if (typeof frame.volume === 'number' && Date.now() - volumeTouched.current > VOLUME_SETTLE_MS) setVolume(frame.volume);
			} catch {
				// A malformed frame is ignored; the next one arrives in 500 ms.
			}
		}).then((cleanup) => (disposed ? cleanup() : cleanups.push(cleanup)));
		listen('player-event', (event) => {
			try {
				const value = JSON.parse(String(event.payload));
				if (value.kind === 'audio' && value.mode === 'pcm')
					showNotice({ key: 'audioFallback', label: 'audioFallbackLabel', dwell: AUDIO_NOTICE_MS });
				if (value.kind === 'engine' && value.state === 'unavailable')
					showNotice({ key: 'engineUnavailable', label: 'engineUnavailableLabel', dwell: null });
			} catch {
				// Same contract as status frames.
			}
		}).then((cleanup) => (disposed ? cleanup() : cleanups.push(cleanup)));
		// The window's own resize, asked of the window rather than of the event
		// bus: the maintainer's report of 24 September 2026 - "quand je prends
		// les flèches pour rétrécir ou agrandir la fenêtre, ça lag de malade" -
		// was measured with the sampler armed and counted, and the count came
		// back zero: `tauri://resize` was never arriving, so this listener had
		// been dead since it was written, and the state it maintains could never
		// change. Reading that state costs two calls across the bridge, so it is
		// asked once the size has stopped moving rather than per pixel of drag.
		const appWindow = getAppWindow();
		// Guarded rather than chained: a shell whose API has no `onResized` must
		// leave the listener absent, not throw inside this effect - which is how
		// the first version of it would have taken the status and event
		// registrations down with it.
		const resized = appWindow
			?.onResized?.(() => {
				osdMeasure.current.arm();
				osdMeasure.current.resizeEvents += 1;
				// Measured on 24 September 2026, with a scripted drag of forty steps:
				// **the page costs nothing here.** With this whole callback reduced
				// to a no-op the window's own event gap stayed at 66 ms, and with the
				// engine not loaded at all it fell to 9.9 - so what lags is mpv
				// reconfiguring its surface per step, not anything on this side. What
				// is kept is the part that must not be lost: the measurement, and the
				// one answer a drag needs.
				document.documentElement.dataset.resizing = 'true';
				if (resizeTimer.current !== null) window.clearTimeout(resizeTimer.current);
				resizeTimer.current = window.setTimeout(() => {
					delete document.documentElement.dataset.resizing;
					void (async () => {
						try {
							setFullscreen(Boolean(await appWindow.isFullscreen()));
							setMaximized(Boolean(await appWindow.isMaximized()));
						} catch {
							// Keep the last known state.
						}
					})();
				}, 150);
			});
		resized?.then((cleanup: () => void) => (disposed ? cleanup() : cleanups.push(cleanup)));
		return () => {
			disposed = true;
			cleanups.forEach((cleanup) => cleanup());
			if (resizeTimer.current !== null) window.clearTimeout(resizeTimer.current);
			if (noticeTimer.current !== null) window.clearTimeout(noticeTimer.current);
		};
	}, [showNotice]);

	// What the settings sheet decided, handed to the engine.
	//
	// Sent on mount and whenever a choice changes. `status.ready` describes a
	// loaded film, not engine readiness: gating on it applied saved languages
	// only after mpv had already selected tracks for the first film or episode.
	useEffect(() => {
		void invoke('player_set_playback', { prefs: JSON.stringify(preferences.playback) }).catch(() => {});
	}, [preferences.playback]);

	const wake = useCallback(() => {
		if (idleTimer.current !== null) window.clearTimeout(idleTimer.current);
		idleTimer.current = null;
		setIdle(false);
		if (
			!preferences.autoHideControls ||
			!status.media ||
			status.pause ||
			!status.ready ||
			trackMenuOpen || helpOpen ||
			focusInFurniture
		)
			return;
		idleTimer.current = window.setTimeout(() => setIdle(true), IDLE_MS);
	}, [helpOpen, focusInFurniture, preferences.autoHideControls, status.media, status.pause, status.ready, trackMenuOpen]);

	useEffect(() => {
		wake();
		return () => {
			if (idleTimer.current !== null) window.clearTimeout(idleTimer.current);
		};
	}, [wake]);

	// A notice belongs to the film that raised it, so a new film starts on a
	// clean screen: the last one's sound is not this one's.

    const editEpisodeHistory = async (id: number, watched: boolean) => {
        const generation = detailGeneration.current;
        setHistoryBusy(true); setErrorKey(null);
        try { await invoke('player_episode_history', { id, watched }); await Promise.all([loadLibrary(), loadHome()]);
            if (selectedSeries && selectedSeason) { const [detail, season] = await Promise.all([invoke<string>('player_series_detail', { id: selectedSeries.id }), invoke<string>('player_season', { seriesId: selectedSeries.id, seasonNumber: selectedSeason.season_number })]); if (generation === detailGeneration.current) { setSelectedSeries(JSON.parse(detail)); setSelectedSeason(JSON.parse(season)); } }
        } catch { setErrorKey('historyFailed'); } finally { setHistoryBusy(false); }
    };
	const startPlayback = async (command: 'player_play' | 'player_play_episode', id: number, failure: string) => {
		clearNotice();
		try {
			await invoke('player_set_playback', { prefs: JSON.stringify(preferences.playback) }).catch(() => {});
			await invoke(command, { id });
		} catch {
			setErrorKey(failure);
		}
	};
	const playMovie = (id: number) => startPlayback('player_play', id, 'playFailed');
	const playEpisode = (id: number) => startPlayback('player_play_episode', id, 'episodeFailed');
	const openMovie = async (id: number) => {
        const generation = ++detailGeneration.current;
		setErrorKey(null);
		try {
			const movie = JSON.parse(await invoke<string>('player_movie_detail', { id })) as Movie;
            if (generation !== detailGeneration.current) return;
            setSelectedMovie(movie);
			setSelectedSeries(null);
			setSelectedSeason(null);
		} catch {
            if (generation !== detailGeneration.current) return;
			setErrorKey('movieFailed');
		}
	};
	const openSeries = async (id: number) => {
        const generation = ++detailGeneration.current;
		setErrorKey(null);
		try {
			const detail = JSON.parse(await invoke<string>('player_series_detail', { id })) as Series;
            if (generation !== detailGeneration.current) return;
			setSelectedSeries(detail);
			const resume = detail.resume_episode ?? seriesHome?.continue_watching.find((episode) => episode.series_id === id) ?? detail.next_unwatched;
			const first = detail.seasons?.find((season) => season.season_number === resume?.season_number) ?? detail.seasons?.[0];
			const season = (
				first
					? (JSON.parse(
							await invoke<string>('player_season', { seriesId: detail.id, seasonNumber: first.season_number })
						) as Season)
					: null
			);
            if (generation !== detailGeneration.current) return;
            setSelectedSeason(season);
		} catch {
            if (generation !== detailGeneration.current) return;
			setSelectedSeries(null);
			setSelectedSeason(null);
			setErrorKey('seriesFailed');
		}
	};
	const openSeason = async (seasonNumber: number) => {
        const generation = ++detailGeneration.current;
		if (!selectedSeries) return;
		try {
			const season = JSON.parse(await invoke<string>('player_season', { seriesId: selectedSeries.id, seasonNumber })) as Season;
            if (generation === detailGeneration.current) setSelectedSeason(season);
		} catch {
            if (generation !== detailGeneration.current) return;
			setErrorKey('seriesFailed');
		}
	};

	const returnToLibrary = useCallback(async () => {
		if (!status.media || returning) return;
        const generation = detailGeneration.current;
		setReturning(true);
		setTrackMenuOpen(false);
		clearNotice();
		try {
			await invoke('player_stop');
			setStatus((current) => ({ ...current, media: null, title: null, pos: null, duration: null, pause: false }));
			await Promise.all([loadLibrary(), loadHome()]);
			if (selectedSeries && selectedSeason) {
				try {
					const [detail, season] = await Promise.all([
						invoke<string>('player_series_detail', { id: selectedSeries.id }),
						invoke<string>('player_season', { seriesId: selectedSeries.id, seasonNumber: selectedSeason.season_number }),
					]);
					if (generation === detailGeneration.current) { setSelectedSeries(JSON.parse(detail) as Series);
					setSelectedSeason(JSON.parse(season) as Season); }
				} catch { setErrorKey('seriesFailed'); }
			}
		} catch {
			setErrorKey('stopFailed');
		} finally {
			setReturning(false);
		}
	}, [clearNotice, loadHome, loadLibrary, returning, selectedSeason, selectedSeries, status.media]);

	const refreshTracks = async () => {
		try {
			setTracks(JSON.parse(await invoke<string>('player_tracks')) as Track[]);
		} catch {
			setTracks([]);
		}
		try {
			setQualities(JSON.parse(await invoke<string>('player_qualities')) as QualityLadder);
		} catch {
			// No ladder is not an error: a machine with no encoder, or a server
			// too old to publish one, is a menu with no quality tab rather than a
			// menu that fails to open.
			setQualities(null);
		}
	};
	const toggleTracks = async () => {
		wake();
		setTrackMenuOpen((open) => !open);
		if (!trackMenuOpen) await refreshTracks();
	};
	const pickTrack = async (kind: 'audio' | 'sub', id: number | null) => {
		try {
			await invoke('player_set_track', { kind, id });
			await refreshTracks();
		} catch {
			setErrorKey('trackFailed');
		}
	};
	/// A rung is a different stream, so this is a reload: the film comes back at
	/// the second it left. The menu stays open and the tick moves when the new
	/// stream answers - the bar is drawn from session state on the Rust side, so
	/// it never disappears while the pipe is being opened.
	const pickQuality = async (height: number | null) => {
		wake();
		try {
			await invoke('player_set_quality', { height });
			await refreshTracks();
		} catch {
			setErrorKey('trackFailed');
		}
	};

	const toggle = useCallback(() => {
		wake();
		void invoke('player_toggle_pause');
	}, [wake]);
	const seek = useCallback(
		(seconds: number) => {
			wake();
			void invoke('player_seek', { seconds, mode: 'relative' });
		},
		[wake]
	);
	const setVolumeTo = useCallback(
		(next: number) => {
			// Rounded to the slider's own step, so the value on the wire is the
			// value on screen: a keyboard step of 0.1 lands on 0.5000000000000001
			// otherwise, and every write would look like a different volume.
			const value = Math.max(0, Math.min(1, Math.round(next * 100) / 100));
			volumeTouched.current = Date.now();
			setVolume(value);
			wake();
			void invoke('player_set_volume', { volume: value });
		},
		[wake]
	);
	const toggleMute = () => {
		wake();
		// Unmuting a slider sitting at zero would bring back a silent film, so
		// the button restores a level instead - the rule the web player applies
		// to its own button, and the reason the two agree when somebody mutes,
		// drags to nothing, then presses play on the sound again.
		if (status.mute && volume === 0) {
			setVolumeTo(0.5);
			return;
		}
		void invoke('player_set_muted', { muted: !status.mute });
	};
	const setFullscreenState = useCallback(async (next: boolean) => {
		const appWindow = getAppWindow();
		if (!appWindow) return;
		await appWindow.setFullscreen(next);
		setFullscreen(Boolean(await appWindow.isFullscreen()));
	}, []);
	const toggleFullscreen = useCallback(async () => {
		wake();
		const appWindow = getAppWindow();
		if (!appWindow) return;
		await setFullscreenState(!(await appWindow.isFullscreen()));
	}, [setFullscreenState, wake]);

	useEffect(() => {
		const onKey = (event: KeyboardEvent) => {
			if (event.key === 'Escape' && helpOpen) { setHelpOpen(false); return; }
            if (settingsOpen || profilesOpen || helpOpen) return;
            if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'f') { event.preventDefault(); navigate('/search'); setTimeout(() => document.querySelector<HTMLInputElement>('.search-field input')?.focus(), 0); return; }
			const editable = (event.target as HTMLElement | null)?.matches('input, textarea, select, [contenteditable="true"]');
			if (editable) {
				if (event.key === 'Escape') (event.target as HTMLElement).blur();
				return;
			}
			if (event.key === '?') { event.preventDefault(); setHelpOpen(true); return; }
            if ((event.target as HTMLElement | null)?.closest('.scrub')) return;
			if (trackMenuOpen && event.key.startsWith('Arrow')) {
				event.preventDefault();
				// Left and right change tab, up and down walk the rows of the tab
				// that is open - the strip is above the rows, so the vertical axis
				// is the one that moves within it.
				if (event.key === 'ArrowLeft' || event.key === 'ArrowRight')
					trackMenu.current?.moveTab(event.key === 'ArrowRight' ? 1 : -1);
				else trackMenu.current?.moveFocus(event.key === 'ArrowDown' ? 1 : -1);
				return;
			}
			if (event.key === ' ' || event.key === 'k') {
				if (event.key === ' ' && (event.target as HTMLElement | null)?.closest('button, [role="slider"]')) return;
				event.preventDefault();
				toggle();
			} else if (status.media && event.key === 'ArrowLeft') seek(-10);
			// Arrows seek only while a film is on. In the library they belong
			// to whatever row has focus, which handles them itself; without
			// this gate the global handler would also fire a seek into the
			// void on every arrow press.
			else if (status.media && event.key === 'ArrowRight') seek(10);
			// The vertical axis is the volume, which is what it does on a TV
			// remote and in the web player. The scrub bar owns these two keys
			// while it has focus - the handler returns early there - and in the
			// library they belong to whichever row has focus.
			else if (status.media && (event.key === 'ArrowUp' || event.key === 'ArrowDown'))
				setVolumeTo(volume + (event.key === 'ArrowUp' ? 0.1 : -0.1));
			else if (event.key === 'f') void toggleFullscreen();
			else if (event.key === 'm') toggleMute();
			else if (event.key === 'c') void toggleTracks();
			else if (event.key === 'l') switchLanguage();
			else if (event.key === 'Escape') {
				if (trackMenuOpen) {
					setTrackMenuOpen(false);
					trackButton.current?.focus();
				} else if (fullscreen) void setFullscreenState(false);
				else if (status.media) void returnToLibrary();
				else if (selectedMovie) setSelectedMovie(null);
				else if (selectedSeries) {
					setSelectedSeries(null);
					setSelectedSeason(null);
				}
			} else wake();
		};
		window.addEventListener('keydown', onKey);
		return () => window.removeEventListener('keydown', onKey);
	}, [helpOpen, navigate, fullscreen, profilesOpen, returnToLibrary, seek, selectedMovie, selectedSeries, setFullscreenState, setVolumeTo, settingsOpen, status.media, switchLanguage, toggle, toggleFullscreen, trackMenuOpen, volume, wake]);

	const seconds = Number(status.pos) || 0;
	const duration = Number(status.duration) || 0;
	const progress = duration > 0 ? Math.min(1, seconds / duration) : 0;
	const submit = (event: FormEvent) => {
		event.preventDefault();
		void connect(address);
	};
	const backgroundClick = (event: ReactMouseEvent<HTMLDivElement>) => {
		if (!status.media) return;
		if ((event.target as HTMLElement).closest('button, input, .scrub, .title-bar, .notice, .track-menu')) return;
		if (trackMenuOpen) setTrackMenuOpen(false);
		else toggle();
	};
	const moveToSection = (next: Section) => {
        detailGeneration.current++;
		setSelectedMovie(null);
		setSelectedSeries(null);
		setSelectedSeason(null);
		navigate(`/${next}`);
	};
	const openSettings = () => {
		lastSection.current = section;
		navigate('/settings');
	};
	const openProfiles = () => {
		lastSection.current = section;
		navigate('/profiles');
	};
	const closeOverlay = () => navigate(`/${lastSection.current}`);
	const changeServer = async () => {
        connectionGeneration.current++; detailGeneration.current++; invalidateCatalogue();
		try {
			await invoke('player_disconnect');
			try { localStorage.removeItem('theia.player.server'); localStorage.removeItem('theia.player.profile'); } catch { /* Native choice is already cleared. */ }
			setServer(null);
			setAddress('');
            setSavedChoice(null);
			clearCatalogue(); clearPreviews();
			setSelectedSeries(null);
			setSelectedMovie(null);
			setSelectedSeason(null);
			setErrorKey(null);
			closeOverlay();
		} catch {
			setErrorKey('disconnectFailed');
		}
	};

	return (
		<MotionConfig reducedMotion={preferences.reducedMotion ? 'always' : 'user'}>
			<div
				className={`osd ${status.media ? '' : 'osd--library'}`}
				data-section={section}
				data-idle={idle}
				data-maximized={maximized}
				data-fullscreen={fullscreen}
				onMouseMove={wake}
				onClick={backgroundClick}
				onPointerDown={() => {
					lastPointerDown.current = performance.now();
				}}
				onFocusCapture={(event) => {
					const fromKeyboard = performance.now() - lastPointerDown.current > 700;
					setFocusInFurniture(fromKeyboard && Boolean((event.target as HTMLElement).closest('.controls, .title-bar, .notice')));
				}}
				onBlurCapture={() => setFocusInFurniture(false)}
			>
				{status.media && (
					<>
						<div className="scrim-top" />
						<div className="scrim-bottom" />
					</>
				)}
				{offline && <p className="connection-status" role="status">{t('serverOffline')}</p>}
                {(status.progressPending ?? 0) > 0 && <p className="save-status" role="status">{t('progressPending')}</p>}
                {status.progressStorageFailed && <p className="save-status" role="alert">{t('progressStorageFailed')}</p>}
                {status.media && status.nextEpisodeId && (status.ended || (status.duration ?? Infinity) - (status.pos ?? 0) <= 45) && <div className="next-episode"><Button onClick={() => playEpisode(status.nextEpisodeId!)}>{t('nextEpisode')}</Button></div>}
                <TitleBar
					title={status.title}
					playing={Boolean(status.media)}
					maximized={maximized || fullscreen}
					language={language}
					labels={{ back: t('backToLibrary'), minimize: t('minimize'), maximize: t('maximize'), restore: t('restore'), close: t('close') }}
					onBack={() => void returnToLibrary()}
					onLanguage={switchLanguage}
					onMinimize={() => getAppWindow()?.minimize()}
					onMaximize={async () => {
						const appWindow = getAppWindow();
						if (await appWindow?.isFullscreen()) {
							await setFullscreenState(false);
							return;
						}
						await appWindow?.toggleMaximize();
						try {
							setMaximized(Boolean(await appWindow?.isMaximized()));
						} catch {
							setMaximized((value) => !value);
						}
					}}
					onClose={() => getAppWindow()?.close()}
				/>

				{notice && <div className="notice" role="status"><span className="label">{t(notice.label)}</span>{t(notice.key)}</div>}
				{status.media && !status.ready && <div className="notice" role="status"><span className="label">{t('loading')}</span></div>}

				{!status.media && (
					<Library
						server={server} booting={booting} busy={busy} address={address} discovered={discovered}
						movies={movies} series={series} section={section} settingsOpen={settingsOpen} selectedMovie={selectedMovie} selectedSeries={selectedSeries} selectedSeason={selectedSeason}
						profilesOpen={profilesOpen} updateStatus={updateStatus} searchQuery={searchQuery}
						home={home} seriesHome={seriesHome} homeError={homeError} language={language}
						errorKey={errorKey} reducedMotion={preferences.reducedMotion} t={t}
						onCancel={cancelConnect} historyBusy={historyBusy} onHistory={editEpisodeHistory} onHelp={() => setHelpOpen(true)} onAddress={(value) => { startupCancelled.current = true; setAddress(value); }} onSubmit={submit} onFind={() => void findServers()} onConnect={(url) => void connect(url)}
						onSection={moveToSection} onSettings={openSettings} onProfiles={openProfiles} onSearchQuery={setSearchQuery} onMovie={openMovie} onPlayMovie={playMovie} onBackMovie={() => { detailGeneration.current++; setSelectedMovie(null); }} onSeries={openSeries}
						onEpisode={playEpisode} onSeason={openSeason}
						onBackSeries={() => { detailGeneration.current++; setSelectedSeries(null); setSelectedSeason(null); }}
					/>
				)}

				<Suspense fallback={null}>
                {settingsOpen && <SettingsModal
					open={settingsOpen && !status.media} language={language} preferences={preferences} server={server} updateStatus={updateStatus} updateBusy={updateBusy} t={t}
					onClose={closeOverlay} onChangeServer={() => void changeServer()} onCheckUpdate={() => void checkUpdate()} onApplyUpdate={() => void applyUpdate()}
					onSave={(nextLanguage, nextPreferences) => {
						persistLanguage(nextLanguage);
						persistPreferences(nextPreferences);
						closeOverlay();
					}}
				/>}

				{helpOpen && <PlayerHelp open={helpOpen} t={t} onClose={() => setHelpOpen(false)} />}
                {profilesOpen && <ProfileDialog
					open={profilesOpen && !status.media}
					profiles={server?.profiles ?? []}
					activeProfile={server?.profile ?? null}
					serverURL={server?.url ?? ''}
					busy={busy || profileBusy}
					t={t}
					onClose={closeOverlay}
					onSelect={(id) => void switchProfile(id)}
					onRename={renameProfile}
					onSetAvatar={setProfileAvatar}
					onClearAvatar={clearProfileAvatar}
				/>
}
                </Suspense>

				<PlaybackControls
					visible={Boolean(status.media)} status={status} seconds={seconds} duration={duration} progress={progress} words={words}
					volume={volume} onVolume={setVolumeTo}
					fullscreen={fullscreen} language={language} tracks={tracks} trackMenuOpen={trackMenuOpen}
					qualities={qualities} currentQuality={status.quality ?? null}
					trackButton={trackButton} trackMenu={trackMenu} t={t} onToggle={toggle} onSeek={seek}
					onPickQuality={(height) => void pickQuality(height)}
					onSeekAbsolute={(value) => { wake(); void invoke('player_seek', { seconds: value, mode: 'absolute' }); }}
					onMute={toggleMute} onTracks={() => void toggleTracks()} onPickTrack={pickTrack}
					onLanguage={switchLanguage} onFullscreen={() => void toggleFullscreen()}
				/>
			</div>
		</MotionConfig>
	);
}
