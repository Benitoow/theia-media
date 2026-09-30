import { AnimatePresence, motion } from 'motion/react';
import { ArrowLeft, ChevronLeft, ChevronRight, Clapperboard, Cog, House, Search, Tv, UserRound } from 'lucide-react';
import { Children, FormEvent, KeyboardEvent as ReactKeyboardEvent, useCallback, useEffect, useRef, useState } from 'react';

import { lazy, Suspense } from 'react';
import { CardGrid } from './CardGrid';
import { EpisodeCard } from './EpisodeCard';
import { MediaCard } from './MediaCard';
const MovieDetail = lazy(() => import('./MovieDetail').then((module) => ({ default: module.MovieDetail })));
const SeriesDetail = lazy(() => import('./SeriesDetail').then((module) => ({ default: module.SeriesDetail })));
import { PlaybackHero } from './PlaybackHero';

import { Button } from './ui/button';

import { artworkCandidates, displayTitle, displayYear } from '../lib/tmdb';
import { searchText } from '../lib/utils';
import { resumeEpisode, watchedAt } from '../lib/progress';

import type { DiscoveredServer, Home, Movie, Profile, Season, Series, SeriesHome, Server, UpdateStatus } from '../types';

import type { Section } from '../types';
import { profileAvatarURL } from '../lib/profiles';
type LibraryProps = {
	server: Server | null; booting: boolean; busy: boolean; address: string; discovered: DiscoveredServer[];
	movies: Movie[]; series: Series[]; section: Section; settingsOpen: boolean; profilesOpen: boolean; updateStatus: UpdateStatus | null;
	searchQuery: string; selectedMovie: Movie | null; selectedSeries: Series | null; selectedSeason: Season | null;
	home: Home | null; seriesHome: SeriesHome | null; homeError: boolean; language: string;
	errorKey: string | null; reducedMotion: boolean; t: (key: string) => string;
	onAddress: (value: string) => void; onSubmit: (event: FormEvent) => void; onFind: () => void;
	onConnect: (url: string) => void; onSection: (value: Section) => void; onSettings: () => void; onProfiles: () => void;
	onSearchQuery: (value: string) => void;
	onMovie: (id: number) => void; onPlayMovie: (id: number, fileId?: number) => void; onBackMovie: () => void; onSeries: (id: number) => void; onEpisode: (id: number) => void;
	onCancel: () => void; onHelp: () => void; historyBusy: boolean; onHistory: (id:number, watched:boolean) => void;
    onSeason: (number: number) => void; onBackSeries: () => void;
};

