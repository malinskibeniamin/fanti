import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import {
  type BrowserContext,
  expect,
  type Locator,
  type Page,
  test,
} from '@playwright/test';

const API_URL = 'http://localhost:8080';
const FIXTURE = join(
  dirname(fileURLToPath(import.meta.url)),
  'fixtures/sample.txt',
);

test.use({ viewport: { width: 375, height: 812 }, hasTouch: true });

async function expectNoHorizontalScroll(page: Page, surface: string) {
  await expect
    .poll(
      () =>
        page.evaluate(() => ({
          viewportWidth: document.documentElement.clientWidth,
          contentWidth: document.documentElement.scrollWidth,
        })),
      { message: `${surface} should not scroll horizontally` },
    )
    .toEqual({ viewportWidth: 375, contentWidth: 375 });
}

async function expectBottomNavigationClear(page: Page, surface: string) {
  const bottomNav = page.locator('nav.fixed');
  await expect(bottomNav).toBeVisible();

  await expect
    .poll(
      async () => {
        await page.evaluate(() =>
          window.scrollTo(0, document.documentElement.scrollHeight),
        );
        return page.locator('main > section').evaluate((section) => {
          const nav = document.querySelector<HTMLElement>('nav.fixed');
          if (!nav) {
            return 0;
          }
          return Math.max(
            0,
            section.getBoundingClientRect().bottom -
              nav.getBoundingClientRect().top,
          );
        });
      },
      { message: `${surface} content should clear the bottom nav` },
    )
    .toBe(0);
}

async function openPage(context: BrowserContext, path: string) {
  const page = await context.newPage();
  await page.goto(path);
  await expect(page.locator('main > section')).toBeVisible();
  await expect(page.locator('main [data-slot="skeleton"]')).toHaveCount(0);
  return page;
}

test('primary routes fit a 375px viewport', async ({ context, page }) => {
  await page.goto('/');
  await expect(page.getByRole('heading', { name: '書庫' })).toBeVisible();
  const bookPath = await page
    .locator('a[href^="/books/"]')
    .first()
    .getAttribute('href');
  expect(bookPath).toBeTruthy();
  await expectNoHorizontalScroll(page, 'Library');
  await expectBottomNavigationClear(page, 'Library');
  const bookPage = await openPage(context, bookPath ?? '/');
  await expect(bookPage.getByRole('link', { name: '線上閱讀' })).toBeVisible();
  await expectNoHorizontalScroll(bookPage, 'Book');
  await bookPage.close();

  const convertPage = await openPage(context, '/convert');
  await expect(
    convertPage.getByRole('button', { name: '點選或拖入檔案' }),
  ).toBeVisible();
  await expectNoHorizontalScroll(convertPage, 'Convert upload');
  await expectBottomNavigationClear(convertPage, 'Convert upload');
  await convertPage.close();
});

test('every study tab fits and navigation stays on one row', async ({
  context,
}) => {
  for (const tab of [
    'lessons',
    'cards',
    'quiz',
    'strokes',
    'dict',
    'origins',
  ]) {
    const page = await openPage(context, `/study?tab=${tab}`);
    const tabs = page.getByRole('navigation', { name: '練習' });
    await expect(tabs).toBeVisible();
    await expectNoHorizontalScroll(page, `Study ${tab}`);
    await expectBottomNavigationClear(page, `Study ${tab}`);

    const tabRows = await tabs
      .getByRole('link')
      .evaluateAll(
        (links) =>
          new Set(
            links.map((link) => Math.round(link.getBoundingClientRect().top)),
          ).size,
      );
    expect(tabRows).toBe(1);

    await expect
      .poll(
        () =>
          tabs.locator('a[data-status="active"]').evaluate((activeTab) => {
            const navigation = activeTab.parentElement;
            if (!navigation) {
              return false;
            }
            const activeBox = activeTab.getBoundingClientRect();
            const navigationBox = navigation.getBoundingClientRect();
            return (
              activeBox.left >= navigationBox.left &&
              activeBox.right <= navigationBox.right
            );
          }),
        { message: `Study ${tab} should show its selected tab` },
      )
      .toBe(true);
    await page.close();
  }
});

