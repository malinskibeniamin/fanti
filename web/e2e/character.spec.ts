import { expect, test } from '@playwright/test';

const LEAN_SEED = process.env.E2E_LEAN_SEED === '1';

test('calligraphy teaches stroke order or reports missing data', async ({
  page,
}) => {
  await page.goto('/characters/馬');
  await page.getByRole('tab', { name: '書法' }).click();

  if (LEAN_SEED) {
    const calligraphy = page.getByRole('tabpanel', { name: '書法' });
    await expect(calligraphy).toContainText('目前資料來源未收錄筆畫資料。');
    await expect(
      calligraphy.getByRole('button', { name: '觀看筆順' }),
    ).toHaveCount(0);
    return;
  }

  await expect(page.getByRole('button', { name: '觀看筆順' })).toHaveAttribute(
    'aria-pressed',
    'true',
  );
  await page.getByRole('button', { name: '播放', exact: true }).click();
  await expect(
    page.getByRole('button', { name: '暫停', exact: true }),
  ).toBeVisible();
  await page.getByRole('button', { name: '下一筆' }).click();
  await expect(page.getByRole('status')).toContainText('第 1 筆');

  const practiceButton = page.getByRole('button', { name: '練習書寫' });
  const practicePad = page.getByRole('img', { name: '練習筆順 馬' });
  await practiceButton.click();
  await expect(practiceButton).toHaveAttribute('aria-pressed', 'true');
  await expect(practicePad).toBeVisible();

  await page.setViewportSize({ width: 320, height: 720 });
  const frame = practicePad.locator('..');
  const box = await frame.boundingBox();
  expect(box?.width).toBeLessThanOrEqual(280);
  expect(box?.width).toBe(box?.height);
});

// 明 is a frequency-seeded character (not one of the authored fixtures),
// so its sentences prove the Tatoeba corpus fill reached the whole set.
// Its traditional and simplified forms are identical, so the row also
// exists under CI's lean seed (--skip-cedict resolves no traditional
// forms for characters that differ between scripts).
test('frequency-seeded character page shows real example sentences', async ({
  page,
}) => {
  await page.goto('/characters/明');

  await test.step('open the sentences tab', async () => {
    await page.getByRole('tab', { name: '例句' }).click();
  });

  await test.step('corpus sentences render with per-sentence audio', async () => {
    const sentences = page
      .getByRole('tabpanel', { name: '例句' })
      .getByRole('listitem');
    await expect(sentences.first()).toBeVisible();

    const first = sentences.first();
    await expect(first.getByText('明')).toBeVisible();
    await expect(
      first.getByRole('button', { name: /^Pronounce / }),
    ).toBeVisible();
  });
});
