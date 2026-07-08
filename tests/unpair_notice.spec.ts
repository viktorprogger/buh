import { test, expect, Page, Browser } from '@playwright/test';
import {
  loginAccountant, loginEntrepreneur, registerEntrepreneur,
  createEntrepreneur, pairEntrepreneur, uniquePib, CURRENT_YEAR,
} from './helpers';

const NOTICE_TEXT = 'Рачуновођа је прекинуо везу са твојим налогом';

async function setupUnpairedEntrepreneur(browser: Browser): Promise<{ entPage: Page; accPage: Page }> {
  const accPage = await browser.newPage();
  const entPage = await browser.newPage();
  const email = `notice-${Date.now()}@test.local`;
  const password = 'notice-pass-123';

  await loginAccountant(accPage);
  const entUrl = await createEntrepreneur(accPage, `Notice Test ${Date.now()}`, uniquePib());
  const entId = new URL(entUrl).pathname.split('/').pop()!;

  await registerEntrepreneur(entPage, email, password);
  await pairEntrepreneur(accPage, entPage, entId);

  // Accountant initiates unpair — sets notice for entrepreneur
  accPage.on('dialog', d => d.accept());
  await accPage.goto(new URL(entUrl).pathname);
  await accPage.locator(`form[action*="/unpair"] button`).click();
  await accPage.waitForURL(new RegExp(entId));

  await loginEntrepreneur(entPage, email, password);
  return { entPage, accPage };
}

// ── Banner visible on multiple pages ─────────────────────────────────────────

test.describe('notice banner is shown on entrepreneur pages after accountant unpairs', () => {
  let entPage: Page, accPage: Page;

  test.beforeAll(async ({ browser }) => {
    ({ entPage, accPage } = await setupUnpairedEntrepreneur(browser));
  });

  test.afterAll(async () => {
    await entPage.close();
    await accPage.close();
  });

  test('banner visible on dashboard', async () => {
    await entPage.goto('/e/');
    await expect(entPage.getByText(NOTICE_TEXT)).toBeVisible();
  });

  test('banner visible on slips page', async () => {
    await entPage.goto('/e/slips');
    await expect(entPage.getByText(NOTICE_TEXT)).toBeVisible();
  });

  test('banner visible on KPO page', async () => {
    await entPage.goto(`/e/kpo/${CURRENT_YEAR}`);
    await expect(entPage.getByText(NOTICE_TEXT)).toBeVisible();
  });
});

// ── Banner persists until dismissed, then clears from DB ─────────────────────

test.describe.serial('notice banner dismiss clears it from DB', () => {
  let entPage: Page, accPage: Page;

  test.beforeAll(async ({ browser }) => {
    ({ entPage, accPage } = await setupUnpairedEntrepreneur(browser));
  });

  test.afterAll(async () => {
    await entPage.close();
    await accPage.close();
  });

  test('banner still visible after page reload (not just a flash)', async () => {
    await entPage.goto('/e/');
    await entPage.reload();
    await expect(entPage.getByText(NOTICE_TEXT)).toBeVisible();
  });

  test('clicking dismiss hides the banner', async () => {
    await entPage.goto('/e/');
    await entPage.locator('form[action="/e/notices/dismiss"] button').click();
    await expect(entPage.getByText(NOTICE_TEXT)).not.toBeVisible();
  });

  test('banner stays gone after reload — DB was cleared', async () => {
    await entPage.goto('/e/');
    await expect(entPage.getByText(NOTICE_TEXT)).not.toBeVisible();
  });
});
