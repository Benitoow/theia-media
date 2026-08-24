import { expect, test } from '@playwright/test';

// The pages that have a layout worth guarding. The player is deliberately
// absent: it needs a real file to open, and what it does with one is the Go
// suite's business.
const pages = [
	['home', '/'],
	['films', '/films'],
	['series', '/series'],
	['search', '/recherche'],
	['settings', '/reglages']
];

/** Waits for the interface to have finished arriving, fonts included. */
async function settled(page) {
	await page.waitForLoadState('networkidle');
	await page.evaluate(() => document.fonts.ready);
}

test.describe('nothing overflows the viewport', () => {
	// The failure this catches: a rule that only applies at one width, written
	// above the rule it means to override, so it silently does nothing and a
	// toolbar hangs off the side of a phone.
	for (const [name, path] of pages) {
		test(name, async ({ page }) => {
			await page.goto(path);
			await settled(page);

			const overflow = await page.evaluate(() => {
				const vw = document.documentElement.clientWidth;
				// An element wider than the viewport is fine inside a scroller
				// that was built to hold it -- the filter chips on a phone are
				// meant to scroll sideways. Only unscrolled overflow is a bug.
				const scrolled = (el) => {
					for (let p = el.parentElement; p && p !== document.body; p = p.parentElement) {
						if (/auto|scroll/.test(getComputedStyle(p).overflowX)) return true;
					}
					return false;
				};
				// Full-bleed artwork painted behind the content is allowed to
				// exceed the viewport, and does on purpose: the hero backdrop is
				// over-scaled by 1.5% so no edge shows a gap. Nothing there can be
				// read or touched, and the page still must not scroll sideways --
				// which the assertion below checks separately.
				const decoration = (el) => {
					const z = parseInt(getComputedStyle(el).zIndex, 10);
					return z < 0 && el.textContent.trim() === '';
				};
				return {
					page: document.body.scrollWidth,
					viewport: vw,
					offenders: [...document.querySelectorAll('body *')]
						.filter((el) => {
							const r = el.getBoundingClientRect();
							if (!(r.width > 0)) return false;
							if (!(r.right > vw + 1 || r.left < -1)) return false;
							return !scrolled(el) && !decoration(el);
						})
						.slice(0, 5)
						.map((el) => (el.className?.toString() || el.tagName).slice(0, 40))
				};
			});

			expect(overflow.offenders, 'elements hanging outside the viewport').toEqual([]);
			expect(overflow.page, 'the page itself scrolls sideways').toBeLessThanOrEqual(
				overflow.viewport + 1
			);
		});
	}
});

