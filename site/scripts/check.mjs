// Fast structural checks for the built site. Rendered browser checks still
// matter and are exercised at the documented widths before any publish; these
// catch broken output before a browser ever opens.

import assert from 'node:assert/strict';
import { readFile, readdir, stat } from 'node:fs/promises';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const dist = join(here, '..', 'dist');

const downloads = [
	'theia-windows-amd64.exe',
	'theia-windows-arm64.exe',
	'theia-darwin-arm64',
	'theia-darwin-amd64',
	'theia-linux-amd64',
	'theia-linux-arm64'
];

const html = await readFile(join(dist, 'index.html'), 'utf8');

assert.equal((html.match(/<h1\b/g) || []).length, 1, 'one h1');
assert.match(html, /<html lang="en">/, 'html language');
assert.match(html, /<main id="content" tabindex="-1"/, 'focusable skip target');
assert.match(html, /rel="canonical" href="https:\/\/benitoow\.github\.io\/theia-media\/"/, 'canonical URL');
assert.match(html, /"@type":"SoftwareApplication"/, 'SoftwareApplication JSON-LD');
assert.match(html, /"@type":"FAQPage"/, 'FAQ JSON-LD');
assert.equal((html.match(/data-faq/g) || []).length, 3, 'three visible FAQ entries');
assert.doesNotMatch(html, /<(?:script|img)[^>]+(?:src)="https?:\/\//i, 'no remote runtime subresource');
assert.doesNotMatch(html, /navigator\.(?:userAgent|userAgentData)/, 'no architecture guessing');
assert.doesNotMatch(html, /undefined/, 'no undefined output');
assert.match(html, /aria-live="polite"/, 'live status region for the download choice');
for (const file of downloads) assert.ok(html.includes(`/latest/download/${file}`), file);

// Every island contract must survive without JavaScript: the three download
// panels and the tracks panel are present in the server-rendered markup
// before hydration.
for (const marker of ['windows-download-title', 'macos-download-title', 'linux-download-title', 'id="tracks-panel"']) {
	assert.ok(html.includes(marker), `island markup server-rendered: ${marker}`);
}

// Generated assets: one stylesheet with the theme inlined, self-hosted fonts,
// the three screenshots, icons, social card, sitemap and robots.
const styleFiles = (await readdir(join(dist, '_astro'))).filter((file) => file.endsWith('.css'));
assert.ok(styleFiles.length > 0, 'a stylesheet is emitted');
const css = await readFile(join(dist, '_astro', styleFiles[0]), 'utf8');
assert.match(css, /--color-ink:#0b0a09/i, 'design tokens reach the emitted CSS');
for (const relative of [
	'fonts/inter.woff2',
	'fonts/playfair-display.woff2',
	'shots/player.webp',
	'shots/library.webp',
	'shots/settings.webp',
	'shots/film.webp',
	'shots/home.webp',
	'shots/search.webp',
	'shots/profiles.webp',
	'shots/series.webp',
	'shots/onboarding.webp',
	'social-preview.png',
	'favicon.svg',
	'apple-touch-icon.png',
	'sitemap.xml',
	'robots.txt'
]) {
	assert.ok((await stat(join(dist, relative))).size > 0, `${relative}: generated and non-empty`);
}
// .nojekyll only has to exist; it is empty by design.
await stat(join(dist, '.nojekyll'));

// Font URLs inside the page must carry the repository subpath.
assert.match(html, /url\('\/theia-media\/fonts\/playfair-display\.woff2'\)/, 'font faces use the subpath');
assert.match(html, /\/theia-media\/shots\/player\.webp/, 'hero capture uses the subpath');

const sitemap = await readFile(join(dist, 'sitemap.xml'), 'utf8');
assert.ok(sitemap.includes('https://benitoow.github.io/theia-media/'), 'sitemap URL');
const robots = await readFile(join(dist, 'robots.txt'), 'utf8');
assert.match(robots, /^User-agent: \*\nAllow: \/\nSitemap: /, 'robots: allow and sitemap');

console.log('Site structure verified (markup, downloads, SEO, tokens, assets, sitemap, robots).');
