import { Play } from 'lucide-react';
import { useState } from 'react';

import { useFileInspection } from '../lib/useFileInspection';
import { displayTitle, displayYear } from '../lib/tmdb';
import { formatRuntime } from '../lib/utils';
import { isResumable } from '../lib/progress';
import type { Movie } from '../types';
import { Button } from './ui/button';
import { MediaDetail } from './MediaDetail';
import { mediaBadges, NativeCompatibility } from './NativeCompatibility';

type Props = { movie: Movie; language: string; t: (key: string) => string; onPlay: (fileId?: number) => void; onMovie: (id: number) => void };

export function MovieDetail({ movie, language, t, onPlay, onMovie }: Props) {
	const metadata = movie.metadata;
	const inspection = useFileInspection('movie', movie.id, movie.files ?? []);
	const files = inspection.files;
	const [fileId, setFileId] = useState(() => (files.find((file) => file.is_primary) ?? files[0])?.id);
	const file = files.find((file) => file.id === fileId);
	const title = displayTitle(movie);
	const credits = [{ label: t('originalTitle'), value: metadata?.original_title !== title ? metadata?.original_title : '' }, { label: t('director'), value: metadata?.director },
		...['writing', 'music', 'cinematography'].map((role) => ({ label: t(role), value: metadata?.crew?.filter((credit) => credit.role === role).map((credit) => credit.name).join(' · ') }))];
	return <>
		<MediaDetail kind="movie" title={title} poster={movie.poster_url} backdrop={movie.hero_url || movie.backdrop_url} tagline={metadata?.tagline}
			facts={[displayYear(movie), formatRuntime(metadata?.runtime_minutes, language), metadata?.genres?.join(' · '), metadata?.certification, metadata?.vote_average ? `${metadata.vote_average.toLocaleString(language, { maximumFractionDigits: 1 })}/10` : null]}
			overview={metadata?.overview} cast={metadata?.cast} credits={credits} progress={movie.progress} t={t}
			actions={<Button onClick={() => onPlay(fileId)}><Play size={17} aria-hidden="true" />{isResumable(movie.progress) ? `${t('resumeAt')} ${Math.max(1, Math.floor((movie.progress?.position_seconds ?? 0) / 60))} min` : t('playMovie')}</Button>}>
			{files.length > 0 && <fieldset className="detail-files"><legend className="label">{t('availableFiles')}</legend>{files.map((candidate) => <label key={candidate.id}><input type="radio" name={`movie-file-${movie.id}`} checked={candidate.id === fileId} onChange={() => setFileId(candidate.id)} /><span>{candidate.file_name}<small>{mediaBadges(candidate.media).join(' · ') || t('mediaUnmeasured')}</small></span></label>)}</fieldset>}
			<NativeCompatibility media={file?.media} t={t} onInspect={file ? () => inspection.inspect(file.id) : undefined} inspecting={inspection.busy} inspectionError={inspection.error} />
		</MediaDetail>
		{Boolean(movie.collection_parts?.length) && <section className="detail-collection"><h2 className="label">{metadata?.collection?.name || t('collection')}</h2><div>{movie.collection_parts?.map((part) => <Button key={part.id} variant="outline" onClick={() => onMovie(part.id)}>{displayTitle(part)}</Button>)}</div></section>}
	</>;
}
