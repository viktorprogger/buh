import { test, expect, Page } from '@playwright/test';
import {
  loginAccountant, loginEntrepreneur, registerEntrepreneur,
  createEntrepreneur, pairEntrepreneur, createEntrepreneurSlip,
  createAccountantSlip, uniquePib, CURRENT_YEAR,
} from './helpers';

// Manual merge: same purpose on both sides, different amounts → conflict.
// Accountant picks their version. After unpair both sides must see the winning
// amount and neither must see the losing amount.

const PURPOSE = `МануелниСпој ${Date.now()}`;
const ACCOUNTANT_AMOUNT = '5000';
const ENTREPRENEUR_AMOUNT = '6000';

let accPage: Page, entPage: Page;
let entPath: string, entId: string;
let entEmail: string;
const entPassword = 'manual-merge-pass-123';

test.beforeAll(async ({ browser }) => {
  accPage = await browser.newPage();
  entPage = await browser.newPage();
  entEmail = `manual-merge-${Date.now()}@test.local`;

  await loginAccountant(accPage);
  const entUrl = await createEntrepreneur(accPage, `ManualMerge Unpair ${Date.now()}`, uniquePib());
  entPath = new URL(entUrl).pathname;
  entId = entPath.split('/').pop()!;

  await registerEntrepreneur(entPage, entEmail, entPassword);
  await pairEntrepreneur(accPage, entPage, entId);
  await loginEntrepreneur(entPage, entEmail, entPassword);

  // Accountant and entrepreneur both create a slip with the same purpose but different amounts
  await createAccountantSlip(accPage, entPath, PURPOSE, ACCOUNTANT_AMOUNT);
  await createEntrepreneurSlip(entPage, PURPOSE, { year: CURRENT_YEAR });

  // Accountant opens the merge view — conflict is shown
  await accPage.goto(`${entPath}/slips/merge/${CURRENT_YEAR}`);

  // Pick accountant's version (first "keep" button is accountant's column)
  await accPage.locator('button[name="keep"]').first().click();

  // After picking, the merge page reloads with no conflicts → auto-redirects
  await accPage.waitForURL(new RegExp(entId));

  // Accountant unpairs
  accPage.on('dialog', d => d.accept());
  await accPage.goto(entPath);
  await accPage.locator('form[action*="/unpair"] button').click();
  await accPage.waitForURL(new RegExp(entId));
});

test.afterAll(async () => {
  await accPage.close();
  await entPage.close();
});

test('entrepreneur sees the winning (accountant) amount after unpair', async () => {
  await entPage.goto('/e/slips');
  await expect(entPage.getByText(ACCOUNTANT_AMOUNT)).toBeVisible();
});

test('accountant sees the winning amount on entrepreneur page after unpair', async () => {
  await accPage.goto(entPath);
  await expect(accPage.getByText(ACCOUNTANT_AMOUNT)).toBeVisible();
});

test('entrepreneur does not see the losing (entrepreneur) amount — it was deleted during merge', async () => {
  await entPage.goto('/e/slips');
  await expect(entPage.getByText(ENTREPRENEUR_AMOUNT)).not.toBeVisible();
});
