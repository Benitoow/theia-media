// Renders the OSD in a real browser and writes pictures of it.
//
// The OSD is a web page with exactly one external dependency - window.__TAURI__
// - so faking that is enough to see what a viewer sees. This exists because
// "it compiled" is not a visual check, and because the first run of it found a
// real fault that no amount of reading would have: the design tokens were
// written as a Tailwind `@theme` block, the OSD has no Tailwind, and a browser
// ignores an at-rule it does not know. Every colour, font, size and duration
// was therefore undefined and the interface quietly wore browser defaults.
//
//   npm run check:render            # needs the OSD built, and a preview running
//
// It borrows Playwright from the web application's node_modules rather than
// installing a second copy of it.

import { createRequire } from 'node:module';
import { mkdirSync, existsSync, readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const repo = join(here, '..', '..', '..');

const require = createRequire(join(repo, 'web', 'package.json'));
const { chromium } = require('playwright');

const URL = process.argv[2] ?? 'http://localhost:5199/';
const OUT = process.argv[3] ?? join(repo, 'player', 'ui', 'render-check');
mkdirSync(OUT, { recursive: true });

// A film behind the OSD, so contrast is judged over picture rather than over a
// browser's white canvas. Any frame will do; the spike wrote some. The same
// bytes answer the artwork requests, so a card is drawn at a realistic size
// instead of collapsing to its text fallback the way a dead image would.
const frameCandidates = [
	join(repo, 'player', 'ui', 'render-check', 'frame.png'),
	join(process.env.TEMP ?? '/tmp', 'theia-v33-probe', 'frame-2.png'),
].filter(existsSync);
const FRAME_BYTES = frameCandidates.length ? readFileSync(frameCandidates[0]) : null;
const FRAME = FRAME_BYTES ? 'data:image/png;base64,' + FRAME_BYTES.toString('base64') : null;

// What mpv reports for a film with two audio tracks and two subtitle tracks.
//
// The external one is what the player itself adds: the server's sidecar for
// `Multi.Track.2021.fr.srt` arrives as `srt` on disk, is served as WebVTT, and
// is handed to mpv with the language in the title position as well as its own -
// because mpv derives a title from the URL when given neither, which put
// "6?profile=1" in the menu once.
// One second of H.264, 64x36, 2085 bytes, generated once with the same pinned
// ffmpeg the playback suite downloads:
//
//   ffmpeg -f lavfi -i testsrc=size=64x36:rate=10:duration=1 -c:v libx264 \
//          -preset veryfast -crf 40 -pix_fmt yuv420p -movflags +faststart probe.mp4
//
// Embedded rather than fetched: the assertion below is about a video that
// decodes, and a harness that answers with an empty body would test the
// interface against a clip nobody can play. It is small enough to live here and
// it is the only fixture in this file that is not hand-written.
const PROBE_CLIP = 'AAAAIGZ0eXBpc29tAAACAGlzb21pc28yYXZjMW1wNDEAAANqbW9vdgAAAGxtdmhkAAAAAAAAAAAAAAAAAAAD6AAAA+gAAQAAAQAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAgAAApR0cmFrAAAAXHRraGQAAAADAAAAAAAAAAAAAAABAAAAAAAAA+gAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAABAAAAAAEAAAAAkAAAAAAAkZWR0cwAAABxlbHN0AAAAAAAAAAEAAAPoAAAIAAABAAAAAAIMbWRpYQAAACBtZGhkAAAAAAAAAAAAAAAAAAAoAAAAKABVxAAAAAAALWhkbHIAAAAAAAAAAHZpZGUAAAAAAAAAAAAAAABWaWRlb0hhbmRsZXIAAAABt21pbmYAAAAUdm1oZAAAAAEAAAAAAAAAAAAAACRkaW5mAAAAHGRyZWYAAAAAAAAAAQAAAAx1cmwgAAAAAQAAAXdzdGJsAAAAv3N0c2QAAAAAAAAAAQAAAK9hdmMxAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAEAAJABIAAAASAAAAAAAAAABFUxhdmM2Mi4yOC4xMDIgbGlieDI2NAAAAAAAAAAAAAAAGP//AAAANWF2Y0MBZAAK/+EAGWdkAAqs2UR/nwEQAAADABAAAAMBQPEiWWABAAVo74OcsP34+AAAAAAQcGFzcAAAAAEAAAABAAAAFGJ0cnQAAAAAAAAkWAAAAAAAAAAYc3R0cwAAAAAAAAABAAAACgAABAAAAAAUc3RzcwAAAAAAAAABAAAAAQAAABhjdHRzAAAAAAAAAAEAAAAKAAAIAAAAABxzdHNjAAAAAAAAAAEAAAABAAAACgAAAAEAAAA8c3RzegAAAAAAAAAAAAAACgAAA78AAAAQAAAAGwAAABQAAAAZAAAAHQAAABYAAAAXAAAAFAAAABYAAAAUc3RjbwAAAAAAAAABAAADmgAAAGJ1ZHRhAAAAWm1ldGEAAAAAAAAAIWhkbHIAAAAAAAAAAG1kaXJhcHBsAAAAAAAAAAAAAAAALWlsc3QAAAAlqXRvbwAAAB1kYXRhAAAAAQAAAABMYXZmNjIuMTIuMTAyAAAACGZyZWUAAASTbWRhdAAAAq4GBf//qtxF6b3m2Ui3lizYINkj7u94MjY0IC0gY29yZSAxNjUgcjMyMjNNIDA0ODBjYjAgLSBILjI2NC9NUEVHLTQgQVZDIGNvZGVjIC0gQ29weWxlZnQgMjAwMy0yMDI1IC0gaHR0cDovL3d3dy52aWRlb2xhbi5vcmcveDI2NC5odG1sIC0gb3B0aW9uczogY2FiYWM9MSByZWY9MSBkZWJsb2NrPTE6MDowIGFuYWx5c2U9MHgzOjB4MTEzIG1lPWhleCBzdWJtZT0yIHBzeT0xIHBzeV9yZD0xLjAwOjAuMDAgbWl4ZWRfcmVmPTAgbWVfcmFuZ2U9MTYgY2hyb21hX21lPTEgdHJlbGxpcz0wIDh4OGRjdD0xIGNxbT0wIGRlYWR6b25lPTIxLDExIGZhc3RfcHNraXA9MSBjaHJvbWFfcXBfb2Zmc2V0PTAgdGhyZWFkcz0xIGxvb2thaGVhZF90aHJlYWRzPTEgc2xpY2VkX3RocmVhZHM9MCBucj0wIGRlY2ltYXRlPTEgaW50ZXJsYWNlZD0wIGJsdXJheV9jb21wYXQ9MCBjb25zdHJhaW5lZF9pbnRyYT0wIGJmcmFtZXM9MyBiX3B5cmFtaWQ9MiBiX2FkYXB0PTEgYl9iaWFzPTAgZGlyZWN0PTEgd2VpZ2h0Yj0xIG9wZW5fZ29wPTAgd2VpZ2h0cD0xIGtleWludD0yNTAga2V5aW50X21pbj0xMCBzY2VuZWN1dD00MCBpbnRyYV9yZWZyZXNoPTAgcmNfbG9va2FoZWFkPTEwIHJjPWNyZiBtYnRyZWU9MSBjcmY9NDAuMCBxY29tcD0wLjYwIHFwbWluPTAgcXBtYXg9NjkgcXBzdGVwPTQgaXBfcmF0aW89MS40MCBhcT0xOjEuMDAAgAAAAQlliIQAn9IdvRVXb/Evh7ITk36LWO1erYjBEpXyf8OGIW5xVLeHw74K8jPJdXYgIRyjORkmpJNPFkPHbhHDf1CDzQDgE5ikkT/+0kiVGhp509rwAcpDGv5I86773UZ97aydBFt9EURYzU24i1DolwQKoDAneJgAhf3LAhfkZfzXbPdZ8cRHZ8neLUypf8y67m3tOCHsNe30BRGSU9L5vm5IVvhGyenVSY/i9ZxowzV5vUb+8984IOq7nf1EQyE9kR3o7oOWQc3b84ot4EGdOVg/0jrxEKzeorAuJfNxuUIvcB3R4Xd9G9X5ticiqOiO/dk4Rzw3xm3ChlBFOidM8p3tCabjGncKj5u/AAAADEGaIRiT/4hla0RywAAAABdBmkIYm/+bB0qTJeUXM+e9WZCNAFiljQAAABBBmmMYm/+ar7ddNZ1qecOAAAAAFUGahBib/5sHSjAfryLhMRLNQrOZZQAAABlBmqUYiP/WMAK+jTjBtQPMTe3sV8t/R4RtAAAAEkGaxhiK/6YBCpksBFVRLprBuwAAABNBmucYiv+mOMhB2Y1ox1WA5yOJAAAAEEGbCBiO/7JLha6/Ki5uSqgAAAASQZspGIS/vAXEWrjDs4N0XEy1';

const TRACKS = [
	{ id: 1, type: 'video', codec: 'hevc', selected: true },
	{ id: 1, type: 'audio', title: 'Francais', lang: 'fra', codec: 'ac3', 'demux-channels': 'stereo', selected: true },
	{ id: 2, type: 'audio', title: 'English', lang: 'eng', codec: 'ac3', 'demux-channels': 'stereo', selected: false },
	{ id: 1, type: 'sub', lang: 'fra', codec: 'subrip', selected: false },
	{ id: 2, type: 'sub', title: 'fra', lang: 'fra', codec: 'webvtt', external: true, selected: true },
];

const STATUS = {
	ready: true,
	engine: 'mpv v0.41.0-1049-g0b7ed670f',
	media: 'http://127.0.0.1:8395/api/stream/3/files/3?profile=1',
	title: 'Multi Track',
	pause: false,
	mute: true,
	pos: 128.4,
	duration: 600,
	vo: 'gpu-next',
	hwdec: 'd3d11va',
	ao: 'wasapi',
	audioMode: 'pcm',
	audioReason: 'endpoint-refused-bitstream',
	videoCodec: 'hevc',
	aid: 1,
	sid: 1,
};

const MOVIES = [
	{
		id: 1,
		title: 'Probe Film',
		year: 2024,
		// The synopsis the preview shows, and the reason the second film has
		// none: a preview that invents one is worse than a preview that says
		// less, so both paths are exercised.
		metadata: { backdrop_path: '/probe-backdrop.jpg', overview: 'A probe film about probes, long enough to be clamped by the frame it is drawn in.' },
		backdrop_url: '/api/images/w780/probe-backdrop.jpg',
		poster_url: '/api/images/w500/probe-poster.jpg',
		progress: { position_seconds: 0, duration_seconds: 600, finished: false },
	},
	{
		id: 2,
		title: 'Second Film',
		year: 2023,
		// A poster and no backdrop: the second fallback, cased in the frame.
		metadata: { poster_path: '/second.jpg' },
		poster_url: '/api/images/w500/second.jpg',
		progress: { position_seconds: 0, duration_seconds: 0, finished: false },
	},
	{
		id: 3,
		title: 'Resume Test',
		year: 2022,
		backdrop_url: '/api/images/w780/resume.jpg',
		progress: { position_seconds: 102.4, duration_seconds: 150, finished: false },
	},
	{
		id: 4,
		title: 'Multi Track',
		year: 2021,
		backdrop_url: '/api/images/w780/multi.jpg',
		progress: { position_seconds: 600, duration_seconds: 600, finished: true },
	},
	{
		// TMDB never matched it and the cache has nothing: the third fallback,
		// the title as text on a surface. Never a broken-image icon.
		id: 5,
		title: 'Unmatched Film',
		year: 2020,
		progress: { position_seconds: 0, duration_seconds: 0, finished: false },
	},
];

const SERIES = [
	{
		id: 9,
		title: 'Shogun',
		year: 2024,
		metadata: { name: 'Shōgun', backdrop_path: '/shogun.jpg' },
		backdrop_url: '/api/images/w780/shogun.jpg',
		poster_url: '/api/images/w500/shogun-poster.jpg',
	},
];

const SERIES_DETAIL = {
	...SERIES[0],
	// Two seasons, not one: a tab that looks chosen can only be checked against
	// a tab that does not, and with a single season the assertion below would
	// have had nothing to compare.
	seasons: [
		{ id: 91, series_id: 9, season_number: 1, metadata: { name: 'Saison 1', episode_count: 2 } },
		{ id: 92, series_id: 9, season_number: 2, metadata: { name: 'Saison 2', episode_count: 1 } },
	],
};

const SEASON = {
	id: 91,
	series_id: 9,
	season_number: 1,
	metadata: { name: 'Saison 1', episode_count: 2 },
	episodes: [
		{
			id: 901,
			series_id: 9,
			series_title: 'Shogun',
			season_number: 1,
			episode_numbers: [1],
			episode_metadata: [
				{ id: 901, episode_number: 1, local_title: 'Anjin', metadata: { name: "L'Anjin", runtime_minutes: 71 } },
			],
			still_url: '/api/images/w780/anjin.jpg',
			progress: { position_seconds: 420, duration_seconds: 4260, finished: false },
		},
		{
			id: 902,
			series_id: 9,
			series_title: 'Shogun',
			season_number: 1,
			episode_numbers: [2],
			episode_metadata: [
				{ id: 902, episode_number: 2, local_title: 'Servants', metadata: { name: 'Serviteurs de deux maîtres', runtime_minutes: 60 } },
			],
			still_url: '/api/images/w780/servants.jpg',
			progress: { position_seconds: 0, duration_seconds: 3600, finished: false },
		},
	],
};

// The home screen, which is where the player opens now. The hero is the film
// that was left - the same part-watched fixture the grid carries - and every
// row the server can build appears at least once, films and series alike, so
// the composition is checked in one shape rather than assumed.
const HOME = {
	hero: MOVIES[2],
	hero_kind: 'resume',
	rows: [
		{ kind: 'continue', movies: [MOVIES[2]] },
		{ kind: 'recent', movies: [MOVIES[0], MOVIES[4]] },
		{ kind: 'tonight', movies: [MOVIES[1]] },
	],
	total: MOVIES.length,
};

const SERIES_HOME = {
	continue_watching: [SEASON.episodes[0]],
	recent_series: [SERIES[0]],
};

const browser = await chromium.launch();
let failures = 0;

// A failure that names only a scrollWidth sends the next person hunting through
// the DOM with a ruler, so the report names the state and what sticks out.

// Repaints the band magenta and compares two crops: one in the corner square the
// card's radius cuts away, one in the band itself. A clipped band is invisible in
// the first and obvious in the second.
async function assertBandRounded(page) {
	const card = page.locator('.film-art').first();
	// Hovered, because that is where the fault was: a playing preview is a
	// composited video layer and it is the one that drew outside the frame. A
	// check on the still would have passed while the maintainer's screen did not.
	await card.hover();
	await page.waitForTimeout(1200);
	const box = await card.boundingBox();
	if (box == null) return { cornerChanged: true, insideChanged: true };
	const corner = { x: box.x + 1, y: box.y + box.height - 4, width: 3, height: 3 };
	const inside = { x: box.x + 10, y: box.y + Math.round(box.height / 2), width: 3, height: 3 };
	const beforeCorner = await page.screenshot({ clip: corner });
	const beforeInside = await page.screenshot({ clip: inside });
	const tag = await page.addStyleTag({ content: '.film-art::after { background: #ff00ff !important; }' });
	const afterCorner = await page.screenshot({ clip: corner });
	const afterInside = await page.screenshot({ clip: inside });
	await tag.evaluate((el) => el.remove());
	return {
		cornerChanged: !beforeCorner.equals(afterCorner),
		insideChanged: !beforeInside.equals(afterInside),
	};
}

async function assertFits(page, state) {
	const measured = await page.evaluate(() => {
		const culprits = [];
		for (const el of document.querySelectorAll('*')) {
			const r = el.getBoundingClientRect();
			if (r.width > 0 && r.right > window.innerWidth + 0.5) {
				// getAttribute, not className: on an SVG element className is an
				// SVGAnimatedString, which stringifies to [object ...].
				const cls = el.getAttribute('class');
				culprits.push({
					what: el.tagName.toLowerCase() + (cls ? '.' + cls.trim().split(/\s+/).join('.') : ''),
					left: Math.round(r.left),
					right: Math.round(r.right),
				});
			}
		}
		return {
			scrollWidth: document.documentElement.scrollWidth,
			innerWidth: window.innerWidth,
			culprits: culprits.slice(0, 8),
		};
	});
	if (measured.scrollWidth <= measured.innerWidth) return;
	console.error(
		`${state} overflows horizontally: ${measured.scrollWidth}px of content in a ${measured.innerWidth}px window`
	);
	for (const c of measured.culprits) console.error(`  ${c.what} left=${c.left} right=${c.right}`);
	failures++;
}

// The faces the OSD is supposed to wear, and the two it must never be caught
// without.
//
// `document.fonts.check()` cannot answer this question. Measured on 15 September
// 2026: with no `@font-face` in the document at all, it answered `true` for
// `Cinzel Variable` and for `Jost Variable` - so a check that passes on a face
// nobody declared is a check that would have let the shipped release go out in
// Georgia. `web/tests/layout.spec.js` carries the same lesson for the web
// application (decision 79). What is asserted here is the chain instead:
//
//   1. a `@font-face` rule declares the family, and its `src` is a real URL;
//   2. a FontFace of that family reached status `loaded` - which browsers only
//      set once the file was fetched and parsed, so a 404 leaves it `error`;
//   3. the stylesheet actually uses the family, and the element that should wear
//      it does not resolve past it.
const OSD_FACES = [
	{ family: 'Cinzel Variable', used: '.library-title, .home-hero-title' },
	{ family: 'Jost Variable', used: '.label' },
];

async function assertFontsLoaded(page, state) {
	const measured = await page.evaluate((faces) => {
		const declared = [];
		for (const sheet of document.styleSheets) {
			let rules;
			try {
				rules = sheet.cssRules;
			} catch {
				continue; // a cross-origin sheet cannot be read, and is not ours
			}
			for (const rule of rules) {
				if (!(rule instanceof CSSFontFaceRule)) continue;
				declared.push({
					family: rule.style.getPropertyValue('font-family').replace(/^["']|["']$/g, '').trim(),
					src: rule.style.getPropertyValue('src'),
					weight: rule.style.getPropertyValue('font-weight').trim(),
				});
			}
		}
		const loaded = [...document.fonts].map((f) => ({ family: f.family.replace(/^["']|["']$/g, ''), status: f.status, weight: f.weight }));
		return {
			declared,
			loaded,
			fontsSize: document.fonts.size,
			used: faces.map(({ family, used }) => {
				const el = document.querySelector(used);
				return {
					family,
					used,
					elementFound: !!el,
					resolved: el ? getComputedStyle(el).fontFamily : null,
				};
			}),
		};
	}, OSD_FACES);

	for (const { family, used } of OSD_FACES) {
		const rule = measured.declared.find((d) => d.family === family);
		if (!rule || !/url\(/.test(rule.src)) {
			console.error(`${state}: no @font-face declares "${family}", so the OSD falls back to whatever the system has`);
			failures++;
		}
		const face = measured.loaded.find((f) => f.family === family);
		if (!face) {
			console.error(`${state}: "${family}" is declared in the stylesheet but no FontFace reached document.fonts (size=${measured.fontsSize})`);
			failures++;
		} else if (face.status !== 'loaded') {
			console.error(`${state}: "${family}" is ${face.status}, not loaded - the file was refused or never fetched`);
			failures++;
		}
		const use = measured.used.find((u) => u.family === family);
		if (!use?.elementFound) {
			console.error(`${state}: "${family}" cannot be verified as used: ${used} is not on screen`);
			failures++;
		} else if (!use.resolved.includes(family)) {
			console.error(`${state}: ${used} resolves to "${use.resolved}", which never reaches "${family}"`);
			failures++;
		}
	}
}

// Every target a finger can land on clears 44x44 (design system section 9).
//
// The exception is written into the check rather than left implicit: `.scrub`
// carries `role="slider"`, and the design system is genuinely of two minds about
// it - section 6b says its hit area is 24px, section 9 says "every interactive
// target is at least 44x44px". D3b was answered on 15 September 2026: 44px
// around a 4px painted line. So it is asserted like the rest, and the message
// says which rule it answers to.
async function assertHitTargets(page, state) {
	const measured = await page.evaluate(() => {
		const out = [];
		for (const el of document.querySelectorAll('button, a, input, [role=slider], select, textarea')) {
			const cs = getComputedStyle(el);
			if (cs.display === 'none' || cs.visibility === 'hidden') continue;
			const r = el.getBoundingClientRect();
			if (r.width === 0 && r.height === 0) continue;
			out.push({
				what: el.tagName.toLowerCase() + (el.getAttribute('class') ? '.' + el.getAttribute('class').trim().split(/\s+/).join('.') : ''),
				label: (el.getAttribute('aria-label') ?? el.textContent ?? '').trim().slice(0, 40),
				w: Math.round(r.width * 100) / 100,
				h: Math.round(r.height * 100) / 100,
			});
		}
		return out;
	});
	for (const target of measured) {
		if (target.w >= 44 - 0.01 && target.h >= 44 - 0.01) continue;
		console.error(
			`${state}: ${target.what} "${target.label}" is ${target.w}x${target.h}, under the 44x44 floor of section 9` +
				(target.what.includes('scrub') ? ' (section 6b says 24px; D3b chose 44px around the 4px line)' : '')
		);
		failures++;
	}
}

// A keyboard shortcut is not a text field's problem.
//
// The OSD listens for keys on the window (App.svelte, `<svelte:window
// onkeydown={onKey}>`), so every shortcut it owns is also live while somebody is
// typing a server address. Measured on 15 September 2026 with the real build:
// `k` and Space never reached the field, and `l` switched the whole interface to
// English mid-address. This asserts the user's gesture instead - type the
// address, then read what happened - rather than the handler's shape, so it
// still fails if the chord is moved rather than guarded.
async function assertTypingIsNotShortcuts(page, address) {
	await page.evaluate(() => {
		window.__commands = [];
	});
	await page.fill('#theia-address', '');
	await page.locator('#theia-address').pressSequentially(address, { delay: 15 });
	const typed = await page.inputValue('#theia-address');
	if (typed !== address) {
		console.error(
			`typing the address: the field holds ${JSON.stringify(typed)} instead of ${JSON.stringify(address)} - a shortcut ate the difference`
		);
		failures++;
	}
	const commands = await page.evaluate(() => window.__commands ?? []);
	const playback = commands.filter((c) => c !== 'player_library' && c !== 'player_tracks');
	if (playback.length) {
		console.error(`typing the address issued ${playback.length} playback command(s): ${playback.join(', ')}`);
		failures++;
	}
	const lang = await page.evaluate(() => document.documentElement.lang);
	if (lang !== 'en') {
		console.error(`typing the address changed the interface language to "${lang}"`);
		failures++;
	}
}

// The furniture hides after three seconds of nothing, and never while paused.
//
// The three seconds are asserted as the two facts a person can see: still there
// at 2.5s, gone by 3.5s. A window of one second either side of the boundary is
// what makes this a behaviour rather than a timer implementation.
async function assertIdleTiming(page) {
	await page.evaluate((status) => window.__handlers['player-status']?.({ payload: JSON.stringify(status) }), STATUS);
	// A pointer move over the *picture* is a sign of life and nothing else. It is
	// deliberately not (10, 10): that lands on the title bar, the OSD correctly
	// keeps the furniture up for a pointer resting on its own controls, and the
	// first version of this check reported that correct behaviour as a fault.
	await page.mouse.move(640, 360);
	await page.waitForTimeout(2500);
	const at25 = await page.getAttribute('.osd', 'data-idle');
	await page.waitForTimeout(1000);
	const at35 = await page.getAttribute('.osd', 'data-idle');
	if (at25 === 'true') {
		console.error('the furniture was already hidden 2.5s after the last sign of life; the rule is three seconds');
		failures++;
	}
	if (at35 !== 'true') {
		console.error('the furniture was still up 3.5s after the last sign of life over the picture; it should have hidden');
		failures++;
	}

	// And it must come back in under half a second, on the two signs of life a
	// person actually gives: a key and a pointer move.
	//
	// The key is dispatched from a control rather than from the address field:
	// this page is connected, so the field has correctly been taken away, and a
	// check that waited for it would time out on a form the product removed on
	// purpose.
	await page.focus('button.control--primary');
	const keyWake = await page.evaluate(async () => {
		const osd = document.querySelector('.osd');
		const t0 = performance.now();
		document.activeElement.dispatchEvent(new KeyboardEvent('keydown', { key: 'a', bubbles: true }));
		while (osd.getAttribute('data-idle') === 'true' && performance.now() - t0 < 2000) {
			await new Promise((r) => requestAnimationFrame(r));
		}
		return Math.round(performance.now() - t0);
	});
	if (keyWake > 500) {
		console.error(`a key took ${keyWake}ms to bring the furniture back; the rule is under 500ms`);
		failures++;
	}
	await page.mouse.move(640, 360);
	await page.waitForTimeout(250);
	await page.mouse.move(700, 400);
	const pointerWake = await page.evaluate(async () => {
		const osd = document.querySelector('.osd');
		const t0 = performance.now();
		while (osd.getAttribute('data-idle') === 'true' && performance.now() - t0 < 2000) {
			await new Promise((r) => requestAnimationFrame(r));
		}
		return Math.round(performance.now() - t0);
	});
	if (pointerWake > 500) {
		console.error(`a pointer move took ${pointerWake}ms to bring the furniture back; the rule is under 500ms`);
		failures++;
	}

	// And it must not hide while the film is paused: the controls are the only
	// way to start it again.
	await page.evaluate((status) => window.__handlers['player-status']?.({ payload: JSON.stringify({ ...status, pause: true }) }), STATUS);
	await page.waitForTimeout(3500);
	const paused = await page.getAttribute('.osd', 'data-idle');
	if (paused === 'true') {
		console.error('the furniture hid while the film was paused, taking the only way to resume it');
		failures++;
	}
}

// One press is one command.
//
// A double-handled click is invisible in a screenshot and obvious in a film that
// toggles twice. Every control is pressed once and the commands it issued are
// counted.
async function assertOnePressOneCommand(page) {
	const controls = await page.locator('.row button.control:visible').count();
	for (let i = 0; i < controls; i++) {
		const button = page.locator('.row button.control:visible').nth(i);
		const label = await button.getAttribute('aria-label');
		await page.evaluate(() => {
			window.__commands = [];
		});
		await button.click();
		await page.waitForTimeout(150);
		const commands = await page.evaluate(() => window.__commands ?? []);
		const stateChanging = commands.filter((c) => c !== 'player_tracks' && c !== 'player_library');
		if (stateChanging.length > 1) {
			console.error(`pressing "${label}" issued ${stateChanging.length} commands: ${stateChanging.join(', ')}`);
			failures++;
		}
	}
}

// The pointer stays stable while the furniture fades.
//
// WebView2 was measured redrawing a CSS-hidden cursor every four to five seconds,
// which is worse than leaving it visible: the viewer sees a perpetual flicker.
// Until the native surface can own the cursor state, no player state may resolve
// it to `none`.
async function assertCursorFollowsFurniture(page) {
	// The pointer sits in the middle of the picture for the whole of this check,
	// because that is where a viewer leaves it. `underPointer` is the question a
	// person experiences - the cursor of the element the browser would resolve -
	// and it is the one the original version of this assertion never asked.
	//
	// Measured on 16 September 2026: `.osd` declared `cursor: none` and answered
	// `none` from getComputedStyle while the element under the pointer was `body`,
	// which answered `auto`. A rule on an element with `pointer-events: none` never
	// applies, and the check below passed anyway because it asked `html || osd`.
	const settled = async () => {
		await page.waitForTimeout(250);
		return page.evaluate(() => {
			const idle = document.querySelector('.osd')?.getAttribute('data-idle');
			const resolve = (sel) => {
				const el = document.querySelector(sel);
				return el ? getComputedStyle(el).cursor : null;
			};
			const under = document.elementFromPoint(640, 360);
			const describe = (el) =>
				el
					? el.tagName.toLowerCase() +
						(el.getAttribute('class') ? '.' + el.getAttribute('class').trim().split(/\s+/).join('.') : '')
					: null;
			return {
				idle,
				html: resolve('html'),
				body: resolve('body'),
				osd: resolve('.osd'),
				primary: resolve('button.control--primary'),
				scrub: resolve('.scrub'),
				field: resolve('#theia-address'),
				underPointer: describe(under),
				underPointerCursor: under ? getComputedStyle(under).cursor : null,
			};
		});
	};

	// (a) the connect screen: no film, no reason to hide anything.
	const connect = await settled();
	if (connect.html === 'none' || connect.body === 'none' || connect.osd === 'none') {
		console.error(
			`the connect screen hides the pointer (html=${connect.html} body=${connect.body} osd=${connect.osd}) - with no film playing, the field and the buttons are the only way forward`
		);
		failures++;
	}
	if (connect.underPointerCursor === 'none') {
		console.error(
			`the connect screen resolves the pointer to none over ${connect.underPointer} - that is the cursor a person actually sees`
		);
		failures++;
	}
	if (connect.field !== 'text') {
		console.error(`the address field shows "${connect.field}" instead of a text cursor`);
		failures++;
	}

	// (b) connected, library panel: same answer, and now with cards to aim at.
	await page.fill('#theia-address', 'http://127.0.0.1:8395');
	await page.click('button[type=submit]');
	await page.waitForTimeout(600);
	const library = await settled();
	if (library.html === 'none' || library.osd === 'none') {
		console.error(`the library panel hides the pointer (html=${library.html} osd=${library.osd})`);
		failures++;
	}

	// (c) a film playing and left alone: the furniture goes, the pointer stays.
	await page.evaluate((status) => window.__handlers['player-status']?.({ payload: JSON.stringify(status) }), STATUS);
	await page.mouse.move(640, 360);
	await page.waitForTimeout(3800);
	const hidden = await settled();
	if (hidden.idle !== 'true') {
		console.error('the furniture did not hide, so the stable-pointer rule cannot be judged');
		failures++;
	} else if (
		hidden.html === 'none' ||
		hidden.body === 'none' ||
		hidden.osd === 'none' ||
		hidden.underPointerCursor === 'none'
	) {
		console.error(
			`the furniture hid and reintroduced cursor flicker over ${hidden.underPointer}: html=${hidden.html} body=${hidden.body} osd=${hidden.osd} under=${hidden.underPointerCursor}`
		);
		failures++;
	}

	// (d) and a sign of life brings both back.
	await page.mouse.move(500, 300);
	const awake = await settled();
	if (awake.idle === 'true') {
		console.error('the furniture did not come back on a pointer move');
		failures++;
	} else if (awake.underPointerCursor === 'none') {
		console.error(`the furniture came back but the pointer is still hidden over ${awake.underPointer}`);
		failures++;
	}

	// A click on the picture is how a person pauses, and it also focuses whatever
	// the browser decides to focus. If that counts as "focus inside the
	// furniture", the bar then stays on screen for the rest of the film: section
	// 6b hides it after three seconds of no sign of life, and a click is one
	// gesture, not a permanent one. Measured on the real window on 16 September
	// 2026: after a click, the furniture was still up eleven seconds later.
	//
	// The click is made with the mouse rather than through a locator: the OSD
	// layer carries `pointer-events: none` on purpose, so a real press lands on
	// the page behind it and arrives at the window handler - which is exactly what
	// happens to a person.
	await page.mouse.click(400, 200);
	const afterClick = await page.evaluate(() => {
		const el = document.activeElement;
		const inFurniture = !!el?.closest?.('.controls, .title-bar, .notice');
		return {
			active: el ? el.tagName.toLowerCase() + (el.getAttribute('class') ? '.' + el.getAttribute('class').trim().split(/\s+/).join('.') : '') : null,
			inFurniture,
		};
	});
	await page.waitForTimeout(3800);
	const clickIdle = await page.getAttribute('.osd', 'data-idle');
	if (clickIdle !== 'true') {
		console.error(
			`the furniture stayed up after a click on the picture: activeElement=${afterClick.active} inFurniture=${afterClick.inFurniture}, data-idle=${clickIdle}`
		);
		failures++;
	}

	// (e) paused: the pointer stays, exactly as the furniture does.
	await page.evaluate((status) => window.__handlers['player-status']?.({ payload: JSON.stringify({ ...status, pause: true }) }), STATUS);
	await page.waitForTimeout(3500);
	const paused = await settled();
	if (paused.html === 'none' && paused.osd === 'none') {
		console.error('the pointer is hidden while the film is paused, so nothing on screen can be aimed at');
		failures++;
	}

	// Native and CSS hiding have both been measured as unable to hold on WebView2.
	// The assertion above therefore has one answer in every state: never `none`.
}

// Every state in which the furniture must stay, whatever the timer thinks.
//
// Section 6b: it "never hides while paused, seeking or buffering, and never with
// focus stranded on a control". The three-second rule is the easy half; these are
// the exceptions, and each one is a state where hiding would take away the thing
// the person is using. A picture cannot show them, which is why they are counted
// and read instead.
async function assertFurnitureNeverHidesWhenBusy(page) {
	const sleep = (ms) => page.waitForTimeout(ms);
	const visible = async () => page.evaluate(() => {
		const controls = document.querySelector('.controls');
		const opacity = getComputedStyle(controls).opacity;
		const box = controls.getBoundingClientRect();
		return { opacity, visible: Number(opacity) > 0.05 && box.height > 0, hidden: document.querySelector('.osd')?.getAttribute('data-idle') === 'true' };
	});
	const playing = { ...STATUS };

	// (a) the track menu is open, and the viewer walks away from the mouse.
	await page.evaluate((s) => window.__handlers['player-status']?.({ payload: JSON.stringify(s) }), playing);
	await page.mouse.move(640, 360);
	await page.click('.menu-anchor button');
	await sleep(500);
	if (!(await page.locator('.track-menu').count())) {
		console.error('could not open the track menu, so its idle behaviour is untested');
		failures++;
	} else {
		await sleep(3600);
		const state = await visible();
		if (state.hidden || !state.visible) {
			console.error(`the furniture hid with the track menu open (data-idle=${state.hidden}, opacity=${state.opacity}) - the menu the viewer opened went with it`);
			failures++;
		}
		await page.keyboard.press('Escape');
		await sleep(250);
	}

	// (b) buffering: the film is loaded, the engine is not ready, and the spinner
	//     is the only thing telling the viewer anything.
	await page.evaluate((s) => window.__handlers['player-status']?.({ payload: JSON.stringify({ ...s, ready: false }) }), playing);
	await sleep(3600);
	const buffering = await visible();
	if (buffering.hidden || !buffering.visible) {
		console.error(`the furniture hid while the engine was not ready (data-idle=${buffering.hidden}, opacity=${buffering.opacity}) - the buffering notice went with it`);
		failures++;
	}

	// (c) nothing is loaded at all: the library panel is up and there is nothing
	//     to hide from. This one deliberately passes through a *ready* status -
	//     the fix for the buffering case returns early on `!status.ready`, so a
	//     check that leaves ready false here would pass without testing anything.
	//
	//     Since decision D1 the bar is not drawn at all in this state, so
	//     "visible" cannot be the question: there is nothing to be visible. What
	//     is asserted is that the idle timer has not run, because that is what
	//     would have taken the library away.
	await page.evaluate((s) => window.__handlers['player-status']?.({ payload: JSON.stringify({ ...s, ready: true, media: '' }) }), playing);
	await page.mouse.move(400, 300);
	await sleep(3600);
	const idled = await page.getAttribute('.osd', 'data-idle');
	if (idled === 'true') {
		console.error('the idle timer ran while nothing was loaded - the library panel went with it');
		failures++;
	}

	// (d) focus is on a control: hiding it would leave focus on something nobody
	//     can see, and pressing Enter would then act on an invisible target.
	await page.evaluate((s) => window.__handlers['player-status']?.({ payload: JSON.stringify(s) }), playing);
	await page.focus('button.control--primary');
	await page.mouse.move(400, 300);
	await sleep(3600);
	const focused = await visible();
	const stillFocused = await page.evaluate(() => document.activeElement?.className ?? null);
	if (focused.hidden || !focused.visible) {
		console.error(
			`the furniture hid with focus on "${stillFocused}" (opacity=${focused.opacity}) - focus stranded on an invisible control`
		);
		failures++;
	}
}

// A menu owns its own keys, and gives focus back when it closes.
//
// Section 6b: the popover has no dismiss button - Escape, the toggle, and a
// press outside. The arrows belong to the menu while it is open, and a viewer
// who closes it with Escape must land back on the control that opened it, or the
// next key press goes somewhere they did not choose.
async function assertMenuOwnsItsKeys(page) {
	// The anchor is the button that says it opens a menu: `.menu-anchor button`
	// also matched the five rows of the popover, because the popover is a child
	// of the button (6b anchors it by construction).
	const anchor = page.locator('button[aria-haspopup=menu]');
	await anchor.click();
	await page.waitForTimeout(350);
	const anchorLabel = await anchor.getAttribute('aria-label');
	const rows = await page.locator('.track-menu .track').count();
	if (!rows) {
		console.error('the track menu did not open, so its keyboard behaviour is untested');
		failures++;
		return;
	}

	// The arrows must not reach the film. The film is being seeked if the OSD
	// asks for it - the mock records every command.
	for (const key of ['ArrowRight', 'ArrowLeft', 'ArrowUp', 'ArrowDown']) {
		await page.evaluate(() => {
			window.__commands = [];
		});
		await page.keyboard.press(key);
		await page.waitForTimeout(100);
		const seeks = await page.evaluate(() => (window.__commands ?? []).filter((c) => c === 'player_seek'));
		if (seeks.length) {
			console.error(`${key} in the open track menu seeked the film ${seeks.length} time(s); the arrows belong to the menu`);
			failures++;
		}
		if (!(await page.locator('.track-menu').count())) {
			console.error(`${key} closed the track menu; only Escape, the toggle and a press outside may`);
			failures++;
			break;
		}
	}

	// A press inside the menu chooses a track, and re-reads the list rather than
	// assuming mpv agreed.
	await page.evaluate(() => {
		window.__commands = [];
	});
	await page.locator('.track-menu .track').nth(1).click();
	await page.waitForTimeout(200);
	const chosen = await page.evaluate(() => window.__commands ?? []);
	if (!chosen.includes('player_set_track')) {
		console.error(`choosing a track issued ${JSON.stringify(chosen)} - no player_set_track`);
		failures++;
	}
	if (!chosen.includes('player_tracks')) {
		console.error('the track list was not re-read after a choice, so the tick can be wrong');
		failures++;
	}

	// Escape closes it, once, and focus comes home.
	await page.keyboard.press('Escape');
	await page.waitForTimeout(250);
	if (await page.locator('.track-menu').count()) {
		console.error('Escape did not close the track menu');
		failures++;
	}
	const focused = await page.evaluate(() => {
		const el = document.activeElement;
		return { cls: el?.getAttribute('class') ?? null, label: el?.getAttribute('aria-label') ?? null };
	});
	if (focused.label !== anchorLabel) {
		console.error(`after Escape focus is on ${JSON.stringify(focused)} instead of the "${anchorLabel}" button that opened the menu`);
		failures++;
	}
}

// Escape leaves fullscreen before it leaves the film, and the order is asserted.
//
// Measured on 17 September 2026 in the real window with `probes/fullscreen-escape.ps1`:
// fullscreen 1440x900, one Escape, and the player was gone. The handler went
// straight from "is the track menu open" to `close()`, so the one key every viewer
// presses to leave fullscreen ended the film instead. Decision D2 asks for this
// exact sequence to be proposed and validated.
//
// The state reaches the OSD through `tauri://resize`, which is the only signal this
// Tauri version emits for a fullscreen change, so the simulation sends that event
// the way the runtime does. Without it the check would be testing a state the
// product never reaches.
async function assertEscapeLeavesFullscreenFirst(page) {
	const state = () =>
		page.evaluate(() => ({
			fullscreen: window.__fullscreen === true,
			closed: window.__windowClosed === true,
			calls: [...(window.__fullscreenCalls ?? [])],
			menu: !!document.querySelector('.track-menu'),
		}));

	// (a) the product's own binding takes the window fullscreen.
	await page.keyboard.press('f');
	await page.evaluate(() => window.__handlers['tauri://resize']?.({ payload: null }));
	await page.waitForTimeout(250);
	let now = await state();
	if (!now.fullscreen) {
		console.error(`"f" did not take the window fullscreen: ${JSON.stringify(now)}`);
		failures++;
		return;
	}
	const fullscreenPressed = await page.locator('button[aria-pressed="true"]').count();
	if (fullscreenPressed !== 1) {
		console.error(`fullscreen state changed but ${fullscreenPressed} control(s) expose aria-pressed=true`);
		failures++;
	}

	// (b) one Escape gives the window back and keeps the film.
	await page.keyboard.press('Escape');
	await page.evaluate(() => window.__handlers['tauri://resize']?.({ payload: null }));
	await page.waitForTimeout(300);
	now = await state();
	if (now.closed) {
		console.error('Escape in fullscreen closed the player instead of leaving fullscreen - this is the fault D2 names');
		failures++;
	}
	if (now.fullscreen) {
		console.error(`Escape did not leave fullscreen: ${JSON.stringify(now)}`);
		failures++;
	}
	if ((await page.locator('button[aria-pressed="true"]').count()) !== 0) {
		console.error('the fullscreen control still exposes its active state after Escape left fullscreen');
		failures++;
	}

	// (c) and the next Escape returns to the library without killing the player.
	await page.keyboard.press('Escape');
	await page.waitForTimeout(300);
	const returned = await page.evaluate(() => ({
		closed: window.__windowClosed === true,
		stopped: (window.__commands ?? []).includes('player_stop'),
		library: document.querySelector('.library-title')?.textContent?.trim() ?? null,
	}));
	if (returned.closed || !returned.stopped || !returned.library) {
		console.error(`the second Escape did not return to the library: ${JSON.stringify(returned)}`);
		failures++;
	}
	await page.keyboard.press('Escape');
	await page.waitForTimeout(150);
	if (await page.evaluate(() => window.__windowClosed === true)) {
		console.error('Escape from the library closed the desktop application');
		failures++;
	}

	// (d) fullscreen does not jump the queue: a menu the viewer opened is still the
	//     first thing Escape undoes, and the window stays fullscreen while it does.
	//     This is the order, not just the outcome - the first version of the rule
	//     checked fullscreen first, and this assertion is what caught it.
	await page.evaluate(() => {
		window.__fullscreen = true;
		window.__windowClosed = false;
	});
	await page.evaluate((status) => window.__handlers['player-status']?.({ payload: JSON.stringify(status) }), STATUS);
	await page.evaluate(() => window.__handlers['tauri://resize']?.({ payload: null }));
	await page.waitForTimeout(200);
	await page.locator('button[aria-haspopup=menu]').click();
	await page.waitForTimeout(300);
	await page.keyboard.press('Escape');
	await page.waitForTimeout(300);
	now = await state();
	if (now.menu) {
		console.error('Escape did not close the open track menu while fullscreen');
		failures++;
	}
	if (now.closed) {
		console.error('Escape closed the player while a track menu was open in fullscreen');
		failures++;
	}
	if (!now.fullscreen) {
		console.error('Escape left fullscreen while a track menu was open; the menu is the most recent thing opened');
		failures++;
	}

	// (e) and once the menu is gone, the same key leaves fullscreen.
	await page.keyboard.press('Escape');
	await page.evaluate(() => window.__handlers['tauri://resize']?.({ payload: null }));
	await page.waitForTimeout(300);
	now = await state();
	if (now.fullscreen || now.closed) {
		console.error(`after the menu, Escape did not simply leave fullscreen: ${JSON.stringify(now)}`);
		failures++;
	}
	await page.evaluate(() => {
		window.__fullscreen = false;
	});
}

// Nothing is loaded: the bar is not drawn, and the language is still reachable.
//
// Decision D1 (15 September 2026) chose to hide the control bar entirely until
// something is loaded, against the recommendation, because a disabled play
// button and an empty clock describe a film that does not exist. That choice
// takes away the only place the language chip lived - section 6b kept it in the
// bar "because the native player has nowhere else to switch it" - so the chip
// moves to the header for exactly the states the bar is gone. Both halves are
// asserted here, because the decision is only acceptable with the second one.
async function assertNoFilmNoBar(page) {
	const read = () =>
		page.evaluate(() => {
			const bar = document.querySelector('.controls');
			const barBox = bar?.getBoundingClientRect();
			const library = document.querySelector('.library')?.getBoundingClientRect();
			const chip = document.querySelector('.title-bar .control--language');
			const inBar = document.querySelector('.controls .control--desktop');
			return {
				barDrawn: !!barBox && barBox.height > 0 && getComputedStyle(bar).display !== 'none',
				barBoxes: [...document.querySelectorAll('.controls button, .controls [role=slider]')].filter(
					(el) => getComputedStyle(el).display !== 'none' && el.getBoundingClientRect().height > 0
				).length,
				libraryHeight: library ? Math.round(library.height) : null,
				headerChip: chip ? { visible: chip.getBoundingClientRect().height > 0, label: chip.textContent.trim() } : null,
				barChip: inBar ? getComputedStyle(inBar).display !== 'none' : null,
			};
		});

	// (a) nothing loaded.
	const idle = await read();
	if (idle.barDrawn || idle.barBoxes > 0) {
		console.error(`no film is loaded but the control bar is still drawn (${idle.barBoxes} live control(s))`);
		failures++;
	}
	if (!idle.headerChip?.visible) {
		console.error('no film is loaded, the bar is gone, and the header carries no language control - the language became unreachable');
		failures++;
	}

	// (b) a film starts: the bar comes back, and the header chip goes, so one
	//     control never appears twice.
	await page.evaluate((status) => window.__handlers['player-status']?.({ payload: JSON.stringify(status) }), STATUS);
	await page.waitForTimeout(350);
	const playing = await read();
	if (!playing.barDrawn || playing.barBoxes === 0) {
		console.error('a film is playing but the control bar is not drawn');
		failures++;
	}
	if (playing.headerChip?.visible) {
		console.error('the header language chip is still drawn while the bar is back, so the control appears twice');
		failures++;
	}
	if (!playing.barChip) {
		console.error('the control bar is back but its own language control is not drawn');
		failures++;
	}
	if (idle.libraryHeight !== null && playing.libraryHeight !== null && playing.libraryHeight >= idle.libraryHeight) {
		console.error(
			`the library did not take the room the bar gave up: ${idle.libraryHeight}px without a film, ${playing.libraryHeight}px with one`
		);
		failures++;
	}
}

// Finding servers: zero, one, and more than one.
//
// `findServers()` sets `busy = true` and then calls `connect()`, and `connect()`
// returns immediately when `busy` is true - so with exactly one server on the
// network, the case the code has a comment for ("One server and nothing else to
// choose between: connect to it, the same way one profile is not a question"),
// did nothing at all. Confirmed by reading the source during the phase-0
// campaign; asserted here so it cannot come back.
//
// What a mock cannot prove: mDNS itself. This machine's responder refuses an
// IPv6 multicast bind and answers nothing, so what is asserted is the OSD's
// behaviour given an answer - not that an answer arrives. A real server on a
// real network is the other half, and it is not claimed here.
async function assertDiscovery(page, { expect }) {
	await page.waitForTimeout(500);
	const commands = await page.evaluate(() => window.__commands ?? []);
	const discovered = await page.locator('.servers li button').count();
	const hue = await page.evaluate(() => ({
		title: document.querySelector('.library-title')?.textContent?.trim() ?? null,
		hint: document.querySelector('.hint')?.textContent?.trim() ?? null,
		busy: !!document.querySelector('button[type=submit][disabled]'),
	}));
	if (!commands.includes('player_discover')) {
		console.error(`startup issued ${JSON.stringify(commands)} - player_discover was never asked`);
		failures++;
	}
	if (expect === 'one' && !commands.includes('player_connect')) {
		console.error('one server answered and the OSD did not connect to it - connect() was refused because busy was still true');
		failures++;
	}
	if (expect === 'many' && discovered < 2) {
		console.error(`two servers answered and the panel lists ${discovered} of them`);
		failures++;
	}
	if (expect === 'none' && discovered !== 0) {
		console.error(`nothing answered and the panel lists ${discovered} server(s)`);
		failures++;
	}
	if (hue.busy) {
		console.error('the search button is still disabled after the search finished, so a second try is impossible');
		failures++;
	}
	if (expect === 'none' && !hue.hint) {
		console.error('nothing answered and no sentence says so');
		failures++;
	}
}

// The real status arrives about once a second, and the furniture must still hide.
//
// This is the case the simulation could not see until it was made to imitate the
// engine. The idle timer is re-armed whenever the session state changes, and the
// session state is a fresh object on every status frame - so a status arriving
// every second cleared the three-second timer before it could ever expire, and
// the furniture stayed on screen over the whole film. Chromium with one injected
// status frame could not reproduce it; the real player, watched for thirteen
// seconds, showed the bar still up.
//
// The frames are sent on the engine's own cadence and the furniture is read after
// more than the timeout has elapsed.
async function assertIdleSurvivesStatusFrames(page) {
	await page.evaluate((status) => window.__handlers['player-status']?.({ payload: JSON.stringify(status) }), STATUS);
	await page.mouse.move(640, 360);
	// Five frames, one second apart - the cadence `--diagnostics` reports.
	for (let i = 0; i < 5; i++) {
		await page.waitForTimeout(1000);
		await page.evaluate((status) => window.__handlers['player-status']?.({ payload: JSON.stringify(status) }), {
			...STATUS,
			pos: STATUS.pos + i + 1,
		});
	}
	// The last frame was 0 s ago; the timer needs its three seconds from there.
	await page.waitForTimeout(3500);
	const idle = await page.getAttribute('.osd', 'data-idle');
	if (idle !== 'true') {
		console.error(
			`a status frame every second kept the furniture up (data-idle=${idle}) - the idle timer is re-armed by every session frame instead of by a state change`
		);
		failures++;
	}
}

// The clock, which has to hold two numbers and survive a film longer than an hour.
//
// Section 6b: elapsed in `--bone` at 500, total in `--muted`, separated by a
// drawn hairline rather than a slash glyph. The three states a real library
// produces are checked together, because the one that breaks is the long film:
// a 2h20 film is where a clock that pads wrong shows "2:8:03" or drops the hour.
async function assertClock(page) {
	const read = () =>
		page.evaluate(() => {
			const box = document.querySelector('.clock');
			if (!box) return null;
			const rule = box.querySelector('.rule');
			return {
				elapsed: box.querySelector('.elapsed')?.textContent.trim() ?? null,
				total: box.querySelector('.total')?.textContent.trim() ?? null,
				elapsedWeight: box.querySelector('.elapsed') ? getComputedStyle(box.querySelector('.elapsed')).fontWeight : null,
				elapsedColour: box.querySelector('.elapsed') ? getComputedStyle(box.querySelector('.elapsed')).color : null,
				totalColour: box.querySelector('.total') ? getComputedStyle(box.querySelector('.total')).color : null,
				separatorDrawn: !!rule && rule.getBoundingClientRect().width > 0,
				separatorIsGlyph: rule ? rule.textContent.trim().length > 0 : null,
			};
		});

	const cases = [
		{ name: 'a film of 10 minutes', pos: 128.4, duration: 600, elapsed: '2:08', total: '10:00' },
		{ name: 'a film of 2h20', pos: 5138.4, duration: 8408, elapsed: '1:25:38', total: '2:20:08' },
		{ name: 'a film whose length is unknown', pos: 12, duration: 0, elapsed: '0:12', total: '--:--' },
	];

	for (const item of cases) {
		await page.evaluate(
			({ pos, duration }) =>
				window.__handlers['player-status']?.({ payload: JSON.stringify({ ...window.__status, pos, duration }) }),
			{ pos: item.pos, duration: item.duration }
		);
		await page.waitForTimeout(250);
		const clock = await read();
		if (!clock) {
			console.error(`${item.name}: the clock is not on screen`);
			failures++;
			continue;
		}
		if (clock.elapsed !== item.elapsed) {
			console.error(`${item.name}: elapsed reads "${clock.elapsed}", expected "${item.elapsed}"`);
			failures++;
		}
		if (clock.total !== item.total) {
			console.error(`${item.name}: total reads "${clock.total}", expected "${item.total}"`);
			failures++;
		}
		if (!clock.separatorDrawn) {
			console.error(`${item.name}: the separator between the two numbers is not drawn`);
			failures++;
		}
		if (clock.separatorIsGlyph) {
			console.error(`${item.name}: the separator is a typed glyph rather than a drawn rule`);
			failures++;
		}
		if (item.duration > 0 && clock.elapsedColour === clock.totalColour) {
			console.error(`${item.name}: both numbers are the same colour, so nothing says which is which`);
			failures++;
		}
	}

	// A long title must be truncated rather than push the controls around.
	const longTitle = 'Un titre de film remarquablement long qui doit etre tronque proprement sans rien pousser';
	await page.evaluate(
		(title) => window.__handlers['player-status']?.({ payload: JSON.stringify({ ...window.__status, title }) }),
		longTitle
	);
	await page.waitForTimeout(250);
	const title = await page.evaluate(() => {
		const el = document.querySelector('.film-title');
		if (!el) return null;
		const box = el.getBoundingClientRect();
		return {
			scrollWidth: el.scrollWidth,
			clientWidth: el.clientWidth,
			right: box.right,
			innerWidth: window.innerWidth,
			overflow: getComputedStyle(el).textOverflow,
			wraps: getComputedStyle(el).whiteSpace,
		};
	});
	if (!title) {
		console.error('the film title is not on screen, so a long one cannot be judged');
		failures++;
	} else {
		if (title.overflow !== 'ellipsis' || title.wraps !== 'nowrap') {
			console.error(`a long title is not truncated: text-overflow=${title.overflow} white-space=${title.wraps}`);
			failures++;
		}
		if (title.right > title.innerWidth + 0.5) {
			console.error(`a long title runs past the window: right=${title.right} in ${title.innerWidth}px`);
			failures++;
		}
	}
}

// The two languages, changed while the film plays and remembered afterwards.
//
// Decision 25 applied to a second interface: Rust sends codes, the catalogue owns
// every sentence. What is asserted here is that the choice is live - no reload -
// that `document.lang` follows it, which is what tells a screen reader and the
// browser's own hyphenation what they are reading, and that the choice survives a
// restart, because a player that forgets it every time is a player somebody has
// to correct every time.
//
// English is the default and French ships complete; `check:i18n` guards parity,
// and this guards that either catalogue is actually reachable.
async function assertLanguages(page, url) {
	const readingOf = () =>
		page.evaluate(() => ({
			htmlLang: document.documentElement.lang,
			title: document.querySelector('.library-title')?.textContent?.trim() ?? null,
			addressLabel: document.querySelector('label[for="theia-address"]')?.textContent?.trim() ?? null,
			connect: document.querySelector('button[type=submit]')?.textContent?.trim() ?? null,
			chip: document.querySelector('.control--language .label')?.textContent?.trim() ?? null,
			stored: (() => {
				try {
					return localStorage.getItem('theia.player.language');
				} catch {
					return 'unavailable';
				}
			})(),
		}));

	const en = await readingOf();
	if (en.htmlLang !== 'en') {
		console.error(`the OSD starts in "${en.htmlLang}" instead of English, which is the default`);
		failures++;
	}
	if (en.chip !== 'EN') {
		console.error(`the language chip reads "${en.chip}" in English, expected "EN"`);
		failures++;
	}

	// The switch itself, from the header control, with no reload.
	await page.click('.control--language');
	await page.waitForTimeout(300);
	const fr = await readingOf();
	if (fr.htmlLang !== 'fr') {
		console.error(`switching to French left document.lang at "${fr.htmlLang}"`);
		failures++;
	}
	if (fr.title === en.title) {
		console.error(`the visible copy did not change with the language: still "${fr.title}"`);
		failures++;
	}
	if (fr.chip !== 'FR') {
		console.error(`the language chip reads "${fr.chip}" after switching, expected "FR"`);
		failures++;
	}
	if (fr.stored !== 'fr') {
		console.error(`the chosen language was not stored (localStorage holds "${fr.stored}"), so it will not survive a restart`);
		failures++;
	}
	if (!fr.title || !fr.connect || !fr.addressLabel) {
		console.error('a sentence is missing in French: the catalogue is not complete on screen');
		failures++;
	}

	// And back, so the default is reachable in both directions.
	await page.click('.control--language');
	await page.waitForTimeout(300);
	const back = await readingOf();
	if (back.htmlLang !== 'en' || back.title !== en.title) {
		console.error(`switching back to English did not restore it: lang=${back.htmlLang} title="${back.title}"`);
		failures++;
	}

	// A restart is the page loading again with a choice already stored. It is a
	// reload and not a new browser context: a context has its own empty storage,
	// and the first version of this asked a brand-new context to remember
	// something it had never been told. The reload happens while the interface is
	// English and the store says French, so only reading the store at startup can
	// produce a French page.
	await page.evaluate(() => {
		localStorage.setItem('theia.player.language', 'fr');
	});
	await page.reload({ waitUntil: 'networkidle' });
	await page.waitForTimeout(400);
	const restarted = await page.evaluate(() => ({
		htmlLang: document.documentElement.lang,
		title: document.querySelector('.library-title')?.textContent?.trim() ?? null,
	}));
	if (restarted.htmlLang !== 'fr') {
		console.error(`the stored language was not honoured on a fresh load: document.lang is "${restarted.htmlLang}"`);
		failures++;
	}
	if (!restarted.title || restarted.title === en.title) {
		console.error(`a fresh load with French stored still draws English: "${restarted.title}"`);
		failures++;
	}
}

// A failure is a sentence in the viewer's language, never a code and never
// silence.
//
// Decision 25 on a second interface: Rust sends codes, the catalogue owns every
// word. The fallback in `t()` is `?? key`, which means a sentence missing from a
// catalogue shows the key itself - "connectionFailed" in the middle of a French
// screen. That is exactly the shape of fault decision 25 was written for, so the
// check asserts the visible text is a sentence and not an identifier, in both
// languages.
//
// The three failures a person can actually meet are covered: a server that
// answers nothing, a library that cannot be read, and an engine that will not
// start. The audio fallback is a notice rather than an error - nothing is broken
// - and is asserted as one.
async function assertFailuresAreReadable(page) {
	const readHint = () =>
		page.evaluate(() => {
			const el = document.querySelector('.hint');
			if (!el) return null;
			const box = el.getBoundingClientRect();
			const cs = getComputedStyle(el);
			return {
				text: el.textContent.trim(),
				isError: el.classList.contains('hint--error'),
				colour: cs.color,
				visible: box.height > 0 && cs.visibility !== 'hidden' && Number(cs.opacity) > 0.05,
			};
		});
	const readNotice = () =>
		page.evaluate(() => {
			const el = document.querySelector('.notice');
			if (!el) return null;
			const box = el.getBoundingClientRect();
			return { text: el.textContent.trim(), role: el.getAttribute('role'), visible: box.height > 0 };
		});
	const looksLikeAnIdentifier = (text) => /^[a-z]+[A-Z][A-Za-z]*$/.test(text) || text === '';

	// (a) a server that answers nothing.
	await page.evaluate(() => {
		window.__connectFails = true;
	});
	await page.fill('#theia-address', 'http://127.0.0.1:9');
	await page.click('button[type=submit]');
	await page.waitForTimeout(600);
	const failed = await readHint();
	if (!failed) {
		console.error('a failed connection left no message on screen at all');
		failures++;
	} else {
		if (!failed.isError) {
			console.error('a failed connection is shown without the error treatment');
			failures++;
		}
		if (looksLikeAnIdentifier(failed.text)) {
			console.error(`a failed connection shows the key itself instead of a sentence: "${failed.text}"`);
			failures++;
		}
		if (!failed.visible) {
			console.error('a failed connection message is not visible on screen');
			failures++;
		}
	}

	// (b) the same failure in English: parity is not enough, the sentence has to
	//     exist.
	await page.click('.control--language');
	await page.waitForTimeout(300);
	await page.click('button[type=submit]');
	await page.waitForTimeout(600);
	const failedEn = await readHint();
	if (!failedEn || looksLikeAnIdentifier(failedEn.text) || failedEn.text === failed?.text) {
		console.error(`the failure message did not change with the language: "${failedEn?.text}"`);
		failures++;
	}
	await page.click('.control--language');
	await page.waitForTimeout(300);

	// (c) the engine cannot start: a notice, because the player is still there.
	await page.evaluate(() =>
		window.__handlers['player-event']?.({ payload: JSON.stringify({ kind: 'engine', state: 'unavailable' }) })
	);
	await page.waitForTimeout(250);
	const engineNotice = await readNotice();
	if (!engineNotice) {
		console.error('an unavailable engine said nothing at all');
		failures++;
	} else {
		if (engineNotice.role !== 'status') {
			console.error(`the engine notice is not announced (role=${engineNotice.role})`);
			failures++;
		}
		if (looksLikeAnIdentifier(engineNotice.text)) {
			console.error(`the engine notice shows a key instead of a sentence: "${engineNotice.text}"`);
			failures++;
		}
	}
}

// The caption bar, measured rather than looked at.
//
// The maintainer supplied a reference caption bar on 20 September 2026 and asked
// for its hover, the placement of its cross and its edges. Every number below
// comes from that image, read with a pixel dump at a 3.9x zoom: cells at 44.5
// CSS px on a 173.5 px pitch, a plate filling the whole cell (169 x 131 real
// pixels) with square corners, glyphs near-white at rest, and a window rounded
// at ~5.9 px carrying a one-pixel light edge.
//
// What a picture cannot settle is whether the plate *is* the cell - a plate that
// covers 90% of it photographs the same at a glance - nor whether the cross is
// flush with the window's edge, since a 2px gap is invisible in a screenshot and
// obvious when a pointer crosses it. Both are numbers here.
async function assertCaptionChrome(page, state) {
	await page.hover('.window-control:nth-of-type(2)');
	await page.waitForTimeout(120);
	const measured = await page.evaluate(() => {
		const header = document.querySelector('.title-bar');
		const controls = [...document.querySelectorAll('.window-control')];
		const close = document.querySelector('.window-control--close');
		const shell = document.querySelector('.osd');
		const box = (el) => {
			const r = el.getBoundingClientRect();
			return { left: r.left, right: r.right, top: r.top, bottom: r.bottom, width: r.width, height: r.height };
		};
		const hovered = controls[1];
		return {
			header: box(header),
			// The bar's *content* height: its 1px bottom border is the bar's own
			// edge, and a plate that stopped one pixel above it is still a plate
			// that fills its cell. Comparing against the border box failed the
			// first run of this assertion by exactly that pixel.
			headerContent: header.clientHeight,
			headerTop: header.getBoundingClientRect().top + parseFloat(getComputedStyle(header).borderTopWidth),
			cells: controls.map(box),
			close: box(close),
			hoveredBackground: getComputedStyle(hovered).backgroundColor,
			barBackground: getComputedStyle(header).backgroundColor,
			restingColour: getComputedStyle(controls[0]).color,
			hoveredColour: getComputedStyle(hovered).color,
			radius: getComputedStyle(shell).borderTopRightRadius,
			ring: getComputedStyle(shell).boxShadow,
			innerWidth: window.innerWidth,
		};
	});
	const pitch = (measured.cells[1].left + measured.cells[1].right) / 2 - (measured.cells[0].left + measured.cells[0].right) / 2;
	const cell = measured.cells[1];
	if (measured.hoveredBackground === measured.barBackground) {
		console.error(`${state}: the hovered caption control paints ${measured.hoveredBackground}, the same as its bar - no plate`);
		failures++;
	}
	// The plate's *value*, not only its presence. Everything above passed while
	// the plate was still bone/8: index.css imported this stylesheet and then
	// re-declared `.window-control` below the import, so every rule written here
	// was silently outranked and the assertion could not tell the two apart.
	// Section 6b names 11%, and this is the number that has to match it.
	const plateAlpha = Number((measured.hoveredBackground.match(/[\d.]+\)$/) ?? ['0)'])[0].replace(')', ''));
	if (Math.abs(plateAlpha - 0.11) > 0.005) {
		console.error(`${state}: the hover plate paints ${measured.hoveredBackground}; section 6b pins bone at 11%, so the rule that wins is not the one in osd.css`);
		failures++;
	}
	if (Math.abs(cell.height - measured.headerContent) > 0.5) {
		console.error(`${state}: the caption plate is ${cell.height}px of a ${measured.headerContent}px bar, it must fill the cell`);
		failures++;
	}
	if (Math.abs(cell.top - measured.headerTop) > 0.5) {
		console.error(`${state}: the caption plate starts at ${cell.top} but the bar's content starts at ${measured.headerTop}, so there is a gap above it`);
		failures++;
	}
	if (Math.abs(cell.width - pitch) > 0.5) {
		console.error(`${state}: the caption cell is ${cell.width}px wide on a ${pitch}px pitch, so the plates do not abut`);
		failures++;
	}
	if (Math.abs(measured.innerWidth - measured.close.right) > 0.5) {
		console.error(`${state}: the close cell ends at ${measured.close.right} in a ${measured.innerWidth}px window, it must sit flush`);
		failures++;
	}
	if (measured.radius !== '8px') {
		console.error(`${state}: the window is rounded at ${measured.radius}; the reference is Windows 11 chrome, which rounds at 8`);
		failures++;
	}
	if (!measured.ring.includes('inset')) {
		console.error(`${state}: the window has no edge - box-shadow is "${measured.ring}"`);
		failures++;
	}
	if (measured.hoveredColour === measured.restingColour) {
		console.error(`${state}: the caption glyph does not answer the pointer (${measured.restingColour} either way)`);
		failures++;
	}

	// The language chip sat among three square plates as a 44px circle with the
	// shadcn ghost defaults, and the maintainer photographed it and asked why
	// one of the four was round. It is a caption cell now: same width, same
	// height, same plate, same radius, same colours - and this reads all of them,
	// once at rest and once under the pointer.
	const readChip = () =>
		page.evaluate(() => {
			const el = document.querySelector('.title-bar .control--language');
			const header = document.querySelector('.title-bar');
			const r = el.getBoundingClientRect();
			const style = getComputedStyle(el);
			return {
				width: r.width,
				height: r.height,
				top: r.top,
				radius: style.borderTopLeftRadius,
				background: style.backgroundColor,
				colour: getComputedStyle(el.querySelector('.label') ?? el).color,
				headerContent: header.clientHeight,
			};
		});
	const chipRest = await readChip();
	await page.hover('.control--language');
	await page.waitForTimeout(120);
	const chip = await readChip();
	if (chip.radius !== '0px') {
		console.error(`${state}: the language chip is rounded at ${chip.radius} - one round control among square ones`);
		failures++;
	}
	if (Math.abs(chip.width - cell.width) > 0.5 || Math.abs(chip.height - cell.height) > 0.5) {
		console.error(`${state}: the language chip is ${chip.width}x${chip.height} where a caption cell is ${cell.width}x${cell.height}`);
		failures++;
	}
	if (Math.abs(chip.top - cell.top) > 0.5 || Math.abs(chip.height - chip.headerContent) > 0.5) {
		console.error(`${state}: the language chip sits at ${chip.top} (a cell sits at ${cell.top}) or does not fill the ${chip.headerContent}px bar`);
		failures++;
	}
	if (chip.background !== measured.hoveredBackground) {
		console.error(`${state}: the language chip hovers ${chip.background} where a caption cell hovers ${measured.hoveredBackground}`);
		failures++;
	}
	// Its text is a `.label`, which brings the product's muted register with it.
	// The chip therefore kept a grey glyph on a lit plate - measured (135,128,118)
	// where the ✕ beside it read (214,207,194) - until the label was told to
	// inherit. Both states are pinned, because either one alone would pass on a
	// control that never changed colour at all.
	if (chipRest.colour !== measured.restingColour) {
		console.error(`${state}: the language chip's text is ${chipRest.colour} at rest where the ✕ beside it is ${measured.restingColour} - its .label rule is winning over the control's colour`);
		failures++;
	}
	if (chip.colour !== measured.hoveredColour) {
		console.error(`${state}: the language chip's text is ${chip.colour} under the pointer where a caption cell is ${measured.hoveredColour}`);
		failures++;
	}
	await page.screenshot({ path: join(OUT, '9-caption-hover.png') });
}

// What the OSD is drawn over.
//
// The page is transparent on purpose - the film is the background - and the real
// window composites it over mpv's own surface, which `force-window` keeps black
// from startup. A browser has no such surface: with nothing painted behind, the
// page sits on Chromium's white canvas and every card title is unreadable in a
// picture of a state that cannot happen. So the floor is painted explicitly -
// black when nothing is loaded, the film frame when one is playing.
async function addBackdrop(page, { frame = false } = {}) {
	await page.evaluate(
		({ src }) => {
			const behind = document.createElement('div');
			behind.id = 'theia-backdrop';
			behind.style.cssText = `position:fixed;inset:0;z-index:-1;background:#0b0a09 center/cover no-repeat${src ? ` url("${src}")` : ''};`;
			document.body.prepend(behind);
		},
		{ src: frame ? FRAME : null }
	);
}

/** A film starts: the black floor becomes the picture, without a reload. */
async function showFilm(page) {
	if (!FRAME) return;
	await page.evaluate((src) => {
		const behind = document.getElementById('theia-backdrop');
		if (behind) behind.style.backgroundImage = `url("${src}")`;
	}, FRAME);
}

async function openPage(
	viewport,
	{
		tracks = TRACKS,
		movies = MOVIES,
		series = SERIES,
		seriesDetail = SERIES_DETAIL,
		season = SEASON,
		home = HOME,
		seriesHome = SERIES_HOME,
		frame = false,
		discovered = [],
		// What the server says about a card preview here. The real states are
		// `ready`, `building` and an absence; the last two are the same thing to
		// the interface, which is why only the first is exercised separately.
		// What the bridge answers, which is the bytes themselves: the clip is
		// fetched by the Rust side and handed to the page as a data URL, because
		// WebView2 refused a `<video src>` pointing at the local server outright.
		preview = { state: 'ready', data_url: `data:video/mp4;base64,${PROBE_CLIP}` },
		// Pinned, not inherited: the host's own locale used to decide the
		// default language, so a French machine and an English one checked
		// different products. en-US here, fr-FR where the system-French rule
		// is the point.
		locale = 'en-US',
	} = {}
) {
	const page = await browser.newPage({
		viewport,
		locale,
		// The settings sheet can copy the server address, and the assertion reads
		// the clipboard back rather than trusting the sentence beside the button.
		// Chromium refuses both without the grant.
		permissions: ['clipboard-read', 'clipboard-write'],
	});

	// The artwork is served by a real server the harness does not have. Answering
	// the requests keeps the cards at the size they will really be: a dead image
	// fires onerror and every card would fall back to text, which is correct
	// behaviour and a useless picture.
	if (FRAME_BYTES) {
		await page.route('**/api/images/**', (route) =>
			route.fulfill({ contentType: 'image/png', body: FRAME_BYTES })
		);
	}
	// The clip is answered from memory here for the same reason the artwork is:
	// a real server is not running beside the harness, and a 404 would send the
	// card back to its still before anything could be measured.
	if (PROBE_CLIP) {
		await page.route('**/api/previews/**', (route) =>
			route.fulfill({ contentType: 'video/mp4', body: Buffer.from(PROBE_CLIP, 'base64') })
		);
	}

	await page.addInitScript(
		({ tracks, movies, series, seriesDetail, season, status, discovered, home, seriesHome, preview }) => {
			window.__handlers = {};
			window.__profiles = [
				{ id: 1, name: 'Alex', is_default: true, has_avatar: false, avatar_version: 0 },
				{ id: 2, name: 'Lina', is_default: false, has_avatar: false, avatar_version: 0 },
				{ id: 3, name: 'Invités', is_default: false, has_avatar: false, avatar_version: 0 },
			];
			// Every command the OSD asks Rust for is recorded, because "one press
			// is one command" and "typing is not a shortcut" are counts, not
			// opinions. A failure that says only "the click did something" would
			// send the next person hunting with a debugger.
			window.__commands = [];
			// The same list with its arguments, for the checks that care *which*
			// film a command named and not only which command ran. Kept separate
			// because the existing assertions compare command names element by
			// element, and a suffix would quietly break every one of them.
			window.__invocations = [];
			// The window's own state, so the Escape rule can be asserted rather than
			// described: fullscreen is a real flag, `close()` leaves a trace, and
			// every `setFullscreen` call is recorded.
			window.__fullscreen = false;
			window.__fullscreenCalls = [];
			window.__windowClosed = false;
			window.__TAURI__ = {
				core: {
					invoke: async (cmd, args) => {
						window.__commands.push(cmd);
						window.__invocations.push({ cmd, args });
						if (cmd === 'player_local_server') return null;
						// Nothing connected at boot in the harness: the whole
						// connection journey is a screen the checks drive.
						if (cmd === 'player_current_server') return null;
						if (cmd === 'player_tracks') return JSON.stringify(tracks);
						if (cmd === 'player_library') return JSON.stringify(movies);
						if (cmd === 'player_series') return JSON.stringify(series);
						if (cmd === 'player_home') return JSON.stringify(home);
						if (cmd === 'player_series_home') return JSON.stringify(seriesHome);
						if (cmd === 'player_preview') return JSON.stringify(preview);
						if (cmd === 'player_series_detail') return JSON.stringify(seriesDetail);
						if (cmd === 'player_season') return JSON.stringify(season);
						if (cmd === 'player_discover') return JSON.stringify(discovered);
						if (cmd === 'player_set_profile') return null;
						if (cmd === 'player_profile_rename') {
							const profile = window.__profiles.find((entry) => entry.id === Number(args?.id));
							profile.name = String(args?.name || '').trim();
							return JSON.stringify(profile);
						}
						if (cmd === 'player_profile_set_avatar') {
							const profile = window.__profiles.find((entry) => entry.id === Number(args?.id));
							profile.has_avatar = true;
							profile.avatar_version += 1;
							return JSON.stringify(profile);
						}
						if (cmd === 'player_profile_clear_avatar') {
							const profile = window.__profiles.find((entry) => entry.id === Number(args?.id));
							profile.has_avatar = false;
							profile.avatar_version += 1;
							return JSON.stringify(profile);
						}
						if (cmd === 'player_update_status' || cmd === 'player_update_check') return JSON.stringify({
							state: 'available', current_version: '3.3.3', latest_version: '3.3.4', available: true,
							message: 'Theia 3.3.4 est prête à être installée.',
						});
						if (cmd === 'player_update_apply') return JSON.stringify({
							state: 'ready', current_version: '3.3.3', latest_version: '3.3.4', available: false,
							message: 'Mise à jour téléchargée.',
						});
						if (cmd === 'player_connect') {
							// A failure has to be reachable on demand: the sentences a
							// person reads when something is wrong are as much a part of
							// the interface as the ones they read when it works.
							if (window.__connectFails) throw new Error('the mock was told to fail');
							return JSON.stringify({
								url: 'http://127.0.0.1:8395',
								health: { status: 'ok', version: 'dev', uptime_seconds: 12 },
								profile: 1,
								profiles: window.__profiles,
							});
						}
						return '{}';
					},
				},
				event: {
					listen: async (name, handler) => {
						window.__handlers[name] = handler;
						return () => {};
					},
				},
				window: {
					getCurrentWindow: () => ({
						minimize() {},
						toggleMaximize: async () => {
							window.__maximized = !window.__maximized;
						},
						isMaximized: async () => window.__maximized === true,
						close() {
							window.__windowClosed = true;
						},
						// A real state rather than a constant `false`, because the OSD
						// now decides what Escape does from it: a mock that always
						// answered false could not tell "Escape left fullscreen" from
						// "Escape closed the player", which is the fault this asserts.
						isFullscreen: async () => window.__fullscreen === true,
						setFullscreen: async (value) => {
							window.__fullscreen = value === true;
							window.__fullscreenCalls.push(value === true);
						},
					}),
				},
			};
			window.__status = status;
		},
		{ tracks, movies, series, seriesDetail, season, status: STATUS, discovered, home, seriesHome, preview }
	);
	await page.goto(URL, { waitUntil: 'networkidle' });
	await page.waitForTimeout(400);

	await addBackdrop(page, { frame });
	return page;
}

async function assertSeriesJourney(page) {
	await page.fill('#theia-address', 'http://127.0.0.1:8395');
	await page.click('button[type=submit]');
	await page.waitForTimeout(350);
	await page.getByRole('button', { name: 'Series', exact: true }).click();
	await page.waitForTimeout(650);
	if ((await page.locator('.film-name', { hasText: 'Shōgun' }).count()) !== 1) {
		console.error('the series tab did not draw the series catalogue');
		failures++;
		return;
	}
	await page.getByRole('button', { name: /Open series.*Shōgun/ }).click();
	await page.waitForTimeout(650);
	const episodes = await page.locator('.films > li').count();
	if (episodes !== 2) {
		console.error(`opening Shōgun drew ${episodes} episodes, expected 2`);
		failures++;
	}
	await assertFits(page, 'the native series and episode library');
	await page.screenshot({ path: join(OUT, '8-series-library.png') });

	// The active season tab must look chosen. It carries `.label` as well as
	// `.season-tab`, and `.label` sets the muted register - so this only held
	// because the state rule happened to sit below it in the stylesheet. The
	// selector is doubled now, and this reads the result rather than trusting it:
	// a tab that lost the tie is a tab nobody can tell they are on.
	const seasons = await page.evaluate(() =>
		[...document.querySelectorAll('.season-tab')].map((el) => ({
			text: el.textContent?.trim() ?? '',
			active: el.classList.contains('season-tab--active'),
			colour: getComputedStyle(el).color,
			background: getComputedStyle(el).backgroundColor,
		}))
	);
	const chosen = seasons.find((s) => s.active);
	const rest = seasons.find((s) => !s.active);
	if (!chosen) {
		console.error(`no season tab is marked active among ${seasons.length} drawn`);
		failures++;
	} else if (rest && chosen.colour === rest.colour && chosen.background === rest.background) {
		console.error(
			`the active season tab "${chosen.text}" paints ${chosen.colour} on ${chosen.background}, the same as "${rest.text}" - the label register won the tie`
		);
		failures++;
	}
	// Every episode names the series it belongs to. The maintainer's word: "chaque
	// épisode doit avoir sa série". On a season page the card's title is the
	// episode, so the series belongs in the legend beside its code.
	const legends = await page.locator('.film').evaluateAll((els) =>
		els.map((el) => el.querySelector('.film-legend')?.textContent ?? '')
	);
	if (!legends.length || legends.some((text) => !text.includes('Shōgun'))) {
		console.error(`an episode card does not name its series: ${JSON.stringify(legends)}`);
		failures++;
	}

	// A series is asked for a preview like everything else: it is not a file, so
	// the server samples the file playback would reach first. One algorithm, and
	// this is the assertion that the interface takes it.
	await page.locator('.film').first().focus();
	await page.waitForTimeout(500);
	const seriesClip = await page.locator('.film').first().evaluate((el) => {
		const video = el.querySelector('video.film-clip');
		return { present: Boolean(video), src: video?.getAttribute('src')?.slice(0, 22) ?? '' };
	});
	if (!seriesClip.present || seriesClip.src !== 'data:video/mp4;base64,') {
		console.error(`a series card did not ask for a preview clip: ${JSON.stringify(seriesClip)}`);
		failures++;
	}

	await page.evaluate(() => {
		window.__commands = [];
	});
	await page.getByRole('button', { name: /Play episode.*S01E01/ }).click();
	const commands = await page.evaluate(() => window.__commands ?? []);
	if (!commands.includes('player_play_episode')) {
		console.error(`pressing an episode issued ${commands.join(', ') || 'no command'}`);
		failures++;
	}
}

// 0. The connect screen at the widths a high-DPI window really has.
//
// This is the first screen a person sees, and the widths between the phone check
// (390) and the desktop one (1280) were never drawn. They are not academic: a
// screen scaled at 200% - which is what the maintainer's machine runs - turns a
// 1100-pixel window into 550 CSS pixels, so the first screen anybody sees there
// lands exactly in the gap. It fits today; this is what says so.
//
// The measurement that prompted this section was wrong, and the wrongness is
// worth keeping: a DPI-unaware capture tool asking for a 1100-pixel window got
// one twice that size, so the picture was clipped by the *screen* while the OSD
// was laid out correctly all along. The harness disagreed with the photograph,
// and the harness was right - which is why the picture is not the authority here.
{
	for (const width of [550, 700, 900]) {
		const page = await openPage({ width, height: 700 });
		await assertFits(page, `the connect screen at ${width}px`);

		// And the control row, which is the other thing this width decides: the
		// bar never wraps (design system 6b), so a row that does not fit has to
		// drop controls rather than reflow - and the rule that drops them was
		// written for 30rem, while a 200%-scaled window lands here.
		await page.evaluate(
			(status) => window.__handlers['player-status']?.({ payload: JSON.stringify(status) }),
			STATUS
		);
		await showFilm(page);
		await page.waitForTimeout(250);
		await assertFits(page, `the control row at ${width}px`);
		if (width === 550) await page.screenshot({ path: join(OUT, '0-controls-550.png') });
		await page.close();
	}
}

// 1. The library panel, connected, films listed as cards.
{
	const page = await openPage({ width: 1280, height: 720 });
	await page.fill('#theia-address', 'http://127.0.0.1:8395');
	await page.click('button[type=submit]');
	await page.waitForTimeout(600);
	// The submit button's screen position becomes a card after connection. Move
	// the pointer away before judging the library at rest or the preview delay.
	// The screen the player opens on is the home now: a hero for the film that
	// was left, with what is left of it, then the server's rows. Checked before
	// anything navigates away from it.
	await page.screenshot({ path: join(OUT, '1-home.png') });
	const heroTitle = await page.locator('.home-hero-title').innerText();
	if (heroTitle !== 'Resume Test') {
		console.error(`the home hero shows "${heroTitle}", expected the part-watched film`);
		failures++;
	}
	const eyebrow = await page.locator('.home-hero-eyebrow').textContent();
	if (eyebrow !== 'You were watching') {
		console.error(`the home hero's eyebrow reads "${eyebrow}", expected the resume sentence`);
		failures++;
	}
	if ((await page.locator('.home-hero-progress-played').count()) !== 1) {
		console.error('the home hero drew no progress bar for a film under way');
		failures++;
	}
	const homeRows = await page.locator('.home-row').count();
	if (homeRows !== HOME.rows.length + 2) {
		console.error(`the home drew ${homeRows} rows, expected ${HOME.rows.length + 2} (three film rows and two series rows)`);
		failures++;
	}
	// A series row names the series on its card, not the episode code: the only
	// thing that tells somebody which show they were continuing.
	if ((await page.locator('.home-row', { hasText: 'Continue a series' }).locator('.film-name', { hasText: 'Shogun' }).count()) !== 1) {
		console.error('the series continue row does not name the series on its card');
		failures++;
	}
	await assertFits(page, 'the home screen');

	// Then the film grid keeps its own page, where the whole instrument below
	// has always run.
	await page.getByRole('button', { name: 'Movies', exact: true }).click();
	await page.waitForTimeout(260);
	await page.mouse.move(1275, 25);
	await page.waitForTimeout(220);
	await page.screenshot({ path: join(OUT, '1-library.png') });

	const films = await page.locator('.film').count();
	if (films !== MOVIES.length) {
		console.error(`library panel drew ${films} films, expected ${MOVIES.length}`);
		failures++;
	}

	// Section 6.1's fallbacks, in order: the backdrop covering the frame, the
	// poster contained in it, then the not-found plate. Never a broken image,
	// never a bare letter.
	const covers = await page.locator('.film-art img:not(.film-art--poster):not([src*="media-not-found"])').count();
	const contained = await page.locator('.film-art img.film-art--poster').count();
	const notFound = await page.locator('.film-art img[src*="media-not-found"]').count();
	const noArtwork = MOVIES.filter((m) => !m.backdrop_url && !m.poster_url).length;
	if (covers !== MOVIES.length - noArtwork - 1) {
		console.error(`drew ${covers} backdrop cards, expected ${MOVIES.length - noArtwork - 1}`);
		failures++;
	}
	if (contained !== 1 || notFound !== noArtwork) {
		console.error(
			`fallbacks drawn are wrong: ${contained} contained poster(s) and ${notFound} not-found plate(s), expected 1 and ${noArtwork}`
		);
		failures++;
	}
	// The frame is 16/9 on every card, which is the one number in 6.1 that a
	// picture cannot be measured against by eye.
	const ratios = await page.locator('.film-art').evaluateAll((els) =>
		els.map((el) => Math.round((el.getBoundingClientRect().width / el.getBoundingClientRect().height) * 100) / 100)
	);
	for (const ratio of ratios) {
		if (Math.abs(ratio - 16 / 9) > 0.02) {
			console.error(`a card frame is ${ratio}:1, expected 1.78:1 (16/9)`);
			failures++;
			break;
		}
	}

	// The frame never shows a hole through it, and it settles into the page. Both
	// are pseudo-elements, so neither is in the DOM a querySelector can reach: the
	// computed style of the pseudo is the only honest way to read them.
	const layers = await page.locator('.film-art').first().evaluate((el) => {
		const before = getComputedStyle(el, '::before');
		const after = getComputedStyle(el, '::after');
		return {
			fill: before.backgroundImage,
			blur: before.filter,
			// The fill is oversized by a negative inset rather than by a
			// transform, because a transformed box put its own rounded clip in
			// the wrong place.
			overscan: before.top,
			fade: after.backgroundImage,
			blurBand: after.backdropFilter,
			mask: after.maskImage,
			bandRadius: after.borderRadius,
			bandClip: after.clipPath,
			fillClip: before.clipPath,
			mediaRadius: [...el.querySelectorAll('img, video')].map(
				(child) => getComputedStyle(child).borderRadius
			),
			media: getComputedStyle(el.querySelector('img')).zIndex,
		};
	});
	if (!layers.fill.startsWith('url(') || !layers.blur.includes('blur(') || !layers.overscan.startsWith('-')) {
		console.error(`the card has no blurred fill behind its artwork: ${JSON.stringify(layers)}`);
		failures++;
	}
	// The bandeau is a *blur*, which is the correction the maintainer made: not a
	// fade over the picture but the picture itself, blurred and darkened from the
	// left. A gradient background alone would have passed the first version of
	// this check and be the wrong effect.
	if (!layers.blurBand.includes('blur(') || !layers.mask.includes('gradient')) {
		console.error(`the card has no blur band: ${JSON.stringify({ band: layers.blurBand, mask: layers.mask })}`);
		failures++;
	}
	// The band must not paint outside the card's rounded corner, and this is
	// checked in pixels rather than in styles: the first version of this guard
	// read `clip-path` and passed while the maintainer was looking at a square
	// corner. The method needs no knowledge of the page's own colours - it
	// repaints the band magenta and compares two crops of the two places. The
	// corner crop must not change (the band is not there); the mid-left crop
	// must (the band is), which is also what proves the probe can see anything
	// at all.
	// Two guards, and the second is not redundant: Chromium already clips the
	// band through the card's own `overflow: hidden` and radius - measured, with
	// the clip-path removed the pixel probe below still passes - but the
	// maintainer photographed a square corner on a build of this bundle running
	// under WebView2, so the band also rounds its own corner and that is
	// asserted here. The pixel probe is the behaviour; this is the belt that
	// holds on the engine the product actually ships.
	// Every layer that can be composited must round itself, and the *mechanism*
	// is asserted because this harness cannot see the failure: Chromium clips a
	// playing video and a blurred band through the card's own radius, while
	// WebView2 drops `clip-path` on a layer carrying a `backdrop-filter` - and
	// only WebView2 showed the maintainer a square corner, three times. Measured
	// on the running window with the page painted green: `clip-path` on the band
	// left a dark square corner, `border-radius` on the same band did not. So the
	// band and the media take a radius, the blurred fill keeps its clip (its box
	// is oversized, so a radius would round the wrong rectangle), and the reading
	// names whichever layer is missing.
	const rounded = {
		band: layers.bandRadius,
		fill: layers.fillClip,
		media: layers.mediaRadius,
	};
	const zero = /^(0px|none)\s*$/;
	const missing = Object.entries(rounded).filter(([, value]) =>
		Array.isArray(value) ? value.some((v) => v === '' || zero.test(v)) : zero.test(String(value))
	);
	if (missing.length > 0) {
		console.error(`a layer would draw outside the card's corner: ${JSON.stringify(rounded)}`);
		failures++;
	}
	const probe = await assertBandRounded(page);
	if (probe.cornerChanged) {
		console.error(`the band paints outside the card's corner: ${JSON.stringify(probe)}`);
		failures++;
	}
	if (!probe.insideChanged) {
		console.error(`the corner probe found no band to compare against: ${JSON.stringify(probe)}`);
		failures++;
	}
	if (Number(layers.media) < 1) {
		console.error('the artwork is not painted above the blurred fill, so the fill would cover it');
		failures++;
	}

	// The progress rule is drawn for a part-watched film and for nothing else:
	// never at zero, never on a finished one.
	const rules = await page.locator('.film-watched').count();
	if (rules !== 1) {
		console.error(`drew ${rules} progress rules, expected 1 (one part-watched film)`);
		failures++;
	}
	const resumed = await page.locator('.film-legend', { hasText: 'min' }).count();
	if (resumed !== 1) {
		console.error(`offered to resume ${resumed} films, expected 1`);
		failures++;
	}

	// The card preview is the card.
	//
	// What stood here was a fixed overlay opened above the grid, one and a half
	// times the size of the card it covered. The maintainer sent two screenshots
	// - one before the pointer arrived and one under it - and asked for the
	// first one, with no change of size. So size is asserted first: the grid's
	// geometry before, during and after the pointer must be identical, and
	// whatever plays must play inside the frame it belongs to.
	const firstCard = page.locator('.film').first();
	const boxes = () =>
		page.locator('.film-art').evaluateAll((els) =>
			els.map((el) => {
				const r = el.getBoundingClientRect();
				return [Math.round(r.left * 10) / 10, Math.round(r.top * 10) / 10, Math.round(r.width * 10) / 10, Math.round(r.height * 10) / 10];
			})
		);
	const beforeHover = await boxes();
	await firstCard.hover();
	await page.waitForTimeout(500);
	const duringHover = await boxes();
	if (JSON.stringify(beforeHover) !== JSON.stringify(duringHover)) {
		console.error(`hovering a card changed the grid: ${JSON.stringify(beforeHover)} -> ${JSON.stringify(duringHover)}`);
		failures++;
	}

	const clip = await firstCard.evaluate((el) => {
		const video = el.querySelector('video.film-clip');
		const frame = el.querySelector('.film-art').getBoundingClientRect();
		if (!video) return null;
		const r = video.getBoundingClientRect();
		return {
			muted: video.muted,
			loop: video.loop,
			autoplay: video.autoplay,
			inline: video.playsInline,
			src: video.getAttribute('src') ?? '',
			poster: video.getAttribute('poster') ?? '',
			decoded: video.videoWidth,
			readyState: video.readyState,
			inside: r.left >= frame.left - 0.5 && r.top >= frame.top - 0.5 && r.right <= frame.right + 0.5 && r.bottom <= frame.bottom + 0.5,
		};
	});
	if (!clip) {
		console.error('hovering a film card drew no preview clip at all');
		failures++;
	} else {
		if (!clip.muted || !clip.loop || !clip.autoplay || !clip.inline) {
			console.error(`the clip is not a silent loop: ${JSON.stringify(clip)}`);
			failures++;
		}
		if (!clip.src.startsWith('data:video/mp4;base64,')) {
			console.error(`the clip is not the bytes the bridge handed over: ${clip.src.slice(0, 40)}`);
			failures++;
		}
		if (!clip.poster) {
			console.error('the clip carries no poster, so a slow first frame shows an empty frame');
			failures++;
		}
		if (!clip.inside) {
			console.error('the clip is drawn outside the frame it belongs to');
			failures++;
		}
		if (clip.decoded === 0) {
			console.error(`the clip never decoded a frame (readyState ${clip.readyState}) - it is a video element, not a preview`);
			failures++;
		}
	}

	// And no zoom, in the same state: the frame and the artwork inside it stay at
	// scale 1 while the pointer is on the card. The frame is allowed its 0.2rem
	// of travel, which is translation and not size.
	const transforms = await firstCard.evaluate((el) => {
		const scaleOf = (node) => {
			const value = getComputedStyle(node).transform;
			if (!value || value === 'none') return 1;
			const open = value.match(/matrix\(([^)]+)\)/);
			return open ? Number(open[1].split(',')[0]) : NaN;
		};
		return { frame: scaleOf(el.querySelector('.film-art')), image: scaleOf(el.querySelector('.film-art img')) };
	});
	if (transforms.frame !== 1 || transforms.image !== 1) {
		console.error(`the card zooms on hover: frame scale ${transforms.frame}, image scale ${transforms.image}, expected 1 and 1`);
		failures++;
	}
	await page.screenshot({ path: join(OUT, '1-card-preview.png') });

	await page.mouse.move(1275, 40);
	await page.waitForTimeout(300);
	const afterHover = await boxes();
	if (JSON.stringify(beforeHover) !== JSON.stringify(afterHover)) {
		console.error('the grid did not come back to its own size once the pointer left');
		failures++;
	}

	// A server still making the clip says `building`, and the card keeps the
	// still it already had rather than an empty frame or a spinner nobody asked
	// for. This is the state every first hover is in.
	const building = await openPage({ width: 1280, height: 720 }, { preview: { state: 'building' } });
	await building.fill('#theia-address', 'http://127.0.0.1:8395');
	await building.click('button[type=submit]');
	await building.waitForTimeout(600);
	await building.getByRole('button', { name: 'Movies', exact: true }).click();
	await building.waitForTimeout(250);
	await building.locator('.film').first().hover();
	await building.waitForTimeout(500);
	if ((await building.locator('video.film-clip').count()) !== 0) {
		console.error('a clip that is still being built was drawn as if it were ready');
		failures++;
	}
	if ((await building.locator('.film-art img').count()) < 1) {
		console.error('the still went missing while the clip was being built');
		failures++;
	}
	await building.close();

	// And the hero's synopsis, which the interface has always drawn and could
	// never receive: server.rs's Metadata struct carried three fields and
	// dropped `overview` before it reached the OSD (docs/v3.3.md). The harness
	// mocks the bridge, so this pins the interface's half; `--list` is what
	// proves the wire.
	const featured = await openPage(
		{ width: 1280, height: 720 },
		{
			// A hero that is not resuming, because that is the state the synopsis
			// is drawn in: the section shows either what is left or what the film
			// is, never both.
			home: {
				...HOME,
				hero_kind: 'featured',
				hero: {
					...HOME.hero,
					progress: { position_seconds: 0, duration_seconds: 0, finished: false },
					metadata: { ...(HOME.hero.metadata ?? {}), overview: 'A probe film about probes, long enough to be clamped by the frame it is drawn in.' },
				},
			},
		}
	);
	await featured.fill('#theia-address', 'http://127.0.0.1:8395');
	await featured.click('button[type=submit]');
	await featured.waitForTimeout(600);
	const overview = await featured.locator('.home-hero-overview').textContent();
	if (!overview || !overview.startsWith('A probe film about probes')) {
		console.error(`the home hero shows no synopsis: ${JSON.stringify(overview)}`);
		failures++;
	}
	await featured.close();

	// The island carries the whole information architecture, including the two
	// actions that were missing from the first native pass. The notification is
	// state, not decoration: it appears because the mocked server reports 3.3.1.
	for (const label of ['Home', 'Movies', 'Series', 'Search', 'Profiles']) {
		// Anchored: the wordmark's own accessible name is "THEIA – Home", and
		// a substring match would count two destinations where there is one.
		if ((await page.getByRole('button', { name: new RegExp(`^${label}$`) }).count()) !== 1) {
			console.error(`the desktop island is missing its ${label} destination`);
			failures++;
		}
	}
	if ((await page.locator('.nav-brand', { hasText: 'THEIA' }).count()) !== 1 || (await page.locator('.nav-update-badge', { hasText: '1' }).count()) !== 1) {
		console.error('the desktop island is missing the THEIA mark or the server-driven update badge');
		failures++;
	}

	await page.getByRole('button', { name: 'Search', exact: true }).click();
	const searchInput = page.getByPlaceholder(/Un film|A movie/);
	await searchInput.waitFor({ state: 'visible' });
	const searchLayers = await page.evaluate(() => ({
		library: Number.parseInt(getComputedStyle(document.querySelector('.library')).zIndex, 10),
		wallpaper: Number.parseInt(getComputedStyle(document.querySelector('.osd--library'), '::after').zIndex, 10),
	}));
	if (!Number.isFinite(searchLayers.library) || !Number.isFinite(searchLayers.wallpaper) || searchLayers.library <= searchLayers.wallpaper) {
		console.error(`the search wallpaper is painting over the readable interface: ${JSON.stringify(searchLayers)}`);
		failures++;
	}
	await page.screenshot({ path: join(OUT, '1-search.png') });
	await searchInput.fill('Resume');
	await page.waitForTimeout(180);
	if ((await page.locator('.film-name', { hasText: 'Resume Test' }).count()) !== 1 || (await page.locator('.film').count()) !== 1) {
		console.error('search did not reduce the mixed library to the matching title');
		failures++;
	}
	await page.getByRole('button', { name: 'Movies', exact: true }).click();
	await page.waitForTimeout(180);

	await page.getByRole('button', { name: 'Profiles', exact: true }).click();
	await page.waitForTimeout(260);
	if ((await page.locator('.profile-card-select').count()) === 0) {
		console.error(`profile chooser missing after navigation to ${await page.evaluate(() => location.pathname)}: ${(await page.locator('body').innerText()).slice(-500)}`);
		failures++;
		throw new Error('profile chooser did not open');
	}
	await page.locator('.profile-card-select', { hasText: 'Lina' }).click();
	await page.waitForTimeout(180);
	const profileCommands = await page.evaluate(() => window.__commands ?? []);
	if (!profileCommands.includes('player_set_profile')) {
		console.error('choosing a profile did not call the native profile switch command');
		failures++;
	}
	await page.getByRole('button', { name: 'Profiles', exact: true }).click();
	await page.getByRole('button', { name: /Personnaliser le profil · Alex|Customize profile · Alex/ }).click();
	await page.getByLabel(/Nom du profil|Profile name/).fill('Alexandra');
	await page.getByRole('button', { name: /Enregistrer le profil|Save profile/ }).click();
	await page.waitForTimeout(180);
	const profileEditCommands = await page.evaluate(() => window.__commands ?? []);
	if (!profileEditCommands.includes('player_profile_rename') || (await page.locator('.profile-card', { hasText: 'Alexandra' }).count()) !== 1) {
		console.error('profile personalization did not rename the real profile state');
		failures++;
	}
	await page.getByRole('button', { name: /Fermer les profils|Close profiles/ }).click();

	// Settings is real app state, not a decorative drawer: a rail of sections and,
	// behind each one, that section's real controls - the shape the maintainer
	// brought back from a 21st.dev reference on 21 September 2026.
	const openSettingsSection = async (name) => {
		await page.locator('.settings-nav-item', { hasText: name }).click();
		await page.waitForTimeout(180);
		// Nothing inside a panel may stick out of it. The update row used to draw
		// a horizontal scrollbar under itself at the sheet's old width, and a
		// scrollbar is exactly what a screenshot does not name.
		const overflow = await page.locator('.settings-panel').evaluate((node) => node.scrollWidth - node.clientWidth);
		if (overflow > 1) {
			console.error(`the settings panel overflows sideways by ${overflow}px on ${name}`);
			failures++;
		}
	};
	await page.getByRole('button', { name: /Réglages|Settings/ }).click();
	await page.waitForTimeout(250);
	if ((await page.getByRole('dialog').count()) !== 1 || (await page.locator('.settings-nav-item').count()) !== 4) {
		console.error('the player settings sheet or its four sections are missing');
		failures++;
	}
	// The two real preference switches live behind Playback.
	await openSettingsSection(/Lecture|Playback/);
	if ((await page.getByRole('switch').count()) !== 2) {
		console.error('the Playback panel does not hold its two real preference switches');
		failures++;
	}
	// The connected facts live behind Server, and the address can be taken out.
	await openSettingsSection(/Serveur|Server/);
	const settingsText = await page.getByRole('dialog').innerText();
	if (!settingsText.includes('127.0.0.1:8395') || !settingsText.includes('dev')) {
		console.error('the Server panel does not expose the connected server facts');
		failures++;
	}
	// The address can be taken out of the sheet. The row ellipsises its value, so
	// what the button copied is asserted against the clipboard itself - and the
	// sentence beside it, because a copy nobody can see has not happened as far
	// as the person clicking it is concerned.
	await page.getByRole('button', { name: /Copier l'adresse du serveur|Copy the server address/ }).click();
	await page.waitForTimeout(150);
	const clipboard = await page.evaluate(() => navigator.clipboard.readText());
	if (clipboard !== 'http://127.0.0.1:8395') {
		console.error(`the copy button put ${JSON.stringify(clipboard)} in the clipboard, expected the server address`);
		failures++;
	}
	const copyNote = await page.locator('.settings-copy-note').innerText().catch(() => '');
	if (!/Adresse copiée|Address copied/.test(copyNote)) {
		console.error(`the copy button said ${JSON.stringify(copyNote)} after copying`);
		failures++;
	}
	// And the update state lives behind Update.
	await openSettingsSection(/Mise à jour|Update/);
	const updateText = await page.getByRole('dialog').innerText();
	if (!updateText.includes('3.3.3') || !updateText.includes('3.3.4') || (await page.getByRole('button', { name: /Installer la mise à jour|Install update/ }).count()) !== 1) {
		console.error('the Update panel does not expose the real current/latest state and install action');
		failures++;
	}
	// Back to the panel whose switch this journey flips, and photograph the sheet.
	await openSettingsSection(/Lecture|Playback/);
	await page.screenshot({ path: join(OUT, '1-settings.png') });
	await page.getByRole('switch', { name: /Réduire les animations|Reduce motion/ }).click();
	await page.getByRole('button', { name: /Enregistrer|Save/ }).click();
	await page.waitForTimeout(180);
	const storedPreferences = await page.evaluate(() => JSON.parse(localStorage.getItem('theia.player.preferences') ?? '{}'));
	if ((await page.getByRole('dialog').count()) !== 0 || storedPreferences.reducedMotion !== true) {
		console.error(`saving player settings did not persist and close: ${JSON.stringify(storedPreferences)}`);
		failures++;
	}
	await page.close();

	// (a2) The desktop app carries the whole Theia library, not only films. This
	// walks the visible product path: series tab, one show, its season, an episode
	// and the Rust command that starts it.
	const seriesPage = await openPage({ width: 1280, height: 720 });
	await assertSeriesJourney(seriesPage);
	await seriesPage.close();
}

// 2. A film playing, then the track menu.
{
	const page = await openPage({ width: 1280, height: 720 }, { frame: true });
	await page.evaluate((status) => window.__handlers['player-status']?.({ payload: JSON.stringify(status) }), STATUS);
	await page.waitForTimeout(300);
	await page.screenshot({ path: join(OUT, '2-player.png') });

	await page.click('.menu-anchor button');
	await page.waitForTimeout(400);
	await page.screenshot({ path: join(OUT, '3-tracks.png') });

	const rows = await page.locator('.track-menu .track').count();
	const ticks = await page.locator('.track-menu .tick svg').count();
	// Three audio choices plus subtitles off plus two subtitle tracks.
	if (rows !== 5) {
		console.error(`track menu drew ${rows} rows, expected 5`);
		failures++;
	}
	if (ticks !== 2) {
		console.error(`track menu drew ${ticks} ticks, expected 2 (one audio, one subtitle)`);
		failures++;
	}
	// The external track is named by its language, never by its address: given
	// neither a title nor a language, mpv derives one from the URL, and the menu
	// read "6?profile=1". Asserted rather than eyeballed.
	const menuText = await page.locator('.track-menu').innerText();
	if (menuText.includes('?profile=') || menuText.includes('http')) {
		console.error(`a track is named after its URL: ${JSON.stringify(menuText.slice(0, 120))}`);
		failures++;
	}
	if (!menuText.includes('FICHIER EXTERNE') && !menuText.includes('EXTERNAL FILE')) {
		console.error('the added subtitle does not say it came from beside the film');
		failures++;
	}
	await page.close();
}

// 3. A phone-width window, where the popover pins to the frame instead of
//    hanging off the left edge, and where the control row has to fit rather
//    than wrap. Every state a phone can be in is measured: the row did fit with
//    the menu open and overflowed by 29px once the menu was closed and the
//    spacer had nothing to give, so one state is not a check.
{
	const page = await openPage({ width: 390, height: 780 });

	// (a) nothing loaded yet: the address field and the library panel, which is
	//     the state the player opens in.
	await assertFits(page, 'the phone library panel');
	await page.fill('#theia-address', 'http://127.0.0.1:8395');
	await page.click('button[type=submit]');
	await page.waitForTimeout(300);
	await page.screenshot({ path: join(OUT, '5-phone-home.png') });
	// The phone window lands on the home too, and this is where the bar's own
	// arithmetic is checked: five destinations, the avatar and 44px floors do
	// not leave room for the wordmark, so the mark is what gives - inside the
	// pill, not past its edge. `scrollWidth` catches what the clipped overflow
	// would hide: a pill quietly wider than the box it is drawn in.
	if ((await page.locator('.home-hero').count()) !== 1) {
		console.error('the phone home is missing its hero');
		failures++;
	}
	if ((await page.locator('.library-nav').evaluate((el) => el.scrollWidth)) > (await page.locator('.library-nav').evaluate((el) => el.clientWidth)) + 2) {
		console.error('the phone navigation overflows its own pill');
		failures++;
	}
	if (await page.locator('.nav-brand').isVisible()) {
		console.error('the phone navigation kept a wordmark it does not have the width for');
		failures++;
	}
	await assertFits(page, 'the phone home, connected');

	// (b) playing, furniture up. The film is behind the OSD from here on.
	await page.evaluate((status) => window.__handlers['player-status']?.({ payload: JSON.stringify(status) }), STATUS);
	await showFilm(page);
	await page.waitForTimeout(300);
	await page.screenshot({ path: join(OUT, '4-phone.png') });
	await assertFits(page, 'the phone control row');

	// (c) the track popover open, which is the tallest thing a phone draws.
	await page.click('.menu-anchor button');
	await page.waitForTimeout(400);
	await page.screenshot({ path: join(OUT, '6-phone-tracks.png') });
	await assertFits(page, 'the phone track popover');

	const controls = await page.locator('.row button.control:visible').count();
	// Play, tracks and film fullscreen stay in the playback row. The desktop
	// close/maximise/minimise controls belong to the title bar, where a normal
	// window puts them, rather than being duplicated in the film controls.
	if (controls !== 3) {
		console.error(`the phone control row drew ${controls} controls, expected 3`);
		failures++;
	}
	await page.close();
}

// 3b. The window's own minimum, which is the one width this campaign measured an
//     overflow at and the one nobody had asserted.
//
// At 320x180 the row asked for 314px of a 272px content box, with the five
// controls already at their 3.25rem floor and both clock numbers drawn. Section
// 6b forbids wrapping the row and dropping a target, so the decision was that the
// clock gives up its second number below 30rem: what a viewer watches by is the
// elapsed time, and the total is what they read before pressing play - still
// there at every width above this one.
//
// The assertion is written as three statements rather than one, because "it fits"
// would also pass if the whole bar had been hidden: the bar is present, its five
// controls are present, and only then does the clock read one number.
{
	const page = await openPage({ width: 320, height: 180 }, { frame: true });
	await page.evaluate((status) => window.__handlers['player-status']?.({ payload: JSON.stringify(status) }), STATUS);
	await showFilm(page);
	await page.waitForTimeout(300);
	await page.screenshot({ path: join(OUT, '7-minimum-window.png') });

	// (a) nothing sticks out. This is the assertion the earlier campaign could not
	//     make at this size.
	await assertFits(page, 'the minimum window, film playing');

	// (b) the bar is really there - the play button is the one without which
	//     nothing else matters.
	const present = await page.evaluate(() => {
		// `:visible` is a Playwright selector extension and does not exist inside
		// page.evaluate, where this runs: the check is a measured box instead.
		const visible = [...document.querySelectorAll('.row button.control')].filter((b) => {
			const r = b.getBoundingClientRect();
			return r.width > 0 && r.height > 0;
		}).length;
		return {
			bar: !!document.querySelector('.controls'),
			play: !!document.querySelector('button.control--primary'),
			controls: visible,
		};
	});
	if (!present.bar) {
		console.error('the minimum window: the control bar is not drawn at all');
		failures++;
	}
	if (!present.play) {
		console.error('the minimum window: no play button, so the film cannot be started');
		failures++;
	}
	if (present.controls !== 3) {
		console.error(`the minimum window drew ${present.controls} visible controls, expected 3 (play, tracks, fullscreen)`);
		failures++;
	}

	// (c) and the clock reads one number, with no orphaned hairline beside it.
	const clock = await page.evaluate(() => {
		const box = document.querySelector('.clock');
		if (!box) return null;
		const rule = box.querySelector('.rule');
		const total = box.querySelector('.total');
		return {
			elapsed: box.querySelector('.elapsed')?.textContent.trim() ?? null,
			totalDrawn: !!total && total.getBoundingClientRect().width > 0,
			ruleDrawn: !!rule && rule.getBoundingClientRect().width > 0,
		};
	});
	if (!clock) {
		console.error('the minimum window: the clock is not on screen');
		failures++;
	} else {
		if (clock.totalDrawn) {
			console.error('the minimum window: the clock still draws its total, which is what overflowed');
			failures++;
		}
		if (clock.ruleDrawn) {
			console.error('the minimum window: the hairline is drawn with nothing on one side of it');
			failures++;
		}
		if (!clock.elapsed) {
			console.error('the minimum window: the clock lost its elapsed time as well');
			failures++;
		}
	}
	await page.close();
}

// 4. What the first four blocks could not see: the faces the OSD actually wears,
//    the size of every target a finger can land on, and whether the keyboard
//    still belongs to the person typing.
//
// These were added on 15 September 2026, after a phase-0 campaign measured the
// shipped build and found the OSD declaring no `@font-face` at all (titles in
// Georgia, labels in Segoe UI), its timeline 24px tall against section 9's
// 44x44 floor, and the window-level shortcuts swallowing `k` and Space while
// somebody typed a server address and switching the interface to English on `l`.
// Every one of them is a defect the previous four blocks passed straight
// through, which is the whole argument for this block existing.
{
	// (a) the faces - measured in the library state, the one a person sees first
	//     and the one whose title carries the display face.
	const page = await openPage({ width: 1280, height: 720 });
	await assertFontsLoaded(page, 'the connect screen');
	await page.fill('#theia-address', 'http://127.0.0.1:8395');
	await page.click('button[type=submit]');
	await page.waitForTimeout(600);
	await assertFontsLoaded(page, 'the library panel');

	// The targets are measured with a film playing, not on the library panel:
	// since decision D1 the control bar is not drawn without one, so a check that
	// only looked at the library would stop seeing the timeline altogether and
	// the 44px rule would look satisfied by an element that is simply absent.
	await page.evaluate((status) => window.__handlers['player-status']?.({ payload: JSON.stringify(status) }), STATUS);
	await page.waitForTimeout(350);
	await assertHitTargets(page, 'the control bar with a film playing');
	await page.close();

	// (b) typing an address is typing, not a keyboard shortcut. The address is
	//     deliberately made of the letters the OSD has claimed: k, l, f, m, c.
	const typing = await openPage({ width: 1280, height: 720 });
	await assertTypingIsNotShortcuts(typing, 'http://127.0.0.1:8395/klfmc');
	await typing.close();

	// (c) the three-second hide, on both sides of the boundary, and its pause
	//     exception.
	const idle = await openPage({ width: 1280, height: 720 }, { frame: true });
	await showFilm(idle);
	await assertIdleTiming(idle);
	await idle.close();

	// (d) one press, one command.
	const presses = await openPage({ width: 1280, height: 720 }, { frame: true });
	await presses.evaluate((status) => window.__handlers['player-status']?.({ payload: JSON.stringify(status) }), STATUS);
	await presses.waitForTimeout(300);
	await assertOnePressOneCommand(presses);
	await presses.close();

	// (e) the pointer, which is visible unless the furniture has taken it.
	const cursor = await openPage({ width: 1280, height: 720 }, { frame: true });
	await showFilm(cursor);
	await assertCursorFollowsFurniture(cursor);
	await cursor.close();

	// (f) the states in which the furniture must not hide at all.
	const busy = await openPage({ width: 1280, height: 720 }, { frame: true });
	await showFilm(busy);
	await assertFurnitureNeverHidesWhenBusy(busy);
	await busy.close();

	// (g) the menu's own keys, and where focus goes when it closes.
	const menu = await openPage({ width: 1280, height: 720 }, { frame: true });
	await menu.evaluate((status) => window.__handlers['player-status']?.({ payload: JSON.stringify(status) }), STATUS);
	await menu.waitForTimeout(300);
	await assertMenuOwnsItsKeys(menu);
	await menu.close();

	// (h) no film, no bar - and the language still reachable without it.
	const noFilm = await openPage({ width: 1280, height: 720 });
	await assertNoFilmNoBar(noFilm);
	await noFilm.close();

	// (i) finding a server: the answer decides what the panel offers.
	const one = await openPage({ width: 1280, height: 720 }, { discovered: [{ name: 'theia', url: 'http://127.0.0.1:8395', version: 'dev' }] });
	await assertDiscovery(one, { expect: 'one' });
	await one.close();

	const many = await openPage(
		{ width: 1280, height: 720 },
		{
			discovered: [
				{ name: 'salon', url: 'http://192.168.1.20:8395', version: 'dev' },
				{ name: 'bureau', url: 'http://192.168.1.21:8395', version: 'dev' },
			],
		}
	);
	await assertDiscovery(many, { expect: 'many' });
	await many.close();

	const none = await openPage({ width: 1280, height: 720 });
	await assertDiscovery(none, { expect: 'none' });
	await none.close();

	// (j) the cadence the real engine sends, which is what the earlier idle
	//     assertions could not imitate with a single injected frame.
	const cadence = await openPage({ width: 1280, height: 720 }, { frame: true });
	await showFilm(cadence);
	await assertIdleSurvivesStatusFrames(cadence);
	await cadence.close();

	// (k) the clock, on a short film, a two-hour film and an unknown one, plus a
	//     title long enough to need truncating.
	const clock = await openPage({ width: 1280, height: 720 }, { frame: true });
	await clock.evaluate((status) => window.__handlers['player-status']?.({ payload: JSON.stringify(status) }), STATUS);
	await clock.waitForTimeout(300);
	await assertClock(clock);
	await clock.close();

	// (l) the two languages, live and remembered.
	const languages = await openPage({ width: 1280, height: 720 });
	await assertLanguages(languages, URL);
	await languages.close();

	// (l2) the default language is English - on every machine, French ones
	// included. The stored choice still wins (proved above); what this pins
	// is that nothing else does. The system locale used to decide, and the
	// maintainer instructed otherwise (decision 131).
	const frenchSystem = await openPage({ width: 1280, height: 720 }, { locale: 'fr-FR' });
	const frenchStart = await frenchSystem.evaluate(() => ({
		htmlLang: document.documentElement.lang,
		chip: document.querySelector('.control--language .label')?.textContent?.trim() ?? null,
		stored: (() => {
			try {
				return localStorage.getItem('theia.player.language');
			} catch {
				return 'unavailable';
			}
		})(),
	}));
	if (frenchStart.htmlLang !== 'en' || frenchStart.chip !== 'EN') {
		console.error(
			`a French system started in "${frenchStart.htmlLang}" (${frenchStart.chip}), expected English - the default is English on every machine`
		);
		failures++;
	}
	if (frenchStart.stored !== null) {
		console.error(`nothing was chosen yet, so the store should be empty, found "${frenchStart.stored}"`);
		failures++;
	}
	await frenchSystem.close();

	// (m) what a person reads when something goes wrong, in both languages.
	const failuresPage = await openPage({ width: 1280, height: 720 });
	await assertFailuresAreReadable(failuresPage);
	await failuresPage.close();

	// (n) Escape in fullscreen, which is what D2 asked to have proposed and
	//     validated rather than assumed. A film is playing because the menu half of
	//     the assertion needs something to open a menu about.
	const escaping = await openPage({ width: 1280, height: 720 }, { frame: true });
	await escaping.evaluate((status) => window.__handlers['player-status']?.({ payload: JSON.stringify(status) }), STATUS);
	await showFilm(escaping);
	await escaping.waitForTimeout(300);
	await assertEscapeLeavesFullscreenFirst(escaping);
	await escaping.close();

	// (o) the caption bar: its hover plate, the placement of the cross, and the
	//     window's edges - the three things the maintainer asked for against a
	//     reference bar on 20 September 2026.
	const caption = await openPage({ width: 1280, height: 720 });
	await assertCaptionChrome(caption, 'caption bar at 1280');
	await caption.close();

	// (p) A film's own play path. The series journey asserts that pressing an
	//     episode issues player_play_episode; the film half had no guard at all,
	//     which is worth stating plainly: the maintainer pressed Play movie, and
	//     nothing anywhere in this harness would have noticed either way. Both
	//     the card and the button inside the preview lead to the same call, and
	//     the id matters - a command that starts the wrong film is not "playing".
	const filmPlay = await openPage({ width: 1280, height: 720 });
	// Connected, then on the film grid, so "the first card" is MOVIES[0] and the
	// id in the assertion is a fact rather than a guess about which row drew
	// first.
	await filmPlay.fill('#theia-address', 'http://127.0.0.1:8395');
	await filmPlay.click('button[type=submit]');
	await filmPlay.waitForTimeout(600);
	await filmPlay.getByRole('button', { name: 'Movies', exact: true }).click();
	await filmPlay.waitForTimeout(300);
	await filmPlay.evaluate(() => {
		window.__commands = [];
		window.__invocations = [];
	});
	await filmPlay.locator('.film').first().click();
	await filmPlay.waitForTimeout(400);
	const playCalls = await filmPlay.evaluate(() => (window.__invocations ?? []).filter((call) => call.cmd === 'player_play'));
	if (playCalls.length !== 1 || playCalls[0].args?.id !== 1) {
		console.error(`pressing the first card issued ${JSON.stringify(playCalls)}, expected exactly one player_play for film 1`);
		failures++;
	}
	await filmPlay.close();
}

// The player's own window, at the scale a real display gives it.
//
// Every other viewport here is a browser's. The player is a Tauri window: on
// this machine 1453x913 physical at 150%, which is a 969x609 CSS viewport, and
// at that width the library's navigation ran off the right edge and took the
// menu - and the profile picture in it - out of the screen. The maintainer found
// it by importing a photo and never seeing it. So the window the product is used
// in is a viewport this harness checks, and `assertFits` names what sticks out.
{
	const page = await openPage({ width: 969, height: 609 });
	await page.fill('#theia-address', 'http://127.0.0.1:8395');
	await page.click('button[type=submit]');
	await page.waitForTimeout(600);
	await assertFits(page, 'the player window (969x609)');
	await page.screenshot({ path: join(OUT, '10-player-window.png') });
	await page.close();
}

await browser.close();
console.log(failures === 0 ? `render check passed, pictures in ${OUT}` : `${failures} render check(s) failed`);
process.exit(failures === 0 ? 0 : 1);
