// Everything that talks to the Go server.
//
// Images go through Theia rather than straight to TMDB: the server caches them
// on disk, and nothing in the interface ever makes an external request.

import { formatRuntime as formatLocalizedRuntime } from '$lib/i18n/index.svelte.js';
import { profiles } from '$lib/profiles.svelte.js';

export async function apiFetch(path, options) {
	return fetch(path, options);
}

export async function getJSON(path, options) {
	const res = await apiFetch(path, options);
	if (!res.ok) {
		const error = new Error(`HTTP ${res.status}`);
		error.status = res.status;
		// The server answers a failure with {"error": "<code>"} and never with a
		// sentence (decision 25). Carrying the code up lets the caller translate
		// it; without this the interface can only say "something went wrong".
		try {
			const body = await res.json();
			if (typeof body?.error === 'string') error.code = body.error;
		} catch {
			// Not every failure has a JSON body. The status alone still stands.
		}
		throw error;
	}
	return res.json();
}

/**
 * Builds the URL for a cached TMDB image.
 * @param {string | null | undefined} path a TMDB image path such as "/abc.jpg"
 * @param {string} size one of the sizes the server whitelists
 */
export function imageURL(path, size = 'w342') {
	if (!path) return null;
	return `/api/images/${size}/${path.replace(/^\//, '')}`;
}

/**
 * The title to show: TMDB's when it recognised the item, the filename's
 * otherwise. Films carry `tmdb_title`, series `tmdb_name` -- one helper rather
 * than two so a card component does not need to know which it is holding.
 */
export function displayTitle(item) {
	return item?.metadata?.tmdb_title || item?.metadata?.tmdb_name || item?.title || '';
}

/** The year to show, preferring TMDB's date over the parsed filename. */
export function displayYear(item) {
	const date = item?.metadata?.release_date || item?.metadata?.first_air_date;
	return date?.slice(0, 4) || item?.year || null;
}

/**
 * Fetches the whole library, one page at a time.
 *
 * The endpoint caps a page at 500, so a library larger than that takes several
 * requests. Everything then lives in memory and search, sorting and filtering
 * happen in the browser, which is what makes them instant. That trade holds for
 * a few thousand films on a LAN; past that the sorting belongs in SQL, and the
 * place to change it is here.
 */
export async function getAllMovies(onProgress) {
	const pageSize = 500;
	let offset = 0;
	let total = Infinity;
	const movies = [];

	while (offset < total) {
		const page = await getJSON(
			profiles.url(`/api/library/movies?limit=${pageSize}&offset=${offset}`)
		);
		total = page.total ?? page.movies.length;
		movies.push(...page.movies);
		offset += pageSize;
		onProgress?.(movies.length, total);
		if (!page.movies.length) break; // never spin on an endpoint that stops paging
	}
	return movies;
}

/** Strips accents and case so "Amelie" finds "Amélie". */
export function searchKey(value) {
	return (value ?? '')
		.normalize('NFD')
		.replace(/\p{Diacritic}/gu, '')
		.toLowerCase();
}

/** A playback timestamp as "1:23:45" or "4:07". */
export function formatTime(seconds) {
	if (!Number.isFinite(seconds) || seconds < 0) return '0:00';
	const total = Math.floor(seconds);
	const h = Math.floor(total / 3600);
	const m = Math.floor((total % 3600) / 60);
	const s = total % 60;
	const pad = (n) => String(n).padStart(2, '0');
	return h > 0 ? `${h}:${pad(m)}:${pad(s)}` : `${m}:${pad(s)}`;
}

/** Runtime in the active interface language. */
export function formatRuntime(minutes) {
	return formatLocalizedRuntime(minutes);
}
