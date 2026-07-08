import { test, expect, Page } from '@playwright/test';
import { login, createEntrepreneur, uniquePib, CURRENT_YEAR } from './helpers';

let entrepreneurUrl: string;

test.beforeAll(async ({ browser }) => {
  const page = await browser.newPage();
  await login(page);
  entrepreneurUrl = await createEntrepreneur(page);
  await page.close();
});

test.beforeEach(async ({ page }) => {
  await login(page);
  await page.goto(entrepreneurUrl);
});

// ── 1. Current-year KPO is auto-created ────────────────────────────────────
test('shows KPO section for the current year by default', async ({ page }) => {
  await expect(page.getByRole('heading', { name: 'КПО — Књига прихода' })).toBeVisible();
  // Year is shown in a <select>, not anchor tabs.
  const yearSelect = page.locator('#year-select');
  await expect(yearSelect).toBeVisible();
  await expect(yearSelect).toHaveValue(String(CURRENT_YEAR));
});

// ── 2. Adding an entry ──────────────────────────────────────────────────────
test('can add a KPO entry', async ({ page }) => {
  await page.fill('#kpo-date', '15.03');
  await page.fill('#kpo-invoice', '2026/1');
  await page.fill('#kpo-product', '15000.00');
  await page.fill('#kpo-service', '5000.00');
  await page.click('button[form="kpo-new"]');
  await page.waitForURL(/entrepreneurs/);

  await expect(page.getByRole('cell', { name: '2026/1' })).toBeVisible();
  await expect(page.getByRole('cell', { name: '15 000' }).first()).toBeVisible();
  await expect(page.getByRole('cell', { name: '5 000' }).first()).toBeVisible();
  await expect(page.getByRole('cell', { name: '20 000' }).first()).toBeVisible();
  await expect(page.getByRole('cell', { name: '1' }).first()).toBeVisible();
});

// ── 3. Inline total updates as you type ────────────────────────────────────
test('live total updates in the new-entry row', async ({ page }) => {
  await page.fill('#kpo-product', '3000');
  await page.fill('#kpo-service', '2000');
  await expect(page.locator('#kpo-row-total')).toHaveText('5000.00');
});

// ── 4. Enter key submits the new entry ─────────────────────────────────────
test('pressing Enter on service revenue submits the new entry', async ({ page }) => {
  await page.fill('#kpo-date', '01.04');
  await page.fill('#kpo-invoice', 'Enter-test');
  await page.fill('#kpo-product', '1000');
  await page.locator('#kpo-service').fill('500');
  await page.locator('#kpo-service').press('Enter');
  await page.waitForURL(/entrepreneurs/);
  await expect(page.getByRole('cell', { name: 'Enter-test' })).toBeVisible();
});

// ── 5. Deleting an entry ────────────────────────────────────────────────────
test('can delete a KPO entry', async ({ page }) => {
  await page.fill('#kpo-date', '01.05');
  await page.fill('#kpo-invoice', 'To-delete');
  await page.fill('#kpo-product', '100');
  await page.fill('#kpo-service', '0');
  await page.click('button[form="kpo-new"]');
  await page.waitForURL(/entrepreneurs/);
  await expect(page.getByRole('cell', { name: 'To-delete' })).toBeVisible();

  page.on('dialog', d => d.accept());
  await page.getByRole('row', { name: /To-delete/ }).getByRole('button', { name: /Обриши/ }).click();
  await page.waitForURL(/entrepreneurs/);
  await expect(page.getByRole('cell', { name: 'To-delete' })).not.toBeVisible();
});

// ── 6. Totals row shows column sums ────────────────────────────────────────
test('totals row appears after entries are added', async ({ page }) => {
  await page.fill('#kpo-date', '01.06');
  await page.fill('#kpo-invoice', 'Sum-test');
  await page.fill('#kpo-product', '1000');
  await page.fill('#kpo-service', '500');
  await page.click('button[form="kpo-new"]');
  await page.waitForURL(/entrepreneurs/);

  await expect(page.getByRole('row').filter({ hasText: 'Укупно:' })).toBeVisible();
});

