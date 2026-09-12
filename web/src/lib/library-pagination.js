/**
 * Loads an endpoint whose response is capped to fixed-size pages.
 *
 * The loader is injected so the loop remains testable without profiles,
 * network globals or a Svelte runtime. Offsets advance by the requested page
 * size rather than by the returned batch: an API may legitimately return a
 * short page while still advertising a larger total.
 *
 * @param {(limit: number, offset: number) => Promise<Record<string, unknown>>} loadPage
 * @param {string} key
 * @param {(loaded: number, total: number) => void} [onProgress]
 * @param {number} [pageSize]
 */
export async function fetchAllPages(loadPage, key, onProgress, pageSize = 500) {
	let offset = 0;
	let total = Infinity;
	const items = [];

	while (offset < total) {
		const page = await loadPage(pageSize, offset);
		const batch = Array.isArray(page?.[key]) ? page[key] : [];
		const pageTotal = Number(page?.total);
		total = Number.isFinite(pageTotal) ? pageTotal : batch.length;
		items.push(...batch);
		offset += pageSize;
		onProgress?.(items.length, total);
		if (!batch.length) break;
	}
	return items;
}
