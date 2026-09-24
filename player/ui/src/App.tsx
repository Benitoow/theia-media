import { AnimatePresence, MotionConfig, motion } from 'motion/react';
import {
	ArrowLeft,
	ArrowRight,
	Check,
	ChevronLeft,
	ChevronRight,
	Clapperboard,
	Clock,
	Cog,
	Copy,
	Download,
	Film,
	House,
	ImagePlus,
	Languages,
	ListVideo,
	type LucideIcon,
	Maximize2,
	Minimize2,
	Minus,
	MonitorPlay,
	Palette,
	Pause,
	Percent,
	Pencil,
	PenLine,
	Play,
	Plus,
	RotateCcw,
	RotateCw,
	Search,
	Server as ServerIcon,
	Settings2,
	Square,
	BarChart3,
	Bold,
	Captions,
	Tv,
	Type,
	UnfoldVertical,
	UserRound,
	Volume1,
	Volume2,
	VolumeX,
	X,
} from 'lucide-react';
import {
	Children,
	FormEvent,
	KeyboardEvent as ReactKeyboardEvent,
	MouseEvent as ReactMouseEvent,
	useCallback,
	useEffect,
	useRef,
	useState,
} from 'react';
import { useLocation, useNavigate } from 'react-router-dom';

import { MediaCard } from './components/MediaCard';
import { TitleBar } from './components/TitleBar';
import { TrackMenu, type TrackMenuHandle } from './components/TrackMenu';
import { Button } from './components/ui/button';
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogTitle } from './components/ui/dialog';
import PartitionBar, {
	PartitionBarSegment,
	PartitionBarSegmentTitle,
	PartitionBarSegmentValue,
} from './components/ui/partition-bar';
import { Switch } from './components/ui/switch';
import notFoundArt from './assets/media-not-found.png';
import { catalogues, initialLanguage, storedLanguage, trackVocabulary } from './lib/catalogues.js';
import { artworkCandidates, displayTitle, displayYear, imageURL } from './lib/tmdb';
import { formatRuntime } from './lib/utils';
import { PLAYBACK_DEFAULTS } from './types';
import type { DiscoveredServer, Home, HomeRow, Movie, PlaybackPreferences, PlayerStatus, Profile, QualityLadder, Season, Series, SeriesHome, Server, SubtitleStyle, Track, TrackVocabulary, UpdateStatus, WatchStats } from './types';

const invoke = async <T,>(command: string, args?: Record<string, unknown>): Promise<T> => {
	const call = window.__TAURI__?.core?.invoke;
	if (!call) return undefined as T;
	return call<T>(command, args);
};
const listen = window.__TAURI__?.event?.listen ?? (async () => () => {});
const getAppWindow = () => window.__TAURI__?.window?.getCurrentWindow?.();
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

type Section = 'home' | 'films' | 'series' | 'search';
type Preferences = { reducedMotion: boolean; autoHideControls: boolean; playback: PlaybackPreferences };

/// The two sliders' own ranges, which the stored-value reader and the controls
/// both use: a number outside them is not a preference, it is a bad file.
const SUBTITLE_SIZE = { min: 24, max: 72, step: 2 };
const SUBTITLE_HEIGHT = { min: 24, max: 200, step: 4 };

/// The colours the sheet offers, as values rather than names: the name is the
/// catalogue key beside it, so the two languages cannot disagree about which
/// swatch is which. Six, not sixteen - the accent is one of them, which is
/// deliberate, and a picker would be a second screen for a personal choice.
const SUBTITLE_COLOURS: Array<{ value: string; key: string }> = [
	{ value: '#EDE7DC', key: 'colourBone' },
	{ value: '#FFFFFF', key: 'colourWhite' },
	{ value: '#F2D46B', key: 'colourYellow' },
	{ value: '#8ED0E8', key: 'colourCyan' },
	{ value: '#C8A24A', key: 'colourGold' },
	{ value: '#0E0D0C', key: 'colourBlack' },
];

/// The band behind the text: none, or four greys. A band is not a designer's
/// palette, it is there to be read through, so the choice is dark, light or
/// nothing - and the engine draws it at 70 % alpha, which is what makes it a
/// band rather than a bar.
const SUBTITLE_BANDS: Array<{ value: string; key: string }> = [
	{ value: 'none', key: 'bandNone' },
	{ value: '#000000', key: 'colourBlack' },
	{ value: '#3A3632', key: 'bandGrey' },
	{ value: '#EDE7DC', key: 'colourWhite' },
];

const AUDIO_LANGUAGES = ['auto', 'vf', 'vo'] as const;
const SUBTITLE_LANGUAGES = ['auto', 'none', 'fr', 'en'] as const;
const SUBTITLE_OUTLINES = ['shadow', 'outline', 'none'] as const;
const SUBTITLE_FONTS = ['standard', 'serif', 'mono'] as const;

/// A stored value when it is one this build offers, the default otherwise.
///
/// A preferences file outlives the build that wrote it: a colour dropped from
/// the palette, or a number typed into the devtools, would otherwise reach the
/// engine as a value it refuses - and the sheet would draw a swatch as chosen
/// while the film showed something else.
function oneOf<T extends string>(value: unknown, allowed: readonly T[], fallback: T): T {
	return allowed.includes(value as T) ? (value as T) : fallback;
}

function storedPlayback(value: unknown): PlaybackPreferences {
	const raw = (value ?? {}) as Record<string, unknown>;
	const style = (raw.subtitleStyle ?? {}) as Record<string, unknown>;
	const fallback = PLAYBACK_DEFAULTS.subtitleStyle;
	// A number inside its control's own range, or the default: the engine
	// refuses a size of nine thousand rather than clamping it, and a sheet that
	// showed one would be showing a value the film never had.
	const slider = (stored: unknown, range: { min: number; max: number }, or: number) =>
		typeof stored === 'number' && Number.isFinite(stored)
			? Math.min(range.max, Math.max(range.min, Math.round(stored)))
			: or;
	return {
		autoPlayNext: raw.autoPlayNext !== false,
		audioLanguage: oneOf(raw.audioLanguage, AUDIO_LANGUAGES, PLAYBACK_DEFAULTS.audioLanguage),
		subtitleLanguage: oneOf(raw.subtitleLanguage, SUBTITLE_LANGUAGES, PLAYBACK_DEFAULTS.subtitleLanguage),
		subtitleStyle: {
			sizePx: slider(style.sizePx, SUBTITLE_SIZE, fallback.sizePx),
			heightPx: slider(style.heightPx, SUBTITLE_HEIGHT, fallback.heightPx),
			colour: oneOf(style.colour, SUBTITLE_COLOURS.map((one) => one.value), fallback.colour),
			outline: oneOf(style.outline, SUBTITLE_OUTLINES, fallback.outline),
			background: oneOf(style.background, SUBTITLE_BANDS.map((one) => one.value), fallback.background),
			font: oneOf(style.font, SUBTITLE_FONTS, fallback.font),
			bold: style.bold === true,
		},
	};
}

/// The colour the outline takes for a given text colour.
///
/// The engine inverts it over dark text - a black outline around black letters
/// is not an outline - and the preview has to obey the same rule or it would
/// show a look the film never takes.
function outlineColourFor(colour: string): string {
	const channels = [1, 3, 5].map((at) => parseInt(colour.slice(at, at + 2), 16) / 255);
	const [r, g, b] = channels.map((one) => (Number.isFinite(one) ? one : 1));
	const luminance = 0.2126 * r + 0.7152 * g + 0.0722 * b;
	return luminance < 0.5 ? '#EDE7DC' : '#000000';
}

/// What the OSD is saying, and for how long.
///
/// `label` is a catalogue key rather than a word, because the two notices are
/// not about the same thing: a refused raw stream is "Son", an engine that will
/// not start is not. A null `dwell` means the notice must not leave on its own -
/// an engine that could not start is still not started.
type Notice = { key: string; label: string; dwell: number | null };

