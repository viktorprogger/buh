import { test, expect, Page } from '@playwright/test';
import { registerEntrepreneur, loginEntrepreneur, uniquePib, CURRENT_YEAR } from './helpers';

const email = `ba-${Date.now()}@test.local`;
const password = 'ba-test-pass-123';

test.beforeAll({ timeout: 15000 }, async ({ browser }) => {
  const page = await browser.newPage();
  await registerEntrepreneur(page, email, password);
  await page.close();
});

test.beforeEach(async ({ page }) => {
  await loginEntrepreneur(page, email, password);
  await page.goto('/e/bank-accounts');
});

// ── Bank account list ─────────────────────────────────────────────────────────

test('bank accounts page has add button', async ({ page }) => {
  await expect(page.getByRole('link', { name: '+ Нови рачун' })).toBeVisible();
});

// ── Local bank account CRUD ───────────────────────────────────────────────────

async function createLocalAccount(page: Page, bankName = 'Банка Тест', accountNumber = '265-1234567890-12') {
  await page.goto('/e/bank-accounts/new');
  await page.locator('input[name="account_type"][value="local"]').check();
  await page.fill('input[name="bank_name"]', bankName);
  await page.fill('input[name="account_number"]', accountNumber);
  await page.getByRole('button', { name: 'Додај рачун' }).click();
  await page.waitForURL('/e/bank-accounts');
}

test('can create a local bank account', async ({ page }) => {
  await createLocalAccount(page, 'OTP Банка', '265-9876543210-12');
  await expect(page.getByRole('cell', { name: 'OTP Банка' })).toBeVisible();
  await expect(page.getByRole('cell', { name: '265-9876543210-12' })).toBeVisible();
});

test('local account shows type as Домаћи', async ({ page }) => {
  await createLocalAccount(page, 'Тест Домаћи ' + Date.now(), '111-222333444-55');
  await expect(page.getByRole('cell', { name: 'Домаћи' }).first()).toBeVisible();
});

test('can edit a local bank account', async ({ page }) => {
  const bankName = 'Банка за измену ' + Date.now();
  await createLocalAccount(page, bankName, '100-200300400-50');
  await page.getByRole('row', { name: new RegExp(bankName) }).getByRole('link', { name: 'Измени' }).click();
  await page.fill('input[name="bank_name"]', bankName + ' — измењена');
  await page.getByRole('button', { name: 'Сачувај' }).click();
  await page.waitForURL(/\/e\/bank-accounts\/[0-9a-f-]+\/edit$/);
  await page.goto('/e/bank-accounts');
  await expect(page.getByRole('cell', { name: bankName + ' — измењена' })).toBeVisible();
});

test('can delete a local bank account', async ({ page }) => {
  const bankName = 'Банка за брисање ' + Date.now();
  await createLocalAccount(page, bankName, '200-300400500-60');
  await expect(page.getByRole('cell', { name: bankName })).toBeVisible();
  page.on('dialog', d => d.accept());
  await page.getByRole('row', { name: new RegExp(bankName) }).getByRole('button', { name: 'Обриши' }).click();
  await page.waitForURL('/e/bank-accounts');
  await expect(page.getByRole('cell', { name: bankName })).not.toBeVisible();
});

// ── Foreign bank account ──────────────────────────────────────────────────────

async function createForeignAccount(page: Page, bankName = 'Deutsche Bank', iban = 'DE89370400440532013000', swift = 'DEUTDEDB') {
  await page.goto('/e/bank-accounts/new');
  await page.locator('input[name="account_type"][value="foreign"]').check();
  await page.fill('input[name="bank_name"]', bankName);
  await page.fill('input[name="iban"]', iban);
  await page.fill('input[name="swift"]', swift);
  await page.getByRole('button', { name: 'Додај рачун' }).click();
  await page.waitForURL('/e/bank-accounts');
}

test('can create a foreign bank account', async ({ page }) => {
  await createForeignAccount(page, 'Commerzbank', 'DE75512108001245126199', 'COBADEFFXXX');
  await expect(page.getByRole('cell', { name: 'Commerzbank' })).toBeVisible();
  await expect(page.getByRole('cell', { name: 'DE75512108001245126199' })).toBeVisible();
  await expect(page.getByRole('cell', { name: 'Девизни' })).toBeVisible();
});

