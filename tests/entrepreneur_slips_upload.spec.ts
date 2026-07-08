import { test, expect } from '@playwright/test';
import path from 'path';
import { loginAccountant, loginEntrepreneur, registerEntrepreneur, createEntrepreneur, pairEntrepreneur, uniquePib } from './helpers';

const noPibEmail = `upload-nopib-${Date.now()}@test.local`;
const noPibPassword = 'upload-nopib-pass';

test.describe('PDF upload — PIB missing', () => {
  test.beforeAll(async ({ browser }) => {
    const page = await browser.newPage();
    await registerEntrepreneur(page, noPibEmail, noPibPassword);
    await page.close();
  });

  test.beforeEach(async ({ page }) => {
    await loginEntrepreneur(page, noPibEmail, noPibPassword);
  });

  test('upload page shows PIB warning, no upload form', async ({ page }) => {
    await page.goto('/e/slips/upload');
    await expect(page.getByText('ПИБ', { exact: false })).toBeVisible();
    await expect(page.locator('input[type="file"]')).not.toBeVisible();
  });
});

const pibEmail = `upload-pib-${Date.now()}@test.local`;
const pibPassword = 'upload-pib-pass';

test.describe('PDF upload — PIB present', () => {
  test.beforeAll(async ({ browser }) => {
    const accPage = await browser.newPage();
    await loginAccountant(accPage);
    const entUrl = await createEntrepreneur(accPage, 'Upload Test', '112750251');
    const entId = new URL(entUrl).pathname.split('/').pop()!;

    // Unpair any previously linked entrepreneur user so we can pair the current test user.
    await accPage.goto(`/a/entrepreneurs/${entId}`);
    const unpairFormVisible = await accPage.locator('form[action$="/unpair"]').isVisible();
    if (unpairFormVisible) {
      // Submit the unpair form directly, bypassing the confirm() dialog.
      await accPage.evaluate(() => {
        const form = document.querySelector('form[action$="/unpair"]') as HTMLFormElement;
        if (form) form.submit();
      });
      await accPage.waitForURL(/unpaired=1/);
    }

    const entPage = await browser.newPage();
    await registerEntrepreneur(entPage, pibEmail, pibPassword);

    await pairEntrepreneur(accPage, entPage, entId);

    await accPage.close();
    await entPage.close();
  });

  test.beforeEach(async ({ page }) => {
    await loginEntrepreneur(page, pibEmail, pibPassword);
  });

  test('upload page shows file input when PIB is set', async ({ page }) => {
    await page.goto('/e/slips/upload');
    await expect(page.locator('input[type="file"]')).toBeVisible();
    await expect(page.getByText('ПИБ', { exact: false })).not.toBeVisible();
  });

  test('uploading valid PDF imports slips and shows results table', async ({ page }) => {
    await page.goto('/e/slips/upload');
    await page.locator('input[type="file"]').setInputFiles(path.join(__dirname, 'fixtures', 'resenye.pdf'));
    await page.click('#btn-import-slips');
    await expect(page.locator('.bg-green-50')).toBeVisible();
    await expect(page.locator('table tbody tr').first()).toBeVisible();
    await page.goto('/e/slips');
    await expect(page.locator('table tbody tr').first()).toBeVisible();
  });

  test('uploading non-PDF shows error', async ({ page }) => {
    await page.goto('/e/slips/upload');
    await page.locator('input[type="file"]').setInputFiles({
      name: 'test.txt',
      mimeType: 'text/plain',
      buffer: Buffer.from('not a pdf')
    });
    await page.click('#btn-import-slips');
    await expect(page.locator('.bg-red-50')).toBeVisible();
  });
});
