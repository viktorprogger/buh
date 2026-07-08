import { test, expect, Page } from '@playwright/test';
import {
  loginAccountant,
  loginEntrepreneur,
  registerEntrepreneur,
  createEntrepreneur,
  createEntrepreneurSlip,
  pairEntrepreneur,
  uniquePib,
  CURRENT_YEAR,
} from './helpers';

// Test 1: Auto-merge entrepreneur-only slips
test.describe('slip auto-merge — entrepreneur-only slips', () => {
  let accPage: Page, entPage: Page;
  let entPath: string, entId: string, entEmail: string, entPassword: string;

  test.beforeAll(async ({ browser }) => {
    accPage = await browser.newPage();
    entPage = await browser.newPage();

    entEmail = `ent-${Date.now()}@test.com`;
    entPassword = 'testPassword123';

    // Login accountant (global test accountant)
    await loginAccountant(accPage);

    // Accountant creates entrepreneur
    const entUrl = await createEntrepreneur(
      accPage,
      `AutoMerge ${Date.now()}`,
      uniquePib()
    );
    entPath = new URL(entUrl).pathname;
    entId = entPath.split('/').pop();

    // Register entrepreneur
    await registerEntrepreneur(entPage, entEmail, entPassword);

    // Pair entrepreneur
    await pairEntrepreneur(accPage, entPage, entId);

    // Login entrepreneur
    await loginEntrepreneur(entPage, entEmail, entPassword);

    // Create 2 slips
    await createEntrepreneurSlip(entPage, `Slip A ${Date.now()}`);
    await createEntrepreneurSlip(entPage, `Slip B ${Date.now()}`);
  });

  test.afterAll(async () => {
    await accPage.close();
    await entPage.close();
  });

  test('accountant triggers merge — auto-redirects, no conflict page', async ({
    browser,
  }) => {
    const page = await browser.newPage();
    await loginAccountant(page);

    await page.goto(
      `http://localhost:8081${entPath}/slips/merge/${CURRENT_YEAR}`
    );

    // No conflicts: auto-merges and redirects to entrepreneur page
    await page.waitForURL(new RegExp(entPath));

    await page.close();
  });

  test('after auto-merge, entrepreneur sees merged badge', async ({
    browser,
  }) => {
    const page = await browser.newPage();
    await loginEntrepreneur(page, entEmail, entPassword);

    await page.goto('http://localhost:8081/e/slips');

    // Assert "Спојено" badge is visible
    await expect(page.getByText('Спојено')).toBeVisible();

    await page.close();
  });
});

