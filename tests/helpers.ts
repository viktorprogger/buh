import { Page } from '@playwright/test';

export const TEST_EMAIL = 'pw@test.local';
export const TEST_PASSWORD = 'pw-test-123';
export const CURRENT_YEAR = new Date().getFullYear();

export async function loginAccountant(page: Page) {
  await page.goto('/login');
  await page.fill('input[name="email"]', TEST_EMAIL);
  await page.fill('input[name="password"]', TEST_PASSWORD);
  await page.click('#btn-login');
  await page.waitForURL('/a/');
}

// Backwards-compatible alias for existing specs.
export const login = loginAccountant;

export async function loginEntrepreneur(page: Page, email: string, password: string) {
  await page.goto('/login');
  await page.click('#tab-entrepreneur');
  await page.fill('input[name="email"]', email);
  await page.fill('input[name="password"]', password);
  await page.click('#btn-login');
  await page.waitForURL('/e/');
}

export async function registerEntrepreneur(page: Page, email: string, password: string) {
  await page.goto('/e/register');
  await page.fill('input[name="email"]', email);
  await page.fill('input[name="password"]', password);
  await page.fill('input[name="confirm_password"]', password);
  await page.click('#btn-register');
  await page.waitForURL('/e/');
}

/** Generates a unique 9-digit PIB for test isolation across runs.
 * The last digit is a valid Serbian mod-11 check digit so the server accepts it. */
export function uniquePib(): string {
  const base = String(Date.now()).slice(-8);
  let p = 10;
  for (const ch of base) {
    p = (p + parseInt(ch, 10)) % 10;
    if (p === 0) p = 10;
    p = (p * 2) % 11;
  }
  return base + ((11 - p) % 10);
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
  await page.click('#btn-save-client');
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
  await page.click('#btn-save-client');
  await page.waitForURL('/e/clients');
}

/** Creates a test entrepreneur via the accountant UI and returns its page URL. */
export async function createEntrepreneur(page: Page, name = 'КПО Тест фирма', pib?: string): Promise<string> {
  pib ??= uniquePib();
  await page.goto('/a/entrepreneurs/new');
  await page.fill('input[name="name"]', name);
  await page.fill('input[name="pib"]', pib);
  await page.click('#btn-save-entrepreneur');
  await page.waitForURL(/\/a\/entrepreneurs\/[0-9a-f-]+/);
  return page.url();
}

/** Pairs an entrepreneur with an accountant via invitation flow. */
export async function pairEntrepreneur(
  accountantPage: Page,
  entPage: Page,
  entId: string,
): Promise<void> {
  // Entrepreneur initiates the invitation
  await entPage.goto('/e/invite-accountant');
  await entPage.click('#btn-generate-invite');
  // Server renders the token in-place; URL stays at /e/invite-accountant
  await entPage.locator('.font-mono').first().waitFor();

  // Read the invitation token path
  const invitePath = (await entPage.locator('.font-mono').first().innerText()).trim();

  // Accountant accepts the invitation
  await accountantPage.goto(invitePath);
  await accountantPage.fill('input[name="managed_entrepreneur_id"]', entId);
  await accountantPage.click('#btn-accept-invite');
  await accountantPage.waitForURL(/\/a\//);
}

/** Creates a slip via the accountant UI for a managed entrepreneur. */
export async function createAccountantSlip(
  page: Page,
  entPath: string,
  purpose: string,
  amount = '5000',
): Promise<void> {
  await page.goto(`${entPath}/slips/new`);
  await page.fill('input[name="SF"]', '253');
  await page.fill('input[name="S"]', purpose);
  await page.fill('input[name="N"]', 'Пореска управа');
  await page.fill('input[name="R"]', '840-3553531843-20');
  await page.fill('input[name="amount"]', amount);
  await page.click('#btn-save-slip-acc');
  await page.waitForURL(/\/a\/slips\//);
}

/** Creates a slip via the entrepreneur UI and returns the slip detail path. */
export async function createEntrepreneurSlip(
  page: Page,
  purpose: string,
  options?: { year?: number; advance?: boolean },
): Promise<string> {
  await page.goto('/e/slips/new');
  await page.fill('input[name="SF"]', '253');
  await page.fill('input[name="S"]', purpose);
  await page.fill('input[name="N"]', 'Пореска управа');
  await page.fill('input[name="R"]', '840-3553531843-20');
  await page.fill('input[name="amount"]', '5000');
  await page.fill('input[name="year"]', String(options?.year ?? CURRENT_YEAR));

  if (options?.advance) {
    await page.check('input[name="advance"]');
  }

  await page.click('#btn-save-slip');
  await page.waitForURL('/e/slips');

  // Click the row with the slip to navigate to its detail page
  await page.locator('tr').filter({ hasText: purpose }).click();
  await page.waitForURL(/\/e\/slips\/[0-9a-f-]+/);

  return new URL(page.url()).pathname;
}
