import { test, expect, Page } from '@playwright/test';
import {
  loginAccountant, loginEntrepreneur, registerEntrepreneur,
  createEntrepreneur, pairEntrepreneur, createEntrepreneurSlip,
  createAccountantSlip, uniquePib, CURRENT_YEAR,
} from './helpers';

// After unpair each side works independently. Changes made by one party
// must not appear in the other party's view.

let accPage: Page, entPage: Page;
let entPath: string, entId: string;
let entEmail: string;
const entPassword = 'isolation-pass-123';

test.beforeAll(async ({ browser }) => {
  accPage = await browser.newPage();
  entPage = await browser.newPage();
  entEmail = `isolation-${Date.now()}@test.local`;

  await loginAccountant(accPage);
  const entUrl = await createEntrepreneur(accPage, `Isolation Unpair ${Date.now()}`, uniquePib());
  entPath = new URL(entUrl).pathname;
  entId = entPath.split('/').pop()!;

  await registerEntrepreneur(entPage, entEmail, entPassword);
  await pairEntrepreneur(accPage, entPage, entId);
  await loginEntrepreneur(entPage, entEmail, entPassword);

  // Accountant unpairs (no data created, clean slate)
  accPage.on('dialog', d => d.accept());
  await accPage.goto(entPath);
  await accPage.locator('form[action*="/unpair"] button').click();
  await accPage.waitForURL(new RegExp(entId));
});

test.afterAll(async () => {
  await accPage.close();
  await entPage.close();
});

test('slip created by entrepreneur after unpair is not visible to accountant', async () => {
  const purpose = `ЕнтПостРаскид ${Date.now()}`;
  await createEntrepreneurSlip(entPage, purpose, { year: CURRENT_YEAR });

  await accPage.goto(entPath);
  await expect(accPage.getByText(purpose)).not.toBeVisible();
});

test('slip created by accountant after unpair is not visible to entrepreneur', async () => {
  const purpose = `РачПостРаскид ${Date.now()}`;
  await createAccountantSlip(accPage, entPath, purpose);

  await entPage.goto('/e/slips');
  await expect(entPage.getByText(purpose)).not.toBeVisible();
});
