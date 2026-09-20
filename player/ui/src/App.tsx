import { AnimatePresence, MotionConfig, motion } from 'motion/react';
import {
	ArrowLeft,
	ArrowRight,
	Check,
	ChevronLeft,
	ChevronRight,
	Clapperboard,
	Cog,
	House,
	ImagePlus,
	Languages,
	Maximize2,
	Minimize2,
	Pause,
	Pencil,
	Play,
	RotateCcw,
	RotateCw,
	Search,
	Settings2,
	Tv,
	UserRound,
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
import { Switch } from './components/ui/switch';
import notFoundArt from './assets/media-not-found.png';
import { catalogues, initialLanguage, storedLanguage } from './lib/catalogues.js';
import { demoInvoke, demoListen, isDemoLibrary } from './lib/demo';
import { artworkCandidates, displayTitle, displayYear } from './lib/tmdb';
import { formatRuntime } from './lib/utils';
import type { DiscoveredServer, Home, HomeRow, Movie, PlayerStatus, Profile, Season, Series, SeriesHome, Server, Track, UpdateStatus } from './types';

const invoke = async <T,>(command: string, args?: Record<string, unknown>): Promise<T> => {
	if (isDemoLibrary) return demoInvoke<T>(command, args);
	const call = window.__TAURI__?.core?.invoke;
	if (!call) return undefined as T;
	return call<T>(command, args);
};
const listen = isDemoLibrary ? demoListen : (window.__TAURI__?.event?.listen ?? (async () => () => {}));
const getAppWindow = () => window.__TAURI__?.window?.getCurrentWindow?.();
const IDLE_MS = 3000;

type Section = 'home' | 'films' | 'series' | 'search';
type Preferences = { reducedMotion: boolean; autoHideControls: boolean };

function initialPreferences(): Preferences {
	try {
		const value = JSON.parse(localStorage.getItem('theia.player.preferences') ?? '{}');
		return {
			reducedMotion: Boolean(value.reducedMotion),
			autoHideControls: value.autoHideControls !== false,
		};
	} catch {
		return { reducedMotion: false, autoHideControls: true };
	}
}

export default function App() {
	const location = useLocation();
	const navigate = useNavigate();
	const [language, setLanguage] = useState(initialLanguage());
	const [preferences, setPreferences] = useState(initialPreferences);
	const catalogue = catalogues[language as keyof typeof catalogues];
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
	const [noticeKey, setNoticeKey] = useState<string | null>(null);
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
	const [trackMenuOpen, setTrackMenuOpen] = useState(false);
	const [focusInFurniture, setFocusInFurniture] = useState(false);
	const idleTimer = useRef<number | null>(null);
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

	useEffect(() => {
		const cleanups: Array<() => void> = [];
		let disposed = false;
		listen('player-status', (event) => {
			try {
				setStatus(JSON.parse(String(event.payload)) as PlayerStatus);
			} catch {
				// A malformed frame is ignored; the next one arrives in 500 ms.
			}
		}).then((cleanup) => (disposed ? cleanup() : cleanups.push(cleanup)));
		listen('player-event', (event) => {
			try {
				const value = JSON.parse(String(event.payload));
				if (value.kind === 'audio' && value.mode === 'pcm') setNoticeKey('audioFallback');
				if (value.kind === 'engine' && value.state === 'unavailable') setNoticeKey('engineUnavailable');
			} catch {
				// Same contract as status frames.
			}
		}).then((cleanup) => (disposed ? cleanup() : cleanups.push(cleanup)));
		listen('tauri://resize', async () => {
			const appWindow = getAppWindow();
			try {
				setFullscreen(Boolean(await appWindow?.isFullscreen()));
				setMaximized(Boolean(await appWindow?.isMaximized()));
			} catch {
				// Keep the last known state.
			}
		}).then((cleanup) => (disposed ? cleanup() : cleanups.push(cleanup)));
		return () => {
			disposed = true;
			cleanups.forEach((cleanup) => cleanup());
		};
	}, []);

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

	const playMovie = async (id: number) => {
		try {
			await invoke('player_play', { id });
		} catch {
			setErrorKey('playFailed');
		}
	};
	const playEpisode = async (id: number) => {
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
		try {
			await invoke('player_stop');
			setStatus((current) => ({ ...current, media: null, title: null, pos: null, duration: null, pause: false }));
			await Promise.all([loadLibrary(), loadHome()]);
		} catch {
			setErrorKey('stopFailed');
		} finally {
			setReturning(false);
		}
	}, [loadHome, loadLibrary, returning, status.media]);

	const refreshTracks = async () => {
		try {
			setTracks(JSON.parse(await invoke<string>('player_tracks')) as Track[]);
		} catch {
			setTracks([]);
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
	const toggleMute = () => {
		wake();
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
				trackMenu.current?.moveFocus(event.key === 'ArrowDown' || event.key === 'ArrowRight' ? 1 : -1);
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
	}, [fullscreen, profilesOpen, returnToLibrary, seek, selectedSeries, setFullscreenState, settingsOpen, status.media, switchLanguage, toggle, toggleFullscreen, trackMenuOpen, wake]);

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

				{noticeKey && <div className="notice" role="status"><span className="label">{t('audioFallbackLabel')}</span>{t(noticeKey)}</div>}
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
					visible={Boolean(status.media)} status={status} seconds={seconds} duration={duration} progress={progress}
					fullscreen={fullscreen} language={language} tracks={tracks} trackMenuOpen={trackMenuOpen}
					trackButton={trackButton} trackMenu={trackMenu} t={t} onToggle={toggle} onSeek={seek}
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
	// failed; demo items always carry their own art, so demo is untouched.
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
			<div className="home-hero-veil" aria-hidden="true" />
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

function SettingsModal({ open, language, preferences, server, updateStatus, updateBusy, t, onClose, onSave, onCheckUpdate, onApplyUpdate }: { open: boolean; language: string; preferences: Preferences; server: Server | null; updateStatus: UpdateStatus | null; updateBusy: boolean; t: (key: string) => string; onClose: () => void; onSave: (language: string, preferences: Preferences) => void; onCheckUpdate: () => void; onApplyUpdate: () => void }) {
	const [draftLanguage, setDraftLanguage] = useState(language);
	const [draft, setDraft] = useState(preferences);
	const updateStateKey = updateStatus ? ({ idle: 'updateUnknown', checking: 'updateChecking', available: 'updateAvailable', downloading: 'updateInstalling', ready: 'updateReady', deferred: 'updateDeferred', failed: 'updateFailed', unsupported: 'updateUnsupported' } as Record<string, string>)[updateStatus.state] ?? 'updateUnknown' : 'updateUnknown';
	useEffect(() => {
		if (open) {
			setDraftLanguage(language);
			setDraft(preferences);
		}
	}, [language, open, preferences]);

	return (
		<Dialog open={open} onOpenChange={(next) => !next && onClose()}>
			<DialogContent aria-describedby="player-settings-description">
				<header className="settings-header">
					<span className="settings-heading-icon" aria-hidden="true"><Cog size={21} /></span>
					<div><DialogTitle>{t('settings')}</DialogTitle><DialogDescription id="player-settings-description">{t('settingsDescription')}</DialogDescription></div>
					<DialogClose asChild><Button className="settings-close" variant="ghost" size="icon" aria-label={t('closeSettings')}><X size={18} /></Button></DialogClose>
				</header>
				<div className="settings-body">
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
					<div className="settings-separator" />
					<section className="settings-section settings-section--connection">
						<div className="settings-section-copy"><h3>{t('server')}</h3><p>{server ? t('serverConnectedHint') : t('serverDisconnectedHint')}</p></div>
						{server && <dl className="settings-server"><div><dt>{t('address')}</dt><dd>{server.url}</dd></div><div><dt>{t('version')}</dt><dd>{server.health.version}</dd></div></dl>}
					</section>
					<div className="settings-separator" />
					<section className="settings-section settings-update">
						<div className="settings-section-copy"><h3>{t('update')}</h3><p>{updateStatus?.message || t(updateStateKey)}</p></div>
						<div className="settings-update-row">
							<dl className="settings-server settings-update-versions"><div><dt>{t('updateCurrent')}</dt><dd>{updateStatus?.current_version || server?.health.version || '—'}</dd></div>{updateStatus?.latest_version && <div><dt>{t('updateLatest')}</dt><dd>{updateStatus.latest_version}</dd></div>}</dl>
							{updateStatus?.available ? <Button onClick={onApplyUpdate} disabled={updateBusy || ['downloading', 'ready'].includes(updateStatus.state)}>{updateBusy ? t('updateInstalling') : t('updateInstall')}</Button> : <Button variant="outline" onClick={onCheckUpdate} disabled={updateBusy}>{updateBusy ? t('updateChecking') : t('updateCheck')}</Button>}
						</div>
					</section>
				</div>
				<footer className="settings-footer"><DialogClose asChild><Button variant="ghost">{t('cancel')}</Button></DialogClose><Button onClick={() => onSave(draftLanguage, draft)}>{t('save')}</Button></footer>
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
	language: string; tracks: Track[]; trackMenuOpen: boolean; trackButton: React.RefObject<HTMLButtonElement | null>;
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
					<Button ref={props.trackButton} className="control" variant="ghost" size="icon" onClick={props.onTracks} aria-label={props.t('tracks')} aria-haspopup="menu" aria-expanded={props.trackMenuOpen}><Settings2 size={21} /></Button>
					<AnimatePresence>{props.trackMenuOpen && <TrackMenu ref={props.trackMenu} tracks={props.tracks} onPick={props.onPickTrack} t={props.t} />}</AnimatePresence>
				</div>
				<Button className="control control--mute" variant="ghost" size="icon" onClick={props.onMute} aria-label={props.status.mute ? props.t('unmute') : props.t('mute')}>{props.status.mute ? <VolumeX size={21} /> : <Volume2 size={21} />}</Button>
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
