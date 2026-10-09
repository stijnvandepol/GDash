import { defineConfig } from '@playwright/test';
import { baseUrl } from './e2e/env';

// End-to-end tests against the real, already-built server binary; see
// e2e/global-setup.ts. Chromium only: Cast receivers and the people who
// run GDash use it, and one engine keeps the suite fast.
export default defineConfig({
  testDir: 'e2e',
  globalSetup: './e2e/global-setup.ts',
  // The journeys share one server and one database, and run in file order:
  // the first one creates the account the others log in with.
  workers: 1,
  fullyParallel: false,
  retries: 0,
  forbidOnly: !!process.env.CI,
  timeout: 30_000,
  expect: { timeout: 7_500 },
  reporter: process.env.CI ? [['list'], ['html', { open: 'never' }]] : 'list',
  outputDir: 'test-results',
  use: {
    baseURL: baseUrl(),
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  projects: [{ name: 'chromium', use: { browserName: 'chromium' } }],
});