function initialPreferences(): Preferences {
	try {
		const value = JSON.parse(localStorage.getItem('theia.player.preferences') ?? '{}');
		return {
			reducedMotion: Boolean(value.reducedMotion),
			autoHideControls: value.autoHideControls !== false,
			playback: storedPlayback(value.playback),
		};
	} catch {
		return { reducedMotion: false, autoHideControls: true, playback: PLAYBACK_DEFAULTS };
	}
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
	const [movies, setMovies] = useState<Movie[]>([]);
	const [series, setSeries] = useState<Series[]>([]);
	const [home, setHome] = useState<Home | null>(null);
	const [seriesHome, setSeriesHome] = useState<SeriesHome | null>(null);
	const [homeError, setHomeError] = useState(false);
	const [searchQuery, setSearchQuery] = useState('');
	const [selectedSeries, setSelectedSeries] = useState<Series | null>(null);
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

	const loadLibrary = useCallback(async () => {
		try {
			const [moviePayload, seriesPayload] = await Promise.all([
				invoke<string>('player_library'),
				invoke<string>('player_series'),
			]);
			setMovies(JSON.parse(moviePayload));
			setSeries(JSON.parse(seriesPayload));
		} catch {
			setMovies([]);
			setSeries([]);
			setErrorKey('libraryFailed');
		}
	}, []);

	// The home screen is a request of its own: it is built from the viewing
	// history, which changes the moment a film is left, while the library
	// catalogue does not. Everything that reloads the library reloads this,
	// so the rows reflect where the viewer actually stopped.
	const loadHome = useCallback(async () => {
		try {
			const [homePayload, seriesPayload] = await Promise.all([
				invoke<string>('player_home'),
				invoke<string>('player_series_home'),
			]);
			setHome(JSON.parse(homePayload) as Home);
			setSeriesHome(JSON.parse(seriesPayload) as SeriesHome);
			setHomeError(false);
		} catch {
			setHome(null);
			setSeriesHome(null);
			setHomeError(true);
		}
	}, []);

	const refreshUpdateStatus = useCallback(async () => {
		try {
			setUpdateStatus(JSON.parse(await invoke<string>('player_update_status')) as UpdateStatus);
		} catch {
			setUpdateStatus(null);
		}
	}, []);

	const connect = useCallback(
		async (url: string, quiet = false) => {
			if (!url) return false;
			setBusy(true);
			if (!quiet) setErrorKey(null);
			try {
				const connected = JSON.parse(await invoke<string>('player_connect', { url })) as Server;
				try {
					const rememberedProfile = Number(localStorage.getItem('theia.player.profile'));
					if (rememberedProfile && connected.profiles?.some((profile) => profile.id === rememberedProfile)) {
						await invoke('player_set_profile', { id: rememberedProfile });
						connected.profile = rememberedProfile;
					}
				} catch {
					// The server-selected profile remains authoritative when storage is unavailable.
				}
				setServer(connected);
				// The language this installation was set up in, which travels with
				// the server's identity. It is a starting point and never a
				// decision: a viewer who has chosen here keeps their choice.
				const served = connected.health?.language;
				if (!storedLanguage() && served && (catalogues as Record<string, unknown>)[served]) {
					setLanguage(served);
				}
				setAddress(connected.url);
				try {
					localStorage.setItem('theia.player.server', connected.url);
				} catch {
					// Discovery remains the fallback when storage is disabled.
				}
				await Promise.all([loadLibrary(), loadHome(), refreshUpdateStatus()]);
				return true;
			} catch {
				setServer(null);
				if (!quiet) setErrorKey('connectionFailed');
				return false;
			} finally {
				setBusy(false);
			}
		},
		[loadHome, loadLibrary, refreshUpdateStatus]
	);

	const switchProfile = useCallback(async (id: number) => {
		if (!server || server.profile === id) {
			navigate(`/${lastSection.current}`);
			return;
		}
		setBusy(true);
		setErrorKey(null);
		try {
			await invoke('player_set_profile', { id });
			setServer((current) => current ? { ...current, profile: id } : current);
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
	}, [loadHome, loadLibrary, navigate, server]);

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
			setBusy(true);
			if (!quiet) setErrorKey(null);
			try {
				const found = JSON.parse(await invoke<string>('player_discover')) as DiscoveredServer[];
				setDiscovered(found);
				if (found.length === 1) return await connect(found[0].url, quiet);
				return false;
			} catch {
				setDiscovered([]);
				return false;
			} finally {
				setBusy(false);
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
				let local: string | null = null;
				try {
					local = await invoke<string | null>('player_local_server');
				} catch {
					// Player-only installs have no local server to prepare.
				}
				if (cancelled) return;
				if (local && (await connect(local, true))) return;
				let remembered = '';
				try {
					remembered = localStorage.getItem('theia.player.server') ?? '';
				} catch {
					// Nothing to remember.
				}
				if (remembered && remembered !== local && (await connect(remembered, true))) return;
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
	// Sent the moment the engine can hear rather than only when Save is pressed,
	// and sent again whenever the preferences change: a subtitle style that
	// needed a restart to appear is a setting nobody can see the effect of, and
	// the frame this rides on is a bridge call that costs nothing. `status.ready`
	// is in the dependencies because the engine is created by the shell, not by
	// this page - a preferences frame sent before it exists is refused, and the
	// refusal is not worth a sentence: what is stored here is re-sent on the next
	// change, and the engine starts from its own copy of the same defaults.
	useEffect(() => {
		if (!status.ready) return;
		void invoke('player_set_playback', { prefs: JSON.stringify(preferences.playback) }).catch(() => {});
	}, [status.ready, preferences.playback]);

	const wake = useCallback(() => {
		if (idleTimer.current !== null) window.clearTimeout(idleTimer.current);
		idleTimer.current = null;
		setIdle(false);
		if (
			!preferences.autoHideControls ||
			!status.media ||
			status.pause ||
			!status.ready ||
			trackMenuOpen ||
			focusInFurniture
		)
			return;
		idleTimer.current = window.setTimeout(() => setIdle(true), IDLE_MS);
	}, [focusInFurniture, preferences.autoHideControls, status.media, status.pause, status.ready, trackMenuOpen]);

	useEffect(() => {
		wake();
		return () => {
			if (idleTimer.current !== null) window.clearTimeout(idleTimer.current);
		};
	}, [wake]);

	// A notice belongs to the film that raised it, so a new film starts on a
	// clean screen: the last one's sound is not this one's.
	const playMovie = async (id: number) => {
		clearNotice();
		try {
			await invoke('player_play', { id });
		} catch {
			setErrorKey('playFailed');
		}
	};
	const playEpisode = async (id: number) => {
		clearNotice();
		try {
			await invoke('player_play_episode', { id });
		} catch {
			setErrorKey('episodeFailed');
		}
	};
	const openSeries = async (id: number) => {
		setErrorKey(null);
		try {
			const detail = JSON.parse(await invoke<string>('player_series_detail', { id })) as Series;
			setSelectedSeries(detail);
			const first = detail.seasons?.[0];
			setSelectedSeason(
				first
					? (JSON.parse(
							await invoke<string>('player_season', { seriesId: detail.id, seasonNumber: first.season_number })
						) as Season)
					: null
			);
		} catch {
			setSelectedSeries(null);
			setSelectedSeason(null);
			setErrorKey('seriesFailed');
		}
	};
	const openSeason = async (seasonNumber: number) => {
		if (!selectedSeries) return;
		try {
			setSelectedSeason(
				JSON.parse(
					await invoke<string>('player_season', { seriesId: selectedSeries.id, seasonNumber })
				) as Season
			);
		} catch {
			setErrorKey('seriesFailed');
		}
	};

	const returnToLibrary = useCallback(async () => {
		if (!status.media || returning) return;
		setReturning(true);
		setTrackMenuOpen(false);
		clearNotice();
		try {
			await invoke('player_stop');
			setStatus((current) => ({ ...current, media: null, title: null, pos: null, duration: null, pause: false }));
			await Promise.all([loadLibrary(), loadHome()]);
		} catch {
			setErrorKey('stopFailed');
		} finally {
			setReturning(false);
		}
	}, [clearNotice, loadHome, loadLibrary, returning, status.media]);

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
			if (settingsOpen || profilesOpen) return;
			const editable = (event.target as HTMLElement | null)?.matches('input, textarea, select, [contenteditable="true"]');
			if (editable) {
				if (event.key === 'Escape') (event.target as HTMLElement).blur();
				return;
			}
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
				else if (selectedSeries) {
					setSelectedSeries(null);
					setSelectedSeason(null);
				}
			} else wake();
		};
		window.addEventListener('keydown', onKey);
		return () => window.removeEventListener('keydown', onKey);
	}, [fullscreen, profilesOpen, returnToLibrary, seek, selectedSeries, setFullscreenState, setVolumeTo, settingsOpen, status.media, switchLanguage, toggle, toggleFullscreen, trackMenuOpen, volume, wake]);

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

	return (
		<MotionConfig reducedMotion={preferences.reducedMotion ? 'always' : 'user'}>
			<div
				className={`osd ${status.media ? '' : 'osd--library'}`}
				data-section={section}
				data-idle={idle}
				data-maximized={maximized}
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
				<TitleBar
					title={status.title}
					playing={Boolean(status.media)}
					maximized={maximized}
					language={language}
					labels={{ back: t('backToLibrary'), minimize: t('minimize'), maximize: t('maximize'), restore: t('restore'), close: t('close') }}
					onBack={() => void returnToLibrary()}
					onLanguage={switchLanguage}
					onMinimize={() => getAppWindow()?.minimize()}
					onMaximize={async () => {
						const appWindow = getAppWindow();
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
						movies={movies} series={series} section={section} settingsOpen={settingsOpen} selectedSeries={selectedSeries} selectedSeason={selectedSeason}
						profilesOpen={profilesOpen} updateStatus={updateStatus} searchQuery={searchQuery}
						home={home} seriesHome={seriesHome} homeError={homeError} language={language}
						errorKey={errorKey} reducedMotion={preferences.reducedMotion} t={t}
						onAddress={setAddress} onSubmit={submit} onFind={() => void findServers()} onConnect={(url) => void connect(url)}
						onSection={moveToSection} onSettings={openSettings} onProfiles={openProfiles} onSearchQuery={setSearchQuery} onMovie={playMovie} onSeries={openSeries}
						onEpisode={playEpisode} onSeason={openSeason}
						onBackSeries={() => { setSelectedSeries(null); setSelectedSeason(null); }}
					/>
				)}

				<SettingsModal
					open={settingsOpen && !status.media} language={language} preferences={preferences} server={server} updateStatus={updateStatus} updateBusy={updateBusy} t={t}
					onClose={closeOverlay} onCheckUpdate={() => void checkUpdate()} onApplyUpdate={() => void applyUpdate()}
					onSave={(nextLanguage, nextPreferences) => {
						persistLanguage(nextLanguage);
						persistPreferences(nextPreferences);
						closeOverlay();
					}}
				/>

				<ProfileDialog
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

type LibraryProps = {
	server: Server | null; booting: boolean; busy: boolean; address: string; discovered: DiscoveredServer[];
	movies: Movie[]; series: Series[]; section: Section; settingsOpen: boolean; profilesOpen: boolean; updateStatus: UpdateStatus | null;
	searchQuery: string; selectedSeries: Series | null; selectedSeason: Season | null;
	home: Home | null; seriesHome: SeriesHome | null; homeError: boolean; language: string;
	errorKey: string | null; reducedMotion: boolean; t: (key: string) => string;
	onAddress: (value: string) => void; onSubmit: (event: FormEvent) => void; onFind: () => void;
	onConnect: (url: string) => void; onSection: (value: Section) => void; onSettings: () => void; onProfiles: () => void;
	onSearchQuery: (value: string) => void;
	onMovie: (id: number) => void; onSeries: (id: number) => void; onEpisode: (id: number) => void;
	onSeason: (number: number) => void; onBackSeries: () => void;
};

function Library(props: LibraryProps) {
	const { server, selectedSeries, selectedSeason, section, t } = props;
	const title = selectedSeries ? displayTitle(selectedSeries) : undefined;
	const count = section === 'series' ? props.series.length : section === 'search' ? props.movies.length + props.series.length : props.movies.length;
	const spotlightSource = section === 'series' ? props.series[0] : props.movies[0];
	const spotlight = spotlightSource ? artworkCandidates(spotlightSource, 'w1280')[0] : undefined;

	return (
		<section className="library">
			{spotlight && !selectedSeries && section !== 'home' && section !== 'search' && <div className="library-ambient" aria-hidden="true"><img src={spotlight} alt="" crossOrigin="anonymous" /></div>}
			{server && <LibraryNav section={section} settingsOpen={props.settingsOpen} profilesOpen={props.profilesOpen} profiles={server.profiles ?? []} activeProfile={server.profile ?? null} serverURL={server.url} updateAvailable={Boolean(props.updateStatus?.available)} t={t} onSection={props.onSection} onSettings={props.onSettings} onProfiles={props.onProfiles} />}

			{/* The home screen has no page heading: its hero is the heading. A
			   series opened from one of the home's rows keeps its own. */}
			{!(section === 'home' && !selectedSeries && server && !props.booting) && <div className={section === 'search' ? 'library-heading library-heading--centered' : 'library-heading'}>
				{selectedSeries && <button className="library-back" onClick={props.onBackSeries}><ArrowLeft size={18} />{t(section === 'home' ? 'home' : 'allSeries')}</button>}
				{/* The search room is a centred stage with no eyebrow: the loop
				   is the decoration and "Your library" said nothing there. */}
				{section !== 'search' && <p className="library-eyebrow label">{server ? (selectedSeries ? t('seriesLabel') : t('yourLibrary')) : t('desktopPlayer')}</p>}
				<h1 className="library-title">{title || (server ? (section === 'series' ? t('series') : section === 'search' ? t('searchTitle') : t('allFilms')) : props.booting ? t('starting') : t('connectTitle'))}</h1>
				{server && !selectedSeries && section !== 'search' && <p className="library-count label">{count} {t(section === 'series' ? (count === 1 ? 'seriesSingular' : 'seriesPlural') : (count === 1 ? 'filmSingular' : 'filmPlural'))}</p>}
			</div>}

			<AnimatePresence mode="wait">
				{props.booting ? (
					<motion.p key="boot" className="hint" initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }}>{t('startingHint')}</motion.p>
				) : !server ? (
					<motion.div key="connect" className="connect-panel" initial={{ opacity: 0, y: 14 }} animate={{ opacity: 1, y: 0 }}>
						<form className="connect" onSubmit={props.onSubmit}>
							<label className="label" htmlFor="theia-address">{t('address')}</label>
							<div className="connect-row">
								<input id="theia-address" value={props.address} onChange={(event) => props.onAddress(event.target.value)} placeholder="http://192.168.1.20:8383" autoComplete="off" spellCheck={false} />
								<Button type="submit" disabled={props.busy}>{props.busy ? t('searching') : t('connect')}</Button>
								<Button type="button" variant="outline" disabled={props.busy} onClick={props.onFind}>{t('findServers')}</Button>
							</div>
						</form>
						{props.discovered.length > 0 && <ul className="servers">{props.discovered.map((entry) => <li key={entry.url}><Button variant="outline" onClick={() => props.onConnect(entry.url)}>{entry.name} — {entry.url}</Button></li>)}</ul>}
						<p className={`hint ${props.errorKey ? 'hint--error' : ''}`}>{props.errorKey ? t(props.errorKey) : t('noServer')}</p>
					</motion.div>
				) : selectedSeries ? (
					<motion.div key={`series-${selectedSeries.id}`} className="contents" initial={{ opacity: 0, y: 12 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0 }}>
						<div className="season-tabs">{selectedSeries.seasons?.map((season) => <button key={season.id} className={`season-tab label ${selectedSeason?.season_number === season.season_number ? 'season-tab--active' : ''}`} onClick={() => props.onSeason(season.season_number)}>{season.metadata?.name || `${t('season')} ${season.season_number}`}</button>)}</div>
						{selectedSeason?.episodes?.length ? <CardGrid>{selectedSeason.episodes.map((episode) => <MediaCard key={episode.id} kind="episode" item={episode} seriesLabel={displayTitle(selectedSeries)} onOpen={props.onEpisode} resumeLabel={t('resumeAt')} actionLabel={t('playEpisode')} kindLabel={t('episodeUntitled')} reducedMotion={props.reducedMotion} t={props.t} />)}</CardGrid> : <p className="hint">{t('emptySeason')}</p>}
					</motion.div>
				) : section === 'home' ? (
					<motion.div key="home" className="home-view" initial={{ opacity: 0, y: 12 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -6 }} transition={{ duration: 0.24, ease: [0.16, 1, 0.3, 1] }}>
						{props.homeError ? (
							<p className="hint hint--error">{t('homeFailed')}</p>
						) : !props.home ? (
							<p className="hint">{t('startingHint')}</p>
						) : (
							<HomeView home={props.home} seriesHome={props.seriesHome} language={props.language} reducedMotion={props.reducedMotion} t={t} onMovie={props.onMovie} onSeries={props.onSeries} onEpisode={props.onEpisode} />
						)}
					</motion.div>
				) : section === 'search' ? (
					<SearchResults {...props} />
				) : (
					<motion.div key={section} className="contents" initial={{ opacity: 0, y: 12 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -6 }} transition={{ duration: 0.24, ease: [0.16, 1, 0.3, 1] }}>
						{section === 'series' ? (props.series.length ? <CardGrid>{props.series.map((item) => <MediaCard key={item.id} kind="series" item={item} onOpen={props.onSeries} resumeLabel={t('resumeAt')} actionLabel={t('openSeries')} kindLabel={t('seriesLabel')} reducedMotion={props.reducedMotion} t={props.t} />)}</CardGrid> : <p className="hint">{t('emptySeries')}</p>) : (props.movies.length ? <CardGrid>{props.movies.map((movie) => <MediaCard key={movie.id} kind="movie" item={movie} onOpen={props.onMovie} resumeLabel={t('resumeAt')} actionLabel={t('playMovie')} kindLabel={t('filmSingular')} reducedMotion={props.reducedMotion} t={props.t} />)}</CardGrid> : <p className="hint">{t('emptyLibrary')}</p>)}
						{props.errorKey && <p className="hint hint--error">{t(props.errorKey)}</p>}
					</motion.div>
				)}
			</AnimatePresence>
		</section>
	);
}

