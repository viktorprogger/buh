import { test, expect, Page } from '@playwright/test';
import { login, createEntrepreneur, uniquePib, CURRENT_YEAR } from './helpers';

let entrepreneurUrl: string;

test.beforeAll(async ({ browser }) => {
  const page = await browser.newPage();
  await login(page);
  entrepreneurUrl = await createEntrepreneur(page, 'Фактура Тест', uniquePib());
  await page.close();
});

test.beforeEach(async ({ page }) => {
  await login(page);
  await page.goto(entrepreneurUrl);
});

// ── Settings page ─────────────────────────────────────────────────────────────

test('settings link is visible on entrepreneur page', async ({ page }) => {
  await expect(page.getByRole('link', { name: 'Подешавања' })).toBeVisible();
});

test('can open settings page', async ({ page }) => {
  await page.getByRole('link', { name: 'Подешавања' }).click();
  await expect(page.getByRole('heading', { name: 'Профил предузетника' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Клијенти' })).toBeVisible();
});

test('can save address and bank account in settings', async ({ page }) => {
  await page.goto(entrepreneurUrl + '/settings');
  await page.fill('input[name="address"]', 'Нушићева 11, Београд');
  await page.fill('input[name="bank_account"]', '265-123456789-56');
  await page.getByRole('button', { name: 'Сачувај' }).click();
  await page.waitForURL(/settings\?saved=1/);
  await expect(page.getByText('Подешавања су сачувана.')).toBeVisible();
  await expect(page.locator('input[name="address"]')).toHaveValue('Нушићева 11, Београд');
  await expect(page.locator('input[name="bank_account"]')).toHaveValue('265-123456789-56');
});

// ── Client CRUD ───────────────────────────────────────────────────────────────

async function createClient(page: Page, entrepreneurUrl: string, name = 'Тест клијент д.о.о.') {
  await page.goto(entrepreneurUrl + '/clients/new');
  await page.fill('input[name="name"]', name);
  await page.fill('input[name="pib"]', uniquePib());
  await page.fill('input[name="email"]', 'klijent@test.rs');
  await page.getByRole('button', { name: 'Додај клијента' }).click();
  await page.waitForURL(/settings$/);
}

test('can create a client', async ({ page }) => {
  await createClient(page, entrepreneurUrl, 'Нови клијент д.о.о.');
  await expect(page.getByRole('cell', { name: 'Нови клијент д.о.о.' })).toBeVisible();
});

test('client name is required', async ({ page }) => {
  await page.goto(entrepreneurUrl + '/clients/new');
  await page.getByRole('button', { name: 'Додај клијента' }).click();
  await expect(page.getByText('Назив клијента је обавезан.')).toBeVisible();
});

test('can edit a client', async ({ page }) => {
  const clientName = 'Клијент за измену ' + Date.now();
  await createClient(page, entrepreneurUrl, clientName);
  await page.getByRole('row', { name: new RegExp(clientName) }).getByRole('link', { name: 'Измени' }).click();
  await page.fill('input[name="name"]', clientName + ' — измењен');
  await page.getByRole('button', { name: 'Сачувај' }).click();
  await page.waitForURL(/settings$/);
  await expect(page.getByRole('cell', { name: clientName + ' — измењен' })).toBeVisible();
});

test('can delete a client', async ({ page }) => {
  const clientName = 'Клијент за брисање ' + Date.now();
  await createClient(page, entrepreneurUrl, clientName);
  await expect(page.getByRole('cell', { name: clientName })).toBeVisible();
  page.on('dialog', d => d.accept());
  await page.getByRole('row', { name: new RegExp(clientName) }).getByRole('button', { name: 'Обриши' }).click();
  await page.waitForURL(/settings$/);
  await expect(page.getByRole('cell', { name: clientName })).not.toBeVisible();
});

// ── Invoice form ──────────────────────────────────────────────────────────────

test('invoice creation form is accessible via Нова фактура button', async ({ page }) => {
  await page.getByRole('link', { name: '+ Нова фактура' }).click();
  await expect(page.getByRole('heading', { name: 'Нова фактура' })).toBeVisible();
});

test('live preview updates when typing invoice number', async ({ page }) => {
  await page.goto(entrepreneurUrl + '/invoices/new');
  await page.fill('#f-invnum', '5/2026');
  await expect(page.locator('#invoice-preview')).toContainText('5/2026');
});

test('live preview updates when changing invoice type', async ({ page }) => {
  await page.goto(entrepreneurUrl + '/invoices/new');
  await page.locator('input[name="invoice_type"][value="advance"]').check();
  await expect(page.locator('#invoice-preview')).toContainText('АВАНСНА ФАКТУРА');
  await page.locator('input[name="invoice_type"][value="standard"]').check();
  await expect(page.locator('#invoice-preview')).toContainText('ФАКТУРА');
});

test('grand total updates when adding line items', async ({ page }) => {
  await page.goto(entrepreneurUrl + '/invoices/new');
  // First item is auto-added; fill it in.
  await page.locator('[name="item_unit_price[]"]').first().fill('1000');
  await page.locator('[name="item_quantity[]"]').first().fill('3');
  await expect(page.locator('#grand-total-label')).toHaveText('3000,00');
});

test('can add and remove line item rows', async ({ page }) => {
  await page.goto(entrepreneurUrl + '/invoices/new');
  // One row already present.
  await expect(page.locator('.item-row')).toHaveCount(1);
  await page.getByText('+ Додај ставку').click();
  await expect(page.locator('.item-row')).toHaveCount(2);
  await page.locator('.item-row').last().getByRole('button').click(); // ✕ button
  await expect(page.locator('.item-row')).toHaveCount(1);
});

// ── Standard invoice create → KPO auto-entry ─────────────────────────────────

test('creating a standard invoice auto-adds a KPO entry', async ({ page }) => {
  // Isolated entrepreneur to avoid state bleed.
  const entUrl = await createEntrepreneur(page, 'Фактура КПО ' + Date.now(), uniquePib());
  await page.goto(entUrl + '/invoices/new');

  await page.fill('#f-invnum', 'AUTO/2026');
  await page.fill('#f-issue', `${CURRENT_YEAR}-06-15`);
  // Fill line item.
  await page.locator('[name="item_description[]"]').fill('IT услуге');
  await page.locator('[name="item_unit_price[]"]').fill('50000');
  await page.locator('[name="item_quantity[]"]').fill('1');

  await page.getByRole('button', { name: /Креирај фактуру/ }).click();
  await page.waitForURL(/invoices\/[0-9a-f-]+$/);

  // Go back to entrepreneur page and verify KPO entry was created.
  await page.goto(entUrl);
  await expect(page.getByRole('cell', { name: 'AUTO/2026' })).toBeVisible();
  await expect(page.getByRole('cell', { name: '50000.00' }).first()).toBeVisible();
});

// ── Advance invoice → muted in KPO ───────────────────────────────────────────

test('advance invoice appears muted in KPO section with tooltip', async ({ page }) => {
  const entUrl = await createEntrepreneur(page, 'Аванс КПО ' + Date.now(), uniquePib());
  await page.goto(entUrl + '/invoices/new');

  await page.locator('input[name="invoice_type"][value="advance"]').check();
  await page.fill('#f-invnum', 'AV/2026');
  await page.fill('#f-issue', `${CURRENT_YEAR}-06-20`);
  await page.locator('[name="item_description[]"]').fill('Аванс за пројекат');
  await page.locator('[name="item_unit_price[]"]').fill('20000');

  await page.getByRole('button', { name: /Креирај фактуру/ }).click();
  await page.waitForURL(/invoices\/[0-9a-f-]+$/);

  await page.goto(entUrl);
  // Advance row should be visible but muted (opacity-50 class).
  const advRow = page.locator('tr.opacity-50', { hasText: 'AV/2026' });
  await expect(advRow).toBeVisible();
  // Tooltip text on the row.
  await expect(advRow).toHaveAttribute('title', 'Ова фактура се не узима у обзир у КПО');
  // KPO totals should NOT include the advance invoice amount (totals row hidden since no regular entries).
  await expect(page.getByRole('row').filter({ hasText: 'Укупно:' })).not.toBeVisible();
});

// ── Invoice detail page ───────────────────────────────────────────────────────

test('invoice detail page shows correct data', async ({ page }) => {
  const entUrl = await createEntrepreneur(page, 'Детаљ Фактура ' + Date.now(), uniquePib());
  await page.goto(entUrl + '/invoices/new');

  await page.fill('#f-invnum', 'DETAIL/1');
  await page.fill('#f-issue', `${CURRENT_YEAR}-05-01`);
  await page.locator('[name="item_description[]"]').fill('Консалтинг');
  await page.locator('[name="item_unit_price[]"]').fill('30000');
  await page.fill('#f-notes', 'Тест напомена');

  await page.getByRole('button', { name: /Креирај фактуру/ }).click();
  await page.waitForURL(/invoices\/[0-9a-f-]+$/);

  await expect(page.getByText('DETAIL/1')).toBeVisible();
  await expect(page.getByRole('cell', { name: 'Консалтинг' })).toBeVisible();
  await expect(page.getByRole('cell', { name: '30000.00' }).first()).toBeVisible();
  await expect(page.getByText('Тест напомена')).toBeVisible();
  await expect(page.getByRole('link', { name: /Преузми PDF/ })).toBeVisible();
});

// ── PDF download ──────────────────────────────────────────────────────────────

test('PDF download returns a PDF file', async ({ page }) => {
  const entUrl = await createEntrepreneur(page, 'PDF Тест ' + Date.now(), uniquePib());
  await page.goto(entUrl + '/invoices/new');

  await page.fill('#f-invnum', 'PDF/1');
  await page.fill('#f-issue', `${CURRENT_YEAR}-04-01`);
  await page.locator('[name="item_description[]"]').fill('PDF услуга');
  await page.locator('[name="item_unit_price[]"]').fill('10000');

  await page.getByRole('button', { name: /Креирај фактуру/ }).click();
  await page.waitForURL(/invoices\/[0-9a-f-]+$/);

  const [download] = await Promise.all([
    page.waitForEvent('download'),
    page.getByRole('link', { name: /Преузми PDF/ }).click(),
  ]);
  expect(download.suggestedFilename()).toMatch(/faktura.*\.pdf/i);
});

// ── Client autocomplete ───────────────────────────────────────────────────────

test('client autocomplete shows matching results', async ({ page }) => {
  const clientName = 'Аутоком клијент ' + Date.now();
  await createClient(page, entrepreneurUrl, clientName);

  await page.goto(entrepreneurUrl + '/invoices/new');
  await page.fill('#client-search', 'Аутоком');
  await expect(page.locator('#client-dropdown li')).toBeVisible({ timeout: 3000 });
  await expect(page.locator('#client-dropdown li')).toContainText(clientName);
});