test.describe('every shipped font actually loaded', () => {
	// The failure this catches, and the reason this file exists at all: a face
	// the browser refuses renders as the fallback behind it, which is a
	// perfectly good font, so the page looks right and the screenshot proves
	// nothing. Section 10 of the design system tells the whole story.
	for (const [name, path] of pages) {
		test(name, async ({ page }) => {
			const failures = [];
			page.on('console', (message) => {
				const text = message.text();
				if (/failed to decode|OTS parsing error/i.test(text)) failures.push(text);
			});

			await page.goto(path);
			await settled(page);

			const fonts = await page.evaluate(() => ({
				entries: [...document.fonts].map((f) => ({ family: f.family, status: f.status })),
				display: getComputedStyle(document.body).getPropertyValue('--font-display'),
				label: getComputedStyle(document.body).getPropertyValue('--font-label')
			}));

			// Every face the page declared has to have arrived.
			const refused = fonts.entries.filter((f) => f.status === 'error');
			expect(refused, 'faces the browser refused').toEqual([]);
			expect(failures, 'font decoding errors on the console').toEqual([]);

			// And the first family of each register has to be one of them, so
			// that renaming a token cannot quietly leave the page on a fallback.
			const first = (stack) => stack.split(',')[0].trim().replace(/^['"]|['"]$/g, '');
			for (const stack of [fonts.display, fonts.label]) {
				const family = first(stack);
				const loaded = fonts.entries.some((f) => f.family === family && f.status === 'loaded');
				expect(loaded, `${family} is named by a token but never loaded`).toBe(true);
			}
		});
	}
});

test.describe('every target can be hit', () => {
	// Section 9: 44px minimum, including in the card grid. Easy to lose to a
	// padding change nobody connects with a remote or a thumb.
	for (const [name, path] of pages) {
		test(name, async ({ page }) => {
			await page.goto(path);
			await settled(page);

			const small = await page.evaluate(() => {
				return [...document.querySelectorAll('a, button, [role="button"], input, select')]
					.filter((el) => {
						const r = el.getBoundingClientRect();
						if (r.width === 0 || r.height === 0) return false; // not on screen
						if (getComputedStyle(el).visibility === 'hidden') return false;
						return r.height < 44 || r.width < 44;
					})
					.slice(0, 6)
					.map((el) => {
						const r = el.getBoundingClientRect();
						return `${(el.className?.toString() || el.tagName).slice(0, 30)} ${Math.round(r.width)}x${Math.round(r.height)}`;
					});
			});

			expect(small, 'targets below 44px').toEqual([]);
		});
	}
});

test.describe('the page has one left edge', () => {
	// The failure this catches: .page-shell caps its width and centres itself
	// while everything else follows the gutter, so on a television the row
	// headings finished 184px to the right of their own cards.
	for (const [name, path] of [
		['home', '/'],
		['films', '/films']
	]) {
		test(name, async ({ page }, testInfo) => {
			await page.goto(path);
			await settled(page);

			const edges = await page.evaluate(() => {
				const left = (selector) => {
					const el = document.querySelector(selector);
					return el ? Math.round(el.getBoundingClientRect().left) : null;
				};
				return {
					nav: left('.site-nav'),
					title: left('.page-title, .section-title'),
					card: left('.poster-card')
				};
			});

			// A library with nothing in it answers / with the first-launch screen,
			// which has no navigation and no grid. That page has its own shape and
			// nothing here applies to it, so this is not applicable rather than
			// failing -- the overflow and font checks above still cover it.
			const found = Object.entries(edges).filter(([, v]) => v !== null);
			test.skip(found.length < 2, 'no shared edge on this page');

			const values = found.map(([, v]) => v);
			const spread = Math.max(...values) - Math.min(...values);
			expect(spread, `left edges disagree: ${JSON.stringify(edges)}`).toBeLessThanOrEqual(2);

			// On the television step the shared edge is also the safe area, which
			// is the thing overscan eats. 96px on 1920, per section 5.
			if (testInfo.project.name === 'television') {
				expect(Math.min(...values), 'inside the 5% safe area').toBeGreaterThanOrEqual(96);
			}
		});
	}
});

test.describe('the page arrives at all', () => {
	// The failure this was written for: a component used without being imported.
	// Svelte compiled it happily, the browser threw ReferenceError while
	// rendering, and the home page stopped at its loading skeleton. That skeleton
	// overflows nothing, loads every font, offers no target under 44px and has
	// exactly one left edge -- so all four assertions above passed against a page
	// that never rendered. Nothing here looks at layout; it asks whether the page
	// ran.
	//
	// Its limit, measured rather than assumed. The fault was reintroduced on
	// purpose and this suite still passed, because serve.mjs starts the binary
	// against an empty throwaway directory: with no films there is no hero, so
	// the component that threw is never rendered. This catches a page that breaks
	// on its own, and cannot catch one that only breaks once there is something
	// to show. Closing that gap means seeding the guard's library -- scripts/bench
	// exists and does exactly this (decision 83) -- and costs the guard a Go
	// toolchain it does not currently need. Written down instead of pretended
	// away.
	for (const [name, path] of pages) {
		test(name, async ({ page }) => {
			const thrown = [];
			page.on('pageerror', (error) => thrown.push(String(error)));

			await page.goto(path);
			await settled(page);
			// A skeleton is honest while data is in flight; networkidle means it
			// has landed, and the beat after it covers the render that follows.
			await page.waitForTimeout(500);

			expect(thrown, `uncaught error on ${path}`).toEqual([]);

			const stalled = await page.locator('[class*="skeleton"]').count();
			expect(stalled, `${path} is still showing its loading skeleton`).toBe(0);
		});
	}
});
