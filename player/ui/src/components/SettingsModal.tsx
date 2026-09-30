import { motion } from 'motion/react';
import { Clock, Cog, Copy, Download, Film, Keyboard, Languages, ListVideo, type LucideIcon, Minus, MonitorPlay, Palette, Percent, PenLine, Plus, Server as ServerIcon, Square, BarChart3, Bold, Captions, Tv, Type, UnfoldVertical, X } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';

import { Button } from './ui/button';
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogTitle } from './ui/dialog';
import PartitionBar, { PartitionBarSegment, PartitionBarSegmentTitle, PartitionBarSegmentValue } from './ui/partition-bar';
import { Switch } from './ui/switch';
import notFoundArt from '../assets/media-not-found.png';

import { imageURL } from '../lib/tmdb';
import { formatRuntime } from '../lib/utils';

import { PLAYBACK_DEFAULTS } from '../types';
import type { PlaybackPreferences, Server, SubtitleStyle, UpdateStatus, WatchStats } from '../types';

import { invoke } from '../lib/bridge';
import { outlineColourFor, SUBTITLE_SIZE, SUBTITLE_HEIGHT, SUBTITLE_COLOURS, SUBTITLE_BANDS, type Preferences } from '../lib/preferences';
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

export default function SettingsModal({ open, language, preferences, server, updateStatus, updateBusy, t, onClose, onHelp, onChangeServer, onSave, onCheckUpdate, onApplyUpdate }: { open: boolean; language: string; preferences: Preferences; server: Server | null; updateStatus: UpdateStatus | null; updateBusy: boolean; t: (key: string) => string; onClose: () => void; onHelp: () => void; onChangeServer: () => void; onSave: (language: string, preferences: Preferences) => void; onCheckUpdate: () => void; onApplyUpdate: () => void }) {
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
								{server && <Button variant="outline" onClick={onChangeServer}>{t('changeServer')}</Button>}
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
				<footer className="settings-footer"><Button className="settings-help-entry" variant="ghost" aria-label={t('keyboardHelp')} onClick={onHelp}><Keyboard size={17} aria-hidden="true" /><span>{t('help')}</span></Button><DialogClose asChild><Button variant="outline">{t('cancel')}</Button></DialogClose><Button onClick={() => onSave(draftLanguage, draft)}>{t('save')}</Button></footer>
			</DialogContent>
		</Dialog>
	);
}
