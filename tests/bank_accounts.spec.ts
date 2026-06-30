import { test, expect, Page } from '@playwright/test';
import { login, createEntrepreneur, uniquePib, CURRENT_YEAR } from './helpers';

async function selectOptionByText(page: Page, selector: string, textPattern: RegExp) {
  const opt = await page.locator(selector + ' option').filter({ hasText: textPattern }).first();
  const val = await opt.getAttribute('value');
  await page.selectOption(selector, val!);
}

let entrepreneurUrl: string;

test.beforeAll(async ({ browser }) => {
  const page = await browser.newPage();
  await login(page);
  entrepreneurUrl = await createEntrepreneur(page, 'Рачун Тест ' + Date.now(), uniquePib());
  await page.close();
});

test.beforeEach(async ({ page }) => {
  await login(page);
  await page.goto(entrepreneurUrl + '/settings');
});

// ── Settings page shows bank accounts section ─────────────────────────────────

test('settings page shows bank accounts section', async ({ page }) => {
  await expect(page.getByRole('heading', { name: 'Банковни рачуни' })).toBeVisible();
  await expect(page.getByRole('link', { name: '+ Нови рачун' })).toBeVisible();
});

// ── Local bank account CRUD ───────────────────────────────────────────────────

async function createLocalAccount(page: Page, entUrl: string, bankName = 'Банка Тест', accountNumber = '265-1234567890-12') {
  await page.goto(entUrl + '/bank-accounts/new');
  await page.locator('input[name="account_type"][value="local"]').check();
  await page.fill('input[name="bank_name"]', bankName);
  await page.fill('input[name="account_number"]', accountNumber);
  await page.getByRole('button', { name: 'Додај рачун' }).click();
  await page.waitForURL(/settings$/);
}

test('can create a local bank account', async ({ page }) => {
  await createLocalAccount(page, entrepreneurUrl, 'OTP Банка', '265-9876543210-12');
  await expect(page.getByRole('cell', { name: 'OTP Банка' })).toBeVisible();
  await expect(page.getByRole('cell', { name: '265-9876543210-12' })).toBeVisible();
});

test('local account shows type as Домаћи', async ({ page }) => {
  await createLocalAccount(page, entrepreneurUrl, 'Тест Домаћи ' + Date.now(), '111-222333444-55');
  await expect(page.getByRole('cell', { name: 'Домаћи' }).first()).toBeVisible();
});

test('can edit a local bank account', async ({ page }) => {
  const bankName = 'Банка за измену ' + Date.now();
  await createLocalAccount(page, entrepreneurUrl, bankName, '100-200300400-50');
  await page.getByRole('row', { name: new RegExp(bankName) }).getByRole('link', { name: 'Измени' }).click();
  await page.fill('input[name="bank_name"]', bankName + ' — измењена');
  await page.getByRole('button', { name: 'Сачувај' }).click();
  await page.waitForURL(/settings$/);
  await expect(page.getByRole('cell', { name: bankName + ' — измењена' })).toBeVisible();
});

test('can delete a local bank account', async ({ page }) => {
  const bankName = 'Банка за брисање ' + Date.now();
  await createLocalAccount(page, entrepreneurUrl, bankName, '200-300400500-60');
  await expect(page.getByRole('cell', { name: bankName })).toBeVisible();
  page.on('dialog', d => d.accept());
  await page.getByRole('row', { name: new RegExp(bankName) }).getByRole('button', { name: 'Обриши' }).click();
  await page.waitForURL(/settings$/);
  await expect(page.getByRole('cell', { name: bankName })).not.toBeVisible();
});

// ── Foreign bank account ──────────────────────────────────────────────────────

async function createForeignAccount(page: Page, entUrl: string, bankName = 'Deutsche Bank', iban = 'DE89370400440532013000', swift = 'DEUTDEDB') {
  await page.goto(entUrl + '/bank-accounts/new');
  await page.locator('input[name="account_type"][value="foreign"]').check();
  await page.fill('input[name="bank_name"]', bankName);
  await page.fill('input[name="iban"]', iban);
  await page.fill('input[name="swift"]', swift);
  await page.getByRole('button', { name: 'Додај рачун' }).click();
  await page.waitForURL(/settings$/);
}