test('account type toggle shows/hides IBAN field', async ({ page }) => {
  await page.goto('/e/bank-accounts/new');
  await expect(page.locator('#foreign-fields')).toHaveClass(/hidden/);
  await page.locator('input[name="account_type"][value="foreign"]').check();
  await expect(page.locator('#foreign-fields')).not.toHaveClass(/hidden/);
  await page.locator('input[name="account_type"][value="local"]').check();
  await expect(page.locator('#foreign-fields')).toHaveClass(/hidden/);
});

// ── Correspondent banks ───────────────────────────────────────────────────────

test('can add a correspondent bank to a foreign account', async ({ page }) => {
  const bankName = 'Кор Банка ' + Date.now();
  const iban = 'DE89' + Date.now().toString().slice(-16);
  await createForeignAccount(page, bankName, iban, 'DEUTDEDB');
  await page.getByRole('row', { name: new RegExp(bankName) }).getByRole('link', { name: 'Измени' }).click();
  await expect(page.getByRole('heading', { name: 'Коресподентне банке' })).toBeVisible();
  await page.getByRole('link', { name: '+ Додај' }).click();
  await page.fill('input[name="bank_name"]', 'JP Morgan Chase');
  await page.fill('input[name="swift"]', 'CHASUS33');
  await page.fill('input[name="bank_address"]', '270 Park Ave, New York');
  await page.getByRole('button', { name: 'Додај банку' }).click();
  await page.waitForURL(/\/e\/bank-accounts\/[0-9a-f-]+\/edit$/);
  await expect(page.getByRole('cell', { name: 'JP Morgan Chase' })).toBeVisible();
  await expect(page.getByRole('cell', { name: 'CHASUS33' })).toBeVisible();
});

test('can delete a correspondent bank', async ({ page }) => {
  const bankName = 'Кор Брисање ' + Date.now();
  const iban = 'GB29' + Date.now().toString().slice(-16);
  await createForeignAccount(page, bankName, iban, 'NWBKGB2L');
  await page.getByRole('row', { name: new RegExp(bankName) }).getByRole('link', { name: 'Измени' }).click();
  await page.getByRole('link', { name: '+ Додај' }).click();
  await page.fill('input[name="bank_name"]', 'Barclays');
  await page.fill('input[name="swift"]', 'BARCGB22');
  await page.getByRole('button', { name: 'Додај банку' }).click();
  await page.waitForURL(/\/e\/bank-accounts\/[0-9a-f-]+\/edit$/);
  await expect(page.getByRole('cell', { name: 'Barclays' })).toBeVisible();
  page.on('dialog', d => d.accept());
  await page.getByRole('row', { name: /Barclays/ }).getByRole('button', { name: 'Обриши' }).click();
  await page.waitForURL(/\/e\/bank-accounts\/[0-9a-f-]+\/edit$/);
  await expect(page.getByRole('cell', { name: 'Barclays' })).not.toBeVisible();
});

// ── Invoice form bank account selection ───────────────────────────────────────

async function selectOptionByText(page: Page, selector: string, textPattern: RegExp) {
  const opt = await page.locator(selector + ' option').filter({ hasText: textPattern }).first();
  const val = await opt.getAttribute('value');
  await page.selectOption(selector, val!);
}

test('invoice form shows bank account selector when accounts exist', async ({ page }) => {
  await createLocalAccount(page, 'Рачун за фактуру ' + Date.now(), '333-444555666-77');
  await page.goto('/e/invoices/new');
  await expect(page.locator('#f-bank-account')).toBeVisible();
  const options = page.locator('#f-bank-account option');
  expect(await options.count()).toBeGreaterThanOrEqual(2);
});

test('selecting a bank account updates live preview', async ({ page }) => {
  const bankName = 'Моја Банка ' + Date.now();
  await createLocalAccount(page, bankName, '265-111222333-44');
  await page.goto('/e/invoices/new');
  await selectOptionByText(page, '#f-bank-account', new RegExp(bankName.slice(0, 10)));
  await expect(page.locator('#invoice-preview')).toContainText(bankName.slice(0, 10));
});

