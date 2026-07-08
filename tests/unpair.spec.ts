import { test, expect, Page } from '@playwright/test';
import {
  loginAccountant,
  loginEntrepreneur,
  registerEntrepreneur,
  createEntrepreneur,
  pairEntrepreneur,
  uniquePib,
} from './helpers';

// ── Accountant-initiated unpair ───────────────────────────────────────────────

test.describe('accountant unpair', () => {
  let accPage: Page, entPage: Page;
  let entPath: string, entId: string;

  test.beforeAll(async ({ browser }) => {
    accPage = await browser.newPage();
    entPage = await browser.newPage();

    const entEmail = `unpair-acc-${Date.now()}@test.local`;
    const entPassword = 'unpair-acc-pass-123';

    await loginAccountant(accPage);

    const entUrl = await createEntrepreneur(accPage, `Unpair Acc ${Date.now()}`, uniquePib());
    entPath = new URL(entUrl).pathname;
    entId = entPath.split('/').pop()!;

    await registerEntrepreneur(entPage, entEmail, entPassword);
    await pairEntrepreneur(accPage, entPage, entId);
  });

  test.afterAll(async () => {
    await accPage.close();
    await entPage.close();
  });

  test('unpair button is visible when entrepreneur is paired', async () => {
    await accPage.goto(entPath);
    await expect(accPage.locator('form[action*="/unpair"] button')).toBeVisible();
  });

  test('accountant can unpair and sees success banner', async () => {
    accPage.on('dialog', d => d.accept());
    await accPage.goto(entPath);
    await accPage.locator('form[action*="/unpair"] button').click();
    await accPage.waitForURL(new RegExp(entId + '\\?unpaired=1'));
    await expect(accPage.locator('.bg-green-50')).toBeVisible();
  });

  test('unpair button is gone after unpair', async () => {
    await accPage.goto(entPath);
    await expect(accPage.locator('form[action*="/unpair"] button')).not.toBeVisible();
  });
});

// ── Entrepreneur-initiated unpair ─────────────────────────────────────────────

test.describe('entrepreneur unpair', () => {
  let accPage: Page, entPage: Page;
  let entId: string;

  test.beforeAll(async ({ browser }) => {
    accPage = await browser.newPage();
    entPage = await browser.newPage();

    const entEmail = `unpair-ent-${Date.now()}@test.local`;
    const entPassword = 'unpair-ent-pass-123';

    await loginAccountant(accPage);

    const entUrl = await createEntrepreneur(accPage, `Unpair Ent ${Date.now()}`, uniquePib());
    entId = new URL(entUrl).pathname.split('/').pop()!;

    await registerEntrepreneur(entPage, entEmail, entPassword);
    await pairEntrepreneur(accPage, entPage, entId);
    await loginEntrepreneur(entPage, entEmail, entPassword);
  });

  test.afterAll(async () => {
    await accPage.close();
    await entPage.close();
  });

  test('unpair button is visible on profile when paired', async () => {
    await entPage.goto('/e/profile');
    await expect(entPage.locator('form[action="/e/unpair"] button')).toBeVisible();
  });

  test('entrepreneur can unpair and sees success banner', async () => {
    entPage.on('dialog', d => d.accept());
    await entPage.goto('/e/profile');
    await entPage.locator('form[action="/e/unpair"] button').click();
    await entPage.waitForURL('/e/profile?unpaired=1');
    await expect(entPage.locator('.bg-green-50')).toBeVisible();
  });

  test('profile shows unpaired alert after unpair', async () => {
    await entPage.goto('/e/profile');
    await expect(entPage.locator('.bg-yellow-50')).toBeVisible();
    await expect(entPage.locator('form[action="/e/unpair"] button')).not.toBeVisible();
  });
});
