import { test, expect } from '@playwright/test';
import { loginEntrepreneur, registerEntrepreneur, CURRENT_YEAR } from './helpers';

const email = `slips-basic-${Date.now()}@test.local`;
const password = 'slips-basic-pass-123';

test.beforeAll(async ({ browser }) => {
  const page = await browser.newPage();
  await registerEntrepreneur(page, email, password);
  await page.close();
});

test.beforeEach(async ({ page }) => {
  await loginEntrepreneur(page, email, password);
});

test.describe('empty state', () => {
  test('shows empty state when no slips exist', async ({ page }) => {
    await page.goto('/e/slips');
    await expect(page.getByText('Немате уплатница. Додајте прву.')).toBeVisible();
    await expect(page.locator('table')).not.toBeVisible();
  });
});

test.describe('manual slip creation', () => {
  const purpose = 'Паушал тест ' + Date.now();

  test.beforeAll(async ({ browser }) => {
    const page = await browser.newPage();
    await loginEntrepreneur(page, email, password);
    await page.goto('/e/slips/new');
    await page.fill('input[name="SF"]', '253');
    await page.fill('input[name="S"]', purpose);
    await page.fill('input[name="N"]', 'Пореска управа');
    await page.fill('input[name="R"]', '840-3553531843-20');
    await page.fill('input[name="amount"]', '5000');
    await page.fill('input[name="year"]', String(CURRENT_YEAR));
    await page.click('#btn-save-slip');
    await page.waitForURL('/e/slips');
    await page.close();
  });

  test('create slip — form saves and redirects to list', async ({ page }) => {
    const newPurpose = 'Паушал тест ' + Date.now();
    await page.goto('/e/slips/new');
    await page.fill('input[name="SF"]', '253');
    await page.fill('input[name="S"]', newPurpose);
    await page.fill('input[name="N"]', 'Пореска управа');
    await page.fill('input[name="R"]', '840-3553531843-20');
    await page.fill('input[name="amount"]', '5000');
    await page.fill('input[name="year"]', String(CURRENT_YEAR));
    await page.click('#btn-save-slip');
    await page.waitForURL('/e/slips');
    await expect(page).toHaveURL('/e/slips');
    await expect(page.getByRole('cell', { name: newPurpose })).toBeVisible();
  });

  test('create slip — appears under correct year header', async ({ page }) => {
    await page.goto('/e/slips');
    await expect(page.getByRole('heading', { name: String(CURRENT_YEAR) })).toBeVisible();
    await expect(page.getByText(purpose)).toBeVisible();
  });

  test('create slip — advance slip appears in advance subsection', async ({ page }) => {
    const advancePurpose = 'Аванс тест ' + Date.now();
    await page.goto('/e/slips/new');
    await page.fill('input[name="SF"]', '253');
    await page.fill('input[name="S"]', advancePurpose);
    await page.fill('input[name="N"]', 'Пореска управа');
    await page.fill('input[name="R"]', '840-3553531843-20');
    await page.fill('input[name="amount"]', '5000');
    await page.fill('input[name="year"]', String(CURRENT_YEAR));
    await page.check('input[name="advance"]');
    await page.click('#btn-save-slip');
    await page.waitForURL('/e/slips');
    await page.goto('/e/slips');
    await expect(page.getByText('Аванс', { exact: true })).toBeVisible();
    await expect(page.getByText(advancePurpose)).toBeVisible();
  });

  test('create slip — previous year appears in collapsed details section', async ({ page }) => {
    const previousYearPurpose = 'Прошла година ' + Date.now();
    await page.goto('/e/slips/new');
    await page.fill('input[name="SF"]', '253');
    await page.fill('input[name="S"]', previousYearPurpose);
    await page.fill('input[name="N"]', 'Пореска управа');
    await page.fill('input[name="R"]', '840-3553531843-20');
    await page.fill('input[name="amount"]', '5000');
    await page.fill('input[name="year"]', String(CURRENT_YEAR - 1));
    await page.click('#btn-save-slip');
    await page.waitForURL('/e/slips');
    await page.goto('/e/slips');
    const detailsElement = page.locator(`details:has-text("${CURRENT_YEAR - 1}")`);
    await expect(detailsElement).toBeVisible();
    await expect(detailsElement).not.toHaveAttribute('open');
  });
});
