import { test, expect } from '@playwright/test';
import { loginEntrepreneur, registerEntrepreneur, CURRENT_YEAR } from './helpers';

const email = `currency-picker-${Date.now()}@test.local`;
const password = 'currency-picker-pass-123';

test.beforeAll(async ({ browser }) => {
  const page = await browser.newPage();
  await registerEntrepreneur(page, email, password);
  await page.close();
});

test.beforeEach(async ({ page }) => {
  await loginEntrepreneur(page, email, password);
});

async function chooseEurWithKeyboard(page: import('@playwright/test').Page, pickerId: string) {
  const picker = page.locator(`[data-picker-id="${pickerId}"]`);
  const menu = page.locator(`#${pickerId}-menu`);
  await picker.locator('[data-currency-toggle]').click();
  const search = menu.locator('[data-currency-search]');

  // Filtering is case-insensitive and exact: a partial query has no match.
  await search.fill('e');
  await expect(menu.locator('[data-currency-empty]')).toBeVisible();
  await search.fill('eUr');
  const visibleOptions = menu.locator('[data-currency-option]:visible');
  await expect(visibleOptions).toHaveCount(1);
  await expect(visibleOptions).toHaveAttribute('data-currency', 'EUR');

  await search.press('ArrowDown');
  await search.press('Enter');
  await expect(menu).toBeHidden();
  await expect.poll(() => menu.evaluate((element, id) =>
    element.parentElement === document.querySelector(`[data-picker-id="${id}"]`), pickerId
  )).toBe(true);
  await expect(picker.locator('[data-currency-value]')).toHaveValue('EUR');
  await expect(picker.locator('[data-currency-label]')).toHaveText('EUR');
}

test('invoice currency picker selects EUR and updates total preview', async ({ page }) => {
  await page.goto('/e/invoices/new');
  await page.locator('[name="item_unit_price[]"]').first().fill('1250');
  await page.locator('[name="item_quantity[]"]').first().fill('2');

  await chooseEurWithKeyboard(page, 'invoice-currency');
  await expect(page.locator('#grand-total-label')).toHaveText('2500,00');
  await expect(page.locator('#grand-total-currency')).toHaveText('EUR');
});

test('entrepreneur KPO currency picker uses exact case-insensitive keyboard filtering', async ({ page }) => {
  await page.goto(`/e/kpo/${CURRENT_YEAR}`);
  await chooseEurWithKeyboard(page, 'currency-new');
  await expect(page.locator('[data-picker-id="currency-new"] [data-currency-value]')).toHaveAttribute('form', 'kpo-new');
});

async function expectMenuInViewport(page: import('@playwright/test').Page, pickerId: string) {
  const menu = page.locator(`#${pickerId}-menu`);
  await expect.poll(() => menu.evaluate((element) => element.parentElement === document.body)).toBe(true);
  const viewport = page.viewportSize();
  expect(viewport).not.toBeNull();
  await expect.poll(async () => {
    const box = await menu.boundingBox();
    return !!box && box.x >= 0 && box.y >= 0 &&
      box.x + box.width <= viewport!.width && box.y + box.height <= viewport!.height;
  }).toBe(true);
  const box = await menu.boundingBox();
  expect(box!.x).toBeGreaterThanOrEqual(0);
  expect(box!.y).toBeGreaterThanOrEqual(0);
  expect(box!.x + box!.width).toBeLessThanOrEqual(viewport!.width);
  expect(box!.y + box!.height).toBeLessThanOrEqual(viewport!.height);
  await expect(menu.locator('[data-currency-option]:visible').first()).toBeVisible();
}

test('KPO bottom-row currency menu is a visible viewport overlay', async ({ page }) => {
  await page.goto(`/e/kpo/${CURRENT_YEAR}`);
  const picker = page.locator('[data-currency-picker]').last();
  await picker.locator('[data-currency-toggle]').scrollIntoViewIfNeeded();
  await picker.locator('[data-currency-toggle]').click();
  await expectMenuInViewport(page, await picker.getAttribute('data-picker-id') || 'currency-new');
});

test('KPO bottom-row currency menu flips above the trigger when needed', async ({ page }) => {
  await page.setViewportSize({width: 800, height: 300});
  await page.goto(`/e/kpo/${CURRENT_YEAR}`);
  const picker = page.locator('[data-picker-id="currency-new"]');
  const toggle = picker.locator('[data-currency-toggle]');
  const menu = page.locator('#currency-new-menu');
  await toggle.click();
  await expect.poll(async () => {
    const toggleBox = await toggle.boundingBox();
    const menuBox = await menu.boundingBox();
    return !!toggleBox && !!menuBox && menuBox.y < toggleBox.y;
  }).toBe(true);
});

