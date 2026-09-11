// Release metadata, loaded once at build time and injected into the page as
// props. The published page never calls an API: version, date, size and
// SHA-256 are build facts, and a missing fact is omitted rather than guessed.
// THEIA_RELEASE_JSON lets GitHub Pages supply a freshly generated snapshot;
// the committed release.json remains the offline fallback.

import { readFile } from 'node:fs/promises';
import { resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

export interface ReleaseAsset {
	file: string;
	platform: string;
	architecture: string;
	size?: number;
	sha256?: string;
}

export interface Release {
	version?: string;
	publishedAt?: string;
	url?: string;
	assets: ReleaseAsset[];
}

export function isPlainObject(value: unknown): value is Record<string, unknown> {
	return value !== null && typeof value === 'object' && !Array.isArray(value);
}

export function validateRelease(release: unknown): asserts release is Release {
	if (!isPlainObject(release)) throw new Error('Release metadata must be an object.');
	if (release.version !== undefined && typeof release.version !== 'string')
		throw new Error('release.version must be a string.');
	if (
		release.publishedAt !== undefined &&
		Number.isNaN(new Date(String(release.publishedAt)).getTime())
	) {
		throw new Error('release.publishedAt must be an ISO date.');
	}
	if (!Array.isArray(release.assets)) throw new Error('release.assets must be an array.');
	for (const asset of release.assets as unknown[]) {
		if (!isPlainObject(asset) || !asset.file || typeof asset.file !== 'string')
			throw new Error('Every release asset needs a file name.');
		const size = asset.size;
		if (size !== undefined && (typeof size !== 'number' || !Number.isInteger(size) || size <= 0)) {
			throw new Error(`${asset.file}: size must be a positive integer.`);
		}
		if (asset.sha256 !== undefined && !/^[a-f0-9]{64}$/.test(String(asset.sha256))) {
			throw new Error(`${asset.file}: sha256 must be 64 lowercase hexadecimal characters.`);
		}
	}
}

export async function loadRelease(): Promise<Release> {
	// Astro's prerender bundle rewrites import.meta.url, so paths are resolved
	// against the working directory the build runs in. The Pages workflow sets
	// THEIA_RELEASE_JSON as a repository-root path (site/release.generated.json)
	// but builds from site/, so both roots are candidates; the first file that
	// exists wins. The default stays the committed offline snapshot.
	const candidates = [];
	const override = process.env.THEIA_RELEASE_JSON;
	if (override) {
		candidates.push(resolve(process.cwd(), override));
		candidates.push(resolve(process.cwd(), '..', override));
	} else {
		candidates.push(resolve(process.cwd(), 'release.json'));
	}
	const { readFile: readFileAsync } = await import('node:fs/promises');
	for (const path of candidates) {
		try {
			const release = JSON.parse(await readFileAsync(path, 'utf8'));
			validateRelease(release);
			return release as Release;
		} catch (error: unknown) {
			if ((error as NodeJS.ErrnoException).code !== 'ENOENT') throw error;
		}
	}
	throw new Error(`Release metadata not found in: ${candidates.join(', ')}`);
}
