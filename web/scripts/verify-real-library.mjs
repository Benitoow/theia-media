// Opt-in acceptance against a running Theia instance and one measured film
// from a real library. It is deliberately outside Playwright's ordinary test
// discovery: CI owns generated fixtures, while this gate owns media the
// maintainer is legally and physically able to access.
//
//   THEIA_REAL_URL=http://127.0.0.1:8395 \
//   THEIA_REAL_MOVIE_ID=123 npm exec -- node scripts/verify-real-library.mjs

import { chromium } from '@playwright/test';

const baseURL = process.env.THEIA_REAL_URL;
const movieID = Number(process.env.THEIA_REAL_MOVIE_ID);
if (!baseURL || !Number.isInteger(movieID) || movieID <= 0) {
	console.error('THEIA_REAL_URL and a positive THEIA_REAL_MOVIE_ID are required');
	process.exit(2);
}

const waitFor = async (description, probe, timeout = 90_000) => {
	const deadline = Date.now() + timeout;
	let last;
	while (Date.now() < deadline) {
		try {
			last = await probe();
			if (last) return last;
		} catch (error) {
			last = error;
		}
		await new Promise((resolve) => setTimeout(resolve, 250));
	}
	throw new Error(`${description} timed out; last result: ${String(last)}`);
};

const profiles = await fetch(`${baseURL}/api/profiles`).then((response) => response.json());
const profile = profiles.profiles?.find((item) => item.is_default) ?? profiles.profiles?.[0];
const browser = await chromium.launch({
	channel: process.env.THEIA_REAL_BROWSER ?? 'msedge',
	headless: process.env.THEIA_REAL_HEADED !== '1',
	args: ['--autoplay-policy=no-user-gesture-required']
});
const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
const pageErrors = [];
page.on('pageerror', (error) => pageErrors.push(error.message));
await page.addInitScript(({ profileID }) => {
	localStorage.setItem('theia.locale', 'fr');
	if (profileID) localStorage.setItem('theia.profile', String(profileID));
	// Exercise the two-minute resource-release path without spending two
	// minutes of every maintainer acceptance run staring at a paused frame.
	const nativeSetTimeout = window.setTimeout.bind(window);
	window.setTimeout = (callback, delay, ...args) =>
		nativeSetTimeout(callback, delay === 120_000 ? 500 : delay, ...args);
}, { profileID: profile?.id ?? null });
await page.route('**/preview*', (route) => route.abort());

try {
	await page.goto(`${baseURL}/movie/${movieID}`, { waitUntil: 'domcontentloaded' });
	const compatibility = page.getByRole('region', { name: 'Lecture sur cet appareil' });
	await compatibility.waitFor({ state: 'visible', timeout: 20_000 });

	await page.getByRole('button', { name: /^(Lire|Reprendre)/ }).first().click();
	const restart = page.getByRole('button', { name: /du début/i });
	await waitFor('player creation', async () =>
		(await page.locator('video').count()) > 0 || await restart.isVisible());
	if (await restart.isVisible()) await restart.click();

	const video = page.locator('video');
	await waitFor('decoded playback', () => video.evaluate((element) => element.currentTime > 2));
	const first = await video.evaluate((element) => {
		const quality = element.getVideoPlaybackQuality?.();
		return {
			currentTime: element.currentTime,
			width: element.videoWidth,
			height: element.videoHeight,
			frames: quality?.totalVideoFrames ?? element.webkitDecodedFrameCount ?? 0,
			source: element.dataset.streamSource
		};
	});
	if (first.width <= 0 || first.height <= 0 || first.frames <= 0) {
		throw new Error(`no decoded picture: ${JSON.stringify(first)}`);
	}

	// A real seek has to replace the stream and preserve the absolute clock.
	const slider = page.getByRole('slider', { name: 'Position dans le film' });
	const maximum = Number(await slider.getAttribute('aria-valuemax'));
	const box = await slider.boundingBox();
	if (!box || maximum <= 60) throw new Error('the real film has no usable duration');
	await page.mouse.click(box.x + box.width * 0.55, box.y + box.height / 2);
	const seekPosition = await waitFor('deep seek', async () => {
		const value = Number(await slider.getAttribute('aria-valuenow'));
		return value > maximum * 0.5 ? value : 0;
	});
	await waitFor('picture after deep seek', () => video.evaluate((element) => element.currentTime > 1));

	// If the file really has several audio tracks, switch to one. The resulting
	// URL is the proof that the choice survived the player split.
	let switchedAudio = false;
	const settings = page.locator('.player-tracks-anchor > button');
	if (await settings.count()) {
		await page.mouse.move(600, 700);
		await settings.click();
		const audioOptions = page.locator('.player-tracks .track-list').first().getByRole('button');
		if (await audioOptions.count() > 2) {
			await audioOptions.nth(2).click();
			await waitFor('audio stream replacement', async () =>
				(await video.getAttribute('data-stream-source'))?.includes('audio='));
			switchedAudio = true;
		}
		if (await page.locator('.player-tracks').isVisible().catch(() => false)) {
			await page.keyboard.press('Escape');
		}
	}

	const pausedAt = Number(await slider.getAttribute('aria-valuenow'));
	await slider.focus();
	await page.keyboard.press('Space');
	await waitFor('paused fragmented stream release', async () => (await page.locator('video').count()) === 0, 10_000);
	await page.keyboard.press('Space');
	await waitFor('playback after released-stream recreation', async () => {
		if ((await page.locator('video').count()) === 0) return false;
		return page.locator('video').evaluate((element) => element.currentTime > 1);
	});
	const resumedAt = Number(await slider.getAttribute('aria-valuenow'));
	if (resumedAt < pausedAt - 2) {
		throw new Error(`resume moved backwards: ${pausedAt} -> ${resumedAt}`);
	}

	for (let attempt = 0; attempt < 3 && await page.locator('video').count(); attempt++) {
		await page.keyboard.press('Escape');
	}
	await waitFor('player close', async () => (await page.locator('video').count()) === 0, 10_000);
	if (pageErrors.length) throw new Error(`page errors: ${pageErrors.join(' | ')}`);

	console.log(JSON.stringify({
		browser: process.env.THEIA_REAL_BROWSER ?? 'msedge',
		movieID,
		decoded: `${first.width}x${first.height}`,
		framesBeforeSeek: first.frames,
		seekPosition,
		switchedAudio,
		pausedAt,
		resumedAt,
		pageErrors: 0
	}, null, 2));
	console.log('PASS: real-library playback, deep seek, stream replacement, pause release and resume');
} finally {
	await browser.close();
}
