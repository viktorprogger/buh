import { test, expect } from '@playwright/test';
import {
  registerEntrepreneur, loginEntrepreneur, uniquePib, uniqueMb, CURRENT_YEAR,
  createLocalClient, createForeignClient,
} from './helpers';

const email = `inv-${Date.now()}@test.local`;
const password = 'inv-test-pass-123';

test.beforeAll(async ({ browser }) => {
  const page = await browser.newPage();
  await registerEntrepreneur(page, email, password);
  await page.close();
});

test.beforeEach(async ({ page }) => {
  await loginEntrepreneur(page, email, password);
  await page.goto('/e/');
});

// ── Invoice form ──────────────────────────────────────────────────────────────

test('new invoice form is accessible', async ({ page }) => {
  await page.goto('/e/invoices');
  await page.getByRole('link', { name: '+ Нова фактура' }).click();
  await expect(page.getByRole('heading', { name: 'Нова фактура' })).toBeVisible();
});

test('live preview updates when typing invoice number', async ({ page }) => {
  await page.goto('/e/invoices/new');
  await page.fill('#f-invnum', '5/2026');
  await expect(page.locator('#invoice-preview')).toContainText('5/2026');
});

test('live preview updates when changing invoice type', async ({ page }) => {
  await page.goto('/e/invoices/new');
  await page.locator('input[name="invoice_type"][value="advance"]').check();
  await expect(page.locator('#invoice-preview')).toContainText('AVANSNA FAKTURA');
  await page.locator('input[name="invoice_type"][value="standard"]').check();
  await expect(page.locator('#invoice-preview')).toContainText('FAKTURA');
});

test('grand total updates when adding line items', async ({ page }) => {
  await page.goto('/e/invoices/new');
  await page.locator('[name="item_unit_price[]"]').first().fill('1000');
  await page.locator('[name="item_quantity[]"]').first().fill('3');
  await expect(page.locator('#grand-total-label')).toHaveText('3000,00');
});

test('can add and remove line item rows', async ({ page }) => {
  await page.goto('/e/invoices/new');
  await expect(page.locator('.item-row')).toHaveCount(1);
  await page.getByText('+ Додај ставку').click();
  await expect(page.locator('.item-row')).toHaveCount(2);
  await page.locator('.item-row').last().getByRole('button').click();
  await expect(page.locator('.item-row')).toHaveCount(1);
});

// ── Client autocomplete ───────────────────────────────────────────────────────

test('client autocomplete shows matching results', async ({ page }) => {
  const clientName = 'Аутоком клијент ' + Date.now();
  await createLocalClient(page, clientName);

  await page.goto('/e/invoices/new');
  await page.fill('#client-search', 'Аутоком');
  await expect(page.locator('#client-dropdown li').first()).toBeVisible({ timeout: 3000 });
  await expect(page.locator('#client-dropdown li').first()).toContainText(clientName);
});

test('saving a client from invoice creation returns to invoice page', async ({ page }) => {
  await page.goto('/e/invoices/new');
  await page.fill('#client-search', 'НеПостоји');
  await expect(page.locator('#client-dropdown')).toBeVisible({ timeout: 3000 });
  await page.locator('#client-dropdown a', { hasText: '+ Додај клијента' }).click();
  await page.waitForURL('/e/clients/new?return_to=/e/invoices/new');
  await page.fill('input[name="name"]', 'Клијент из фактуре ' + Date.now());
  await page.fill('input[name="pib"]', uniquePib());
  await page.fill('input[name="registration_number"]', uniqueMb());
  await page.click('#btn-save-client');
  await page.waitForURL('/e/invoices/new');
});

// ── Standard invoice create → KPO auto-entry ─────────────────────────────────

