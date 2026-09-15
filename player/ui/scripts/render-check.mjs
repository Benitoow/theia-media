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
		metadata: { backdrop_path: '/probe-backdrop.jpg' },
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

const browser = await chromium.launch();
let failures = 0;

// A failure that names only a scrollWidth sends the next person hunting through
// the DOM with a ruler, so the report names the state and what sticks out.
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

async function openPage(viewport, { tracks = TRACKS, movies = MOVIES, frame = false } = {}) {
	const page = await browser.newPage({ viewport });

	// The artwork is served by a real server the harness does not have. Answering
	// the requests keeps the cards at the size they will really be: a dead image
	// fires onerror and every card would fall back to text, which is correct
	// behaviour and a useless picture.
	if (FRAME_BYTES) {
		await page.route('**/api/images/**', (route) =>
			route.fulfill({ contentType: 'image/png', body: FRAME_BYTES })
		);
	}

	await page.addInitScript(
		({ tracks, movies, status }) => {
			window.__handlers = {};
			window.__TAURI__ = {
				core: {
					invoke: async (cmd) => {
						if (cmd === 'player_tracks') return JSON.stringify(tracks);
						if (cmd === 'player_library') return JSON.stringify(movies);
						if (cmd === 'player_discover') return JSON.stringify([]);
						if (cmd === 'player_connect')
							return JSON.stringify({
								url: 'http://127.0.0.1:8395',
								health: { status: 'ok', version: 'dev', uptime_seconds: 12 },
								profile: 1,
								profiles: [{ id: 1, name: '', is_default: true }],
							});
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
						close() {},
						isFullscreen: async () => false,
						setFullscreen: async () => {},
					}),
				},
			};
			window.__status = status;
		},
		{ tracks, movies, status: STATUS }
	);
	await page.goto(URL, { waitUntil: 'networkidle' });
	await page.waitForTimeout(400);

	await addBackdrop(page, { frame });
	return page;
}

// 1. The library panel, connected, films listed as cards.
{
	const page = await openPage({ width: 1280, height: 720 });
	await page.fill('#theia-address', 'http://127.0.0.1:8395');
	await page.click('button[type=submit]');
	await page.waitForTimeout(600);
	await page.screenshot({ path: join(OUT, '1-library.png') });

	const films = await page.locator('.film').count();
	if (films !== MOVIES.length) {
		console.error(`library panel drew ${films} films, expected ${MOVIES.length}`);
		failures++;
	}

	// Section 6.1's fallbacks, in order: the backdrop covering the frame, the
	// poster contained in it, then the title as text. Never a broken image.
	const covers = await page.locator('.film-art img:not(.film-art--poster)').count();
	const contained = await page.locator('.film-art img.film-art--poster').count();
	const asText = await page.locator('.film-art-title').count();
	const noArtwork = MOVIES.filter((m) => !m.backdrop_url && !m.poster_url).length;
	if (covers !== MOVIES.length - noArtwork - 1) {
		console.error(`drew ${covers} backdrop cards, expected ${MOVIES.length - noArtwork - 1}`);
		failures++;
	}
	if (contained !== 1 || asText !== noArtwork) {
		console.error(
			`fallbacks drawn are wrong: ${contained} contained poster(s) and ${asText} text card(s), expected 1 and ${noArtwork}`
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
	await page.close();
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
	await page.screenshot({ path: join(OUT, '5-phone-library.png') });
	await assertFits(page, 'the phone library panel, connected');

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
	// Play, tracks, fullscreen, close: the volume, the language chip, the codec
	// badge and the ten-second pair all go below 30rem (design system 6b).
	if (controls !== 4) {
		console.error(`the phone control row drew ${controls} controls, expected 4`);
		failures++;
	}
	await page.close();
}

await browser.close();
console.log(failures === 0 ? `render check passed, pictures in ${OUT}` : `${failures} render check(s) failed`);
process.exit(failures === 0 ? 0 : 1);
