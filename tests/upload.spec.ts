import { test, expect } from '@playwright/test';
import path from 'path';
import { loginAccountant } from './helpers';

test.describe('upload решење', () => {
  test.beforeEach(async ({ page }) => {
    await loginAccountant(page);
  });

  test('accountant can upload a решење PDF', async ({ page }) => {
    await page.goto('/a/');

    const fileInput = page.locator('input[name="pdfs"]');
    await fileInput.setInputFiles(path.join(__dirname, 'fixtures', 'resenye.pdf'));

    await page.click('#btn-process-upload');

    await page.waitForURL(/\/a\/import\/batches\//);
    await expect(page.locator('h1')).toBeVisible();
  });
});