test('discover, character, and guide surfaces fit a phone', async ({
  context,
}) => {
  for (const tab of ['chars', 'words', 'guides']) {
    const page = await openPage(context, `/discover?tab=${tab}`);
    await expect(page.getByRole('navigation', { name: '探索' })).toBeVisible();
    await expectNoHorizontalScroll(page, `Discover ${tab}`);
    await expectBottomNavigationClear(page, `Discover ${tab}`);
    await page.close();
  }

  const page = await openPage(context, '/characters/馬');
  for (const tab of ['字源', '字形演變', '書法', '例句']) {
    await page.getByRole('tab', { name: tab, exact: true }).click();
    await expectNoHorizontalScroll(page, `Character ${tab}`);
  }
  await page.close();

  for (const guide of ['pinyin', 'zhuyin', 'typing', 'strokes']) {
    const guidePage = await openPage(context, `/guides/${guide}`);
    await expectNoHorizontalScroll(guidePage, `Guide ${guide}`);
    await guidePage.close();
  }
});

test('reader toolbar and both sheets fit the mobile viewport', async ({
  page,
}) => {
  await page.goto('/');
  await page.getByRole('link', { name: /我的一天/ }).click();
  await expect(page.locator('ruby').first()).toBeVisible();
  await expectNoHorizontalScroll(page, 'Reader');
  await expect(
    page.locator('main section > div.bg-reading-background'),
  ).toHaveCSS('padding-left', '16px');

  const settingsButton = page.getByRole('button', {
    name: 'Reader settings',
  });
  await settingsButton.click();
  const settingsSheet = page.getByRole('dialog', { name: '閱讀設定' });
  await expect(settingsSheet).toBeVisible();
  await expectSheetFitsViewport(settingsSheet);
  await page.keyboard.press('Escape');

  await page
    .getByRole('button', { name: /^Look up / })
    .first()
    .click();
  const dictionarySheet = page.getByRole('dialog', { name: '字典' });
  await expect(dictionarySheet).toBeVisible();
  await expectSheetFitsViewport(dictionarySheet);
  await expect(dictionarySheet).toHaveCSS('overflow-y', 'auto');
  await expectNoHorizontalScroll(page, 'Reader dictionary');
});

test('stroke practice pad is finger-sized and touch-ready', async ({
  page,
}) => {
  await page.goto('/study?tab=strokes');
  await page.getByRole('button', { name: '練習書寫' }).click();

  const canvas = page.getByTestId('stroke-pad-canvas');
  await expect(canvas).toBeVisible();
  const box = await canvas.boundingBox();
  expect(box?.width).toBeGreaterThanOrEqual(260);
  expect(box?.width).toBeLessThanOrEqual(335);
  expect(box?.width).toBe(box?.height);
  await expect(canvas).toHaveCSS('touch-action', 'none');
});

test('all conversion steps fit a phone', async ({ page, request }) => {
  let conversionName = '';

  try {
    await page.goto('/convert');
    await page.locator('input[type="file"]').setInputFiles(FIXTURE);
    await page.waitForURL(/\/convert\?c=/);
    const conversionId = new URL(page.url()).searchParams.get('c');
    expect(conversionId).toBeTruthy();
    conversionName = `conversions/${conversionId}`;

    await expect(page.getByText('2 設定')).toHaveClass(/bg-primary/);
    await expectNoHorizontalScroll(page, 'Convert settings');
    const previewRows = await page
      .locator(
        'button[aria-label="封面"], button[aria-label="目錄"], button[aria-label="扉頁"]',
      )
      .evaluateAll(
        (previews) =>
          new Set(
            previews.map((preview) =>
              Math.round(preview.getBoundingClientRect().top),
            ),
          ).size,
      );
    expect(previewRows, 'Convert previews should stay on one row').toBe(1);

    await page.getByRole('button', { name: /開始轉換/ }).click();
    await expect(page.getByText('精確對應')).toBeVisible();
    await expectNoHorizontalScroll(page, 'Convert report');
  } finally {
    if (conversionName) {
      await request.post(
        `${API_URL}/fanti.v1.ConversionService/DeleteConversion`,
        { data: { name: conversionName } },
      );
    }
  }
});

async function expectSheetFitsViewport(sheet: Locator) {
  const box = await sheet.boundingBox();
  expect(box?.x).toBeGreaterThanOrEqual(0);
  expect(box?.width).toBeLessThanOrEqual(375);
  expect(box?.y).toBeGreaterThanOrEqual(0);
  expect(box?.height).toBeLessThanOrEqual(812);
}
