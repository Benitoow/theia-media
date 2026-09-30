import { Play } from 'lucide-react';
import { useEffect, useState, type ReactNode } from 'react';
import { useFileInspection } from '../lib/useFileInspection';
import { invoke } from '../lib/bridge';
import { displayTitle, displayYear } from '../lib/tmdb';
import { episodeCode, isResumable } from '../lib/progress';
import type { Episode, Series } from '../types';
import { Button } from './ui/button';
import { MediaDetail } from './MediaDetail';
import { mediaBadges, NativeCompatibility } from './NativeCompatibility';

type Props = { series: Series; episode?: Episode | null; t: (key: string) => string; language: string; onPlay: (id: number) => void; children: ReactNode };

export function SeriesDetail({ series, episode, t, language, onPlay, children }: Props) {
	const [detail, setDetail] = useState<Episode | null>(null);
	useEffect(() => {
		let current = true; setDetail(null);
		if (episode) invoke<string>('player_episode_detail', { id: episode.id }).then((raw) => { if (current) setDetail(JSON.parse(raw)); }).catch(() => {});
		return () => { current = false; };
	}, [episode?.id]);
	const metadata = series.metadata;
	const inspection = useFileInspection('episode', episode?.id ?? 0, detail?.files ?? []);
	const file = inspection.files.find((file) => file.is_primary) ?? inspection.files[0];
	const title = displayTitle(series);
	const firstYear = displayYear(series);
	const lastYear = metadata?.last_air_date?.slice(0, 4);
	const years = lastYear && lastYear !== String(firstYear) && metadata?.air_status === 'ended' ? `${firstYear} – ${lastYear}` : firstYear;
	const episodeTitle = episode?.episode_metadata?.[0]?.metadata?.name || episode?.episode_metadata?.[0]?.local_title;
	return <>
		<MediaDetail kind="series" title={title} poster={series.poster_url} backdrop={series.hero_url || series.backdrop_url} tagline={metadata?.tagline}
			facts={[years, metadata?.genres?.join(' · '), metadata?.certification, metadata?.air_status ? t(`airStatus_${metadata.air_status}`) : '', metadata?.vote_average ? `${metadata.vote_average.toLocaleString(language, { maximumFractionDigits: 1 })}/10` : null]}
			overview={metadata?.overview} cast={metadata?.cast} t={t} progress={episode?.progress}
			credits={[{ label: t('originalTitle'), value: metadata?.original_name !== title ? metadata?.original_name : '' }, { label: t('creators'), value: metadata?.creators?.join(' · ') }, { label: t('networks'), value: metadata?.networks?.join(' · ') }]}
            badges={mediaBadges(file?.media)}
            playback={episode && <><p className="detail-file-scope">{t('episodeCharacteristics')} · {episodeCode(episode)}</p><NativeCompatibility media={file?.media} t={t} onInspect={file ? () => inspection.inspect(file.id) : undefined} inspecting={inspection.busy} inspectionError={inspection.error} /></>}
			actions={episode && <div className="detail-episode-action"><p>{episodeCode(episode)}{episodeTitle && ` · ${episodeTitle}`}</p><Button onClick={() => onPlay(episode.id)}><Play size={17} aria-hidden="true" />{isResumable(episode.progress) ? `${t('resumeAt')} ${Math.max(1, Math.floor((episode.progress?.position_seconds ?? 0) / 60))} min` : t('nextUnwatched')}</Button></div>}>
            <div className="series-episodes">{children}</div>
		</MediaDetail>
	</>;
}
