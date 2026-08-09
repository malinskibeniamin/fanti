import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { expect, test } from '@playwright/test';

const API_URL = 'http://localhost:8080';
const FIXTURE = join(
  dirname(fileURLToPath(import.meta.url)),
  'fixtures/sample.txt',
);

test.describe('conversion wizard', () => {
  const createdBooks: string[] = [];
  const createdConversions: string[] = [];

  test.afterEach(async ({ request }) => {
    // The specs mutate the real dev database — remove what this run created.
    for (const name of createdBooks.splice(0)) {
      await request.post(`${API_URL}/fanti.v1.LibraryService/DeleteBook`, {
        data: { name },
      });
    }
    for (const name of createdConversions.splice(0)) {
      await request.post(
        `${API_URL}/fanti.v1.ConversionService/DeleteConversion`,
        { data: { name } },
      );
    }
  });

  test('converts a simplified book and reads it online', async ({ page }) => {
    await test.step('upload the fixture', async () => {
      await page.goto('/convert');
      await page.locator('input[type="file"]').setInputFiles(FIXTURE);
      await page.waitForURL(/\/convert\?c=/);
      const conversionId = new URL(page.url()).searchParams.get('c');
      expect(conversionId).toBeTruthy();
      createdConversions.push(`conversions/${conversionId}`);
    });

    await test.step('step 2 detects 简体 → 繁 direction', async () => {
      await expect(page.getByText('2 設定')).toHaveClass(/bg-primary/);
      await expect(
        page.getByRole('button', { name: /Simplified → Traditional/ }),
      ).toHaveClass(/bg-gold-300\/24/);
      // Both fixture 第一章/第二章 markers were detected.
      await expect(page.getByText('章 2', { exact: true })).toBeVisible();
      // Do not wait for the 500 ms autosave: starting immediately must flush
      // the latest local draft before the conversion changes steps.
      await page.getByLabel('書名').fill('即時儲存測試');
    });

    await test.step('run the conversion and wait for the report', async () => {
      await page.getByRole('button', { name: /開始轉換/ }).click();
      // Stat cards appear once the polled job succeeds; the suite-wide
      // expect timeout (15s) bounds the poll-driven wait.
      await expect(page.getByText('精確對應')).toBeVisible();
      await expect(page.getByText('語境判定').first()).toBeVisible();
      await expect(page.getByText('需人工確認')).toBeVisible();
    });

    await test.step('resolve the 发 exception to 髮', async () => {
      const row = page
        .locator('div.flex.items-start')
        .filter({ has: page.getByText('发', { exact: true }) });
      await row.getByRole('button', { name: '髮', exact: true }).click();
      await expect(row.getByText('已確認')).toBeVisible();
    });

    await test.step('add to library and land on the book detail', async () => {
      await page.getByRole('button', { name: '加入書庫' }).click();
      await page.waitForURL(/\/books\//);
      const bookId = new URL(page.url()).pathname.split('/').pop();
      expect(bookId).toBeTruthy();
      createdBooks.push(`books/${bookId}`);
      await expect(
        page.getByRole('heading', { name: '即時儲存測試' }),
      ).toBeVisible();
      await expect(page.getByRole('link', { name: '線上閱讀' })).toBeVisible();
    });

    await test.step('read online shows the converted 髮 text', async () => {
      await page.getByRole('link', { name: '線上閱讀' }).click();
      await expect(page).toHaveURL(/\/read\//);
      await expect(
        page.locator('ruby').filter({ hasText: '髮' }).first(),
      ).toBeVisible();
      // Chapter detection carried through to the reader heading.
      await expect(page.getByText('第一章')).toBeVisible();
    });
  });
});
