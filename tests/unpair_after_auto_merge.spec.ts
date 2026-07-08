import { test, expect, Page } from '@playwright/test';
import {
  loginAccountant, loginEntrepreneur, registerEntrepreneur,
  createEntrepreneur, pairEntrepreneur, createEntrepreneurSlip,
  uniquePib, CURRENT_YEAR,
} from './helpers';

// Auto-merge: entrepreneur has a slip, accountant has none for that purpose.
// GET /slips/merge/{year} auto-merges (moves slip to managed side) and redirects.
// After unpair, both sides must see the same slip.

const PURPOSE = `Аутоспај ${Date.now()}`;

let accPage: Page, entPage: Page;
let entPath: string, entId: string;
let entEmail: string;
const entPassword = 'auto-merge-pass-123';

test.beforeAll(async ({ browser }) => {
  accPage = await browser.newPage();
  entPage = await browser.newPage();
  entEmail = `auto-merge-${Date.now()}@test.local`;

  await loginAccountant(accPage);
  const entUrl = await createEntrepreneur(accPage, `AutoMerge Unpair ${Date.now()}`, uniquePib());
  entPath = new URL(entUrl).pathname;
  entId = entPath.split('/').pop()!;

  await registerEntrepreneur(entPage, entEmail, entPassword);
  await pairEntrepreneur(accPage, entPage, entId);
  await loginEntrepreneur(entPage, entEmail, entPassword);

  // Entrepreneur creates a slip
  await createEntrepreneurSlip(entPage, PURPOSE);

  // Accountant triggers auto-merge — GET redirects immediately after moving the slip
  await accPage.goto(`${entPath}/slips/merge/${CURRENT_YEAR}`);
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

test('entrepreneur sees the merged slip after unpair', async () => {
  await entPage.goto('/e/slips');
  await expect(entPage.getByText(PURPOSE)).toBeVisible();
});

test('accountant sees the merged slip on entrepreneur page after unpair', async () => {
  await accPage.goto(entPath);
  await expect(accPage.getByText(PURPOSE)).toBeVisible();
});