test('creating a standard invoice auto-adds a KPO entry', async ({ page }) => {
  await page.goto('/e/invoices/new');
  await page.fill('#f-invnum', 'AUTO/2026');
  await page.fill('#f-issue', `${CURRENT_YEAR}-06-15`);
  await page.locator('[name="item_description[]"]').fill('IT услуге');
  await page.locator('[name="item_unit_price[]"]').fill('50000');
  await page.locator('[name="item_quantity[]"]').fill('1');
  await page.click('#btn-create-invoice');
  await page.waitForURL(/\/e\/invoices\/[0-9a-f-]+$/);

  await page.goto('/e/kpo/' + CURRENT_YEAR);
  await expect(page.getByRole('cell', { name: 'AUTO/2026' })).toBeVisible();
  await expect(page.getByRole('cell', { name: '50000.00' }).first()).toBeVisible();
});

// ── Advance invoice → muted in KPO ───────────────────────────────────────────

test('advance invoice appears muted in KPO section with tooltip', async ({ page }) => {
  await page.goto('/e/invoices/new');
  await page.locator('input[name="invoice_type"][value="advance"]').check();
  await page.fill('#f-invnum', 'AV/2026');
  await page.fill('#f-issue', `${CURRENT_YEAR}-06-20`);
  await page.locator('[name="item_description[]"]').fill('Аванс за пројекат');
  await page.locator('[name="item_unit_price[]"]').fill('20000');
  await page.click('#btn-create-invoice');
  await page.waitForURL(/\/e\/invoices\/[0-9a-f-]+$/);

  await page.goto('/e/kpo/' + CURRENT_YEAR);
  const advRow = page.locator('tr.opacity-50', { hasText: 'AV/2026' });
  await expect(advRow).toBeVisible();
  await expect(advRow).toHaveAttribute('title', 'Ова фактура се не узима у обзир у КПО');
});

// ── Invoice detail page ───────────────────────────────────────────────────────

test('invoice detail page shows correct data', async ({ page }) => {
  await page.goto('/e/invoices/new');
  await page.fill('#f-invnum', 'DETAIL/1');
  await page.fill('#f-issue', `${CURRENT_YEAR}-05-01`);
  await page.locator('[name="item_description[]"]').fill('Консалтинг');
  await page.locator('[name="item_unit_price[]"]').fill('30000');
  await page.fill('#f-notes', 'Тест напомена');
  await page.click('#btn-create-invoice');
  await page.waitForURL(/\/e\/invoices\/[0-9a-f-]+$/);

  await expect(page.getByText('DETAIL/1')).toBeVisible();
  await expect(page.getByRole('cell', { name: 'Консалтинг' })).toBeVisible();
  await expect(page.getByRole('cell', { name: '30000.00' }).first()).toBeVisible();
  await expect(page.getByText('Тест напомена')).toBeVisible();
  await expect(page.getByRole('link', { name: /Преузми PDF/ })).toBeVisible();
});

// ── Invoice detail: client identification labels ──────────────────────────────

test('invoice detail shows PIB and MB for local client', async ({ page }) => {
  const pib = uniquePib();
  const mb = uniqueMb();
  const name = 'Домаћи клијент ' + Date.now();

  await page.goto('/e/clients/new');
  await page.fill('input[name="name"]', name);
  await page.fill('input[name="pib"]', pib);
  await page.fill('#f-reg', mb);
  await page.click('#btn-save-client');
  await page.waitForURL('/e/clients');

  await page.goto('/e/invoices/new');
  await page.fill('#client-search', name);
  await expect(page.locator('#client-dropdown li').first()).toBeVisible({ timeout: 3000 });
  await page.locator('#client-dropdown li').first().click();

  await page.fill('#f-invnum', 'LOC/1');
  await page.fill('#f-issue', `${CURRENT_YEAR}-06-01`);
  await page.locator('[name="item_description[]"]').fill('Услуга');
  await page.locator('[name="item_unit_price[]"]').fill('1000');
  await page.click('#btn-create-invoice');
  await page.waitForURL(/\/e\/invoices\/[0-9a-f-]+$/);

  await expect(page.getByText(`PIB:`)).toBeVisible();
  await expect(page.getByText(pib, { exact: true })).toBeVisible();
  await expect(page.getByText(`MB:`)).toBeVisible();
  await expect(page.getByText(mb, { exact: true })).toBeVisible();
  await expect(page.getByText('Tax ID / Reg No.:')).not.toBeVisible();
});

