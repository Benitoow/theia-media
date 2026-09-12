import { defineConfig } from '@playwright/test';
export default defineConfig({
 testDir: './tests', testMatch: 'playback.spec.js', fullyParallel: false, workers: 1,
 forbidOnly: !!process.env.CI, retries: 0, timeout: 60_000,
 use: { baseURL: 'http://127.0.0.1:8397', viewport: { width: 1280, height: 800 }, screenshot: 'only-on-failure', trace: 'retain-on-failure' },
 projects: [
  { name: 'chromium', use: { browserName: 'chromium', launchOptions: { args: ['--autoplay-policy=no-user-gesture-required'] } } },
  // Chromium's engine is not enough proof for Edge's shipped integration:
  // its media quotas and codecs differ. Exercise the installed stable browser.
  { name: 'edge', use: { browserName: 'chromium', channel: 'msedge', launchOptions: { args: ['--autoplay-policy=no-user-gesture-required'] } } },
  { name: 'firefox', use: { browserName: 'firefox', firefoxUserPrefs: { 'media.autoplay.default': 0 } } },
  // This is WebKit's integration contract, not a claim about Safari hardware.
  { name: 'webkit', use: { browserName: 'webkit' } }
 ],
 webServer: { command: 'node tests/serve-playback.mjs', url: 'http://127.0.0.1:8397/api/health', timeout: 600_000, reuseExistingServer: false }
});
