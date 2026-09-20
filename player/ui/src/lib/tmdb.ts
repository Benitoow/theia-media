import type { Episode, Movie, Series } from '../types';

/**
 * The TMDB helpers, ported from the web application's `web/src/lib/api.js`
 * rather than reinvented: one product, one vocabulary for what a title, a
 * year and a cached artwork URL are.
 *
 * Images always go through Theia (`/api/images/…`), never straight to TMDB:
 * the server caches them on disk, and nothing in the interface ever makes an
 * external request.
 */

export type TmdbSizedImage = 'w92' | 'w154' | 'w185' | 'w342' | 'w500' | 'w780' | 'w1280' | 'original';

/** Builds the URL for a cached TMDB image. Null when there is no path. */
export function imageURL(path: string | null | undefined, size: TmdbSizedImage = 'w342'): string | null {
	if (!path) return null;
	return `/api/images/${size}/${path.replace(/^\//, '')}`;
}

const cardWidths = [342, 500, 780] as const;

/** A srcset for a TMDB image across the card sizes, narrowest first. */
export function imageSrcSet(path: string | null | undefined): string | null {
	if (!path) return null;
	return cardWidths.map((width) => `${imageURL(path, `w${width}` as TmdbSizedImage)} ${width}w`).join(', ');
}

type Titled = {
	title?: string;
	metadata?: {
		title?: string;
		name?: string;
		tmdb_title?: string;
		tmdb_name?: string;
	} | null;
};

/**
 * The title to show: TMDB's when it recognised the item, the filename's
 * otherwise. Films carry `tmdb_title`, series `tmdb_name` — one helper
 * rather than two so a card component does not need to know which it is
 * holding.
 */
export function displayTitle(item: Titled | null | undefined): string {
	return (
		item?.metadata?.tmdb_title ||
		item?.metadata?.tmdb_name ||
		item?.metadata?.title ||
		item?.metadata?.name ||
		item?.title ||
		''
	);
}

type Dated = {
	year?: number;
	metadata?: {
		release_date?: string;
		first_air_date?: string;
	} | null;
};

/** The year to show, preferring TMDB's date over the parsed filename. */
export function displayYear(item: Dated | null | undefined): number | string | null {
	const date = item?.metadata?.release_date || item?.metadata?.first_air_date;
	if (date) return date.slice(0, 4);
	return item?.year ?? null;
}

type Artworked = {
	backdrop_url?: string;
	poster_url?: string;
	still_url?: string;
	metadata?: {
		backdrop_path?: string;
		poster_path?: string;
		still_path?: string;
	} | null;
};

/**
 * Every artwork URL a card, preview or hero may try, in order: the
 * server-resolved URLs first, then the raw TMDB paths through the image
 * cache. Backdrops lead — every frame here is 16/9.
 */
export function artworkCandidates(item: Artworked | null | undefined, heroSize: TmdbSizedImage = 'w780'): string[] {
	if (!item) return [];
	return [
		item.backdrop_url,
		item.still_url,
		imageURL(item.metadata?.backdrop_path, heroSize),
		imageURL(item.metadata?.still_path, heroSize),
		item.poster_url,
		imageURL(item.metadata?.poster_path, 'w342'),
	].filter((url): url is string => Boolean(url));
}

export type { Episode, Movie, Series };
