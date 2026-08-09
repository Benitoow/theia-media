// Builds the bilingual presentation site into site/dist.
//
// The default release metadata is committed in site/release.json for an
// offline, reproducible local build. GitHub Pages supplies a freshly generated
// file through THEIA_RELEASE_JSON after every successful release; nothing in
// the published page calls an API at runtime.

import { copyFile, mkdir, readFile, readdir, rm, writeFile } from 'node:fs/promises';
import { existsSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { dirname, isAbsolute, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

import fr from './locales/fr.js';
import en from './locales/en.js';
import { PLATFORMS, render } from './page.mjs';

const here = dirname(fileURLToPath(import.meta.url));
const root = join(here, '..');
const dist = join(here, 'dist');
const releaseInput = process.env.THEIA_RELEASE_JSON || join(here, 'release.json');
const releasePath = isAbsolute(releaseInput) ? releaseInput : resolve(root, releaseInput);

const isPlainObject = (value) => value !== null && typeof value === 'object' && !Array.isArray(value);

function compareShape(a, b, path = 'catalogue') {
	const problems = [];
	if (Array.isArray(a) || Array.isArray(b)) {
		if (!Array.isArray(a) || !Array.isArray(b)) return [`${path} changes type between fr and en`];
		if (a.length !== b.length) problems.push(`${path} has ${a.length} entries in fr and ${b.length} in en`);
		for (let index = 0; index < Math.min(a.length, b.length); index += 1) {
			problems.push(...compareShape(a[index], b[index], `${path}[${index}]`));
		}
		return problems;
	}
	if (isPlainObject(a) || isPlainObject(b)) {
		if (!isPlainObject(a) || !isPlainObject(b)) return [`${path} changes type between fr and en`];
		const keys = new Set([...Object.keys(a), ...Object.keys(b)]);
		for (const key of keys) {
			const next = `${path}.${key}`;
			if (!(key in a)) problems.push(`fr is missing ${next}`);
			else if (!(key in b)) problems.push(`en is missing ${next}`);
			else problems.push(...compareShape(a[key], b[key], next));
		}
		return problems;
	}
	if (typeof a !== typeof b) problems.push(`${path} changes type between fr and en`);
	return problems;
}

function validateRelease(release) {
	if (!isPlainObject(release)) throw new Error('Release metadata must be an object.');
	if (release.version && typeof release.version !== 'string') throw new Error('release.version must be a string.');
	if (release.publishedAt && Number.isNaN(new Date(release.publishedAt).getTime())) {
		throw new Error('release.publishedAt must be an ISO date.');
	}
	if (!Array.isArray(release.assets)) throw new Error('release.assets must be an array.');
	for (const asset of release.assets) {
		if (!asset.file || typeof asset.file !== 'string') throw new Error('Every release asset needs a file name.');
		if (asset.size !== undefined && (!Number.isInteger(asset.size) || asset.size <= 0)) {
			throw new Error(`${asset.file}: size must be a positive integer.`);
		}
		if (asset.sha256 !== undefined && !/^[a-f0-9]{64}$/.test(asset.sha256)) {
			throw new Error(`${asset.file}: sha256 must be 64 lowercase hexadecimal characters.`);
		}
	}
}

function validateDocument(html, lang) {
	const h1 = html.match(/<h1\b/g) || [];
	if (h1.length !== 1) throw new Error(`${lang}: expected one h1, found ${h1.length}.`);
	if (!html.includes(`<html lang="${lang}">`)) throw new Error(`${lang}: html lang is missing.`);
	if (!html.includes('<main id="content" tabindex="-1">')) throw new Error(`${lang}: skip-link target is not focusable.`);
	if (html.includes('undefined')) throw new Error(`${lang}: rendered output contains undefined.`);
	if ((html.match(/data-faq/g) || []).length !== 3) throw new Error(`${lang}: expected three visible FAQ entries.`);
	for (const platform of PLATFORMS) {
		for (const asset of platform.assets) {
			if (!html.includes(`${LATEST_DOWNLOAD}/${asset.file}`)) throw new Error(`${lang}: missing ${asset.file}.`);
		}
	}
}

const LATEST_DOWNLOAD = 'https://github.com/Benitoow/theia-media/releases/latest/download';
const localeProblems = compareShape(fr, en);
if (localeProblems.length) {
	console.error(`Catalogues have drifted:\n  ${localeProblems.join('\n  ')}`);
	process.exit(1);
}

const release = JSON.parse(await readFile(releasePath, 'utf8'));
validateRelease(release);

const styles = await readFile(join(here, 'styles.css'), 'utf8');
const styleVersion = createHash('sha256').update(styles).digest('hex').slice(0, 12);

const frHTML = render(fr, release, { styleVersion });
const enHTML = render(en, release, { styleVersion });
validateDocument(frHTML, 'fr');
validateDocument(enHTML, 'en');

await rm(dist, { recursive: true, force: true });
await mkdir(join(dist, 'en'), { recursive: true });
await mkdir(join(dist, 'shots'), { recursive: true });
await mkdir(join(dist, 'fonts'), { recursive: true });

await writeFile(join(dist, 'index.html'), frHTML);
await writeFile(join(dist, 'en', 'index.html'), enHTML);
await writeFile(join(dist, 'styles.css'), styles);

const screenshots = ['player.webp', 'library.webp', 'settings.webp'];
for (const name of screenshots) {
	const from = join(root, 'docs', 'screenshots', name);
	if (!existsSync(from)) throw new Error(`Missing screenshot: ${from}`);
	await copyFile(from, join(dist, 'shots', name));
}

const assets = [
	[join(root, 'assets', 'social-preview.png'), join(dist, 'social-preview.png')],
	[join(root, 'web', 'static', 'favicon.svg'), join(dist, 'favicon.svg')],
	[join(root, 'web', 'static', 'apple-touch-icon.png'), join(dist, 'apple-touch-icon.png')],
	[
		join(root, 'web', 'node_modules', '@fontsource-variable', 'inter', 'files', 'inter-latin-wght-normal.woff2'),
		join(dist, 'fonts', 'inter.woff2')
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
		join(dist, 'fonts', 'playfair-display.woff2')
	]
];

for (const [from, to] of assets) {
	if (!existsSync(from)) throw new Error(`Missing asset: ${from}`);
	await copyFile(from, to);
}

const sitemap = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:xhtml="http://www.w3.org/1999/xhtml">
	<url>
		<loc>https://benitoow.github.io/theia-media/</loc>
		<xhtml:link rel="alternate" hreflang="fr" href="https://benitoow.github.io/theia-media/"/>
		<xhtml:link rel="alternate" hreflang="en" href="https://benitoow.github.io/theia-media/en/"/>
	</url>
	<url>
		<loc>https://benitoow.github.io/theia-media/en/</loc>
		<xhtml:link rel="alternate" hreflang="fr" href="https://benitoow.github.io/theia-media/"/>
		<xhtml:link rel="alternate" hreflang="en" href="https://benitoow.github.io/theia-media/en/"/>
	</url>
</urlset>
`;
await writeFile(join(dist, 'sitemap.xml'), sitemap);
await writeFile(
	join(dist, 'robots.txt'),
	'User-agent: *\nAllow: /\nSitemap: https://benitoow.github.io/theia-media/sitemap.xml\n'
);

await writeFile(join(dist, '.nojekyll'), '');

console.log(
	`Site built into site/dist (fr + en, ${screenshots.length} screenshots, release ${release.version || 'metadata hidden'}, ${(await readdir(dist)).length} root entries).`
);
