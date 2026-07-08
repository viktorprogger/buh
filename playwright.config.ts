import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './tests',
  fullyParallel: false,
  retries: 0,
  workers: 1,
  reporter: 'list',
  globalSetup: './tests/global-setup.ts',
  timeout: 120000,
  expect: { timeout: 15000 },

  use: {
    baseURL: 'http://localhost:8081',
    headless: true,
    locale: 'sr-Latn-RS',
    timezoneId: 'Europe/Belgrade',
    actionTimeout: 15000,
    navigationTimeout: 15000,
    extraHTTPHeaders: {
      'Accept-Language': 'sr',
    },
  },

  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
});