export function Library(props: LibraryProps) {
	const { server, selectedMovie, selectedSeries, selectedSeason, section, t } = props;
    const scroller = useRef<HTMLElement>(null);
    const scrollPositions = useRef(new Map<string, number>());
    const restoring = useRef(false);
    const scope = `${server?.url ?? ''}:${server?.profile ?? ''}`;
    const view = `${scope}:${selectedMovie ? 'movie:'+selectedMovie.id : selectedSeries ? 'series:'+selectedSeries.id+':'+selectedSeason?.season_number : section === 'search' ? 'search:'+props.searchQuery : section}`;
    useEffect(() => {
        const wanted = scrollPositions.current.get(view) ?? 0;
        restoring.current = true;
        let frame = 0, attempts = 0;
        const restore = () => {
            const element = scroller.current;
            if (!element) return;
            // AnimatePresence first removes the detail, then mounts the grid.
            // Its initial window has no spacer yet; restoring into that short
            // document would clamp and overwrite the remembered scroll position.
            if (element.scrollHeight - element.clientHeight < wanted && attempts++ < 60) {
                frame = requestAnimationFrame(restore); return;
            }
            element.scrollTop = wanted; restoring.current = false;
        };
        frame = requestAnimationFrame(restore);
        return () => { cancelAnimationFrame(frame); restoring.current = false; };
    }, [view]);

	const resumedEpisode = selectedSeries?.resume_episode ?? resumeEpisode([
		...(props.seriesHome?.continue_watching ?? []), ...(selectedSeason?.episodes ?? []),
	].filter((episode) => episode.series_id === selectedSeries?.id));
	const heroEpisode = resumedEpisode ?? selectedSeries?.next_unwatched;
    const title = selectedMovie ? displayTitle(selectedMovie) : selectedSeries ? displayTitle(selectedSeries) : undefined;
	const count = section === 'series' ? props.series.length : section === 'search' ? props.movies.length + props.series.length : props.movies.length;
	const spotlightSource = section === 'series' ? props.series[0] : props.movies[0];
	// The ambient picture is the same size of frame as the home's hero - a
	// wallpaper behind the nav - so it draws the player's window-sized URL too.
	const spotlight = spotlightSource ? (spotlightSource.hero_url ?? artworkCandidates(spotlightSource, 'w1280')[0]) : undefined;

	return (
		<section className="library" ref={scroller} onScroll={(event) => { if (!restoring.current) scrollPositions.current.set(view, event.currentTarget.scrollTop); }}>
            {server && <button className="help-trigger" onClick={props.onHelp} aria-label={t('keyboardHelp')}>?</button>}
			{spotlight && !selectedMovie && !selectedSeries && section !== 'home' && section !== 'search' && <div className="library-ambient" aria-hidden="true"><img src={spotlight} alt="" crossOrigin="anonymous" /></div>}
			{server && <LibraryNav section={section} settingsOpen={props.settingsOpen} profilesOpen={props.profilesOpen} profiles={server.profiles ?? []} activeProfile={server.profile ?? null} serverURL={server.url} updateAvailable={Boolean(props.updateStatus?.available)} t={t} onSection={props.onSection} onSettings={props.onSettings} onProfiles={props.onProfiles} />}
			{server && props.errorKey && <p className="hint hint--error" role="alert">{t(props.errorKey)}</p>}

			{/* The home screen has no page heading: its hero is the heading. A
			   series opened from one of the home's rows keeps its own. */}
			{!(section === 'home' && !selectedMovie && !selectedSeries && server && !props.booting) && <div className={section === 'search' ? 'library-heading library-heading--centered' : 'library-heading'}>
				{selectedMovie && <button className="library-back" onClick={props.onBackMovie}><ArrowLeft size={18} />{t(section === 'home' ? 'home' : 'allFilms')}</button>}
				{selectedSeries && <button className="library-back" onClick={props.onBackSeries}><ArrowLeft size={18} />{t(section === 'home' ? 'home' : 'allSeries')}</button>}
				{/* The search room is a centred stage with no eyebrow: the loop
				   is the decoration and "Your library" said nothing there. */}
				{section !== 'search' && <p className="library-eyebrow label">{server ? (selectedMovie ? t('filmSingular') : selectedSeries ? t('seriesLabel') : t('yourLibrary')) : t('desktopPlayer')}</p>}
				{!selectedMovie && !selectedSeries && <h1 className="library-title">{title || (server ? (section === 'series' ? t('series') : section === 'search' ? t('searchTitle') : t('allFilms')) : props.booting ? t('starting') : t('connectTitle'))}</h1>}
				{server && !selectedMovie && !selectedSeries && section !== 'search' && <p className="library-count label">{count} {t(section === 'series' ? (count === 1 ? 'seriesSingular' : 'seriesPlural') : (count === 1 ? 'filmSingular' : 'filmPlural'))}</p>}
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
                                {props.busy && <Button type="button" variant="outline" onClick={props.onCancel}>{t('cancel')}</Button>}
								<Button type="button" variant="outline" disabled={props.busy} onClick={props.onFind}>{t('findServers')}</Button>
							</div>
						</form>
						{props.discovered.length > 0 && <ul className="servers">{props.discovered.map((entry) => <li key={entry.url}><Button variant="outline" onClick={() => props.onConnect(entry.url)}>{entry.name} — {entry.url}</Button></li>)}</ul>}
						<p className={`hint ${props.errorKey ? 'hint--error' : ''}`}>{props.errorKey ? t(props.errorKey) : t('noServer')}</p>
					</motion.div>
				) : selectedMovie ? (
					<Suspense key={`movie-${selectedMovie.id}`} fallback={<p className="hint">{t('loading')}</p>}><MovieDetail movie={selectedMovie} language={props.language} t={t} onMovie={props.onMovie} onPlay={(fileId) => props.onPlayMovie(selectedMovie.id, fileId)} /></Suspense>
				) : selectedSeries ? (
					<Suspense key={`series-${selectedSeries.id}`} fallback={<p className="hint">{t('loading')}</p>}><SeriesDetail series={selectedSeries} episode={heroEpisode} language={props.language} t={t} onPlay={props.onEpisode}>
						<div className="season-tabs">{selectedSeries.seasons?.map((season) => <button key={season.id} className={`season-tab label ${selectedSeason?.season_number === season.season_number ? 'season-tab--active' : ''}`} onClick={() => props.onSeason(season.season_number)}>{season.metadata?.name || `${t('season')} ${season.season_number}`}</button>)}</div>
						{selectedSeason?.episodes?.length ? <CardGrid>{selectedSeason.episodes.map((episode) => <EpisodeCard key={episode.id} item={episode} seriesLabel={displayTitle(selectedSeries)} onPlay={props.onEpisode} onHistory={props.onHistory} busy={props.historyBusy} reducedMotion={props.reducedMotion} t={props.t} />)}</CardGrid> : <p className="hint">{t('emptySeason')}</p>}
					</SeriesDetail></Suspense>
				) : section === 'home' ? (
					<motion.div key="home" className="home-view" initial={{ opacity: 0, y: 12 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -6 }} transition={{ duration: 0.24, ease: [0.16, 1, 0.3, 1] }}>
						{props.homeError ? (
							<p className="hint hint--error">{t('homeFailed')}</p>
						) : !props.home && !props.seriesHome ? (
							<p className="hint">{t('startingHint')}</p>
						) : (
							<HomeView home={props.home ?? { hero: null, hero_kind: undefined, rows: [], total: 0 }} seriesHome={props.seriesHome} language={props.language} reducedMotion={props.reducedMotion} t={t} onMovie={props.onMovie} onPlayMovie={props.onPlayMovie} onSeries={props.onSeries} onEpisode={props.onEpisode} />
						)}
					</motion.div>
				) : section === 'search' ? (
					<SearchResults {...props} />
				) : (
					<motion.div key={section} className="contents" initial={{ opacity: 0, y: 12 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -6 }} transition={{ duration: 0.24, ease: [0.16, 1, 0.3, 1] }}>
						{section === 'series' ? (props.series.length ? <CardGrid>{props.series.map((item) => <MediaCard key={item.id} kind="series" item={item} onOpen={props.onSeries} resumeLabel={t('resumeAt')} actionLabel={t('openSeries')} kindLabel={t('seriesLabel')} reducedMotion={props.reducedMotion} t={props.t} />)}</CardGrid> : <p className="hint">{t('emptySeries')}</p>) : (props.movies.length ? <CardGrid>{props.movies.map((movie) => <MediaCard key={movie.id} kind="movie" item={movie} onOpen={props.onMovie} resumeLabel={t('resumeAt')} actionLabel={t('openMovie')} kindLabel={t('filmSingular')} reducedMotion={props.reducedMotion} t={props.t} />)}</CardGrid> : <p className="hint">{t('emptyLibrary')}</p>)}
					</motion.div>
				)}
			</AnimatePresence>
		</section>
	);
}

