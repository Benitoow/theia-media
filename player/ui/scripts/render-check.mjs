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
	{ family: 'Cinzel Variable', used: '.library-title' },
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
	if (lang !== 'fr') {
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

// The pointer is visible unless the furniture has deliberately gone.
//
// Design system 6b: the furniture hides after three seconds and "takes the
// cursor with it". Everywhere else - the connect screen, the library panel while
// nothing plays, and any moment a person is actually pointing at something - the
// pointer has to be there. The shipped build set `cursor: none` on `html, body`
// unconditionally, so a viewer who had not started a film yet had no pointer at
// all, and the library panel was unusable with a mouse.
//
// A simulation can only answer the CSS half: it reads the computed cursor of the
// element under the pointer. Whether WebView2 honours it over a real film is the
// maintainer's look, and it is recorded as unverified until somebody takes it.
async function assertCursorFollowsFurniture(page) {
	const settled = async () => {
		await page.waitForTimeout(250);
		return page.evaluate(() => {
			const idle = document.querySelector('.osd')?.getAttribute('data-idle');
			const resolve = (sel) => {
				const el = document.querySelector(sel);
				return el ? getComputedStyle(el).cursor : null;
			};
			return {
				idle,
				html: resolve('html'),
				body: resolve('body'),
				osd: resolve('.osd'),
				primary: resolve('button.control--primary'),
				scrub: resolve('.scrub'),
				field: resolve('#theia-address'),
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

	// (c) a film playing and left alone: the furniture goes, and takes the
	//     pointer with it.
	await page.evaluate((status) => window.__handlers['player-status']?.({ payload: JSON.stringify(status) }), STATUS);
	await page.mouse.move(640, 360);
	await page.waitForTimeout(3800);
	const hidden = await settled();
	if (hidden.idle !== 'true') {
		console.error('the furniture did not hide, so the pointer rule cannot be judged');
		failures++;
	} else if (hidden.html !== 'none' && hidden.osd !== 'none') {
		console.error(`the furniture hid but the pointer stayed (html=${hidden.html} osd=${hidden.osd})`);
		failures++;
	}

	// (d) and a sign of life brings both back.
	await page.mouse.move(500, 300);
	const awake = await settled();
	if (awake.idle === 'true') {
		console.error('the furniture did not come back on a pointer move');
		failures++;
	} else if (awake.html === 'none' && awake.osd === 'none') {
		console.error('the furniture came back but the pointer is still hidden');
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

	// The CSS half is all a browser can see, and on the real window it was not
	// enough: WebView2 drew its own pointer over the video surface and ignored
	// `cursor: none` - measured on 16 September 2026, with the furniture
	// demonstrably gone and Windows still reporting the pointer drawn in 15
	// samples out of 15. The window hides it natively through
	// `player_set_cursor`, and what is asserted here is the OSD's half of that
	// contract: the request is made when the furniture hides and unmade when it
	// returns. Chromium cannot observe a Win32 cursor, so this is a weaker claim
	// than "the pointer is hidden", and it is written down as one.
	const asked = await page.evaluate(() => window.__cursorRequests ?? []);
	const lastAsked = asked.length ? asked[asked.length - 1] : null;
	if (lastAsked === true) {
		console.error(
			`the film is paused and the OSD is still asking for the pointer to be hidden: ${JSON.stringify(asked)}`
		);
		failures++;
	}
	if (!asked.includes(true)) {
		console.error(`the furniture hid at some point and the pointer was never asked to hide: ${JSON.stringify(asked)}`);
		failures++;
	}
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
	await page.click('button.action--quiet');
	await page.waitForTimeout(500);
	const commands = await page.evaluate(() => window.__commands ?? []);
	const discovered = await page.locator('.servers li button').count();
	const hue = await page.evaluate(() => ({
		title: document.querySelector('.library-title')?.textContent?.trim() ?? null,
		hint: document.querySelector('.hint')?.textContent?.trim() ?? null,
		busy: !!document.querySelector('button[type=submit][disabled]'),
	}));
	if (!commands.includes('player_discover')) {
		console.error(`"Find a server" issued ${JSON.stringify(commands)} - player_discover was never asked`);
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
// French is the default and English ships complete; `check:i18n` guards parity,
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

	const fr = await readingOf();
	if (fr.htmlLang !== 'fr') {
		console.error(`the OSD starts in "${fr.htmlLang}" instead of French, which is the default`);
		failures++;
	}
	if (fr.chip !== 'FR') {
		console.error(`the language chip reads "${fr.chip}" in French, expected "FR"`);
		failures++;
	}

	// The switch itself, from the header control, with no reload.
	await page.click('.control--language');
	await page.waitForTimeout(300);
	const en = await readingOf();
	if (en.htmlLang !== 'en') {
		console.error(`switching to English left document.lang at "${en.htmlLang}"`);
		failures++;
	}
	if (en.title === fr.title) {
		console.error(`the visible copy did not change with the language: still "${en.title}"`);
		failures++;
	}
	if (en.chip !== 'EN') {
		console.error(`the language chip reads "${en.chip}" after switching, expected "EN"`);
		failures++;
	}
	if (en.stored !== 'en') {
		console.error(`the chosen language was not stored (localStorage holds "${en.stored}"), so it will not survive a restart`);
		failures++;
	}
	if (!en.title || !en.connect || !en.addressLabel) {
		console.error('a sentence is missing in English: the catalogue is not complete on screen');
		failures++;
	}

	// And back, so the default is reachable in both directions.
	await page.click('.control--language');
	await page.waitForTimeout(300);
	const back = await readingOf();
	if (back.htmlLang !== 'fr' || back.title !== fr.title) {
		console.error(`switching back to French did not restore it: lang=${back.htmlLang} title="${back.title}"`);
		failures++;
	}

	// A restart is the page loading again with a choice already stored. It is a
	// reload and not a new browser context: a context has its own empty storage,
	// and the first version of this asked a brand-new context to remember
	// something it had never been told. The reload happens while the interface is
	// French and the store says English, so only reading the store at startup can
	// produce an English page.
	await page.evaluate(() => {
		localStorage.setItem('theia.player.language', 'en');
	});
	await page.reload({ waitUntil: 'networkidle' });
	await page.waitForTimeout(400);
	const restarted = await page.evaluate(() => ({
		htmlLang: document.documentElement.lang,
		title: document.querySelector('.library-title')?.textContent?.trim() ?? null,
	}));
	if (restarted.htmlLang !== 'en') {
		console.error(`the stored language was not honoured on a fresh load: document.lang is "${restarted.htmlLang}"`);
		failures++;
	}
	if (!restarted.title || restarted.title === fr.title) {
		console.error(`a fresh load with English stored still draws French: "${restarted.title}"`);
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

async function openPage(viewport, { tracks = TRACKS, movies = MOVIES, frame = false, discovered = [] } = {}) {
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
		({ tracks, movies, status, discovered }) => {
			window.__handlers = {};
			// Every command the OSD asks Rust for is recorded, because "one press
			// is one command" and "typing is not a shortcut" are counts, not
			// opinions. A failure that says only "the click did something" would
			// send the next person hunting with a debugger.
			window.__commands = [];
			// The native cursor command is recorded too: the OSD's half of the
			// pointer rule is "ask for it at the right moment", and that is all a
			// browser can check about a Win32 cursor.
			window.__cursorRequests = [];
			window.__TAURI__ = {
				core: {
					invoke: async (cmd, args) => {
						window.__commands.push(cmd);
						if (cmd === 'player_set_cursor') window.__cursorRequests.push(args?.hidden === true);
						if (cmd === 'player_tracks') return JSON.stringify(tracks);
						if (cmd === 'player_library') return JSON.stringify(movies);
						if (cmd === 'player_discover') return JSON.stringify(discovered);
						if (cmd === 'player_connect') {
							// A failure has to be reachable on demand: the sentences a
							// person reads when something is wrong are as much a part of
							// the interface as the ones they read when it works.
							if (window.__connectFails) throw new Error('the mock was told to fail');
							return JSON.stringify({
								url: 'http://127.0.0.1:8395',
								health: { status: 'ok', version: 'dev', uptime_seconds: 12 },
								profile: 1,
								profiles: [{ id: 1, name: '', is_default: true }],
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
						close() {},
						isFullscreen: async () => false,
						setFullscreen: async () => {},
					}),
				},
			};
			window.__status = status;
		},
		{ tracks, movies, status: STATUS, discovered }
	);
	await page.goto(URL, { waitUntil: 'networkidle' });
	await page.waitForTimeout(400);

	await addBackdrop(page, { frame });
	return page;
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
	if (present.controls !== 4) {
		console.error(`the minimum window drew ${present.controls} visible controls, expected 4 (play, tracks, fullscreen, close)`);
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

	// (m) what a person reads when something goes wrong, in both languages.
	const failuresPage = await openPage({ width: 1280, height: 720 });
	await assertFailuresAreReadable(failuresPage);
	await failuresPage.close();
}

await browser.close();
console.log(failures === 0 ? `render check passed, pictures in ${OUT}` : `${failures} render check(s) failed`);
process.exit(failures === 0 ? 0 : 1);
