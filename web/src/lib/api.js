// Everything that talks to the Go server.
//
// Images go through Theia rather than straight to TMDB: the server caches them
// on disk, and nothing in the interface ever makes an external request.

export async function getJSON(path, options) {
	const res = await fetch(path, options);
	if (!res.ok) {
		const error = new Error(`HTTP ${res.status}`);
		error.status = res.status;
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

/** The title to show: TMDB's when it recognised the film, the filename's otherwise. */
export function displayTitle(movie) {
	return movie?.metadata?.tmdb_title || movie?.title || '';
}

/** The year to show, preferring TMDB's release date over the parsed filename. */
export function displayYear(movie) {
	return movie?.metadata?.release_date?.slice(0, 4) || movie?.year || null;
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
		const page = await getJSON(`/api/library/movies?limit=${pageSize}&offset=${offset}`);
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

/** Runtime as "2 h 18" or "94 min". */
export function formatRuntime(minutes) {
	if (!minutes) return null;
	if (minutes < 60) return `${minutes} min`;
	const h = Math.floor(minutes / 60);
	const m = minutes % 60;
	return m === 0 ? `${h} h` : `${h} h ${String(m).padStart(2, '0')}`;
}