// ── 7. Finalize a year (isolated entrepreneur) ─────────────────────────────
test('can finalize (укњижити) a year', async ({ page }) => {
  const url = await createEntrepreneur(page, 'Финал Тест', uniquePib());
  await page.goto(url);

  page.on('dialog', d => d.accept());
  await page.click('#btn-kpo-finalize');
  await page.waitForURL(/entrepreneurs/);

  await expect(page.getByText(/Укњижено/)).toBeVisible();
  await expect(page.locator('#kpo-new-row')).not.toBeVisible();
  await expect(page.locator('#btn-kpo-unlock')).toBeVisible();
  await expect(page.locator('#btn-kpo-finalize')).not.toBeVisible();
});

// ── 8. Unfinalize a year (isolated entrepreneur) ───────────────────────────
test('can unfinalize (откључати) a finalized year', async ({ page }) => {
  const url = await createEntrepreneur(page, 'Откључај Тест', uniquePib());
  await page.goto(url);

  page.on('dialog', d => d.accept());
  await page.click('#btn-kpo-finalize');
  await page.waitForURL(/entrepreneurs/);
  await expect(page.locator('#btn-kpo-unlock')).toBeVisible();

  await page.click('#btn-kpo-unlock');
  await page.waitForURL(/entrepreneurs/);

  await expect(page.locator('#btn-kpo-finalize')).toBeVisible();
  await expect(page.locator('#kpo-new-row')).toBeVisible();
});

// ── 9. Add and navigate to a previous year ─────────────────────────────────
test('can open a KPO for a previous year', async ({ page }) => {
  const prevYear = CURRENT_YEAR - 1;
  // Reveal the hidden year form first.
  await page.click('#btn-toggle-add-year');
  await page.fill('input[name="year"]', String(prevYear));
  await page.click('#btn-open-year');
  await page.waitForURL(new RegExp(`year=${prevYear}`));

  // Year select should show the previous year as selected.
  await expect(page.locator('#year-select')).toHaveValue(String(prevYear));
  await expect(page.locator('#kpo-new-row')).toBeVisible();
});

// ── 10. Multiple years show as tabs ────────────────────────────────────────
test('all opened years appear as tabs', async ({ page }) => {
  for (const y of [CURRENT_YEAR - 2, CURRENT_YEAR - 3]) {
    await page.click('#btn-toggle-add-year');
    await page.fill('input[name="year"]', String(y));
    await page.click('#btn-open-year');
    await page.waitForURL(new RegExp(`year=${y}`));
  }
  await page.goto(entrepreneurUrl);
  // At least 3 years should be available in the year select.
  const options = page.locator('#year-select option');
  expect(await options.count()).toBeGreaterThanOrEqual(3);
});

// ── 11. Description field saves and is shown in the table ─────────────────
test('KPO entry description is saved and shown in the table', async ({ page }) => {
  await page.fill('#kpo-date', '20.08');
  await page.fill('#kpo-invoice', 'Desc-test/1');
  await page.fill('#kpo-description', 'Test client name');
  await page.fill('#kpo-product', '0');
  await page.fill('#kpo-service', '2500.00');
  await page.click('button[form="kpo-new"]');
  await page.waitForURL(/entrepreneurs/);
  await expect(page.getByRole('cell', { name: 'Desc-test/1' })).toBeVisible();
  await expect(page.getByRole('cell', { name: 'Test client name' })).toBeVisible();
});

// ── 12. Keyboard tab navigation through the non-date entry fields ──────────
test('tab key moves focus through KPO numeric entry fields', async ({ page }) => {
  const freshYear = CURRENT_YEAR - 10;
  await page.goto(`${entrepreneurUrl}?year=${freshYear}`);
  await page.waitForURL(new RegExp(`year=${freshYear}`));

  await page.locator('#kpo-invoice').focus();
  await expect(page.locator('#kpo-invoice')).toBeFocused();

  await page.keyboard.press('Tab');
  await expect(page.locator('#kpo-description')).toBeFocused();

  await page.keyboard.press('Tab');
  await expect(page.locator('#kpo-product')).toBeFocused();

  await page.keyboard.press('Tab');
  await expect(page.locator('#kpo-service')).toBeFocused();
});
