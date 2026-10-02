#!/usr/bin/env node
//
// Structural checks on site/dist, run after node site/build.mjs (or with
// `node site/build.mjs --check`). No dependencies.
//
//	node site/check.mjs [--site-url https://example.org]
//
// It checks what a screenshot cannot: that nothing is fetched from another
// site, that every local reference resolves, that the page has one h1, that
// every image declares its size (no layout shift), that the share card is the
// size it claims, and that the page keeps the words decisions 161 and 163 ask
// for. It does not judge the copy against the repository; that is a reading.

import { existsSync, readFileSync, readdirSync, statSync } from 'node:fs';
import { dirname, join, relative } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const dist = join(here, 'dist');
const argv = process.argv.slice(2);
const given = argv.includes('--site-url') ? argv[argv.indexOf('--site-url') + 1] : null;
const config = JSON.parse(readFileSync(join(here, 'site.config.json'), 'utf8'));
const siteUrl = (given || process.env.SITE_URL || config.siteUrl).replace(/\/+$/, '');

if (!existsSync(join(dist, 'index.html'))) {
	console.error('check: site/dist/index.html is missing; run node site/build.mjs first.');
	process.exit(1);
}
const html = readFileSync(join(dist, 'index.html'), 'utf8');
const failures = [];
const check = (ok, message) => {
	if (!ok) failures.push(message);
};

// Anchors may leave the page, to these hosts and no others. Subresources may not
// leave it at all.
const HOSTS = new Set(['github.com', 'discord.gg', 'polyformproject.org', 'www.gnu.org', 'www.themoviedb.org']);

