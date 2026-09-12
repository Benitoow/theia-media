import { test } from 'node:test';
import assert from 'node:assert/strict';

import { fetchAllPages } from '../../src/lib/library-pagination.js';

test('a catalogue larger than two server pages is returned without a 500-row ceiling', async () => {
	const catalogue = Array.from({ length: 1203 }, (_, id) => ({ id: id + 1 }));
	const requests = [];
	const progress = [];

	const result = await fetchAllPages(
		async (limit, offset) => {
			requests.push({ limit, offset });
			return { series: catalogue.slice(offset, offset + limit), total: catalogue.length };
		},
		'series',
		(loaded, total) => progress.push({ loaded, total })
	);

	assert.deepEqual(requests, [
		{ limit: 500, offset: 0 },
		{ limit: 500, offset: 500 },
		{ limit: 500, offset: 1000 }
	]);
	assert.equal(result.length, 1203);
	assert.equal(new Set(result.map((item) => item.id)).size, 1203);
	assert.deepEqual(result.at(-1), { id: 1203 });
	assert.deepEqual(progress.at(-1), { loaded: 1203, total: 1203 });
});

test('an endpoint that stops paging cannot trap the client in an infinite loop', async () => {
	let calls = 0;
	const result = await fetchAllPages(async () => {
		calls++;
		return calls === 1 ? { movies: [{ id: 1 }], total: 900 } : { movies: [], total: 900 };
	}, 'movies');

	assert.deepEqual(result, [{ id: 1 }]);
	assert.equal(calls, 2);
});
