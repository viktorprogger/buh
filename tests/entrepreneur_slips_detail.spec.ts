import { test, expect } from '@playwright/test';
import { loginEntrepreneur, registerEntrepreneur, createEntrepreneurSlip, CURRENT_YEAR } from './helpers';

const email = `slips-detail-${Date.now()}@test.local`;
const password = 'slips-detail-pass-123';

test.beforeAll(async ({ browser }) => {
  const page = await browser.newPage();
  await registerEntrepreneur(page, email, password);
  await page.close();
});

test.beforeEach(async ({ page }) => {
  await loginEntrepreneur(page, email, password);
});

test.describe('slip detail', () => {
  let slipPath: string;

  test.beforeAll(async ({ browser }) => {
    const page = await browser.newPage();
    await loginEntrepreneur(page, email, password);
    slipPath = await createEntrepreneurSlip(page, 'Детаљ тест ' + Date.now());
    await page.close();
  });

  test('navigating to slip shows saved field values', async ({ page }) => {
    await page.goto(slipPath);
    await expect(page.locator('input[name="S"]')).not.toHaveValue('');
    await expect(page.locator('input[name="SF"]')).toHaveValue('253');
    await expect(page.locator('input[name="year"]')).toHaveValue(String(CURRENT_YEAR));
  });

  test('edit and save persists changes', async ({ page }) => {
    await page.goto(slipPath);
    await page.locator('input[name="amount"]').fill('9999');
    await page.click('#btn-save-slip-detail');
    await expect(page.getByText('Уплатница је сачувана.')).toBeVisible();
    await page.goto(slipPath);
    await expect(page.locator('input[name="amount"]')).toHaveValue('9999.00');
  });

  test('download button triggers PDF download', async ({ page }) => {
    await page.goto(slipPath);
    const downloadPromise = page.waitForEvent('download');
    await page.click('#btn-download-slip');
    const download = await downloadPromise;
    expect(download.suggestedFilename()).toMatch(/\.pdf$/);
  });

  test('delete removes slip from list', async ({ page }) => {
    const deleteTestPurpose = 'Обриши тест ' + Date.now();
    const deletePath = await createEntrepreneurSlip(page, deleteTestPurpose);
    await page.goto(deletePath);
    page.on('dialog', d => d.accept());
    await page.click('#btn-delete-slip');
    await page.waitForURL('/e/slips');
    await expect(page.getByText(deleteTestPurpose)).not.toBeVisible();
  });
});
