// Fast structural checks for the generated site. Rendered browser checks still
// matter; these catch broken output before a browser ever opens.

import assert from 'node:assert/strict';
import { readFile, stat } from 'node:fs/promises';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const dist = join(here, 'dist');
const pages = [
	['fr', join(dist, 'index.html')],
	['en', join(dist, 'en', 'index.html')]
];
const downloads = [
	'theia-windows-amd64.exe',
	'theia-windows-arm64.exe',
	'theia-darwin-arm64',
	'theia-darwin-amd64',
	'theia-linux-amd64',
	'theia-linux-arm64'
];

for (const [lang, path] of pages) {
	const html = await readFile(path, 'utf8');
	assert.equal((html.match(/<h1\b/g) || []).length, 1, `${lang}: one h1`);
	assert.equal((html.match(/data-faq/g) || []).length, 3, `${lang}: three FAQs`);
	assert.match(html, new RegExp(`<html lang="${lang}">`), `${lang}: html language`);
	assert.match(html, /<main id="content" tabindex="-1">/, `${lang}: focusable skip target`);
	assert.match(html, /"@type":"SoftwareApplication"/, `${lang}: SoftwareApplication JSON-LD`);
	assert.match(html, /"@type":"FAQPage"/, `${lang}: FAQ JSON-LD`);
	assert.match(html, /hreflang="x-default"/, `${lang}: x-default alternate`);
	assert.doesNotMatch(html, /<(?:script|img)[^>]+(?:src)="https?:\/\//i, `${lang}: no remote runtime subresource`);
	assert.doesNotMatch(html, /navigator\.(?:userAgent|userAgentData)/, `${lang}: no architecture guessing`);
	assert.doesNotMatch(html, /undefined/, `${lang}: no undefined output`);
	for (const file of downloads) assert.ok(html.includes(`/latest/download/${file}`), `${lang}: ${file}`);
}

for (const relative of [
	'styles.css',
	'social-preview.png',
	'favicon.svg',
	'apple-touch-icon.png',
	'fonts/inter.woff2',
	'fonts/playfair-display.woff2',
	'shots/player.webp',
	'shots/library.webp',
	'shots/settings.webp',
	'sitemap.xml',
	'robots.txt'
]) {
	assert.ok((await stat(join(dist, relative))).size > 0, `${relative}: generated and non-empty`);
}

const sitemap = await readFile(join(dist, 'sitemap.xml'), 'utf8');
assert.ok(sitemap.includes('https://benitoow.github.io/theia-media/'), 'sitemap: French URL');
assert.ok(sitemap.includes('https://benitoow.github.io/theia-media/en/'), 'sitemap: English URL');
const robots = await readFile(join(dist, 'robots.txt'), 'utf8');
assert.match(robots, /^User-agent: \*\nAllow: \/\nSitemap: /, 'robots: allow and sitemap');

console.log('Site structure verified (FR/EN, downloads, SEO, local assets, sitemap and robots).');