function SearchResults(props: LibraryProps) {
	const query = props.searchQuery.trim().toLocaleLowerCase();
	const matchingMovies = query ? props.movies.filter((item) => `${displayTitle(item)} ${displayYear(item) ?? ''}`.toLocaleLowerCase().includes(query)) : [];
	const matchingSeries = query ? props.series.filter((item) => `${displayTitle(item)} ${displayYear(item) ?? ''}`.toLocaleLowerCase().includes(query)) : [];
	return (
		<motion.div key="search" className="contents search-view" initial={{ opacity: 0, y: 12 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0 }}>
			<label className="search-field">
				<Search size={22} aria-hidden="true" />
				<span className="sr-only">{props.t('search')}</span>
				<input autoFocus value={props.searchQuery} onChange={(event) => props.onSearchQuery(event.target.value)} placeholder={props.t('searchPlaceholder')} />
			</label>
			{!query ? <p className="hint">{props.t('searchHint')}</p> : matchingMovies.length + matchingSeries.length === 0 ? <p className="hint">{props.t('noSearchResults')}</p> : (
				<div className="search-results">
					{matchingMovies.length > 0 && <section><h2 className="search-result-title label">{props.t('filmResults')} · {matchingMovies.length}</h2><CardGrid>{matchingMovies.map((movie) => <MediaCard key={movie.id} kind="movie" item={movie} onOpen={props.onMovie} resumeLabel={props.t('resumeAt')} actionLabel={props.t('playMovie')} kindLabel={props.t('filmSingular')} reducedMotion={props.reducedMotion} t={props.t} />)}</CardGrid></section>}
					{matchingSeries.length > 0 && <section><h2 className="search-result-title label">{props.t('seriesResults')} · {matchingSeries.length}</h2><CardGrid>{matchingSeries.map((item) => <MediaCard key={item.id} kind="series" item={item} onOpen={props.onSeries} resumeLabel={props.t('resumeAt')} actionLabel={props.t('openSeries')} kindLabel={props.t('seriesLabel')} reducedMotion={props.reducedMotion} t={props.t} />)}</CardGrid></section>}
				</div>
			)}
		</motion.div>
	);
}

const ROW_TITLES: Record<string, string> = {
	continue: 'rowContinue',
	recent: 'rowRecent',
	top_rated: 'rowTopRated',
	tonight: 'rowTonight',
	series_continue: 'rowSeriesContinue',
	series_recent: 'rowSeriesRecent',
};

type HomeViewProps = {
	home: Home; seriesHome: SeriesHome | null; language: string; reducedMotion: boolean;
	t: (key: string) => string;
	onMovie: (id: number) => void; onSeries: (id: number) => void; onEpisode: (id: number) => void;
};

/**
 * The web home's composition, carried into the player: one hero for the film
 * that was left, then short rows. The server decides what each row is; this
 * screen decides what it is called. The series rows sit after the film rows
 * on purpose - same as the web, where the two halves are answered separately.
 */
