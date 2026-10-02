#!/usr/bin/env node
//
// Builds site/dist from the sources next to this file. No dependencies.
//
//	node site/build.mjs                                  the URL in site.config.json
//	SITE_URL=https://example.org node site/build.mjs     another address, once
//	node site/build.mjs --site-url https://example.org   the same, as an argument
//	node site/build.mjs --check                          build, then run check.mjs
//
// The canonical URL is the only variable. It reaches the page in four places
// (canonical, Open Graph, structured data, sitemap) and in robots.txt; every
// other link is relative, so the page works under any base path - a GitHub
// Pages project address included.
//
// Everything printed from a release comes from release.json, written by
// fetch-release.mjs. A missing fact stops the build rather than being omitted
// silently, because a stale fact on a download button is worse than no build.

import { cpSync, existsSync, mkdirSync, readFileSync, readdirSync, rmSync, statSync, writeFileSync } from 'node:fs';
import { dirname, join, relative } from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawnSync } from 'node:child_process';

const here = dirname(fileURLToPath(import.meta.url));
const dist = join(here, 'dist');
const argv = process.argv.slice(2);
const flag = (name) => {
	const at = argv.indexOf(name);
	return at === -1 ? null : argv[at + 1];
};
const fail = (message) => {
	console.error(`build: ${message}`);
	process.exit(1);
};

// ------------------------------------------------------------------ the URL
const config = JSON.parse(readFileSync(join(here, 'site.config.json'), 'utf8'));
const rawUrl = flag('--site-url') || process.env.SITE_URL || config.siteUrl;
let site;
try {
	site = new URL(rawUrl);
} catch {
	fail(`"${rawUrl}" is not a URL`);
}
const local = ['localhost', '127.0.0.1', '[::1]'].includes(site.hostname);
if (site.protocol !== 'https:' && !(local && site.protocol === 'http:')) fail('the site URL must be https (http only for localhost)');
if (site.search || site.hash) fail('the site URL carries no query or fragment');
const siteUrl = `${site.origin}${site.pathname.replace(/\/+$/, '')}`;

// --------------------------------------------------------------- the release
const release = JSON.parse(readFileSync(join(here, 'release.json'), 'utf8'));
const version = release.tag.replace(/^v/, '');
const [major, minor] = version.split('.').map((part) => parseInt(part, 10));
// 3.4.0 and every earlier release are GPL-3.0; what follows is PolyForm
// Noncommercial 1.0.0 (decision 163). The page names the two together.
const licence = major < 3 || (major === 3 && minor <= 4) ? 'GPL-3.0' : 'PolyForm Noncommercial 1.0.0';
const published = new Date(release.publishedAt);
const tokens = {
	site_url: siteUrl,
	build_date: new Date().toISOString().slice(0, 10),
	tag: release.tag,
	version,
	date: published.toLocaleDateString('en-GB', { day: 'numeric', month: 'long', year: 'numeric', timeZone: 'UTC' }),
	date_iso: release.publishedAt.slice(0, 10),
	release_url: release.url,
	licence
};
const megabytes = (bytes) => `${(bytes / 1048576).toFixed(1)} MB`;

const resolve = (match, name) => {
	const [key, argument] = name.split(':');
	if (key === 'size' || key === 'sha') {
		const asset = release.assets[argument];
		if (!asset) fail(`release.json has no asset "${argument}" (used by {{${name}}})`);
		return key === 'size' ? megabytes(asset.size) : asset.sha256;
	}
	if (!(key in tokens) || argument !== undefined) fail(`unknown token {{${name}}}`);
	return tokens[key];
};
const fill = (text) => text.replace(/\{\{\s*([a-z_]+(?::[A-Za-z0-9._-]+)?)\s*\}\}/g, resolve);

// ------------------------------------------------------------------- the page
// The stylesheet is inlined: one request fewer before the first paint, and the
// source keeps its own file. Fonts are referenced from the stylesheet with a
// path relative to it, which has to be re-based once it sits in the page.
const css = readFileSync(join(here, 'assets/css/site.css'), 'utf8')
	.replace(/\/\*[\s\S]*?\*\//g, '')
	.replace(/url\((['"]?)\.\.\//g, 'url($1assets/')
	.replace(/\s+/g, ' ')
	.replace(/ ?([{};]) ?/g, '$1')
	.trim();
const link = '<link rel="stylesheet" href="assets/css/site.css">';
let page = readFileSync(join(here, 'index.html'), 'utf8');
if (!page.includes(link)) fail(`index.html must contain ${link}`);
page = fill(page.replace(link, () => `<style>${css}</style>`));

rmSync(dist, { recursive: true, force: true });
mkdirSync(dist, { recursive: true });
writeFileSync(join(dist, 'index.html'), page);
writeFileSync(join(dist, 'robots.txt'), fill(readFileSync(join(here, 'robots.txt'), 'utf8')));
writeFileSync(join(dist, 'sitemap.xml'), fill(readFileSync(join(here, 'sitemap.xml'), 'utf8')));
writeFileSync(join(dist, '.nojekyll'), '');

// Assets travel as they are, minus the notes that document them.
cpSync(join(here, 'assets'), join(dist, 'assets'), {
	recursive: true,
	filter: (source) => !/\.md$/i.test(source) && !/[\\/]css$/.test(source) && !/site\.css$/.test(source)
});

const files = [];
const walk = (directory) => {
	for (const entry of readdirSync(directory)) {
		const path = join(directory, entry);
		if (statSync(path).isDirectory()) walk(path);
		else files.push({ path: relative(dist, path).replaceAll('\\', '/'), size: statSync(path).size });
	}
};
walk(dist);
const kib = (bytes) => `${(bytes / 1024).toFixed(1)} KiB`;
const total = files.reduce((sum, file) => sum + file.size, 0);
const video = files.filter((file) => file.path.endsWith('.mp4')).reduce((sum, file) => sum + file.size, 0);
console.log(`build: ${siteUrl}/  ->  ${relative(process.cwd(), dist) || dist}`);
console.log(`       release ${release.tag} (${tokens.date}, ${licence}); ${files.length} files, ${kib(total)} (${kib(video)} of it the recording)`);

if (argv.includes('--check')) {
	const run = spawnSync(process.execPath, [join(here, 'check.mjs'), '--site-url', siteUrl], { stdio: 'inherit' });
	process.exit(run.status ?? 1);
}