test('open KPO currency menu repositions on resize and table scroll', async ({ page }) => {
  await page.goto(`/e/kpo/${CURRENT_YEAR}`);
  const picker = page.locator('[data-picker-id="currency-new"]');
  await picker.locator('[data-currency-toggle]').click();
  const menu = page.locator('#currency-new-menu');
  await menu.locator('[data-currency-search]').fill('eur');
  const before = await menu.boundingBox();
  await page.setViewportSize({width: 420, height: 500});
  await expect(menu.locator('[data-currency-search]')).toHaveValue('eur');
  await expectMenuInViewport(page, 'currency-new');
  const afterResize = await menu.boundingBox();
  expect(afterResize).not.toEqual(before);

  const scroller = page.locator('.overflow-x-auto').filter({has: page.locator('table[aria-label^="КПО"]')});
  await scroller.evaluate((element) => { element.scrollLeft = element.scrollWidth; });
  await expect.poll(async () => {
    const toggleBox = await picker.locator('[data-currency-toggle]').boundingBox();
    const menuBox = await menu.boundingBox();
    if (!toggleBox || !menuBox) return false;
    const viewport = page.viewportSize()!;
    const expectedLeft = Math.max(4, Math.min(toggleBox.x, viewport.width - menuBox.width - 4));
    return Math.abs(menuBox.x - expectedLeft) < 1;
  }).toBe(true);
  await expect(menu.locator('[data-currency-search]')).toHaveValue('eur');
  await expect(menu.locator('[data-currency-option]:visible')).toHaveAttribute('data-currency', 'EUR');
});

test('currency menu closes on click-away and restores its markup position', async ({ page }) => {
  await page.goto(`/e/kpo/${CURRENT_YEAR}`);
  const picker = page.locator('[data-picker-id="currency-new"]');
  const menu = page.locator('#currency-new-menu');
  const toggle = picker.locator('[data-currency-toggle]');
  const search = menu.locator('[data-currency-search]');
  await expect(toggle).toHaveAttribute('aria-controls', 'currency-new-options');
  await expect(toggle).toHaveAttribute('aria-expanded', 'false');
  await expect(search).toHaveAttribute('aria-expanded', 'false');
  await toggle.click();
  await expect(menu).toBeVisible();
  await expect(search).toHaveAttribute('aria-expanded', 'true');
  await expect.poll(() => menu.evaluate((element) => element.parentElement === document.body)).toBe(true);
  await page.locator('body').click({position: {x: 5, y: 5}});
  await expect(menu).toBeHidden();
  await expect.poll(() => menu.evaluate((element, id) =>
    element.parentElement === document.querySelector(`[data-picker-id="${id}"]`), 'currency-new'
  )).toBe(true);
  await expect(picker.locator('[data-currency-toggle]')).toHaveAttribute('aria-expanded', 'false');
});

test('HTMX cleanup closes and restores an open currency menu', async ({ page }) => {
  await page.goto(`/e/kpo/${CURRENT_YEAR}`);
  const picker = page.locator('[data-picker-id="currency-new"]');
  const menu = page.locator('#currency-new-menu');
  await picker.locator('[data-currency-toggle]').click();
  await expect(menu).toBeVisible();
  await page.evaluate(() => {
    const picker = document.querySelector('[data-picker-id="currency-new"]');
    document.dispatchEvent(new CustomEvent('htmx:beforeCleanupElement', {detail: {elt: picker}}));
  });
  await expect(menu).toBeHidden();
  await expect.poll(() => menu.evaluate((element, id) =>
    element.parentElement === document.querySelector(`[data-picker-id="${id}"]`), 'currency-new'
  )).toBe(true);
});

test('currency menu closes on Escape and restores its markup position', async ({ page }) => {
  await page.goto(`/e/kpo/${CURRENT_YEAR}`);
  const picker = page.locator('[data-picker-id="currency-new"]');
  const menu = page.locator('#currency-new-menu');
  await picker.locator('[data-currency-toggle]').click();
  await menu.locator('[data-currency-search]').press('Escape');
  await expect(menu).toBeHidden();
  await expect.poll(() => menu.evaluate((element, id) =>
    element.parentElement === document.querySelector(`[data-picker-id="${id}"]`), 'currency-new'
  )).toBe(true);
  await expect(menu.locator('[data-currency-search]')).toHaveAttribute('aria-expanded', 'false');
});

test('open currency menu remains within a resized mobile visual viewport', async ({ page }) => {
  await page.setViewportSize({width: 375, height: 667});
  await page.goto(`/e/kpo/${CURRENT_YEAR}`);
  const picker = page.locator('[data-picker-id="currency-new"]');
  await picker.locator('[data-currency-toggle]').click();
  const menu = page.locator('#currency-new-menu');
  await page.setViewportSize({width: 320, height: 480});
  await expectMenuInViewport(page, 'currency-new');
  await expect(menu.locator('[data-currency-search]')).toHaveAttribute('aria-expanded', 'true');
});

test('currency menu fits mobile viewport and can select a currency', async ({ page }) => {
  await page.setViewportSize({width: 375, height: 667});
  await page.goto(`/e/kpo/${CURRENT_YEAR}`);
  const picker = page.locator('[data-picker-id="currency-new"]');
  await picker.locator('[data-currency-toggle]').click();
  const menu = page.locator('#currency-new-menu');
  await expectMenuInViewport(page, 'currency-new');
  await menu.locator('[data-currency-option][data-currency="EUR"]').click();
  await expect(picker.locator('[data-currency-value]')).toHaveValue('EUR');
  await expect(menu).toBeHidden();
});