function HomeView({ home, seriesHome, language, reducedMotion, t, onMovie, onSeries, onEpisode }: HomeViewProps) {
	const rows: Array<{ kind: string; hint: string | null; cards: React.ReactNode }> = [];
	for (const row of home.rows ?? []) {
		rows.push({
			kind: row.kind,
			hint: row.kind === 'tonight' ? t('rowTonightHint') : null,
			cards: row.movies.map((movie) => (
				<MediaCard key={movie.id} kind="movie" item={movie} onOpen={onMovie} resumeLabel={t('resumeAt')} actionLabel={t('playMovie')} kindLabel={t('filmSingular')} reducedMotion={reducedMotion} t={t} />
			)),
		});
	}
	if (seriesHome?.continue_watching?.length) {
		rows.push({
			kind: 'series_continue',
			hint: null,
			cards: seriesHome.continue_watching.map((episode) => (
				<MediaCard key={episode.id} kind="episode" item={episode} heading={episode.series_title} onOpen={onEpisode} resumeLabel={t('resumeAt')} actionLabel={t('playEpisode')} kindLabel={t('episodeUntitled')} reducedMotion={reducedMotion} t={t} />
			)),
		});
	}
	if (seriesHome?.recent_series?.length) {
		rows.push({
			kind: 'series_recent',
			hint: null,
			cards: seriesHome.recent_series.map((series) => (
				<MediaCard key={series.id} kind="series" item={series} onOpen={onSeries} resumeLabel={t('resumeAt')} actionLabel={t('openSeries')} kindLabel={t('seriesLabel')} reducedMotion={reducedMotion} t={t} />
			)),
		});
	}
	const hero = home.hero ?? null;
	return (
		<>
			{hero && <HomeHero movie={hero} resuming={home.hero_kind === 'resume'} language={language} t={t} onPlay={onMovie} />}
			{rows.map((row) => (
				<MediaRow key={row.kind} title={t(ROW_TITLES[row.kind] ?? row.kind)} hint={row.hint} t={t} reducedMotion={reducedMotion}>
					{row.cards}
				</MediaRow>
			))}
			{!hero && rows.length === 0 && <p className="hint">{t('emptyLibrary')}</p>}
		</>
	);
}

/**
 * The film you were watching, stated properly rather than as a 3px rule: the
 * eyebrow says which of the two states this is, the progress bar carries what
 * is left, and the one button does the one thing. There is no "view details"
 * beside it - the player has no film page, and a button that led nowhere
 * would be worse than its absence.
 */
function HomeHero({ movie, resuming, language, t, onPlay }: { movie: Movie; resuming: boolean; language: string; t: (key: string) => string; onPlay: (id: number) => void }) {
	const title = displayTitle(movie);
	const yearValue = displayYear(movie);
	const year = yearValue ? String(yearValue) : '';
	const runtime = formatRuntime(movie.metadata?.runtime_minutes, language);
	const director = movie.metadata?.director ?? '';
	const rating = movie.metadata?.vote_average ?? 0;
	const [heroFailed, setHeroFailed] = useState(false);
	useEffect(() => setHeroFailed(false), [movie.id]);
	// The not-found plate answers when no artwork exists or the picture
	// failed; an item that has its own artwork keeps it.
	const heroArt = heroFailed ? notFoundArt : (artworkCandidates(movie, 'w1280')[0] ?? notFoundArt);
	const position = movie.progress?.position_seconds ?? 0;
	const duration = movie.progress?.duration_seconds ?? 0;
	const playing = resuming && position > 0 && duration > 0;
	const percent = playing ? Math.min(100, (position / duration) * 100) : 0;
	const remaining = playing ? formatRuntime(Math.round((duration - position) / 60), language) : null;
	const overview = playing ? '' : movie.metadata?.overview ?? '';
	return (
		<section className="home-hero" aria-label={title}>
			<img className="home-hero-art" src={heroArt} alt="" crossOrigin="anonymous" fetchPriority="high" onError={() => setHeroFailed(true)} />
			<div className="home-hero-content">
				<p className="label home-hero-eyebrow">{playing ? t('heroResumeEyebrow') : t('heroFeaturedEyebrow')}</p>
				<h1 className="home-hero-title">{title}</h1>
				<div className="home-hero-meta">
					{year && <span className="label">{year}</span>}
					{runtime && <span className="label">{runtime}</span>}
					{director && <span className="label">{director}</span>}
					{rating > 0 && (
						<span className="home-hero-rating">
							<span className="home-hero-rating-figure">{rating.toLocaleString(language === 'en' ? 'en-US' : 'fr-FR', { minimumFractionDigits: 1, maximumFractionDigits: 1 })}</span>
							<span className="label">{t('ratingScale')}</span>
						</span>
					)}
				</div>
				{playing ? (
					<div className="home-hero-progress">
						<div className="home-hero-progress-track"><div className="home-hero-progress-played" style={{ width: `${percent}%` }} /></div>
						{remaining && <span className="label home-hero-remaining">{t('remainingPattern').replace('{d}', remaining)}</span>}
					</div>
				) : overview ? (
					<p className="home-hero-overview">{overview}</p>
				) : null}
				<div className="home-hero-actions">
					<Button className="home-hero-action" onClick={() => onPlay(movie.id)}>
						{playing ? t('resume') : t('playMovie')}
						<ArrowRight size={17} aria-hidden="true" />
					</Button>
				</div>
			</div>
		</section>
	);
}

/**
 * A row is a strip you skim, not a grid you search: horizontal scroll, no
 * scrollbar (design system 6.3 - the chevrons answer for a mouse, the arrows
 * for a keyboard), and cards at the fixed row width.
 */
function MediaRow({ title, hint, t, reducedMotion, children }: { title: string; hint: string | null; t: (key: string) => string; reducedMotion: boolean; children: React.ReactNode }) {
	const scroller = useRef<HTMLUListElement>(null);
	const [edges, setEdges] = useState({ left: false, right: false });
	const count = Children.count(children);
	const updateEdges = useCallback(() => {
		const element = scroller.current;
		if (!element) return;
		const max = Math.max(0, element.scrollWidth - element.clientWidth);
		setEdges({ left: element.scrollLeft > 2, right: element.scrollLeft < max - 2 });
	}, []);
	useEffect(() => {
		updateEdges();
		const element = scroller.current;
		if (!element || typeof ResizeObserver === 'undefined') return;
		const observer = new ResizeObserver(updateEdges);
		observer.observe(element);
		return () => observer.disconnect();
	}, [count, updateEdges]);
	const scrollBy = (direction: number) => {
		const element = scroller.current;
		if (!element) return;
		element.scrollBy({ left: direction * Math.max(320, element.clientWidth * 0.78), behavior: reducedMotion ? 'auto' : 'smooth' });
	};
	const moveFocus = (event: ReactKeyboardEvent<HTMLUListElement>) => {
		if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return;
		const cards = [...event.currentTarget.querySelectorAll<HTMLButtonElement>('button.film')];
		const current = (event.target as HTMLElement).closest('button.film');
		const index = current instanceof HTMLButtonElement ? cards.indexOf(current) : -1;
		if (index < 0) return;
		const next = cards[event.key === 'ArrowRight' ? index + 1 : index - 1];
		if (!next) return;
		event.preventDefault();
		next.focus({ preventScroll: true });
		next.scrollIntoView({ block: 'nearest', inline: 'center', behavior: reducedMotion ? 'auto' : 'smooth' });
	};
	return (
		<section className="home-row" aria-label={title}>
			<div className="home-row-heading">
				<div>
					<h2 className="home-row-title">{title}</h2>
					{hint && <p className="label">{hint}</p>}
				</div>
			</div>
			<div className="row-scroll-frame">
				{edges.left && <button type="button" className="row-scroll-button row-scroll-button--left" tabIndex={-1} aria-label={t('scrollLeft')} onPointerDown={(event) => event.preventDefault()} onClick={() => scrollBy(-1)}><ChevronLeft size={28} /></button>}
				<ul ref={scroller} className="row-scroll" onScroll={updateEdges} onKeyDown={moveFocus}>{children}</ul>
				{edges.right && <button type="button" className="row-scroll-button row-scroll-button--right" tabIndex={-1} aria-label={t('scrollRight')} onPointerDown={(event) => event.preventDefault()} onClick={() => scrollBy(1)}><ChevronRight size={28} /></button>}
			</div>
		</section>
	);
}

function LibraryNav({ section, settingsOpen, profilesOpen, profiles, activeProfile, serverURL, updateAvailable, t, onSection, onSettings, onProfiles }: { section: Section; settingsOpen: boolean; profilesOpen: boolean; profiles: Profile[]; activeProfile: number | null; serverURL: string; updateAvailable: boolean; t: (key: string) => string; onSection: (section: Section) => void; onSettings: () => void; onProfiles: () => void }) {
	const items = [
		{ key: 'home' as const, label: t('home'), icon: House },
		{ key: 'films' as const, label: t('films'), icon: Clapperboard },
		{ key: 'series' as const, label: t('series'), icon: Tv },
		{ key: 'search' as const, label: t('search'), icon: Search },
	];
	const profile = profiles.find((entry) => entry.id === activeProfile) ?? profiles[0];
	const initial = (profile?.name || t('profileDefaultName')).trim().slice(0, 1).toUpperCase();
	const avatar = profileAvatarURL(serverURL, profile);
	return (
		<nav className="library-nav" aria-label={t('libraryNavigation')}>
			<button className="nav-brand" aria-label={`THEIA — ${t('home')}`} onClick={() => onSection('home')}><span>THEIA</span></button>
			<div className="nav-items">
				{items.map((item) => {
					const Icon = item.icon;
					const active = item.key === section && !settingsOpen && !profilesOpen;
					return <button key={item.key} className={`nav-link ${active ? 'nav-link--active' : ''}`} aria-current={active ? 'page' : undefined} aria-label={item.label} onClick={() => onSection(item.key)}>{active && <motion.span layoutId="library-nav-active" className="nav-active-surface" transition={{ type: 'spring', stiffness: 430, damping: 36 }} />}<Icon size={17} strokeWidth={1.8} /><span className="nav-label">{item.label}</span></button>;
				})}
				<button className={`nav-link ${settingsOpen ? 'nav-link--active' : ''}`} aria-current={settingsOpen ? 'page' : undefined} aria-label={`${t('settings')}${updateAvailable ? ` · ${t('updateAvailable')}` : ''}`} onClick={onSettings}>{settingsOpen && <motion.span layoutId="library-nav-active" className="nav-active-surface" />}<Cog size={17} strokeWidth={1.8} /><span className="nav-label">{t('settings')}</span>{updateAvailable && <span className="nav-update-badge" aria-hidden="true">1</span>}</button>
				<button className={`nav-profile ${profilesOpen ? 'nav-profile--active' : ''}`} aria-label={t('profiles')} aria-current={profilesOpen ? 'page' : undefined} title={profile?.name || t('profileDefaultName')} onClick={onProfiles}>{profilesOpen && <motion.span layoutId="library-nav-active" className="nav-active-surface" />}{avatar ? <img src={avatar} alt="" crossOrigin="anonymous" /> : <span aria-hidden="true">{initial || <UserRound size={18} />}</span>}</button>
			</div>
		</nav>
	);
}