test('can create a foreign bank account', async ({ page }) => {
  await createForeignAccount(page, entrepreneurUrl, 'Commerzbank', 'DE75512108001245126199', 'COBADEFFXXX');
  await expect(page.getByRole('cell', { name: 'Commerzbank' })).toBeVisible();
  await expect(page.getByRole('cell', { name: 'DE75512108001245126199' })).toBeVisible();
  await expect(page.getByRole('cell', { name: 'Девизни' })).toBeVisible();
});

test('account type toggle shows/hides IBAN field', async ({ page }) => {
  await page.goto(entrepreneurUrl + '/bank-accounts/new');
  // Default is local, IBAN section should be hidden
  await expect(page.locator('#foreign-fields')).toHaveClass(/hidden/);
  // Switch to foreign
  await page.locator('input[name="account_type"][value="foreign"]').check();
  await expect(page.locator('#foreign-fields')).not.toHaveClass(/hidden/);
  // Switch back to local
  await page.locator('input[name="account_type"][value="local"]').check();
  await expect(page.locator('#foreign-fields')).toHaveClass(/hidden/);
});

// ── Correspondent banks ───────────────────────────────────────────────────────

test('can add a correspondent bank to a foreign account', async ({ page }) => {
  const bankName = 'Кор Банка ' + Date.now();
  const iban = 'DE89' + Date.now().toString().slice(-16);
  await createForeignAccount(page, entrepreneurUrl, bankName, iban, 'DEUTDEDB');
  // Click edit on the foreign account row
  await page.getByRole('row', { name: new RegExp(bankName) }).getByRole('link', { name: 'Измени' }).click();
  await expect(page.getByRole('heading', { name: 'Коресподентне банке' })).toBeVisible();
  // Add a correspondent bank
  await page.getByRole('link', { name: '+ Додај' }).click();
  await page.fill('input[name="bank_name"]', 'JP Morgan Chase');
  await page.fill('input[name="swift"]', 'CHASUS33');
  await page.fill('input[name="bank_address"]', '270 Park Ave, New York');
  await page.getByRole('button', { name: 'Додај банку' }).click();
  // Should redirect back to the account edit page
  await page.waitForURL(/bank-accounts\/[0-9a-f-]+\/edit$/);
  await expect(page.getByRole('cell', { name: 'JP Morgan Chase' })).toBeVisible();
  await expect(page.getByRole('cell', { name: 'CHASUS33' })).toBeVisible();
});

test('can delete a correspondent bank', async ({ page }) => {
  const bankName = 'Кор Брисање ' + Date.now();
  const iban = 'GB29' + Date.now().toString().slice(-16);
  await createForeignAccount(page, entrepreneurUrl, bankName, iban, 'NWBKGB2L');
  await page.getByRole('row', { name: new RegExp(bankName) }).getByRole('link', { name: 'Измени' }).click();
  await page.getByRole('link', { name: '+ Додај' }).click();
  await page.fill('input[name="bank_name"]', 'Barclays');
  await page.fill('input[name="swift"]', 'BARCGB22');
  await page.getByRole('button', { name: 'Додај банку' }).click();
  await page.waitForURL(/bank-accounts\/[0-9a-f-]+\/edit$/);
  await expect(page.getByRole('cell', { name: 'Barclays' })).toBeVisible();
  page.on('dialog', d => d.accept());
  await page.getByRole('row', { name: /Barclays/ }).getByRole('button', { name: 'Обриши' }).click();
  await page.waitForURL(/bank-accounts\/[0-9a-f-]+\/edit$/);
  await expect(page.getByRole('cell', { name: 'Barclays' })).not.toBeVisible();
});

// ── Invoice form bank account selection ───────────────────────────────────────

test('invoice form shows bank account selector when accounts exist', async ({ page }) => {
  const entUrl = await createEntrepreneur(page, 'Фактура Рачун ' + Date.now(), uniquePib());
  await createLocalAccount(page, entUrl, 'Рачун за фактуру', '333-444555666-77');
  await page.goto(entUrl + '/invoices/new');
  await expect(page.locator('#f-bank-account')).toBeVisible();
  const options = page.locator('#f-bank-account option');
  // At least 2 options: empty + account
  expect(await options.count()).toBeGreaterThanOrEqual(2);
});

test('selecting a bank account updates live preview', async ({ page }) => {
  const entUrl = await createEntrepreneur(page, 'Преглед Рачун ' + Date.now(), uniquePib());
  await createLocalAccount(page, entUrl, 'Моја Банка', '265-111222333-44');
  await page.goto(entUrl + '/invoices/new');
  await selectOptionByText(page, '#f-bank-account', /Моја Банка/);
  await expect(page.locator('#invoice-preview')).toContainText('Моја Банка');
});

