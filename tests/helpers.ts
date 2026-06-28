import { Page } from '@playwright/test';

export const TEST_EMAIL = 'pw@test.local';
export const TEST_PASSWORD = 'pw-test-123';
export const CURRENT_YEAR = new Date().getFullYear();

export async function login(page: Page) {
  await page.goto('/login');
  await page.fill('input[name="email"]', TEST_EMAIL);
  await page.fill('input[name="password"]', TEST_PASSWORD);
  await page.click('button[type="submit"]');
  await page.waitForURL('/');
}

/** Generates a unique 9-digit PIB for test isolation across runs. */
export function uniquePib(): string {
  return String(Date.now()).slice(-9);
}

/** Creates a test entrepreneur via the UI and returns its page URL. */
export async function createEntrepreneur(page: Page, name = 'КПО Тест фирма', pib?: string): Promise<string> {
  pib ??= uniquePib();
  await page.goto('/entrepreneurs/new');
  await page.fill('input[name="name"]', name);
  await page.fill('input[name="pib"]', pib);
  await page.click('button[type="submit"]');
  await page.waitForURL(/\/entrepreneurs\/[0-9a-f-]+/);
  return page.url();
}