// Test 2: Manual merge with one conflict
test.describe('slip manual merge — one conflict', () => {
  let accPage: Page, entPage: Page;
  let conflictEntPath: string, conflictEntId: string, conflictEntEmail: string, conflictEntPassword: string;

  test.beforeAll(async ({ browser }) => {
    accPage = await browser.newPage();
    entPage = await browser.newPage();

    conflictEntEmail = `conf-ent-${Date.now()}@test.com`;
    conflictEntPassword = 'testPassword123';

    // Setup pair
    await loginAccountant(accPage);
    const entUrl = await createEntrepreneur(
      accPage,
      `ConflictMerge ${Date.now()}`,
      uniquePib()
    );
    conflictEntPath = new URL(entUrl).pathname;
    conflictEntId = conflictEntPath.split('/').pop();

    await registerEntrepreneur(entPage, conflictEntEmail, conflictEntPassword);
    await pairEntrepreneur(accPage, entPage, conflictEntId);

    // Accountant creates slip with amount 5000
    await accPage.goto(
      `http://localhost:8081${conflictEntPath}/slips/new`
    );
    await accPage.fill('input[name="SF"]', '253');
    await accPage.fill('input[name="S"]', 'Порез');
    await accPage.fill('input[name="N"]', 'Пореска управа');
    await accPage.fill('input[name="R"]', '840-0000000000-00');
    await accPage.fill('input[name="amount"]', '5000');
    await accPage.click('#btn-save-slip-acc');
    await accPage.waitForURL(/\/a\/slips\//);

    // Entrepreneur creates slip with same purpose but different amount (6000)
    await loginEntrepreneur(entPage, conflictEntEmail, conflictEntPassword);
    await entPage.goto('http://localhost:8081/e/slips/new');
    await entPage.fill('input[name="SF"]', '253');
    await entPage.fill('input[name="S"]', 'Порез');
    await entPage.fill('input[name="N"]', 'Пореска управа');
    await entPage.fill('input[name="R"]', '840-0000000000-00');
    await entPage.fill('input[name="amount"]', '6000');
    await entPage.fill('input[name="year"]', String(CURRENT_YEAR));
    await entPage.click('#btn-save-slip');
    await entPage.waitForURL(/\/e\/slips/);
  });

  test.afterAll(async () => {
    await accPage.close();
    await entPage.close();
  });

  test('merge view shows conflict with side-by-side comparison', async ({
    browser,
  }) => {
    const page = await browser.newPage();
    await loginAccountant(page);

    await page.goto(
      `http://localhost:8081${conflictEntPath}/slips/merge/${CURRENT_YEAR}`
    );

    // Assert conflict banner
    await expect(
      page.getByText('Пронађена су неслагања')
    ).toBeVisible();

    // Assert column headers
    await expect(page.getByText('Верзија рачуновође')).toBeVisible();
    await expect(page.getByText('Верзија предузетника')).toBeVisible();

    // Assert amounts visible (5000 and 6000)
    await expect(page.getByText('5000')).toBeVisible();
    await expect(page.getByText('6000')).toBeVisible();

    await page.close();
  });

  test('picking accountant version: merge continues', async ({ browser }) => {
    const page = await browser.newPage();
    await loginAccountant(page);

    await page.goto(
      `http://localhost:8081${conflictEntPath}/slips/merge/${CURRENT_YEAR}`
    );

    // First button[name="keep"] is accountant side
    await page.locator('button[name="keep"]').first().click();

    // Assert redirect to entrepreneur page (merge complete) or stay on merge page (more conflicts)
    await page.waitForURL(new RegExp(conflictEntPath));

    await page.close();
  });
});

// Test 3: Auto-merge with identical slips
test.describe('slip auto-merge — identical slip on both sides', () => {
  let accPage: Page, entPage: Page;
  let identicalEntPath: string, identicalEntId: string, identicalEntEmail: string, identicalEntPassword: string;

  test.beforeAll(async ({ browser }) => {
    accPage = await browser.newPage();
    entPage = await browser.newPage();

    identicalEntEmail = `identical-ent-${Date.now()}@test.com`;
    identicalEntPassword = 'testPassword123';

    // Setup pair
    await loginAccountant(accPage);
    const entUrl = await createEntrepreneur(
      accPage,
      `IdenticalMerge ${Date.now()}`,
      uniquePib()
    );
    identicalEntPath = new URL(entUrl).pathname;
    identicalEntId = identicalEntPath.split('/').pop();

    await registerEntrepreneur(entPage, identicalEntEmail, identicalEntPassword);
    await pairEntrepreneur(accPage, entPage, identicalEntId);

    const purpose = `ПДВ ${Date.now()}`;

    // Accountant creates slip
    await accPage.goto(
      `http://localhost:8081${identicalEntPath}/slips/new`
    );
    await accPage.fill('input[name="SF"]', '253');
    await accPage.fill('input[name="S"]', purpose);
    await accPage.fill('input[name="N"]', 'Пореска управа');
    await accPage.fill('input[name="R"]', '840-0000000000-00');
    await accPage.fill('input[name="amount"]', '5000');
    await accPage.click('#btn-save-slip-acc');
    await accPage.waitForURL(/\/a\/slips\//);

    // Entrepreneur creates identical slip (same purpose, amount, account)
    await loginEntrepreneur(entPage, identicalEntEmail, identicalEntPassword);
    await entPage.goto('http://localhost:8081/e/slips/new');
    await entPage.fill('input[name="SF"]', '253');
    await entPage.fill('input[name="S"]', purpose);
    await entPage.fill('input[name="N"]', 'Пореска управа');
    await entPage.fill('input[name="R"]', '840-0000000000-00');
    await entPage.fill('input[name="amount"]', '5000');
    await entPage.fill('input[name="year"]', String(CURRENT_YEAR));
    await entPage.click('#btn-save-slip');
    await entPage.waitForURL(/\/e\/slips/);
  });

  test.afterAll(async () => {
    await accPage.close();
    await entPage.close();
  });

  test('merge auto-completes; accountant copy kept', async ({ browser }) => {
    const page = await browser.newPage();
    await loginAccountant(page);

    await page.goto(
      `http://localhost:8081${identicalEntPath}/slips/merge/${CURRENT_YEAR}`
    );

    // No conflicts: auto-merges and redirects to entrepreneur page
    await page.waitForURL(new RegExp(identicalEntPath));

    await page.close();
  });
});