function profileAvatarURL(serverURL: string, profile?: Profile) {
	if (!profile?.has_avatar) return null;
	if (profile.avatar_url) return profile.avatar_url;
	return `${serverURL.replace(/\/$/, '')}/api/profiles/${profile.id}/avatar?v=${profile.avatar_version ?? 0}`;
}

function CardGrid({ children }: { children: React.ReactNode }) {
	return <ul className="films">{children}</ul>;
}

// The sections of the settings sheet, in the order its rail shows them. The
// name is also the catalogue key of the panel's heading, so the rail, the panel
// and the sentences cannot drift apart.
//
// `playback` is the playback row and `subtitles` its own pane, which is the
// disposition the maintainer brought on 24 September 2026: the two switches
// that were in `playback` describe the interface, not the film, so they moved
// to `interface` where they belong and the pane below them can be the three
// things a viewer actually changes about films.
type SettingsPane = 'interface' | 'playback' | 'subtitles' | 'watching' | 'server' | 'update';

/// A row of mutually exclusive choices, drawn as one segmented pill - the
/// control the sheet already uses for the interface language, so a viewer
/// learns it once. `aria-pressed` rather than colour alone, as the design
/// system requires: the fill is bone, not the accent, for the same reason.
function SettingsChoices<T extends string>({ label, options, value, onPick, t }: {
	label: string;
	options: Array<{ value: T; key: string }>;
	value: T;
	onPick: (value: T) => void;
	t: (key: string) => string;
}) {
	return (
		<div className="settings-segment" role="group" aria-label={label}>
			{options.map((one) => (
				<button key={one.value} type="button" aria-pressed={value === one.value} onClick={() => onPick(one.value)}>{t(one.key)}</button>
			))}
		</div>
	);
}

/// The colours, as colours rather than words: a swatch is the only control that
/// answers "what will it look like" without being read. `none` is drawn as a
/// struck-through disc, the way a palette says "nothing here" without a word.
function SettingsSwatches({ label, options, value, onPick, t }: {
	label: string;
	options: Array<{ value: string; key: string }>;
	value: string;
	onPick: (value: string) => void;
	t: (key: string) => string;
}) {
	return (
		<div className="settings-swatches" role="group" aria-label={label}>
			{options.map((one) => (
				<button
					key={one.value}
					type="button"
					className={`settings-swatch${one.value === 'none' ? ' settings-swatch--none' : ''}`}
					style={{ '--swatch': one.value } as React.CSSProperties}
					aria-pressed={value === one.value}
					aria-label={t(one.key)}
					title={t(one.key)}
					onClick={() => onPick(one.value)}
				/>
			))}
		</div>
	);
}

/// One measurement row: the glyph, the sentence, the number, and the slider.
///
/// The minus and plus buttons are not decoration - a thumb is precise to about
/// three pixels and the size is a decision about reading at three metres - and
/// they carry their own accessible names because a lone glyph is not a name.
function SettingsSlider({ icon: Icon, label, hint, value, range, suffix, onChange, t }: {
	icon: LucideIcon;
	label: string;
	hint: string;
	value: number;
	range: { min: number; max: number; step: number };
	suffix: string;
	onChange: (value: number) => void;
	t: (key: string) => string;
}) {
	const hold = (delta: number) => onChange(Math.min(range.max, Math.max(range.min, value + delta)));
	return (
		<section className="settings-row">
			<div className="settings-row-head">
				<span className="settings-row-icon" aria-hidden="true"><Icon size={17} strokeWidth={1.8} /></span>
				<div className="settings-section-copy"><h3>{label}</h3><p>{hint}</p></div>
				<span className="settings-value">{value} {suffix}</span>
			</div>
			<div className="settings-slider">
				<button type="button" className="settings-step" onClick={() => hold(-range.step)} disabled={value <= range.min} aria-label={`${t('decrease')} · ${label}`}><Minus size={16} strokeWidth={2} /></button>
				<input
					type="range"
					min={range.min}
					max={range.max}
					step={range.step}
					value={value}
					onChange={(event) => onChange(Number(event.currentTarget.value))}
					aria-label={label}
					aria-valuetext={`${value} ${suffix}`}
				/>
				<button type="button" className="settings-step" onClick={() => hold(range.step)} disabled={value >= range.max} aria-label={`${t('increase')} · ${label}`}><Plus size={16} strokeWidth={2} /></button>
			</div>
		</section>
	);
}

/// What the film will look like, at the film's own proportion.
///
/// The stage is 16:9 and stands for a picture 1080 pixels tall, so one CSS rule
/// - `1cqh / 1080` - turns the stored pixels into this box's own, and what the
/// sheet draws is the arithmetic the engine is handed rather than a decoration
/// shaped like subtitles. Sizes, colour, font and the band are exact; the
/// outline and the shadow are CSS approximations of what libass does, and the
/// film stays the authority. No artwork: the ground is a gradient, which is
/// also what a bright scene and a dark one need to be judged against.
function SubtitlePreview({ subtitles, line, label }: { subtitles: SubtitleStyle; line: string; label: string }) {
	return (
		<div className="subtitle-preview">
			<span className="label subtitle-preview-title">{label}</span>
			<div
				className="subtitle-preview-stage"
				data-outline={subtitles.outline}
				role="img"
				aria-label={`${label} — ${line}`}
				style={{
					'--sub-size': subtitles.sizePx,
					'--sub-height': subtitles.heightPx,
					'--sub-colour': subtitles.colour,
					'--sub-outline': outlineColourFor(subtitles.colour),
					'--sub-band': subtitles.background === 'none' ? 'transparent' : `${subtitles.background}B3`,
					'--sub-font': subtitles.font === 'serif' ? 'Georgia, "Times New Roman", serif' : subtitles.font === 'mono' ? 'Consolas, "SFMono-Regular", monospace' : 'var(--font-ui)',
					'--sub-weight': subtitles.bold ? 700 : 400,
				} as React.CSSProperties}
			>
				<div className="subtitle-preview-band"><span className="subtitle-preview-line">{line}</span></div>
			</div>
		</div>
	);
}

/// The subtitles pane: what the reference laid out, in this product's language.
///
/// Seven decisions, in the order somebody makes them - how big, how high, what
/// colour, what detaches it, what holds it, in which family, how heavy - and a
/// preview above all of them, because every one of the seven is a question the
/// eye answers faster than the mind.
function SubtitlePane({ subtitles, t, onStyle, onReset }: {
	subtitles: SubtitleStyle;
	t: (key: string) => string;
	onStyle: (patch: Partial<SubtitleStyle>) => void;
	onReset: () => void;
}) {
	// Whether anything has been changed at all, which is the only thing the
	// reset link needs to know: a link that is always lit is a link nobody
	// believes, and pressing it would write a value that is already there.
	const changed = (Object.keys(PLAYBACK_DEFAULTS.subtitleStyle) as Array<keyof SubtitleStyle>).some(
		(key) => subtitles[key] !== PLAYBACK_DEFAULTS.subtitleStyle[key],
	);
	return (
		<>
			<section className="settings-section settings-section--row">
				<div className="settings-section-copy"><h3>{t('subtitles')}</h3><p>{t('subtitlesHint')}</p></div>
				<button type="button" className="settings-reset" onClick={onReset} disabled={!changed}>{t('subtitlesReset')}</button>
			</section>
			<SubtitlePreview subtitles={subtitles} line={t('subtitlePreviewLine')} label={t('subtitlePreview')} />
			<SettingsSlider
				icon={Type} label={t('subtitleSize')} hint={t('subtitleSizeHint')} suffix={t('pixels')}
				value={subtitles.sizePx} range={SUBTITLE_SIZE} onChange={(sizePx) => onStyle({ sizePx })} t={t}
			/>
			<div className="settings-separator" />
			<SettingsSlider
				icon={UnfoldVertical} label={t('subtitleHeight')} hint={t('subtitleHeightHint')} suffix={t('pixels')}
				value={subtitles.heightPx} range={SUBTITLE_HEIGHT} onChange={(heightPx) => onStyle({ heightPx })} t={t}
			/>
			<div className="settings-separator" />
			<section className="settings-row">
				<div className="settings-row-head">
					<span className="settings-row-icon" aria-hidden="true"><Palette size={17} strokeWidth={1.8} /></span>
					<div className="settings-section-copy"><h3>{t('subtitleColour')}</h3><p>{t('subtitleColourHint')}</p></div>
				</div>
				<SettingsSwatches label={t('subtitleColour')} options={SUBTITLE_COLOURS} value={subtitles.colour} onPick={(colour) => onStyle({ colour })} t={t} />
			</section>
			<div className="settings-separator" />
			<section className="settings-row">
				<div className="settings-row-head">
					<span className="settings-row-icon" aria-hidden="true"><PenLine size={17} strokeWidth={1.8} /></span>
					<div className="settings-section-copy"><h3>{t('subtitleOutline')}</h3><p>{t('subtitleOutlineHint')}</p></div>
				</div>
				<SettingsChoices
					label={t('subtitleOutline')}
					options={[{ value: 'shadow', key: 'outlineShadow' }, { value: 'outline', key: 'outlineLine' }, { value: 'none', key: 'outlineNone' }] as const}
					value={subtitles.outline}
					onPick={(outline) => onStyle({ outline })}
					t={t}
				/>
			</section>
			<div className="settings-separator" />
			<section className="settings-row">
				<div className="settings-row-head">
					<span className="settings-row-icon" aria-hidden="true"><Square size={17} strokeWidth={1.8} /></span>
					<div className="settings-section-copy"><h3>{t('subtitleBand')}</h3><p>{t('subtitleBandHint')}</p></div>
				</div>
				<SettingsSwatches label={t('subtitleBand')} options={SUBTITLE_BANDS} value={subtitles.background} onPick={(background) => onStyle({ background })} t={t} />
			</section>
			<div className="settings-separator" />
			<section className="settings-row">
				<div className="settings-row-head">
					<span className="settings-row-icon settings-row-icon--text" aria-hidden="true">Aa</span>
					<div className="settings-section-copy"><h3>{t('subtitleFont')}</h3><p>{t('subtitleFontHint')}</p></div>
				</div>
				<SettingsChoices
					label={t('subtitleFont')}
					options={[{ value: 'standard', key: 'fontStandard' }, { value: 'serif', key: 'fontSerif' }, { value: 'mono', key: 'fontMono' }] as const}
					value={subtitles.font}
					onPick={(font) => onStyle({ font })}
					t={t}
				/>
			</section>
			<div className="settings-separator" />
			<section className="settings-row">
				<div className="settings-row-head">
					<span className="settings-row-icon" aria-hidden="true"><Bold size={17} strokeWidth={1.8} /></span>
					<div className="settings-section-copy"><h3>{t('subtitleBold')}</h3><p>{t('subtitleBoldHint')}</p></div>
				</div>
				<SettingsChoices
					label={t('subtitleBold')}
					options={[{ value: 'normal', key: 'boldNormal' }, { value: 'thick', key: 'boldThick' }] as const}
					value={subtitles.bold ? 'thick' : 'normal'}
					onPick={(weight) => onStyle({ bold: weight === 'thick' })}
					t={t}
				/>
			</section>
		</>
	);
}

