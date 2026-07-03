/**
 * Smoke tests: every route returns the correct page with proper auth.
 * Covers all main GET endpoints without deep behavioral assertions.
 */
import { test, expect } from '@playwright/test';
import {
  loginAccountant,
  loginEntrepreneur,
  registerEntrepreneur,
  createEntrepreneur,
  uniquePib,
  CURRENT_YEAR,
} from './helpers';

// ── Public routes — no auth needed ────────────────────────────────────────────

test('login page renders', async ({ page }) => {
  await page.goto('/login');
  await expect(page.locator('input[name="email"]')).toBeVisible();
  await expect(page.locator('input[name="password"]')).toBeVisible();
});

test('register page renders', async ({ page }) => {
  await page.goto('/e/register');
  await expect(page.locator('h1')).toContainText('Регистрација');
  await expect(page.locator('input[name="email"]')).toBeVisible();
});

test('privacy page renders', async ({ page }) => {
  await page.goto('/privacy');
  await expect(page).not.toHaveURL(/login/);
  await expect(page.locator('h1')).toBeVisible();
});

test('terms page renders', async ({ page }) => {
  await page.goto('/terms');
  await expect(page).not.toHaveURL(/login/);
  await expect(page.locator('h1')).toBeVisible();
});

test('root redirects unauthenticated to login', async ({ page }) => {
  await page.goto('/');
  await expect(page).toHaveURL('/login');
});

// ── Accountant routes ─────────────────────────────────────────────────────────

test.describe('accountant', () => {
  let entUrl: string;
  let entPath: string;

  test.beforeAll(async ({ browser }) => {
    const page = await browser.newPage();
    await loginAccountant(page);
    entUrl = await createEntrepreneur(page, 'Смок Тест ' + Date.now(), uniquePib());
    entPath = new URL(entUrl).pathname;
    await page.close();
  });

  test.beforeEach(async ({ page }) => {
    await loginAccountant(page);
  });

  test('index — accountant sees entrepreneur list', async ({ page }) => {
    await page.goto('/a/');
    await expect(page).not.toHaveURL(/login/);
    await expect(page.locator('table, [role="table"]')).toBeVisible();
  });

  test('new entrepreneur form renders', async ({ page }) => {
    await page.goto('/a/entrepreneurs/new');
    await expect(page.locator('input[name="name"]')).toBeVisible();
    await expect(page.locator('input[name="pib"]')).toBeVisible();
  });

  test('entrepreneur detail page — KPO section present', async ({ page }) => {
    await page.goto(entPath);
    await expect(page).not.toHaveURL(/login/);
    await expect(page.getByRole('heading', { name: 'КПО — Књига прихода' })).toBeVisible();
  });

  test('entrepreneur detail page — address and bank account fields visible in edit mode', async ({ page }) => {
    await page.goto(entPath);
    await expect(page).not.toHaveURL(/login/);
    await page.getByRole('button', { name: 'Измени' }).click();
    await expect(page.locator('#f-address')).toBeVisible();
    await expect(page.locator('#f-bank')).toBeVisible();
  });

  test('new slip form renders', async ({ page }) => {
    await page.goto(entPath + '/slips/new');
    await expect(page).not.toHaveURL(/login/);
    await expect(page.locator('form')).toBeVisible();
  });

  test('creating a slip records history', async ({ page }) => {
    await page.goto(entPath + '/slips/new');
    await page.fill('input[name="SF"]', '253');
    await page.fill('input[name="S"]', 'Паушални порез');
    await page.click('button[type="submit"]');
    await page.waitForURL(/\/a\/slips\//);
    await expect(page.getByText('Уплатница је креирана')).toBeVisible();
  });

  test('KPO merge view renders', async ({ page }) => {
    await page.goto(entPath + '/kpo/' + CURRENT_YEAR + '/merge');
    await expect(page).not.toHaveURL(/login/);
  });

  test('protected routes redirect unauthenticated requests', async ({ page }) => {
    // Log out so we're unauthenticated.
    await page.goto('/logout');
    await page.goto('/a/');
    await expect(page).toHaveURL(/login/);
  });
});

// ── Entrepreneur routes ───────────────────────────────────────────────────────

test.describe('entrepreneur', () => {
  const email = `smoke-ent-${Date.now()}@test.local`;
  const password = 'smoke-test-pass-123';

  test.beforeAll(async ({ browser }) => {
    const page = await browser.newPage();
    await registerEntrepreneur(page, email, password);
    await page.close();
  });

  test.beforeEach(async ({ page }) => {
    await loginEntrepreneur(page, email, password);
  });

  test('dashboard renders', async ({ page }) => {
    await page.goto('/e/');
    await expect(page).not.toHaveURL(/login/);
    await expect(page.getByRole('heading', { name: 'Dashboard' })).toBeVisible();
  });

  test('invite accountant page renders', async ({ page }) => {
    await page.goto('/e/invite-accountant');
    await expect(page).not.toHaveURL(/login/);
  });

  test('client list renders', async ({ page }) => {
    await page.goto('/e/clients');
    await expect(page).not.toHaveURL(/login/);
  });

  test('new client form renders', async ({ page }) => {
    await page.goto('/e/clients/new');
    await expect(page.locator('input[name="name"]')).toBeVisible();
  });

  test('bank account list renders', async ({ page }) => {
    await page.goto('/e/bank-accounts');
    await expect(page).not.toHaveURL(/login/);
  });

  test('new bank account form renders', async ({ page }) => {
    await page.goto('/e/bank-accounts/new');
    await expect(page.locator('input[name="bank_name"]')).toBeVisible();
  });

  test('invoice list renders', async ({ page }) => {
    await page.goto('/e/invoices');
    await expect(page).not.toHaveURL(/login/);
  });

  test('new invoice form renders', async ({ page }) => {
    await page.goto('/e/invoices/new');
    await expect(page.locator('#f-invnum')).toBeVisible();
  });

  test('KPO page renders', async ({ page }) => {
    await page.goto('/e/kpo/' + CURRENT_YEAR);
    await expect(page).not.toHaveURL(/login/);
    await expect(page.locator('h1, h2')).toBeVisible();
  });

  test('profile page renders', async ({ page }) => {
    await page.goto('/e/profile');
    await expect(page).not.toHaveURL(/login/);
    await expect(page.locator('h1')).toContainText('Мој профил');
  });

  test('protected routes redirect unauthenticated requests', async ({ page }) => {
    await page.goto('/logout');
    await page.goto('/e/');
    await expect(page).toHaveURL(/login/);
  });
});
