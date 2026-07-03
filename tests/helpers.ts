import { Page } from '@playwright/test';

export const TEST_EMAIL = 'pw@test.local';
export const TEST_PASSWORD = 'pw-test-123';
export const CURRENT_YEAR = new Date().getFullYear();

export async function loginAccountant(page: Page) {
  await page.goto('/login');
  await page.fill('input[name="email"]', TEST_EMAIL);
  await page.fill('input[name="password"]', TEST_PASSWORD);
  await page.click('button[type="submit"]');
  await page.waitForURL('/a/');
}

// Backwards-compatible alias for existing specs.
export const login = loginAccountant;

export async function loginEntrepreneur(page: Page, email: string, password: string) {
  await page.goto('/login');
  await page.click('#tab-entrepreneur');
  await page.fill('input[name="email"]', email);
  await page.fill('input[name="password"]', password);
  await page.click('button[type="submit"]');
  await page.waitForURL('/e/');
}

export async function registerEntrepreneur(page: Page, email: string, password: string) {
  await page.goto('/e/register');
  await page.fill('input[name="email"]', email);
  await page.fill('input[name="password"]', password);
  await page.fill('input[name="confirm_password"]', password);
  await page.click('button[type="submit"]');
  await page.waitForURL('/e/');
}

/** Generates a unique 9-digit PIB for test isolation across runs. */
export function uniquePib(): string {
  return String(Date.now()).slice(-9);
}

/** Generates a unique 8-digit MB for test isolation across runs. */
export function uniqueMb(): string {
  return String(Date.now()).slice(-8);
}

/** Creates a local (Serbian) client. Requires PIB + MB. */
export async function createLocalClient(page: Page, name: string): Promise<void> {
  await page.goto('/e/clients/new');
  await page.fill('input[name="name"]', name);
  await page.fill('input[name="pib"]', uniquePib());
  await page.fill('#f-reg', uniqueMb());
  await page.fill('input[name="email"]', 'klijent@test.rs');
  await page.getByRole('button', { name: 'Додај клијента' }).click();
  await page.waitForURL('/e/clients');
}

/** Creates a foreign client. Requires only Tax ID / Reg No. */
export async function createForeignClient(
  page: Page,
  name: string,
  taxId = 'TEST-TAX-ID-001',
): Promise<void> {
  await page.goto('/e/clients/new');
  await page.fill('input[name="name"]', name);
  await page.check('input[name="is_foreign"]');
  await page.fill('#f-taxid', taxId);
  await page.fill('input[name="address"]', 'Almaty, Kazakhstan');
  await page.getByRole('button', { name: 'Додај клијента' }).click();
  await page.waitForURL('/e/clients');
}

/** Creates a test entrepreneur via the accountant UI and returns its page URL. */
export async function createEntrepreneur(page: Page, name = 'КПО Тест фирма', pib?: string): Promise<string> {
  pib ??= uniquePib();
  await page.goto('/a/entrepreneurs/new');
  await page.fill('input[name="name"]', name);
  await page.fill('input[name="pib"]', pib);
  await page.click('button[type="submit"]');
  await page.waitForURL(/\/a\/entrepreneurs\/[0-9a-f-]+/);
  return page.url();
}
