import { defineConfig, devices } from '@playwright/test';

// The interface guard.
//
// Every visual bug this project has shipped looked completely fine on screen:
// cards overlapping their neighbours by 37px because an unlayered rule beat a
// layered one, a phone rule written above its own base rule and therefore
// silently dead, a display font refused by the browser and replaced by a
// fallback nobody could tell apart. None of them were caught by reading the
// CSS, and all of them would have been caught by a handful of assertions in a
// real browser.
//
// So this suite deliberately does not test behaviour -- the Go tests cover the
// API, and clicking through the player in CI would be slow and flaky. It asserts
// the four things that are invisible to the eye and expensive to get wrong:
// nothing overflows, every shipped font actually loaded, every target is big
// enough to hit, and the page has one left edge.
//
// THEIA_TEST_URL points it at an already-running server. Without one it starts
// the binary itself against a throwaway data directory.
const baseURL = process.env.THEIA_TEST_URL ?? 'http://127.0.0.1:8396';

export default defineConfig({
	testDir: './tests',
	fullyParallel: true,
	forbidOnly: !!process.env.CI,
	retries: process.env.CI ? 1 : 0,
	reporter: process.env.CI ? 'github' : 'list',

	use: {
		baseURL,
		// A failure here is a layout failure, so keep the picture.
		screenshot: 'only-on-failure',
		trace: 'retain-on-failure'
	},

	// The three screens section 4 and section 9 are written against. Widths are
	// what the design system steps on, so they are the thing to pin: 375 is the
	// phone step below 36rem, 1280 an ordinary desktop, 1920 the television step
	// above 100rem.
	projects: [
		{ name: 'phone', use: { ...devices['Desktop Chrome'], viewport: { width: 375, height: 812 } } },
		{ name: 'desktop', use: { ...devices['Desktop Chrome'], viewport: { width: 1280, height: 800 } } },
		{ name: 'television', use: { ...devices['Desktop Chrome'], viewport: { width: 1920, height: 1080 } } }
	],

	webServer: process.env.THEIA_TEST_URL
		? undefined
		: {
				command: 'node tests/serve.mjs',
				url: baseURL + '/api/health',
				reuseExistingServer: !process.env.CI,
				timeout: 60_000
			}
});