// ------------------------------------------------------------------ the tags
const tags = [...html.matchAll(/<([a-zA-Z][a-zA-Z0-9-]*)((?:\s+[^\s=>"']+(?:\s*=\s*(?:"[^"]*"|'[^']*'|[^\s>]+))?)*)\s*\/?>/g)].map((match) => {
	const attributes = {};
	for (const attribute of match[2].matchAll(/([^\s=>"']+)(?:\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+)))?/g)) {
		attributes[attribute[1].toLowerCase()] = attribute[2] ?? attribute[3] ?? attribute[4] ?? '';
	}
	return { name: match[1].toLowerCase(), attributes, index: match.index };
});
const ofTag = (name) => tags.filter((tag) => tag.name === name);
const remote = (value) => /^(?:https?:)?\/\//i.test(value ?? '');

// ------------------------------------------------------------------- document
check(!/\{\{/.test(html), 'an unresolved {{token}} is left in the page');
check(/<html[^>]*\blang="en"/.test(html), '<html lang="en"> is missing');
check(ofTag('h1').length === 1, `the page has ${ofTag('h1').length} <h1>; it must have exactly one`);

let depth = 0;
for (const heading of tags.filter((tag) => /^h[1-6]$/.test(tag.name))) {
	const level = Number(heading.name[1]);
	check(level <= depth + 1, `heading level jumps from h${depth} to h${level}`);
	depth = level;
}

const title = /<title>([^<]*)<\/title>/.exec(html)?.[1].trim() ?? '';
check(title.length >= 20 && title.length <= 70, `the title is ${title.length} characters; keep it between 20 and 70`);
const meta = (key) => tags.find((tag) => tag.name === 'meta' && (tag.attributes.name === key || tag.attributes.property === key))?.attributes.content;
const description = meta('description') ?? '';
check(description.length >= 70 && description.length <= 160, `the meta description is ${description.length} characters; keep it between 70 and 160`);

const canonical = tags.find((tag) => tag.name === 'link' && tag.attributes.rel === 'canonical')?.attributes.href;
check(canonical === `${siteUrl}/`, `canonical is ${canonical}, expected ${siteUrl}/`);
check(meta('og:url') === `${siteUrl}/`, 'og:url is not the canonical URL');
for (const key of ['og:title', 'og:description', 'og:image', 'og:image:alt', 'og:type', 'twitter:card', 'twitter:image', 'twitter:title', 'twitter:description']) {
	check(Boolean(meta(key)), `${key} is missing`);
}
check(meta('twitter:card') === 'summary_large_image', 'twitter:card should be summary_large_image');

// The share card is a file the page promises: it must exist and be 1200x630.
const card = meta('og:image') ?? '';
check(card.startsWith(`${siteUrl}/`), 'og:image must be absolute and under the site URL');
const cardPath = join(dist, card.slice(siteUrl.length + 1));
if (existsSync(cardPath)) {
	const bytes = readFileSync(cardPath);
	let width = 0;
	let height = 0;
	if (bytes[0] === 0x89 && bytes.toString('latin1', 1, 4) === 'PNG') {
		width = bytes.readUInt32BE(16);
		height = bytes.readUInt32BE(20);
	} else if (bytes[0] === 0xff && bytes[1] === 0xd8) {
		for (let at = 2; at < bytes.length; ) {
			if (bytes[at] !== 0xff) {
				at += 1;
				continue;
			}
			const marker = bytes[at + 1];
			if (marker >= 0xc0 && marker <= 0xc3) {
				height = bytes.readUInt16BE(at + 5);
				width = bytes.readUInt16BE(at + 7);
				break;
			}
			at += 2 + bytes.readUInt16BE(at + 2);
		}
	}
	check(width === 1200 && height === 630, `the share card is ${width}x${height}, expected 1200x630`);
	check(bytes.length < 600 * 1024, `the share card is ${(bytes.length / 1024).toFixed(0)} KiB; keep it under 600 KiB for crawlers`);
} else {
	check(false, `the share card ${card} does not exist in dist`);
}

// ------------------------------------------------------------ structured data
const ld = [...html.matchAll(/<script type="application\/ld\+json">([\s\S]*?)<\/script>/g)];
check(ld.length === 1, `expected one JSON-LD block, found ${ld.length}`);
try {
	const data = JSON.parse(ld[0]?.[1] ?? '');
	check(data['@type'] === 'SoftwareApplication', 'JSON-LD is not a SoftwareApplication');
	check(data.offers?.price === '0', 'JSON-LD offers.price must be "0": nothing paid exists');
	check(data.url === `${siteUrl}/`, 'JSON-LD url is not the canonical URL');
	check(!('aggregateRating' in data) && !('review' in data), 'JSON-LD must not carry ratings or reviews');
} catch (error) {
	check(false, `JSON-LD does not parse: ${error.message}`);
}

// --------------------------------------------------------------- references
const ids = new Set();
for (const tag of tags) {
	const id = tag.attributes.id;
	if (id === undefined) continue;
	check(!ids.has(id), `id "${id}" is used twice`);
	ids.add(id);
}

const local = (reference) => reference.split('#')[0].split('?')[0];
const exists = (reference) => existsSync(join(dist, decodeURIComponent(local(reference))));
const referenced = [];
for (const tag of tags) {
	for (const name of ['src', 'poster', 'href']) {
		const value = tag.attributes[name];
		if (value === undefined) continue;
		const subresource = !(tag.name === 'a' || (tag.name === 'link' && ['canonical', 'alternate'].includes(tag.attributes.rel)));
		referenced.push({ tag, name, value, subresource });
	}
	for (const candidate of (tag.attributes.srcset ?? '').split(',').map((part) => part.trim().split(/\s+/)[0]).filter(Boolean)) {
		referenced.push({ tag, name: 'srcset', value: candidate, subresource: true });
	}
}
for (const { tag, name, value, subresource } of referenced) {
	const where = `<${tag.name} ${name}="${value}">`;
	if (value.startsWith('#')) {
		if (value !== '#') check(ids.has(value.slice(1)), `${where} points at a missing id`);
	} else if (/^(?:mailto|tel|data|javascript):/i.test(value)) {
		check(!/^javascript:/i.test(value), `${where} uses javascript:`);
	} else if (remote(value)) {
		if (subresource) {
			check(false, `${where} loads a subresource from another site`);
		} else if (tag.name === 'a') {
			const host = new URL(value.startsWith('//') ? `https:${value}` : value).hostname;
			check(HOSTS.has(host), `${where} leaves for ${host}, which is not on the list`);
			check(/^https:/i.test(value), `${where} is not https`);
		}
	} else {
		check(exists(value), `${where} does not exist in dist`);
	}
}
check(!/@import|url\(\s*['"]?\s*(?:https?:)?\/\//i.test(html), 'the inline CSS reaches another site');
for (const frame of ['iframe', 'embed', 'object', 'audio']) check(ofTag(frame).length === 0, `<${frame}> is not allowed`);
for (const script of ofTag('script')) check(!script.attributes.src || !remote(script.attributes.src), 'a script is loaded from another site');
check(ofTag('script').filter((script) => !script.attributes.src && script.attributes.type !== 'application/ld+json').length <= 1, 'more than one inline script');

// ------------------------------------------------------------------- images
for (const image of ofTag('img')) {
	const src = image.attributes.src ?? '';
	check(image.attributes.alt !== undefined, `<img ${src}> has no alt attribute (an empty one is allowed)`);
	check(Boolean(image.attributes.width) && Boolean(image.attributes.height), `<img ${src}> does not declare width and height`);
}
for (const source of ofTag('source').filter((tag) => tag.attributes.srcset)) {
	check(Boolean(source.attributes.width) && Boolean(source.attributes.height), `<source ${source.attributes.srcset}> does not declare width and height`);
}

// -------------------------------------------------------------------- video
const videos = ofTag('video');
check(videos.length === 1, `expected one <video>, found ${videos.length}`);
for (const video of videos) {
	check('controls' in video.attributes, '<video> has no controls');
	check(!('autoplay' in video.attributes), '<video> must not autoplay');
	check(video.attributes.preload === 'metadata', '<video> must use preload="metadata"');
	check(Boolean(video.attributes.poster), '<video> has no poster');
}
const track = ofTag('track')[0];
check(Boolean(track) && track.attributes.kind === 'captions', 'the video has no captions track');
if (track && exists(track.attributes.src)) {
	check(readFileSync(join(dist, local(track.attributes.src)), 'utf8').startsWith('WEBVTT'), 'the captions file does not start with WEBVTT');
}

// --------------------------------------------------------------- the words
// The visible text, without tags, scripts, styles and attribute values.
const text = html
	.replace(/<script[\s\S]*?<\/script>|<style[\s\S]*?<\/style>/g, ' ')
	.replace(/<[^>]+>/g, ' ')
	.replace(/&nbsp;/g, ' ')
	.replace(/\s+/g, ' ');
const forbidden = [
	[/\bopen[- ]source\b(?! initiative)/i, 'says "open source": from 4.0 the word is source-available (decision 163)'],
	[/\bcoming soon\b/i, 'says "coming soon": nothing paid is announced on this page (decisions 161, 163)'],
	[/[$€£]\s?\d|\b\d+\s?(?:USD|EUR|dollars?|euros?)\b/i, 'prints a price: nothing paid exists'],
	[/\b(?:Pro|Premium|Plus|Supporter)\b(?! (?:Wars|Pass))/, 'names a paid tier: nothing paid exists'],
	[/\bautoplay\b/i, 'mentions autoplay (the page never autoplays; the player setting is not this page\'s subject)'],
	[/\b(?:atmos|dts-hd(?: ma)?|truehd)\b[^.]{0,80}\bend[- ]to[- ]end\b(?![^.]{0,40}\bnot\b)/i, 'claims bitstream audio end to end: not observed (docs/v3.3.md)']
];
for (const [pattern, why] of forbidden) {
	const hit = pattern.exec(text);
	check(!hit, `the page ${why} - found "${hit?.[0]}"`);
}
check(/source-available/i.test(text), 'the page never says "source-available"');
check(/PolyForm Noncommercial 1\.0\.0/.test(text) && /GPL-3\.0/.test(text), 'the licence sentence must name PolyForm Noncommercial 1.0.0 and GPL-3.0');
check(/This product uses the TMDB API but is not endorsed or certified by TMDB\./.test(text), 'the TMDB attribution sentence is missing or altered (decision 10)');
check(/Illustration/.test(text), 'the illustrations are not labelled');

// -------------------------------------------------------------------- weight
const files = [];
const walk = (directory) => {
	for (const entry of readdirSync(directory)) {
		const path = join(directory, entry);
		if (statSync(path).isDirectory()) walk(path);
		else files.push({ path: relative(dist, path).replaceAll('\\', '/'), size: statSync(path).size });
	}
};
walk(dist);
const heavy = files.filter((file) => !file.path.endsWith('.mp4'));
const weight = heavy.reduce((sum, file) => sum + file.size, 0);
check(Buffer.byteLength(html) < 90 * 1024, `index.html is ${(Buffer.byteLength(html) / 1024).toFixed(0)} KiB; keep it under 90`);
check(weight < 1.5 * 1024 * 1024, `everything but the recording weighs ${(weight / 1024).toFixed(0)} KiB; keep it under 1536`);
const unreferenced = files.filter((file) => !/\.(txt|xml|html)$|^\.nojekyll$|LICENSE/.test(file.path) && !html.includes(file.path) && !html.includes(file.path.replace(/^assets\//, '')));
for (const file of unreferenced) check(false, `${file.path} is shipped but never referenced`);

// ------------------------------------------------------------------- report
if (failures.length > 0) {
	console.error(`\ncheck: ${failures.length} problem(s)\n`);
	for (const failure of failures) console.error(`  - ${failure}`);
	console.error('');
	process.exit(1);
}
console.log(`check: ${files.length} files, ${(weight / 1024).toFixed(0)} KiB without the recording, no problem found.`);
