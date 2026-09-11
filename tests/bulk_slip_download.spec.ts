import { test, expect, Page } from '@playwright/test';
import { readFile } from 'node:fs/promises';
import { inflateRawSync } from 'node:zlib';
import {
  createAccountantSlip,
  createEntrepreneur,
  createEntrepreneurSlip,
  login,
  loginEntrepreneur,
  pairEntrepreneur,
  registerEntrepreneur,
} from './helpers';

async function assertZipDownload(
  page: Page,
  expectedZipFilename: string,
  expectedPdfEntryNames: string[],
) {
  const downloadPromise = page.waitForEvent('download');
  await page.locator('#btn-download-selected').click();
  const download = await downloadPromise;
  expect(download.suggestedFilename()).toBe(expectedZipFilename);
  const path = await download.path();
  expect(path).not.toBeNull();
  const bytes = await readFile(path!);
  expect(bytes.subarray(0, 4).toString('binary')).toBe('PK\x03\x04');

  const endOfCentralDirectory = bytes.lastIndexOf(Buffer.from('PK\x05\x06', 'binary'));
  expect(endOfCentralDirectory).toBeGreaterThanOrEqual(0);
  const entryCount = bytes.readUInt16LE(endOfCentralDirectory + 10);
  const centralDirectoryOffset = bytes.readUInt32LE(endOfCentralDirectory + 16);
  expect(entryCount).toBe(2);

  let cursor = centralDirectoryOffset;
  const entries: string[] = [];
  for (let i = 0; i < entryCount; i += 1) {
    expect(bytes.subarray(cursor, cursor + 4).toString('binary')).toBe('PK\x01\x02');
    const compressionMethod = bytes.readUInt16LE(cursor + 10);
    const compressedSize = bytes.readUInt32LE(cursor + 20);
    const fileNameLength = bytes.readUInt16LE(cursor + 28);
    const extraLength = bytes.readUInt16LE(cursor + 30);
    const commentLength = bytes.readUInt16LE(cursor + 32);
    const localHeaderOffset = bytes.readUInt32LE(cursor + 42);
    const fileName = bytes
      .subarray(cursor + 46, cursor + 46 + fileNameLength)
      .toString('utf8');
    entries.push(fileName);

    expect(bytes.subarray(localHeaderOffset, localHeaderOffset + 4).toString('binary')).toBe('PK\x03\x04');
    const localFileNameLength = bytes.readUInt16LE(localHeaderOffset + 26);
    const localExtraLength = bytes.readUInt16LE(localHeaderOffset + 28);
    const payloadStart = localHeaderOffset + 30 + localFileNameLength + localExtraLength;
    const compressedPayload = bytes.subarray(payloadStart, payloadStart + compressedSize);
    const payload = compressionMethod === 8
      ? inflateRawSync(compressedPayload)
      : compressedPayload;
    expect(payload.subarray(0, 5).toString('ascii')).toBe('%PDF-');

    cursor += 46 + fileNameLength + extraLength + commentLength;
  }
  expect(entries).toEqual(expectedPdfEntryNames);
}

test.describe('entrepreneur bulk slip ZIP download', () => {
  const email = `bulk-slips-${Date.now()}@test.local`;
  const password = 'bulk-slips-pass-123';
  let firstPurpose = '';
  let secondPurpose = '';

  test.beforeAll(async ({ browser }) => {
    const page = await browser.newPage();
    await registerEntrepreneur(page, email, password);
    firstPurpose = `zip-entrepreneur-1-${Date.now()}`;
    secondPurpose = `zip-entrepreneur-2-${Date.now()}`;
    await createEntrepreneurSlip(page, firstPurpose);
    await createEntrepreneurSlip(page, secondPurpose);
    await page.close();
  });

  test('select-all downloads selected slips as a ZIP', async ({ page }) => {
    await loginEntrepreneur(page, email, password);
    await page.goto('/e/slips');

    const button = page.locator('#btn-download-selected');
    await expect(button).toBeHidden();
    const table = page.locator('table').first();
    const selectAll = table.locator('.slip-select-all');
    await selectAll.check();
    await expect(selectAll).toBeChecked();
    await expect(table.locator('.slip-select:checked')).toHaveCount(2);
    await expect(button).toBeVisible();

    await assertZipDownload(page, 'uplatnice-slip.zip', [
      `uplatnica-slip-${firstPurpose}.pdf`,
      `uplatnica-slip-${secondPurpose}.pdf`,
    ]);
  });
});

test.describe('accountant managed-entrepreneur bulk slip ZIP download', () => {
  const entrepreneurEmail = `managed-bulk-slips-${Date.now()}@test.local`;
  const entrepreneurPassword = 'managed-bulk-slips-pass-123';
  let entrepreneurPath: string;
  let entrepreneurName = '';
  let firstPurpose = '';
  let secondPurpose = '';

  test.beforeAll(async ({ browser }) => {
    const accountantPage = await browser.newPage();
    const entrepreneurPage = await browser.newPage();
    await login(accountantPage);
    await registerEntrepreneur(entrepreneurPage, entrepreneurEmail, entrepreneurPassword);
    entrepreneurName = `zip-managed-${Date.now()}`;
    firstPurpose = `zip-managed-1-${Date.now()}`;
    secondPurpose = `zip-managed-2-${Date.now()}`;
    entrepreneurPath = await createEntrepreneur(accountantPage, entrepreneurName);
    const pathParts = entrepreneurPath.split('/').filter(Boolean);
    const entrepreneurID = pathParts[pathParts.length - 1];
    await pairEntrepreneur(accountantPage, entrepreneurPage, entrepreneurID);
    await createAccountantSlip(accountantPage, entrepreneurPath, firstPurpose);
    await createAccountantSlip(accountantPage, entrepreneurPath, secondPurpose);
    await accountantPage.close();
    await entrepreneurPage.close();
  });

  test('select-all downloads managed entrepreneur slips as a ZIP', async ({ page }) => {
    await login(page);
    await page.goto(entrepreneurPath);

    const button = page.locator('#btn-download-selected');
    await expect(button).toBeHidden();
    const table = page.locator('#slip-tab-latest table').first();
    await table.locator('.slip-select-all').check();
    await expect(table.locator('.slip-select:checked')).toHaveCount(2);
    await expect(button).toBeVisible();

    await assertZipDownload(page, `uplatnice-${entrepreneurName}.zip`, [
      `uplatnica-${entrepreneurName}-${firstPurpose}.pdf`,
      `uplatnica-${entrepreneurName}-${secondPurpose}.pdf`,
    ]);
  });
});
