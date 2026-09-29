import { Play } from 'lucide-react';
import { motion } from 'motion/react';

import { displayYear } from '../lib/tmdb';
import { formatRuntime } from '../lib/utils';
import type { Movie } from '../types';
import { Button } from './ui/button';

type Props = { movie: Movie; language: string; t: (key: string) => string; onPlay: () => void };

export function MovieDetail({ movie, language, t, onPlay }: Props) {
	const metadata = movie.metadata;
	const art = movie.hero_url || movie.backdrop_url || movie.poster_url;
	const runtime = metadata?.runtime_minutes ? formatRuntime(metadata.runtime_minutes, language) : null;
	const facts = [displayYear(movie), runtime, metadata?.genres?.join(' · '), metadata?.certification, metadata?.vote_average ? `${metadata.vote_average.toFixed(1)}/10` : null].filter(Boolean);
	const originalTitle = metadata?.original_title && metadata.original_title !== movie.title ? metadata.original_title : null;
	const cast = metadata?.cast?.filter((credit) => credit.name).slice(0, 5) ?? [];
	return (
		<motion.article key={`movie-${movie.id}`} className="movie-detail" initial={{ opacity: 0, y: 12 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0 }}>
			{art && <img className="movie-detail-art" src={art} alt="" crossOrigin="anonymous" />}
			<div className="movie-detail-content">
				{metadata?.tagline && <p className="movie-detail-tagline">{metadata.tagline}</p>}
				{facts.length > 0 && <p className="movie-detail-facts">{facts.join(' · ')}</p>}
				{originalTitle && <p className="movie-detail-credit"><strong>{t('originalTitle')}</strong> {originalTitle}</p>}
				{metadata?.overview && <p className="movie-detail-overview">{metadata.overview}</p>}
				{metadata?.director && <p className="movie-detail-credit"><strong>{t('director')}</strong> {metadata.director}</p>}
				{cast.length > 0 && <section className="movie-detail-cast"><h3>{t('cast')}</h3><ul>{cast.map((credit, index) => <li key={`${credit.name}-${index}`}>{credit.name}{credit.character && <span> · {credit.character}</span>}</li>)}</ul></section>}
				<Button onClick={onPlay}><Play size={17} aria-hidden="true" />{t('playMovie')}</Button>
			</div>
		</motion.article>
	);
}
