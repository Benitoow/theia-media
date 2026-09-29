import type { Episode, Progress } from '../types';

export function isResumable(progress?: Progress): boolean {
	return !progress?.finished && (progress?.position_seconds ?? 0) >= 30;
}

export function watchedAt(progress?: Progress): number {
	const value = progress?.watched_at ? Date.parse(progress.watched_at) : 0;
	return Number.isFinite(value) ? value : 0;
}

export function resumeEpisode(episodes: Episode[]): Episode | undefined {
	return episodes.filter((episode) => isResumable(episode.progress))
		.sort((a, b) => watchedAt(b.progress) - watchedAt(a.progress))[0];
}

export function episodeCode(episode: Episode): string {
	return `S${String(episode.season_number).padStart(2, '0')}${(episode.episode_numbers ?? []).map((number) => `E${String(number).padStart(2, '0')}`).join('')}`;
}
