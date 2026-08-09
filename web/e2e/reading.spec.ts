import { expect, test } from '@playwright/test';

// CI seeds without CC-CEDICT, so most tokens lack dictionary entries there.
// Locally the fully-seeded stack backs the strict thresholds.
const LEAN_SEED = process.env.E2E_LEAN_SEED === '1';
const MIN_RUBY_COUNT = LEAN_SEED ? 10 : 50;
const MIN_VISIBLE_RT_COUNT = LEAN_SEED ? 5 : 50;

const PINYIN_RE = /[a-zāáǎàēéěèīíǐìōóǒòūúǔùǖǘǚǜü]/i;
const CJK_RE = /[一-鿿]/;

test('reads a graded story with pinyin settings and tap-to-define', async ({
  page,
}) => {
  await test.step('open the library', async () => {
    await page.goto('/');
    await expect(page.getByRole('heading', { name: '書庫' })).toBeVisible();
  });

  await test.step('open the 我的一天 graded story', async () => {
    await page.getByRole('link', { name: /我的一天/ }).click();
    await expect(page).toHaveURL(/\/read\//);
    await expect
      .poll(() => page.locator('ruby').count(), {
        message: `reader should show more than ${MIN_RUBY_COUNT} ruby tokens`,
      })
      .toBeGreaterThan(MIN_RUBY_COUNT);
  });

  const settingsButton = page.getByRole('button', { name: 'Reader settings' });
  const settingsSheet = page.getByRole('dialog', { name: '閱讀設定' });

  await test.step('switch pinyin to 全部 in reader settings', async () => {
    await settingsButton.click();
    await expect(settingsSheet).toBeVisible();
    await expect(settingsSheet).toHaveAttribute('aria-modal', 'true');

    const allChip = settingsSheet.getByRole('button', {
      name: '全部',
      exact: true,
    });
    await allChip.click();
    await expect(allChip).toHaveAttribute('aria-pressed', 'true');
    await expect
      .poll(() => page.locator('rt:visible').count(), {
        message: 'pinyin annotations should become visible in 全部 mode',
      })
      .toBeGreaterThan(MIN_VISIBLE_RT_COUNT);
  });

  await test.step('Escape closes settings and restores focus', async () => {
    await page.keyboard.press('Escape');
    await expect(settingsSheet).toBeHidden();
    await expect(settingsButton).toBeFocused();
  });

  const dictionarySheet = page.getByRole('dialog', { name: '字典' });
  const dictionaryToken = page
    .getByRole('button', { name: /^Look up / })
    .first();

  await test.step('tap a gold token to open the dictionary', async () => {
    await dictionaryToken.click();

    await expect(dictionarySheet).toBeVisible();
    await expect(
      dictionarySheet.getByRole('button', { name: '加入記憶庫' }),
    ).toBeVisible();
    // The 田-grid tile shows the tapped glyph.
    await expect(
      dictionarySheet.locator('span.font-display').first(),
    ).toHaveText(CJK_RE);
    if (!LEAN_SEED) {
      // Pronunciation requires the CEDICT-backed dictionary entry.
      await expect(
        dictionarySheet.locator('span.font-semibold.text-lg').first(),
      ).toHaveText(PINYIN_RE);
    }
  });

  await test.step('Escape closes the dictionary and releases focus', async () => {
    await page.keyboard.press('Escape');
    await expect(dictionarySheet).toBeHidden();
    await expect(dictionaryToken).toBeFocused();
  });
});