function SearchResults(props: LibraryProps) {
	const query = searchText(props.searchQuery.trim());
	const matchingMovies = query ? props.movies.filter((item) => searchText(`${displayTitle(item)} ${displayYear(item) ?? ''}`).includes(query)) : [];
	const matchingSeries = query ? props.series.filter((item) => searchText(`${displayTitle(item)} ${displayYear(item) ?? ''}`).includes(query)) : [];
	return (
		<motion.div key="search" className="contents search-view" initial={{ opacity: 0, y: 12 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0 }}>
			<label className="search-field">
				<Search size={22} aria-hidden="true" />
				<span className="sr-only">{props.t('search')}</span>
				<input autoFocus value={props.searchQuery} onChange={(event) => props.onSearchQuery(event.target.value)} placeholder={props.t('searchPlaceholder')} />
			</label>
			{!query ? <p className="hint">{props.t('searchHint')}</p> : matchingMovies.length + matchingSeries.length === 0 ? <p className="hint">{props.t('noSearchResults')}</p> : (
				<div className="search-results">
					{matchingMovies.length > 0 && <section><h2 className="search-result-title label">{props.t('filmResults')} · {matchingMovies.length}</h2><CardGrid>{matchingMovies.map((movie) => <MediaCard key={movie.id} kind="movie" item={movie} onOpen={props.onMovie} resumeLabel={props.t('resumeAt')} actionLabel={props.t('openMovie')} kindLabel={props.t('filmSingular')} reducedMotion={props.reducedMotion} t={props.t} />)}</CardGrid></section>}
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
	onMovie: (id: number) => void; onPlayMovie: (id: number) => void; onSeries: (id: number) => void; onEpisode: (id: number) => void;
};

/**
 * The web home's composition, carried into the player: one hero for the film
 * that was left, then short rows. The server decides what each row is; this
 * screen decides what it is called. The series rows sit after the film rows
 * on purpose - same as the web, where the two halves are answered separately.
 */
function HomeView({ home, seriesHome, language, reducedMotion, t, onMovie, onPlayMovie, onSeries, onEpisode }: HomeViewProps) {
	const rows: Array<{ kind: string; hint: string | null; cards: React.ReactNode }> = [];
	for (const row of home.rows ?? []) {
		rows.push({
			kind: row.kind,
			hint: row.kind === 'tonight' ? t('rowTonightHint') : null,
			cards: row.movies.map((movie) => (
				<MediaCard key={movie.id} kind="movie" item={movie} onOpen={onMovie} resumeLabel={t('resumeAt')} actionLabel={t('openMovie')} kindLabel={t('filmSingular')} reducedMotion={reducedMotion} t={t} />
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
	const episode = resumeEpisode(seriesHome?.continue_watching ?? []);
	const episodeIsLatest = episode && (!hero || home.hero_kind !== 'resume' || watchedAt(episode.progress) > watchedAt(hero.progress));
	return (
		<>
			{episodeIsLatest ? <PlaybackHero media={{ kind: 'episode', item: episode, series: seriesHome?.recent_series.find((series) => series.id === episode.series_id) }} language={language} t={t} onPlay={onEpisode} /> : hero && <PlaybackHero media={{ kind: 'movie', item: hero }} resuming={home.hero_kind === 'resume'} language={language} t={t} onPlay={onPlayMovie} />}
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

// The sections of the settings sheet, in the order its rail shows them. The
// name is also the catalogue key of the panel's heading, so the rail, the panel
// and the sentences cannot drift apart.
//
// `playback` is the playback row and `subtitles` its own pane, which is the
// disposition the maintainer brought on 24 September 2026: the two switches
// that were in `playback` describe the interface, not the film, so they moved
// to `interface` where they belong and the pane below them can be the three
// things a viewer actually changes about films.
