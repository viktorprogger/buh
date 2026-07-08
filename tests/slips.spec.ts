import { test, expect } from '@playwright/test';
import { loginAccountant, createEntrepreneur, uniquePib } from './helpers';

test.describe('slip detail', () => {
  let entPath: string;
  let slipPath: string;

  test.beforeAll(async ({ browser }) => {
    const page = await browser.newPage();
    await loginAccountant(page);
    const entUrl = await createEntrepreneur(page, 'Слип Тест ' + Date.now(), uniquePib());
    entPath = new URL(entUrl).pathname;

    await page.goto(entPath + '/slips/new');
    await page.fill('input[name="SF"]', '253');
    await page.fill('input[name="S"]', 'Паушални порез');
    await page.fill('input[name="N"]', 'Пореска управа');
    await page.fill('input[name="R"]', '840-3553531843-20');
    await page.fill('input[name="amount"]', '5000');
    await page.click('#btn-save-slip-acc');
    await page.waitForURL(/\/a\/slips\//);
    slipPath = new URL(page.url()).pathname;
    await page.close();
  });

  test.beforeEach(async ({ page }) => {
    await loginAccountant(page);
  });

  test('can delete a slip from the detail page', async ({ page }) => {
    await page.goto(slipPath);
    page.on('dialog', dialog => dialog.accept());
    await page.click('#btn-delete-slip');
    await page.waitForURL(new RegExp(entPath));
  });
});
