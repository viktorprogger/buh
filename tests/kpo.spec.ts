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
  const yearTab = page.locator(`a[href*="year=${CURRENT_YEAR}"]`).first();
  await expect(yearTab).toBeVisible();
  await expect(yearTab).toHaveClass(/text-primary|border-primary/);
});

// ── 2. Adding an entry ──────────────────────────────────────────────────────
test('can add a KPO entry', async ({ page }) => {
  await page.fill('#kpo-date', `${CURRENT_YEAR}-03-15`);
  await page.fill('#kpo-invoice', '2026/1');
  await page.fill('#kpo-product', '15000.00');
  await page.fill('#kpo-service', '5000.00');
  await page.click('button[form="kpo-new"]');
  await page.waitForURL(/entrepreneurs/);

  await expect(page.getByRole('cell', { name: '2026/1' })).toBeVisible();
  await expect(page.getByRole('cell', { name: '15000.00' }).first()).toBeVisible();
  await expect(page.getByRole('cell', { name: '5000.00' }).first()).toBeVisible();
  await expect(page.getByRole('cell', { name: '20000.00' }).first()).toBeVisible();
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
  await page.fill('#kpo-date', `${CURRENT_YEAR}-04-01`);
  await page.fill('#kpo-invoice', 'Enter-test');
  await page.fill('#kpo-product', '1000');
  await page.locator('#kpo-service').fill('500');
  await page.locator('#kpo-service').press('Enter');
  await page.waitForURL(/entrepreneurs/);
  await expect(page.getByRole('cell', { name: 'Enter-test' })).toBeVisible();
});

// ── 5. Deleting an entry ────────────────────────────────────────────────────
test('can delete a KPO entry', async ({ page }) => {
  await page.fill('#kpo-date', `${CURRENT_YEAR}-05-01`);
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
  await page.fill('#kpo-date', `${CURRENT_YEAR}-06-01`);
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

  await page.getByRole('button', { name: 'Укњижи' }).click();
  await page.waitForURL(/entrepreneurs/);

  await expect(page.getByText(/Укњижено/)).toBeVisible();
  await expect(page.locator('#kpo-new-row')).not.toBeVisible();
  await expect(page.getByRole('button', { name: 'Откључај' })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Укњижи' })).not.toBeVisible();
});

// ── 8. Unfinalize a year (isolated entrepreneur) ───────────────────────────
test('can unfinalize (откључати) a finalized year', async ({ page }) => {
  const url = await createEntrepreneur(page, 'Откључај Тест', uniquePib());
  await page.goto(url);

  await page.getByRole('button', { name: 'Укњижи' }).click();
  await page.waitForURL(/entrepreneurs/);
  await expect(page.getByRole('button', { name: 'Откључај' })).toBeVisible();

  await page.getByRole('button', { name: 'Откључај' }).click();
  await page.waitForURL(/entrepreneurs/);

  await expect(page.getByRole('button', { name: 'Укњижи' })).toBeVisible();
  await expect(page.locator('#kpo-new-row')).toBeVisible();
});

// ── 9. Add and navigate to a previous year ─────────────────────────────────
test('can open a KPO for a previous year', async ({ page }) => {
  const prevYear = CURRENT_YEAR - 1;
  await page.fill('input[name="year"]', String(prevYear));
  await page.getByRole('button', { name: 'Отвори годину' }).click();
  await page.waitForURL(new RegExp(`year=${prevYear}`));

  const prevTab = page.locator(`a[href*="year=${prevYear}"]`).first();
  await expect(prevTab).toHaveClass(/text-primary|border-primary/);
  await expect(page.locator('#kpo-new-row')).toBeVisible();
});

// ── 10. Multiple years show as tabs ────────────────────────────────────────
test('all opened years appear as tabs', async ({ page }) => {
  for (const y of [CURRENT_YEAR - 2, CURRENT_YEAR - 3]) {
    await page.fill('input[name="year"]', String(y));
    await page.getByRole('button', { name: 'Отвори годину' }).click();
    await page.waitForURL(new RegExp(`year=${y}`));
  }
  await page.goto(entrepreneurUrl);
  const tabs = page.locator('a[href*="?year="]');
  expect(await tabs.count()).toBeGreaterThanOrEqual(3);
});

// ── 11. Keyboard tab navigation through the non-date entry fields ──────────
// Note: type="date" in Chromium has internal sub-parts (day/month/year) that
// consume Tab presses before leaving the input. We start from #kpo-invoice to
// test the tab flow that actually matters for accountant keyboard entry.
test('tab key moves focus through KPO numeric entry fields', async ({ page }) => {
  const freshYear = CURRENT_YEAR - 10;
  await page.goto(`${entrepreneurUrl}?year=${freshYear}`);
  await page.waitForURL(new RegExp(`year=${freshYear}`));

  await page.locator('#kpo-invoice').focus();
  await expect(page.locator('#kpo-invoice')).toBeFocused();

  await page.keyboard.press('Tab');
  await expect(page.locator('#kpo-product')).toBeFocused();

  await page.keyboard.press('Tab');
  await expect(page.locator('#kpo-service')).toBeFocused();
});
