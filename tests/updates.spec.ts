import { test, expect, Page } from '@playwright/test';
import { loginAccountant, loginEntrepreneur, registerEntrepreneur } from './helpers';

const UPDATE_TITLE_SR = 'Тестовое обновление';
const UPDATE_TITLE_EN = 'Test update';
const UPDATE_TITLE_RU = 'Тестовое обновление';

let accountantPage: Page;

// The global setup inserts an update after the test accountant exists and
// clears its read marker. This keeps the feed/read-state checks deterministic.
test.describe.configure({ mode: 'serial' });

test.beforeAll(async ({ browser }) => {
  accountantPage = await browser.newPage();
  await loginAccountant(accountantPage);
});

test.afterAll(async () => {
  await accountantPage.close();
});

test('updates link is visible above logout and new items are prominent', async () => {
  await accountantPage.locator('form[action="/language"] button[name="lang"][value="sr"]').click();
  await accountantPage.waitForURL('**/a/');
  const link = accountantPage.locator('a[href="/updates"]');
  await expect(link).toContainText('Ажурирања');
  await expect(link).toHaveAttribute('href', '/updates');
  await expect(accountantPage.locator('a[href="/logout"]')).toBeVisible();

  await accountantPage.goto('/updates');
  const item = accountantPage.getByTestId('update-item').filter({ hasText: UPDATE_TITLE_SR });
  await expect(item).toBeVisible();
  await expect(item.locator('p')).toHaveText('Проверка списка обновлений.');
  await expect(item).toHaveAttribute('data-read', 'false');
});

test('viewed updates remain visible but become subdued', async () => {
  await accountantPage.goto('/updates');
  const item = accountantPage.getByTestId('update-item').filter({ hasText: UPDATE_TITLE_SR });
  await expect(item).toBeVisible();
  await expect(item).toHaveAttribute('data-read', 'true');
  await expect(item).toHaveClass(/opacity-70/);
});

test('updates are localized in English and Russian', async () => {
  await accountantPage.goto('/updates');
  await accountantPage.locator('form[action="/language"] button[name="lang"][value="en"]').click();
  await accountantPage.waitForURL('**/updates');
  await expect(accountantPage.getByRole('heading', { name: 'Updates' })).toBeVisible();
  await expect(accountantPage.getByTestId('update-item').filter({ hasText: UPDATE_TITLE_EN })).toBeVisible();

  await accountantPage.locator('form[action="/language"] button[name="lang"][value="ru"]').click();
  await accountantPage.waitForURL('**/updates');
  await expect(accountantPage.getByRole('heading', { name: 'Обновления' })).toBeVisible();
  await expect(accountantPage.getByTestId('update-item').filter({ hasText: UPDATE_TITLE_RU })).toBeVisible();

  // Keep the shared test accountant's locale stable for the remaining suite.
  await accountantPage.locator('form[action="/language"] button[name="lang"][value="sr"]').click();
  await accountantPage.waitForURL('**/updates');
});

test('a newly registered entrepreneur does not see pre-registration updates', async ({ browser }) => {
  const page = await browser.newPage();
  const email = `updates-cutoff-${Date.now()}@test.local`;
  await registerEntrepreneur(page, email, 'updates-cutoff-pass-123');
  await loginEntrepreneur(page, email, 'updates-cutoff-pass-123');
  await expect(page.locator('aside a[href="/updates"]')).toBeVisible();
  await page.goto('/updates');
  await expect(page.getByText('Нема нових ажурирања.')).toBeVisible();
  await page.close();
});
