// Copies the page's runtime assets from their single sources of truth into
// site/public before the Astro build. Nothing binary is committed twice; the
// screenshots live in docs/screenshots, the fonts in the web/ workspace's
// @fontsource-variable packages, exactly as the previous build pipeline did.
//
// The release snapshot (site/release.json) is deliberately NOT copied: it is
// read at build time and injected into the page as build-time data.

import { copyFile, mkdir, stat, writeFile } from 'node:fs/promises';
import { existsSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const site = dirname(here);
const root = resolve(site, '..');
const pub = join(site, 'public');

const jobs = [
	// Screenshots of the real application.
	...[
		'player.webp',
		'library.webp',
		'settings.webp',
		'film.webp',
		'home.webp',
		'search.webp',
		'profiles.webp',
		'series.webp',
		'onboarding.webp'
	].map((name) => [join(root, 'docs', 'screenshots', name), join(pub, 'shots', name)]),
	// Social card, rendered from assets/social-preview-source.svg per the
	// provenance rules in docs/design-system.md.
	[join(root, 'assets', 'social-preview.png'), join(pub, 'social-preview.png')],
	// Icons shared with the application.
	[join(root, 'web', 'static', 'favicon.svg'), join(pub, 'favicon.svg')],
	[join(root, 'web', 'static', 'apple-touch-icon.png'), join(pub, 'apple-touch-icon.png')],
	// The two variable fonts, self-hosted. No font service, no CDN.
	[
		join(root, 'web', 'node_modules', '@fontsource-variable', 'inter', 'files', 'inter-latin-wght-normal.woff2'),
		join(pub, 'fonts', 'inter.woff2')
	],
	[
		join(
			root,
			'web',
			'node_modules',
			'@fontsource-variable',
			'playfair-display',
			'files',
			'playfair-display-latin-wght-normal.woff2'
		),
		join(pub, 'fonts', 'playfair-display.woff2')
	]
];

for (const [from, to] of jobs) {
	if (!existsSync(from)) throw new Error(`Missing asset: ${from}`);
	await mkdir(dirname(to), { recursive: true });
	await copyFile(from, to);
}

// Keeps GitHub Pages from running the output through Jekyll.
await writeFile(join(pub, '.nojekyll'), '');

const sizes = await Promise.all(
	jobs.map(async ([, to]) => (await stat(to)).size)
);
console.log(`sync-assets: ${jobs.length + 1} assets, ${(sizes.reduce((a, b) => a + b, 0) / 1_000_000).toFixed(1)} MB total.`);