function SettingsModal({ open, language, preferences, server, updateStatus, updateBusy, t, onClose, onSave, onCheckUpdate, onApplyUpdate }: { open: boolean; language: string; preferences: Preferences; server: Server | null; updateStatus: UpdateStatus | null; updateBusy: boolean; t: (key: string) => string; onClose: () => void; onSave: (language: string, preferences: Preferences) => void; onCheckUpdate: () => void; onApplyUpdate: () => void }) {
	const [draftLanguage, setDraftLanguage] = useState(language);
	const [draft, setDraft] = useState(preferences);
	// What the copy button said last, and when it goes quiet again. The address
	// is ellipsised in a narrow sheet, so what the button copies is sometimes
	// more than the eye can read - which is the reason it exists.
	const [copyState, setCopyState] = useState<'idle' | 'copied' | 'failed'>('idle');
	const copyTimer = useRef<number | null>(null);
	// What was watched, asked for when the sheet opens rather than kept: a
	// viewer who has just finished an episode opens the settings panel to see
	// the number move, and one request per opening is what that costs.
	const [watching, setWatching] = useState<WatchStats | null>(null);
	// Which section the sheet is showing. The maintainer asked for the shape of
	// a reference from 21st.dev (v-card-17, 21 September 2026): a rail of
	// sections on the left, the chosen one on the right. Four sections in one
	// column was a page of switches and links with no way to see what was in it
	// without scrolling past everything else.
	const [pane, setPane] = useState<SettingsPane>('interface');
	// The numbers are per viewer, so the sentence names one: the profile this
	// player is set to, or the server's word for the profile it defaults to.
	const viewer = server?.profiles.find((one) => one.id === server.profile)?.name || t('profileDefaultName');
	// The server counts seconds and the formatter speaks minutes, which is the
	// conversion the home screen's runtime already makes.
	const spent = (seconds: number) => formatRuntime(Math.round(seconds / 60), language);
	// A bar that draws itself says "this was measured", and one that draws
	// itself while somebody has asked for less motion is a fault: the
	// preference the sheet edits is honoured here as it is everywhere else.
	const statsBarDuration = (reduced: boolean) => (reduced ? 0 : 0.5);
	// The completion rate is the share of what was opened that was finished,
	// across both families, and it is absent rather than zero on a library
	// nobody has opened anything in: nought per cent of nothing is not a
	// measurement.
	const startedCount = (watching?.movies.started ?? 0) + (watching?.episodes.started ?? 0);
	const finishedCount = (watching?.movies.finished ?? 0) + (watching?.episodes.finished ?? 0);
	const completion = watching && startedCount > 0 ? Math.round((finishedCount / startedCount) * 100) : null;
	// The month's line reads as a sentence rather than as three labels, so the
	// two counts carry their own singular: "1 film", "2 épisodes".
	const countFilms = (n: number) => (n === 1 ? t('countFilmsOne') : t('countFilms')).replace('{n}', String(n));
	const countEpisodes = (n: number) => (n === 1 ? t('countEpisodesOne') : t('countEpisodes')).replace('{n}', String(n));
	// One share and its complement, because two roundings of the same total can
	// add up to 99 or 101.
	const watchedSeconds = (watching?.movies.seconds ?? 0) + (watching?.series.seconds ?? 0);
	const share = watching && watchedSeconds > 0 ? Math.round((watching.movies.seconds / watchedSeconds) * 100) : null;
	const updateStateKey = updateStatus ? ({ idle: 'updateUnknown', checking: 'updateChecking', available: 'updateAvailable', downloading: 'updateInstalling', ready: 'updateReady', deferred: 'updateDeferred', failed: 'updateFailed', unsupported: 'updateUnsupported' } as Record<string, string>)[updateStatus.state] ?? 'updateUnknown' : 'updateUnknown';
	const panes: Array<{ id: SettingsPane; icon: LucideIcon; label: string }> = [
		{ id: 'interface', icon: Languages, label: t('interface') },
		{ id: 'playback', icon: MonitorPlay, label: t('playback') },
		{ id: 'subtitles', icon: Captions, label: t('subtitles') },
		{ id: 'watching', icon: BarChart3, label: t('watching') },
		{ id: 'server', icon: ServerIcon, label: t('server') },
		{ id: 'update', icon: Download, label: t('update') },
	];
	// The two writers the panes below use, so a row never has to know how the
	// draft is nested. A patch rather than a whole object: the style has seven
	// fields and a slider that sent all seven would overwrite a swatch chosen
	// while the thumb was moving.
	const patchPlayback = (patch: Partial<PlaybackPreferences>) =>
		setDraft((current) => ({ ...current, playback: { ...current.playback, ...patch } }));
	const patchStyle = (patch: Partial<SubtitleStyle>) =>
		setDraft((current) => ({
			...current,
			playback: { ...current.playback, subtitleStyle: { ...current.playback.subtitleStyle, ...patch } },
		}));
	useEffect(() => {
		if (open) {
			setDraftLanguage(language);
			setDraft(preferences);
			setCopyState('idle');
			// The sheet opens where it always opens: the section whose settings
			// a person changes most, not wherever it was left.
			setPane('interface');
		}
	}, [language, open, preferences]);
	useEffect(() => () => {
		if (copyTimer.current !== null) window.clearTimeout(copyTimer.current);
	}, []);
	useEffect(() => {
		if (!open) return;
		// A server without the route answers an error, and the pane says so
		// instead of drawing six zeroes that look like a measurement.
		let cancelled = false;
		setWatching(null);
		invoke<string>('player_watch_stats')
			.then((json) => { if (!cancelled) setWatching(JSON.parse(json) as WatchStats); })
			.catch(() => { if (!cancelled) setWatching(null); });
		return () => { cancelled = true; };
	}, [open]);

	const copyAddress = async () => {
		if (!server) return;
		// The WebView2 origin is a secure context, which is what the asynchronous
		// clipboard requires; a refused write is reported rather than swallowed,
		// because a copy button that silently does nothing is worse than none.
		let next: 'copied' | 'failed' = 'copied';
		try {
			await navigator.clipboard.writeText(server.url);
		} catch {
			next = 'failed';
		}
		setCopyState(next);
		if (copyTimer.current !== null) window.clearTimeout(copyTimer.current);
		copyTimer.current = window.setTimeout(() => setCopyState('idle'), 2600);
	};

	return (
		<Dialog open={open} onOpenChange={(next) => !next && onClose()}>
			<DialogContent className="settings-dialog" aria-describedby="player-settings-description">
				<header className="settings-header">
					<span className="settings-heading-icon" aria-hidden="true"><Cog size={21} /></span>
					<div><DialogTitle>{t('settings')}</DialogTitle><DialogDescription id="player-settings-description">{t('settingsDescription')}</DialogDescription></div>
					<DialogClose asChild><Button className="settings-close" variant="ghost" size="icon" aria-label={t('closeSettings')}><X size={18} /></Button></DialogClose>
				</header>
				<div className="settings-panes">
					<nav className="settings-nav" aria-label={t('settingsSections')}>
						{panes.map(({ id, icon: Icon, label }) => (
							<button key={id} type="button" className="settings-nav-item" aria-current={pane === id ? 'true' : undefined} onClick={() => setPane(id)}>
								<Icon size={16} strokeWidth={1.8} aria-hidden="true" />
								<span>{label}</span>
							</button>
						))}
					</nav>
					<div className="settings-panel" role="region" aria-label={t(pane)}>
						{pane === 'interface' && (
							<>
								<section className="settings-section">
									<div className="settings-section-copy"><h3>{t('interface')}</h3><p>{t('languageHint')}</p></div>
									<div className="settings-segment" role="group" aria-label={t('language')}>{(['fr', 'en'] as const).map((locale) => <button key={locale} type="button" aria-pressed={draftLanguage === locale} onClick={() => setDraftLanguage(locale)}>{locale === 'fr' ? 'Français' : 'English'}</button>)}</div>
								</section>
								<div className="settings-separator" />
								<section className="settings-section settings-section--row">
									<div className="settings-section-copy"><h3>{t('autoHideControls')}</h3><p>{t('autoHideControlsHint')}</p></div>
									<Switch checked={draft.autoHideControls} onCheckedChange={(checked) => setDraft((current) => ({ ...current, autoHideControls: checked }))} aria-label={t('autoHideControls')} />
								</section>
								<div className="settings-separator" />
								<section className="settings-section settings-section--row">
									<div className="settings-section-copy"><h3>{t('reducedMotion')}</h3><p>{t('reducedMotionHint')}</p></div>
									<Switch checked={draft.reducedMotion} onCheckedChange={(checked) => setDraft((current) => ({ ...current, reducedMotion: checked }))} aria-label={t('reducedMotion')} />
								</section>
							</>
						)}
						{pane === 'playback' && (
							<>
								<section className="settings-section">
									<div className="settings-section-copy"><h3>{t('playback')}</h3><p>{t('playbackHint')}</p></div>
								</section>
								<div className="settings-separator" />
								<section className="settings-section settings-section--row">
									<div className="settings-section-copy"><h3>{t('autoPlayNext')}</h3><p>{t('autoPlayNextHint')}</p></div>
									<Switch checked={draft.playback.autoPlayNext} onCheckedChange={(checked) => patchPlayback({ autoPlayNext: checked })} aria-label={t('autoPlayNext')} />
								</section>
								<div className="settings-separator" />
								<section className="settings-section settings-section--row">
									<div className="settings-section-copy"><h3>{t('audioLanguage')}</h3><p>{t('audioLanguageHint')}</p></div>
									<SettingsChoices
										label={t('audioLanguage')}
										options={[{ value: 'auto', key: 'audioAuto' }, { value: 'vf', key: 'audioVf' }, { value: 'vo', key: 'audioVo' }]}
										value={draft.playback.audioLanguage}
										onPick={(audioLanguage) => patchPlayback({ audioLanguage })}
										t={t}
									/>
								</section>
								<div className="settings-separator" />
								<section className="settings-section settings-section--row">
									<div className="settings-section-copy"><h3>{t('subtitleLanguage')}</h3><p>{t('subtitleLanguageHint')}</p></div>
									<SettingsChoices
										label={t('subtitleLanguage')}
										options={[{ value: 'auto', key: 'subtitleAuto' }, { value: 'none', key: 'subtitleNone' }, { value: 'fr', key: 'subtitleFr' }, { value: 'en', key: 'subtitleEn' }]}
										value={draft.playback.subtitleLanguage}
										onPick={(subtitleLanguage) => patchPlayback({ subtitleLanguage })}
										t={t}
									/>
								</section>
							</>
						)}
						{pane === 'subtitles' && (
							<SubtitlePane
								subtitles={draft.playback.subtitleStyle}
								t={t}
								onStyle={patchStyle}
								onReset={() => patchStyle(PLAYBACK_DEFAULTS.subtitleStyle)}
							/>
						)}
						{pane === 'watching' && (
							<section className="settings-section settings-stats">
								<div className="settings-section-copy"><h3>{t('watching')}</h3><p>{t('watchingHint').replace('{who}', viewer)}</p></div>
{watching ? (
									<>
										<div className="settings-stats-block">
											<p className="settings-stats-block-title">{t('overview')}</p>
											<ul className="settings-stats-tiles">
												<li><Clock size={16} strokeWidth={1.8} aria-hidden="true" /><b>{spent(watchedSeconds) ?? t('noTime')}</b><span>{t('totalTime')}</span></li>
												<li><Film size={16} strokeWidth={1.8} aria-hidden="true" /><b>{watching.movies.finished}</b><span>{t('watchedMovies')}</span></li>
												<li><Tv size={16} strokeWidth={1.8} aria-hidden="true" /><b>{watching.series.finished}</b><span>{t('watchedSeries')}</span></li>
												<li><ListVideo size={16} strokeWidth={1.8} aria-hidden="true" /><b>{watching.episodes.finished}</b><span>{t('watchedEpisodes')}</span></li>
												<li><Percent size={16} strokeWidth={1.8} aria-hidden="true" /><b>{completion === null ? t('noTime') : `${completion}%`}</b><span>{t('completionRate')}</span></li>
											</ul>
										</div>
										<div className="settings-stats-block">
											<p className="settings-stats-block-title">{t('thisMonth')}</p>
											<p className="settings-stats-month">
												<Clock size={15} strokeWidth={1.8} aria-hidden="true" />
												<span>{countFilms(watching.month.movies)} · {countEpisodes(watching.month.episodes)} · {spent(watching.month.seconds) ?? t('noTime')} {t('watchedWord')}</span>
											</p>
										</div>
										{share !== null && (
											<div className="settings-stats-block">
												<p className="settings-stats-block-title">{t('ratio')}</p>
												<PartitionBar size="md" gap={2}>
													<PartitionBarSegment num={share} variant="default" alignment="left">
														<PartitionBarSegmentTitle>{t('films')}</PartitionBarSegmentTitle>
														<PartitionBarSegmentValue>{share}%</PartitionBarSegmentValue>
													</PartitionBarSegment>
													<PartitionBarSegment num={100 - share} variant="secondary" alignment="right">
														<PartitionBarSegmentTitle>{t('series')}</PartitionBarSegmentTitle>
														<PartitionBarSegmentValue>{100 - share}%</PartitionBarSegmentValue>
													</PartitionBarSegment>
												</PartitionBar>
											</div>
										)}
										{watching.top_series.length > 0 ? (
											<div className="settings-stats-block">
												<p className="settings-stats-block-title">{t('topSeries')}</p>
												<ul className="settings-stats-ranking">
													{watching.top_series.map((one) => (
														<li key={one.id}>
															{/* crossOrigin, like the cards: the shell's origin is its own
															    and the server admits an artwork read from it only when the
															    request carries that origin, which a plain image element does
															    not send. Without it the answer is a 403 the log never sees
															    and the pane draws a broken picture - measured on 24
															    September 2026, and the reason this line is commented. */}
															<img className="settings-stats-poster" src={one.poster_url || imageURL(one.poster_path, 'w185') || notFoundArt} alt="" crossOrigin="anonymous" decoding="async" />
															<div className="settings-stats-row">
																<div className="settings-stats-row-head">
																	<span className="settings-stats-name">{one.title}</span>
																	<span className="settings-stats-detail">{(one.total === 1 ? t('episodesOfOne') : t('episodesOf')).replace('{seen}', String(one.episodes)).replace('{total}', String(one.total))}{spent(one.seconds) ? ` · ${spent(one.seconds)}` : ''}</span>
																</div>
																<div className="settings-stats-progress" role="img" aria-label={(one.total === 1 ? t('episodesOfOne') : t('episodesOf')).replace('{seen}', String(one.episodes)).replace('{total}', String(one.total))}>
																	<motion.span initial={{ width: 0 }} animate={{ width: `${one.total > 0 ? Math.round((one.episodes / one.total) * 100) : 0}%` }} transition={{ duration: statsBarDuration(preferences.reducedMotion) }} />
																</div>
															</div>
														</li>
													))}
												</ul>
											</div>
										) : <p>{t('nothingWatched')}</p>}
									</>
																) : <p>{t('watchingUnavailable')}</p>}
							</section>
						)}
						{pane === 'server' && (
							<section className="settings-section settings-section--connection">
								<div className="settings-section-copy"><h3>{t('server')}</h3><p>{server ? t('serverConnectedHint') : t('serverDisconnectedHint')}</p></div>
								{server && <dl className="settings-server">
									<div>
										<dt>{t('address')}</dt>
										<dd className="settings-address">
											<span className="settings-address-value">{server.url}</span>
											<button type="button" className="settings-copy" onClick={() => void copyAddress()} aria-label={t('copyAddress')} title={t('copyAddress')}><Copy size={15} strokeWidth={1.8} /></button>
										</dd>
									</div>
									<div><dt>{t('version')}</dt><dd>{server.health.version}</dd></div>
								</dl>}
								{copyState !== 'idle' && <p className={`settings-copy-note${copyState === 'failed' ? ' settings-copy-note--error' : ''}`} role="status">{copyState === 'copied' ? t('addressCopied') : t('addressCopyFailed')}</p>}
							</section>
						)}
						{pane === 'update' && (
							<section className="settings-section settings-update">
								<div className="settings-section-copy"><h3>{t('update')}</h3><p>{updateStatus?.message || t(updateStateKey)}</p></div>
								<div className="settings-update-row">
									<dl className="settings-server settings-update-versions"><div><dt>{t('updateCurrent')}</dt><dd>{updateStatus?.current_version || server?.health.version || '—'}</dd></div>{updateStatus?.latest_version && <div><dt>{t('updateLatest')}</dt><dd>{updateStatus.latest_version}</dd></div>}</dl>
									{updateStatus?.available ? <Button onClick={onApplyUpdate} disabled={updateBusy || ['downloading', 'ready'].includes(updateStatus.state)}>{updateBusy ? t('updateInstalling') : t('updateInstall')}</Button> : <Button variant="outline" onClick={onCheckUpdate} disabled={updateBusy}>{updateBusy ? t('updateChecking') : t('updateCheck')}</Button>}
								</div>
							</section>
						)}
					</div>
				</div>
				{/* The two actions wear the reference's own shape: a bordered,
				    quiet Cancel beside a filled Save, both rounded rectangles
				    rather than the pills the player's controls use - a dialog's
				    footer is not a control on the picture. */}
				<footer className="settings-footer"><DialogClose asChild><Button variant="outline">{t('cancel')}</Button></DialogClose><Button onClick={() => onSave(draftLanguage, draft)}>{t('save')}</Button></footer>
			</DialogContent>
		</Dialog>
	);
}

