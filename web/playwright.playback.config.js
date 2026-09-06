import { defineConfig } from '@playwright/test';
export default defineConfig({
 testDir: './tests', testMatch: 'playback.spec.js', fullyParallel: false, workers: 1,
 forbidOnly: !!process.env.CI, retries: 0, timeout: 60_000,
 use: { baseURL: 'http://127.0.0.1:8397', viewport: { width: 1280, height: 800 }, screenshot: 'only-on-failure', trace: 'retain-on-failure', launchOptions: { args: ['--autoplay-policy=no-user-gesture-required'] } },
 webServer: { command: 'node tests/serve-playback.mjs', url: 'http://127.0.0.1:8397/api/health', timeout: 600_000, reuseExistingServer: false }
});