test('invoice detail shows Tax ID label for foreign client', async ({ page }) => {
  const taxId = 'KZ-BIN-' + Date.now();
  const name = 'Страни клијент ' + Date.now();
  await createForeignClient(page, name, taxId);

  await page.goto('/e/invoices/new');
  await page.fill('#client-search', name);
  await expect(page.locator('#client-dropdown li').first()).toBeVisible({ timeout: 3000 });
  await page.locator('#client-dropdown li').first().click();

  await page.fill('#f-invnum', 'FOR/1');
  await page.fill('#f-issue', `${CURRENT_YEAR}-06-01`);
  await page.locator('[name="item_description[]"]').fill('Export service');
  await page.locator('[name="item_unit_price[]"]').fill('2000');
  await page.click('#btn-create-invoice');
  await page.waitForURL(/\/e\/invoices\/[0-9a-f-]+$/);

  await expect(page.getByText('Tax ID / Reg No.:')).toBeVisible();
  await expect(page.getByText(taxId)).toBeVisible();
  await expect(page.getByText('PIB:')).not.toBeVisible();
  await expect(page.getByText('MB:')).not.toBeVisible();
});

// ── Live preview: client identification labels ────────────────────────────────

test('live preview shows PIB and MB for local client', async ({ page }) => {
  const pib = uniquePib();
  const mb = uniqueMb();
  const name = 'Преглед домаћи ' + Date.now();

  await page.goto('/e/clients/new');
  await page.fill('input[name="name"]', name);
  await page.fill('input[name="pib"]', pib);
  await page.fill('#f-reg', mb);
  await page.click('#btn-save-client');
  await page.waitForURL('/e/clients');

  await page.goto('/e/invoices/new');
  await page.fill('#client-search', name);
  await expect(page.locator('#client-dropdown li').first()).toBeVisible({ timeout: 3000 });
  await page.locator('#client-dropdown li').first().click();

  const preview = page.locator('#invoice-preview');
  await expect(preview).toContainText('PIB:');
  await expect(preview).toContainText(pib);
  await expect(preview).toContainText('MB:');
  await expect(preview).toContainText(mb);
  await expect(preview).not.toContainText('Tax ID');
});

test('live preview shows Tax ID for foreign client', async ({ page }) => {
  const taxId = 'PREVIEW-TAX-' + Date.now();
  const name = 'Преглед страни ' + Date.now();
  await createForeignClient(page, name, taxId);

  await page.goto('/e/invoices/new');
  await page.fill('#client-search', name);
  await expect(page.locator('#client-dropdown li').first()).toBeVisible({ timeout: 3000 });
  await page.locator('#client-dropdown li').first().click();

  const preview = page.locator('#invoice-preview');
  await expect(preview).toContainText('Tax ID');
  await expect(preview).toContainText(taxId);
  await expect(preview).not.toContainText('PIB:');
});

// ── PDF download ──────────────────────────────────────────────────────────────

test('PDF download returns a PDF file', async ({ page }) => {
  await page.goto('/e/invoices/new');
  await page.fill('#f-invnum', 'PDF/1');
  await page.fill('#f-issue', `${CURRENT_YEAR}-04-01`);
  await page.locator('[name="item_description[]"]').fill('PDF услуга');
  await page.locator('[name="item_unit_price[]"]').fill('10000');
  await page.click('#btn-create-invoice');
  await page.waitForURL(/\/e\/invoices\/[0-9a-f-]+$/);

  const [download] = await Promise.all([
    page.waitForEvent('download'),
    page.getByRole('link', { name: /Преузми PDF/ }).click(),
  ]);
  expect(download.suggestedFilename()).toMatch(/faktura.*\.pdf/i);
});