function ProfileDialog({ open, profiles, activeProfile, serverURL, busy, t, onClose, onSelect, onRename, onSetAvatar, onClearAvatar }: { open: boolean; profiles: Profile[]; activeProfile: number | null; serverURL: string; busy: boolean; t: (key: string) => string; onClose: () => void; onSelect: (id: number) => void; onRename: (id: number, name: string) => Promise<Profile>; onSetAvatar: (id: number, file: File) => Promise<Profile>; onClearAvatar: (id: number) => Promise<Profile> }) {
	const [editingID, setEditingID] = useState<number | null>(null);
	const [draftName, setDraftName] = useState('');
	const [draftFile, setDraftFile] = useState<File | null>(null);
	const [previewURL, setPreviewURL] = useState<string | null>(null);
	const [removeAvatar, setRemoveAvatar] = useState(false);
	const [formError, setFormError] = useState(false);
	const editing = profiles.find((profile) => profile.id === editingID) ?? null;
	useEffect(() => () => { if (previewURL) URL.revokeObjectURL(previewURL); }, [previewURL]);
	useEffect(() => {
		if (!open) setEditingID(null);
	}, [open]);
	const edit = (profile: Profile) => {
		setEditingID(profile.id);
		setDraftName(profile.name || t('profileDefaultName'));
		setDraftFile(null);
		setPreviewURL(null);
		setRemoveAvatar(false);
		setFormError(false);
	};
	const chooseFile = (file?: File) => {
		if (!file) return;
		setDraftFile(file);
		setPreviewURL(URL.createObjectURL(file));
		setRemoveAvatar(false);
	};
	const saveProfile = async (event: FormEvent) => {
		event.preventDefault();
		if (!editing) return;
		setFormError(false);
		try {
			if (draftName.trim() !== (editing.name || t('profileDefaultName'))) await onRename(editing.id, draftName.trim());
			if (draftFile) await onSetAvatar(editing.id, draftFile);
			else if (removeAvatar && editing.has_avatar) await onClearAvatar(editing.id);
			setEditingID(null);
		} catch {
			setFormError(true);
		}
	};
	return (
		<Dialog open={open} onOpenChange={(next) => !next && onClose()}>
			<DialogContent className="profiles-dialog" aria-describedby="profiles-description">
				<header className="settings-header">
					<span className="settings-heading-icon" aria-hidden="true">{editing ? <Pencil size={20} /> : <UserRound size={21} />}</span>
					<div><DialogTitle>{editing ? t('personalizeProfile') : t('profiles')}</DialogTitle><DialogDescription id="profiles-description">{editing ? t('personalizeProfileDescription') : t('profileDescription')}</DialogDescription></div>
					<DialogClose asChild><Button className="settings-close" variant="ghost" size="icon" aria-label={t('closeProfiles')}><X size={18} /></Button></DialogClose>
				</header>
				{editing ? <form className="profile-editor" onSubmit={saveProfile}>
					<div className="profile-editor-avatar" style={{ '--profile-hue': `${(editing.id * 71) % 360}` } as React.CSSProperties}>
						{previewURL || (!removeAvatar && profileAvatarURL(serverURL, editing)) ? <img src={previewURL || profileAvatarURL(serverURL, editing) || ''} alt="" crossOrigin="anonymous" /> : <span>{(draftName || t('profileDefaultName')).trim().slice(0, 1).toUpperCase()}</span>}
					</div>
					<label className="profile-editor-field"><span className="label">{t('profileName')}</span><input value={draftName} onChange={(event) => setDraftName(event.target.value)} maxLength={40} required /></label>
					<div className="profile-picture-actions">
						<label className="profile-file-button"><ImagePlus size={18} /><span>{t('changePicture')}</span><input type="file" accept="image/png,image/jpeg,image/webp" onChange={(event) => chooseFile(event.target.files?.[0])} /></label>
						{editing.has_avatar && !removeAvatar && !draftFile && <Button type="button" variant="ghost" onClick={() => setRemoveAvatar(true)}>{t('removePicture')}</Button>}
					</div>
					<p className="profile-picture-hint">{t('profilePictureHint')}</p>
					{formError && <p className="hint hint--error" role="status">{t('profileSaveFailed')}</p>}
					<footer className="profile-editor-actions"><Button type="button" variant="ghost" onClick={() => setEditingID(null)}>{t('back')}</Button><Button type="submit" disabled={busy || !draftName.trim()}>{busy ? t('savingProfile') : t('saveProfile')}</Button></footer>
				</form> : <div className="profile-list">
					{profiles.map((profile, index) => {
						const active = profile.id === activeProfile;
						const name = profile.name || t('profileDefaultName');
						const avatar = profileAvatarURL(serverURL, profile);
						return <div key={profile.id} className={`profile-card ${active ? 'profile-card--active' : ''}`} style={{ '--profile-hue': `${(profile.id * 71 + index * 43) % 360}` } as React.CSSProperties}><button className="profile-card-select" onClick={() => onSelect(profile.id)} disabled={busy}><span className="profile-avatar">{avatar ? <img src={avatar} alt="" /> : name.trim().slice(0, 1).toUpperCase()}</span><span className="profile-copy"><strong>{name}</strong><small>{active ? t('currentProfile') : t('switchProfile')}</small></span>{active && <Check size={19} aria-hidden="true" />}</button><Button className="profile-edit" type="button" variant="ghost" size="icon" aria-label={`${t('personalizeProfile')} · ${name}`} onClick={() => edit(profile)}><Pencil size={17} /></Button></div>;
					})}
					{profiles.length === 0 && <p className="hint">{t('noProfiles')}</p>}
				</div>}
			</DialogContent>
		</Dialog>
	);
}

