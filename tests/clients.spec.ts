import { test, expect } from '@playwright/test';
import {
  registerEntrepreneur, loginEntrepreneur, uniquePib, uniqueMb,
  createLocalClient, createForeignClient,
} from './helpers';

const email = `clients-${Date.now()}@test.local`;
const password = 'clients-test-pass-123';

test.beforeAll(async ({ browser }) => {
  const page = await browser.newPage();
  await registerEntrepreneur(page, email, password);
  await page.close();
});

test.beforeEach(async ({ page }) => {
  await loginEntrepreneur(page, email, password);
});

// ── Local client CRUD ─────────────────────────────────────────────────────────

test('can create a local client', async ({ page }) => {
  const name = 'Нови клијент д.о.о. ' + Date.now();
  await createLocalClient(page, name);
  await expect(page.getByRole('cell', { name })).toBeVisible();
});

test('can edit a client', async ({ page }) => {
  const name = 'Клијент за измену ' + Date.now();
  await createLocalClient(page, name);
  await page.getByRole('row', { name: new RegExp(name) }).getByRole('link', { name: 'Измени' }).click();
  await page.fill('input[name="name"]', name + ' — измењен');
  await page.click('#btn-save-client');
  await page.waitForURL('/e/clients');
  await expect(page.getByRole('cell', { name: name + ' — измењен' })).toBeVisible();
});

test('can delete a client', async ({ page }) => {
  const name = 'Клијент за брисање ' + Date.now();
  await createLocalClient(page, name);
  await expect(page.getByRole('cell', { name })).toBeVisible();
  page.on('dialog', d => d.accept());
  await page.getByRole('row', { name: new RegExp(name) }).getByRole('button', { name: 'Обриши' }).click();
  await page.waitForURL('/e/clients');
  await expect(page.getByRole('cell', { name })).not.toBeVisible();
});

// ── Local client validation ───────────────────────────────────────────────────

test('client name is required', async ({ page }) => {
  await page.goto('/e/clients/new');
  await page.click('#btn-save-client');
  await expect(page.getByText('Назив клијента је обавезан.')).toBeVisible();
});

test('local client requires PIB', async ({ page }) => {
  await page.goto('/e/clients/new');
  await page.fill('input[name="name"]', 'Фирма д.о.о.');
  // MB filled, PIB intentionally left blank
  await page.fill('input[name="registration_number"]', uniqueMb());
  await page.click('#btn-save-client');
  await expect(page.getByText('ПИБ је обавезан за домаће клијенте.')).toBeVisible();
});

test('local client requires MB', async ({ page }) => {
  await page.goto('/e/clients/new');
  await page.fill('input[name="name"]', 'Фирма д.о.о.');
  await page.fill('input[name="pib"]', uniquePib());
  // registration_number intentionally left blank
  await page.click('#btn-save-client');
  await expect(page.getByText('Матични број је обавезан за домаће клијенте.')).toBeVisible();
});

// ── Local client UX hints ─────────────────────────────────────────────────────

test('APR name hint is visible for local client by default', async ({ page }) => {
  await page.goto('/e/clients/new');
  await expect(page.locator('#hint-name-local')).toBeVisible();
});

test('APR name hint hides when switching to foreign client', async ({ page }) => {
  await page.goto('/e/clients/new');
  await page.check('input[name="is_foreign"]');
  await expect(page.locator('#hint-name-local')).toBeHidden();
});

test('PIB and MB fields are visible for local client by default', async ({ page }) => {
  await page.goto('/e/clients/new');
  await expect(page.locator('#local-fields')).toBeVisible();
  await expect(page.locator('#foreign-fields')).toBeHidden();
});

// ── Foreign client CRUD ───────────────────────────────────────────────────────

test('can create a foreign client', async ({ page }) => {
  const name = 'TOO Тест Казахстан ' + Date.now();
  await createForeignClient(page, name);
  await expect(page.getByRole('cell', { name })).toBeVisible();
});

// ── Foreign client validation ─────────────────────────────────────────────────

test('foreign client requires Tax ID', async ({ page }) => {
  await page.goto('/e/clients/new');
  await page.fill('input[name="name"]', 'Foreign Co Ltd');
  await page.check('input[name="is_foreign"]');
  // registration_number intentionally left blank
  await page.click('#btn-save-client');
  await expect(page.getByText('Порески / регистрациони број је обавезан за стране клијенте.')).toBeVisible();
});

// ── Foreign client UX hints ───────────────────────────────────────────────────

test('Tax ID field and address hint appear when switching to foreign client', async ({ page }) => {
  await page.goto('/e/clients/new');
  await page.check('input[name="is_foreign"]');
  await expect(page.locator('#foreign-fields')).toBeVisible();
  await expect(page.locator('#local-fields')).toBeHidden();
  await expect(page.locator('#hint-addr-foreign')).toBeVisible();
});

test('address hint hides when switching back to local client', async ({ page }) => {
  await page.goto('/e/clients/new');
  await page.check('input[name="is_foreign"]');
  await page.uncheck('input[name="is_foreign"]');
  await expect(page.locator('#hint-addr-foreign')).toBeHidden();
  await expect(page.locator('#local-fields')).toBeVisible();
});
