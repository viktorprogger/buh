import { test, expect, Page } from '@playwright/test';
import {
  loginAccountant, loginEntrepreneur, registerEntrepreneur,
  createEntrepreneur, pairEntrepreneur, createEntrepreneurSlip,
  createAccountantSlip, uniquePib,
} from './helpers';

// No merge was ever run. Each side has its own slip with a different purpose.
// DESIRED: on unpair, nothing is copied across — each side keeps only what they had.
// Some of these tests will FAIL with the current implementation (which always copies).

const ENT_PURPOSE = `ЕнтПорез ${Date.now()}`;
const ACC_PURPOSE = `РачПДВ ${Date.now()}`;

let accPage: Page, entPage: Page;
let entPath: string, entId: string;
let entEmail: string;
const entPassword = 'no-merge-pass-123';

test.beforeAll(async ({ browser }) => {
  accPage = await browser.newPage();
  entPage = await browser.newPage();
  entEmail = `no-merge-${Date.now()}@test.local`;

  await loginAccountant(accPage);
  const entUrl = await createEntrepreneur(accPage, `NoMerge Unpair ${Date.now()}`, uniquePib());
  entPath = new URL(entUrl).pathname;
  entId = entPath.split('/').pop()!;

  await registerEntrepreneur(entPage, entEmail, entPassword);
  await pairEntrepreneur(accPage, entPage, entId);
  await loginEntrepreneur(entPage, entEmail, entPassword);

  // Each side creates their own slip — different purposes, so no auto-merge key match
  await createEntrepreneurSlip(entPage, ENT_PURPOSE);
  await createAccountantSlip(accPage, entPath, ACC_PURPOSE);

  // Accountant unpairs WITHOUT running any merge
  accPage.on('dialog', d => d.accept());
  await accPage.goto(entPath);
  await accPage.locator('form[action*="/unpair"] button').click();
  await accPage.waitForURL(new RegExp(entId));
});

test.afterAll(async () => {
  await accPage.close();
  await entPage.close();
});

// Entrepreneur side
test('entrepreneur still sees their own slip after unpair without merge', async () => {
  await entPage.goto('/e/slips');
  await expect(entPage.getByText(ENT_PURPOSE)).toBeVisible();
});

test('entrepreneur does NOT see the accountant slip — nothing should be copied without merge', async () => {
  await entPage.goto('/e/slips');
  await expect(entPage.getByText(ACC_PURPOSE)).not.toBeVisible();
});

// Accountant side
test('accountant still sees their own slip on entrepreneur page after unpair without merge', async () => {
  await accPage.goto(entPath);
  await expect(accPage.getByText(ACC_PURPOSE)).toBeVisible();
});

test('accountant does NOT see the entrepreneur slip — it was never merged', async () => {
  await accPage.goto(entPath);
  await expect(accPage.getByText(ENT_PURPOSE)).not.toBeVisible();
});