type PlaybackProps = {
	visible: boolean; status: PlayerStatus; seconds: number; duration: number; progress: number; fullscreen: boolean;
	language: string; tracks: Track[]; words: TrackVocabulary; trackMenuOpen: boolean; trackButton: React.RefObject<HTMLButtonElement | null>;
	qualities: QualityLadder | null; currentQuality: number | null; onPickQuality: (height: number | null) => void;
	volume: number; onVolume: (value: number) => void;
	trackMenu: React.RefObject<TrackMenuHandle | null>; t: (key: string) => string; onToggle: () => void;
	onSeek: (seconds: number) => void; onSeekAbsolute: (seconds: number) => void; onMute: () => void;
	onTracks: () => void; onPickTrack: (kind: 'audio' | 'sub', id: number | null) => void;
	onLanguage: () => void; onFullscreen: () => void;
};

function PlaybackControls(props: PlaybackProps) {
	const scrub = (event: ReactMouseEvent<HTMLDivElement>) => {
		if (!props.duration) return;
		const rect = event.currentTarget.getBoundingClientRect();
		props.onSeekAbsolute(Math.max(0, Math.min(1, (event.clientX - rect.left) / rect.width)) * props.duration);
	};
	const scrubKey = (event: ReactKeyboardEvent<HTMLDivElement>) => {
		if (event.key === 'ArrowLeft' || event.key === 'ArrowDown') { event.preventDefault(); props.onSeek(-10); }
		else if (event.key === 'ArrowRight' || event.key === 'ArrowUp') { event.preventDefault(); props.onSeek(10); }
		else if (event.key === 'Home') { event.preventDefault(); props.onSeekAbsolute(0); }
		else if (event.key === 'End') { event.preventDefault(); props.onSeekAbsolute(props.duration); }
	};
	return (
		<div className={`controls ${props.visible ? '' : 'controls--hidden'}`}>
			<div className="scrub" role="slider" tabIndex={0} aria-label={props.t('play')} aria-valuemin={0} aria-valuemax={Math.round(props.duration)} aria-valuenow={Math.round(props.seconds)} onClick={scrub} onKeyDown={scrubKey}>
				<div className="scrub-track"><div className="scrub-played" style={{ width: `${props.progress * 100}%` }} /></div>
				<div className="scrub-thumb" style={{ left: `${props.progress * 100}%` }} />
			</div>
			<div className="row">
				<Button className="control control--skip" variant="ghost" size="icon" onClick={() => props.onSeek(-10)} aria-label={props.t('back10')}><RotateCcw size={21} /></Button>
				<Button className="control control--primary" size="icon" onClick={props.onToggle} aria-label={props.status.pause ? props.t('play') : props.t('pause')}>{props.status.pause ? <Play size={20} fill="currentColor" /> : <Pause size={20} />}</Button>
				<Button className="control control--skip" variant="ghost" size="icon" onClick={() => props.onSeek(10)} aria-label={props.t('forward10')}><RotateCw size={21} /></Button>
				<span className="clock"><span className="elapsed">{clock(props.seconds)}</span><span className="rule" /><span className="total">{clock(props.duration)}</span></span>
				<span className="spacer" />
				<div className="menu-anchor">
					<Button ref={props.trackButton} className="control" variant="ghost" size="icon" onClick={props.onTracks} aria-label={props.t('tracks')} aria-haspopup="dialog" aria-expanded={props.trackMenuOpen}><Settings2 size={21} /></Button>
					<AnimatePresence>{props.trackMenuOpen && <TrackMenu ref={props.trackMenu} tracks={props.tracks} words={props.words} qualities={props.qualities} currentQuality={props.currentQuality} onPick={props.onPickTrack} onPickQuality={props.onPickQuality} t={props.t} />}</AnimatePresence>
				</div>
				{/* The slider is the volume; the button is the mute. The engine
				    keeps the two apart, so the thumb shows the level a press on
				    the icon would bring back rather than reporting zero - the
				    struck-through icon already says the sound is off. */}
				<div className="player-volume">
					<Button className="control control--mute" variant="ghost" size="icon" onClick={props.onMute} aria-label={props.status.mute ? props.t('unmute') : props.t('mute')}>{props.status.mute || props.volume === 0 ? <VolumeX size={21} /> : props.volume < 0.5 ? <Volume1 size={21} /> : <Volume2 size={21} />}</Button>
					<input
						type="range"
						className="volume-slider"
						min={0}
						max={1}
						step={0.02}
						value={props.volume}
						onChange={(event) => props.onVolume(Number(event.currentTarget.value))}
						aria-label={props.t('volume')}
					/>
				</div>
				{props.status.audioMode && <span className="label audio-mode">{props.status.audioMode === 'passthrough' ? 'BITSTREAM' : props.status.audioMode.toUpperCase()}</span>}
				<Button className="control control--desktop" variant="ghost" size="icon" onClick={props.onLanguage} aria-label="Français / English"><Languages size={20} /><span className="sr-only">{props.language}</span></Button>
				<Button className="control" variant="ghost" size="icon" onClick={props.onFullscreen} aria-label={props.fullscreen ? props.t('exitFullscreen') : props.t('fullscreen')} aria-pressed={props.fullscreen}>{props.fullscreen ? <Minimize2 size={21} /> : <Maximize2 size={21} />}</Button>
			</div>
		</div>
	);
}

function clock(value: number) {
	if (!Number.isFinite(value) || value <= 0) return '--:--';
	const total = Math.floor(value);
	const h = Math.floor(total / 3600);
	const m = Math.floor((total % 3600) / 60);
	const s = total % 60;
	return h ? `${h}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}` : `${m}:${String(s).padStart(2, '0')}`;
}
