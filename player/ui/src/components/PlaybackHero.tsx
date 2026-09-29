import { ArrowRight } from 'lucide-react';
import { useEffect, useState } from 'react';

import notFoundArt from '../assets/media-not-found.png';
import { episodeCode, isResumable } from '../lib/progress';
import { artworkCandidates, displayTitle, displayYear } from '../lib/tmdb';
import { formatRuntime } from '../lib/utils';
import type { Episode, Movie, Series } from '../types';
import { Button } from './ui/button';

type Media = { kind: 'movie'; item: Movie } | { kind: 'episode'; item: Episode; series?: Series };
type Props = { media: Media; resuming?: boolean; headingLevel?: 1 | 2; language: string; t: (key: string) => string; onPlay: (id: number) => void };

/** Films and episodes share the same resume surface and progress register. */
export function PlaybackHero({ media, resuming = true, headingLevel = 1, language, t, onPlay }: Props) {
	const Heading = headingLevel === 1 ? 'h1' : 'h2';
	const movie = media.kind === 'movie' ? media.item : undefined;
	const episode = media.kind === 'episode' ? media.item : undefined;
	const series = media.kind === 'episode' ? media.series : undefined;
	const record = episode?.episode_metadata?.[0];
	const title = movie ? displayTitle(movie) : (series ? displayTitle(series) : episode?.series_title ?? '');
	const subtitle = episode ? [episodeCode(episode), record?.metadata?.name || record?.local_title].filter(Boolean).join(' · ') : '';
	const year = displayYear(movie ?? series) || null;
	const runtime = formatRuntime(movie?.metadata?.runtime_minutes ?? record?.metadata?.runtime_minutes, language);
	const director = movie?.metadata?.director;
	const rating = movie?.metadata?.vote_average ?? 0;
	const [heroFailed, setHeroFailed] = useState(false);
	useEffect(() => setHeroFailed(false), [media.kind, media.item.id]);
	const artwork = movie ?? series ?? episode;
	const heroArt = heroFailed ? notFoundArt : (movie?.hero_url || series?.hero_url || episode?.hero_url || artworkCandidates(artwork, 'original')[0] || notFoundArt);
	const progress = media.item.progress;
	const position = progress?.position_seconds ?? 0;
	const duration = progress?.duration_seconds || (record?.metadata?.runtime_minutes ?? 0) * 60;
	const playing = resuming && isResumable(progress);
	const percent = playing && duration > 0 ? Math.min(100, position / duration * 100) : 0;
	const remaining = playing && duration > position ? formatRuntime(Math.max(1, Math.ceil((duration - position) / 60)), language) : null;
	const overview = movie?.metadata?.overview ?? record?.metadata?.overview ?? '';
	return (
		<section className={`home-hero ${headingLevel === 2 ? 'home-hero--detail' : ''}`} aria-label={title}>
			<img className="home-hero-art" src={heroArt} alt="" crossOrigin="anonymous" fetchPriority="high" onError={() => setHeroFailed(true)} />
			<div className="home-hero-content">
				<p className="label home-hero-eyebrow">{playing ? t('heroResumeEyebrow') : t('heroFeaturedEyebrow')}</p>
				<Heading className="home-hero-title">{title}</Heading>
				{subtitle && <p className="home-hero-episode">{subtitle}</p>}
				<div className="home-hero-meta">
					{year && <span className="label">{year}</span>}
					{runtime && <span className="label">{runtime}</span>}
					{director && <span className="label">{director}</span>}
					{rating > 0 && <span className="home-hero-rating"><span className="home-hero-rating-figure">{rating.toLocaleString(language === 'en' ? 'en-US' : 'fr-FR', { minimumFractionDigits: 1, maximumFractionDigits: 1 })}</span><span className="label">{t('ratingScale')}</span></span>}
				</div>
				{playing ? (
					<>
						{episode && <p className="label home-hero-position">{t('resumeAt')} {Math.max(1, Math.floor(position / 60))} min</p>}
						<div className="home-hero-progress">
						{duration > 0 && <div className="home-hero-progress-track"><div className="home-hero-progress-played" style={{ width: `${percent}%` }} /></div>}
						{remaining && <span className="label home-hero-remaining">{t('remainingPattern').replace('{d}', remaining)}</span>}
						</div>
					</>
				) : overview ? <p className="home-hero-overview">{overview}</p> : null}
				<div className="home-hero-actions"><Button className="home-hero-action" onClick={() => onPlay(media.item.id)}>{playing ? t('resume') : t(movie ? 'playMovie' : 'playEpisode')}<ArrowRight size={17} aria-hidden="true" /></Button></div>
			</div>
		</section>
	);
}