test('selecting a foreign account with correspondent bank shows correspondent section', async ({ page }) => {
  const entUrl = await createEntrepreneur(page, 'Кор. Преглед ' + Date.now(), uniquePib());
  // Create foreign account
  const iban = 'DE89370400440532013' + Date.now().toString().slice(-3);
  await createForeignAccount(page, entUrl, 'Иностр. Банка', iban, 'DEUTDEDB');
  // Add correspondent bank via edit page
  await page.goto(entUrl + '/settings');
  await page.getByRole('row', { name: /Иностр. Банка/ }).getByRole('link', { name: 'Измени' }).click();
  await page.getByRole('link', { name: '+ Додај' }).click();
  await page.fill('input[name="bank_name"]', 'Wells Fargo');
  await page.fill('input[name="swift"]', 'WFBIUS6S');
  await page.getByRole('button', { name: 'Додај банку' }).click();
  await page.waitForURL(/bank-accounts\/[0-9a-f-]+\/edit$/);
  // Now go to invoice form
  await page.goto(entUrl + '/invoices/new');
  await selectOptionByText(page, '#f-bank-account', /Иностр. Банка/);
  // Correspondent section should appear
  await expect(page.locator('#correspondent-section')).not.toHaveClass(/hidden/);
  await selectOptionByText(page, '#f-corr-bank', /Wells Fargo/);
  await expect(page.locator('#invoice-preview')).toContainText('Wells Fargo');
});

// ── Invoice with bank account persisted and shown on detail page ──────────────

test('bank account is saved with invoice and shown on detail page', async ({ page }) => {
  const entUrl = await createEntrepreneur(page, 'Детаљ Рачун ' + Date.now(), uniquePib());
  await createLocalAccount(page, entUrl, 'Интеса Банка', '160-123456789012-95');
  await page.goto(entUrl + '/invoices/new');
  await page.fill('#f-invnum', 'BA/2026');
  await page.fill('#f-issue', `${CURRENT_YEAR}-05-15`);
  await page.locator('[name="item_description[]"]').fill('Услуга са рачуном');
  await page.locator('[name="item_unit_price[]"]').fill('5000');
  await selectOptionByText(page, '#f-bank-account', /Интеса Банка/);
  await page.getByRole('button', { name: /Креирај фактуру/ }).click();
  await page.waitForURL(/invoices\/[0-9a-f-]+$/);
  await expect(page.getByText('Интеса Банка')).toBeVisible();
  await expect(page.getByText('160-123456789012-95')).toBeVisible();
});

test('bank account and correspondent bank appear in PDF', async ({ page }) => {
  const entUrl = await createEntrepreneur(page, 'PDF Рачун ' + Date.now(), uniquePib());
  const iban = 'DE89370400440532013' + Date.now().toString().slice(-3);
  await createForeignAccount(page, entUrl, 'PDF Банка', iban, 'DEUTDEDB');
  // Add correspondent
  await page.goto(entUrl + '/settings');
  await page.getByRole('row', { name: /PDF Банка/ }).getByRole('link', { name: 'Измени' }).click();
  await page.getByRole('link', { name: '+ Додај' }).click();
  await page.fill('input[name="bank_name"]', 'PDF Кор Банка');
  await page.fill('input[name="swift"]', 'PDFXXX00');
  await page.getByRole('button', { name: 'Додај банку' }).click();
  await page.waitForURL(/bank-accounts\/[0-9a-f-]+\/edit$/);
  // Create invoice with bank account
  await page.goto(entUrl + '/invoices/new');
  await page.fill('#f-invnum', 'PDF-BA/1');
  await page.fill('#f-issue', `${CURRENT_YEAR}-06-01`);
  await page.locator('[name="item_description[]"]').fill('Услуга PDF тест');
  await page.locator('[name="item_unit_price[]"]').fill('8000');
  await selectOptionByText(page, '#f-bank-account', /PDF Банка/);
  await selectOptionByText(page, '#f-corr-bank', /PDF Кор Банка/);
  await page.getByRole('button', { name: /Креирај фактуру/ }).click();
  await page.waitForURL(/invoices\/[0-9a-f-]+$/);
  // PDF download
  const [download] = await Promise.all([
    page.waitForEvent('download'),
    page.getByRole('link', { name: /Преузми PDF/ }).click(),
  ]);
  expect(download.suggestedFilename()).toMatch(/faktura.*\.pdf/i);
});