test('selecting a foreign account with correspondent bank shows correspondent section', async ({ page }) => {
  const bankName = 'Иностр. Банка ' + Date.now();
  const iban = 'DE89370400440532013' + Date.now().toString().slice(-3);
  await createForeignAccount(page, bankName, iban, 'DEUTDEDB');
  // Add correspondent bank.
  await page.getByRole('row', { name: new RegExp(bankName.slice(0, 10)) }).getByRole('link', { name: 'Измени' }).click();
  await page.getByRole('link', { name: '+ Додај' }).click();
  await page.fill('input[name="bank_name"]', 'Wells Fargo');
  await page.fill('input[name="swift"]', 'WFBIUS6S');
  await page.getByRole('button', { name: 'Додај банку' }).click();
  await page.waitForURL(/\/e\/bank-accounts\/[0-9a-f-]+\/edit$/);
  // Invoice form: select the foreign account and check correspondent section.
  await page.goto('/e/invoices/new');
  await selectOptionByText(page, '#f-bank-account', new RegExp(bankName.slice(0, 10)));
  await expect(page.locator('#correspondent-section')).not.toHaveClass(/hidden/);
  await selectOptionByText(page, '#f-corr-bank', /Wells Fargo/);
  await expect(page.locator('#invoice-preview')).toContainText('Wells Fargo');
});

test('bank account is saved with invoice and shown on detail page', async ({ page }) => {
  const bankName = 'Интеса Банка ' + Date.now();
  await createLocalAccount(page, bankName, '160-123456789012-95');
  await page.goto('/e/invoices/new');
  await page.fill('#f-invnum', 'BA/' + Date.now());
  await page.fill('#f-issue', `${CURRENT_YEAR}-05-15`);
  await page.locator('[name="item_description[]"]').fill('Услуга са рачуном');
  await page.locator('[name="item_unit_price[]"]').fill('5000');
  await selectOptionByText(page, '#f-bank-account', new RegExp(bankName.slice(0, 8)));
  await page.getByRole('button', { name: /Креирај фактуру/ }).click();
  await page.waitForURL(/\/e\/invoices\/[0-9a-f-]+$/);
  await expect(page.getByText(bankName.slice(0, 8))).toBeVisible();
  await expect(page.getByText('160-123456789012-95')).toBeVisible();
});

test('bank account and correspondent bank appear on invoice detail page', async ({ page }) => {
  test.setTimeout(10_000);
  const bankName = 'PDF Банка ' + Date.now();
  const iban = 'DE89370400440532013' + Date.now().toString().slice(-3);
  await createForeignAccount(page, bankName, iban, 'DEUTDEDB');
  await page.getByRole('row', { name: new RegExp(bankName.slice(0, 8)) }).getByRole('link', { name: 'Измени' }).click();
  await page.getByRole('link', { name: '+ Додај' }).click();
  await page.fill('input[name="bank_name"]', 'PDF Кор Банка');
  await page.fill('input[name="swift"]', 'PDFXXX00');
  await page.getByRole('button', { name: 'Додај банку' }).click();
  await page.waitForURL(/\/e\/bank-accounts\/[0-9a-f-]+\/edit$/);
  // Create invoice.
  await page.goto('/e/invoices/new');
  await page.fill('#f-invnum', 'PDF-BA/' + Date.now());
  await page.fill('#f-issue', `${CURRENT_YEAR}-06-01`);
  await page.locator('[name="item_description[]"]').fill('Услуга PDF тест');
  await page.locator('[name="item_unit_price[]"]').fill('8000');
  await selectOptionByText(page, '#f-bank-account', new RegExp(bankName.slice(0, 8)));
  await selectOptionByText(page, '#f-corr-bank', /PDF Кор Банка/);
  await page.getByRole('button', { name: /Креирај фактуру/ }).click();
  await page.waitForURL(/\/e\/invoices\/[0-9a-f-]+$/);
  // PDF download.
  const [download] = await Promise.all([
    page.waitForEvent('download'),
    page.getByRole('link', { name: /Преузми PDF/ }).click(),
  ]);
  expect(download.suggestedFilename()).toMatch(/faktura.*\.pdf/i);
});
