import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './tests',
  fullyParallel: false,
  retries: 0,
  workers: 1,
  reporter: 'list',
  globalSetup: './tests/global-setup.ts',
  timeout: 10000,

  use: {
    baseURL: 'http://localhost:8081',
    headless: true,
    locale: 'sr-Latn-RS',
    timezoneId: 'Europe/Belgrade',
  },

  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
});
