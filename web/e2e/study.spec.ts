import { expect, test } from '@playwright/test';

test('lessons tab shows the study mission', async ({ page }) => {
  await page.goto('/study');
  await expect(page.getByLabel('學習使命')).toBeVisible();
});

test('lessons tab shows what the learner can say now', async ({ page }) => {
  await page.goto('/study');

  await test.step('speakable summary card renders', async () => {
    await expect(
      page.getByRole('progressbar', { name: '你現在能說的話' }),
    ).toBeVisible();
  });

  await test.step('nearest-unlockable sentences offer character links', async () => {
    await expect(page.getByText('就差一點')).toBeVisible();
    await expect(
      page.locator('a[href^="/characters/"][aria-label^="待學"]').first(),
    ).toBeVisible();
  });
});

test('flips and grades a flashcard', async ({ page }) => {
  await page.goto('/study?tab=cards');

  const counter = page.getByText(/^\d+ \/ \d+$/);
  const showAnswer = page.getByRole('button', { name: '顯示答案' });

  let positionBefore = '';
  await test.step('flip the first due card', async () => {
    await expect(counter).toBeVisible();
    positionBefore = (await counter.textContent()) ?? '';
    await showAnswer.click();
    await expect(showAnswer).toBeHidden();
  });

  await test.step('grade it 記得 and advance', async () => {
    await page.getByRole('button', { name: '記得', exact: true }).click();
    // The deck advances to the next unflipped prompt once the grade lands.
    await expect(showAnswer).toBeVisible();
    await expect(counter).not.toHaveText(positionBefore);
  });
});

test('starts a quiz and answers the first question', async ({ page }) => {
  await page.goto('/study?tab=quiz');

  await test.step('start the quiz', async () => {
    await page.getByRole('button', { name: '開始測驗' }).click();
    await expect(page.getByText(/^1 \/ \d+$/)).toBeVisible();
    await expect(page.getByText(/^得分 \d+$/)).toBeVisible();
  });

  await test.step('answer the first question', async () => {
    if ((await page.getByText('憑記憶默寫', { exact: true }).count()) > 0) {
      // Lean seed: enter the no-stroke-data fallback, then self-assess.
      const fallback = page.getByRole('button', {
        name: '不看動畫直接練習',
      });
      await expect(fallback).toBeVisible();
      await fallback.click();
      await page.getByRole('button', { name: '顯示範字' }).click();
      await page.getByRole('button', { name: '我寫對了' }).click();
    } else if ((await page.getByRole('textbox').count()) > 0) {
      // IME typing question: any answer moves the quiz along.
      await page.getByRole('textbox').fill('好');
      await page.getByRole('button', { name: '送出' }).click();
    } else {
      // Option-grid question (reading, meaning, script, audio, cloze…).
      await page.locator('.grid-cols-2 > button').first().click();
    }
    await expect(page.getByText(/答對了|正確答案/).first()).toBeVisible();
  });

  await test.step('advance to the next question', async () => {
    await page.getByRole('button', { name: '下一題' }).click();
    await expect(
      page.getByText(/^2 \/ \d+$/).or(page.getByText('測驗完成')),
    ).toBeVisible();
  });
});

test('keeps the eight stroke principles inside their grid cells', async ({
  page,
}) => {
  await page.setViewportSize({ width: 1280, height: 720 });
  await page.goto('/study?tab=strokes');
  await page.getByRole('button', { name: 'EN' }).click();

  const principles = page.locator('button').filter({ hasText: '—' });
  await expect(principles).toHaveCount(8);

  const overflowing = await principles.evaluateAll((buttons) =>
    buttons
      .filter((button) => button.scrollWidth > button.clientWidth)
      .map((button) => button.textContent),
  );
  expect(overflowing).toEqual([]);
});

test('keeps a word inside the flashcard answer tile', async ({ page }) => {
  await page.goto('/study?tab=cards');
  await page.getByRole('button', { name: 'EN' }).click();
  await page.getByRole('button', { name: 'Words' }).click();
  await page.getByRole('button', { name: 'Show answer' }).click();

  const glyph = page.locator('span.font-display').filter({ hasText: /^你好$/ });
  await expect(glyph).toHaveCount(1);

  const fitsTile = await glyph.evaluate((element) => {
    const glyphBounds = element.getBoundingClientRect();
    const tileBounds = element.parentElement?.getBoundingClientRect();
    return (
      tileBounds !== undefined &&
      glyphBounds.left >= tileBounds.left &&
      glyphBounds.right <= tileBounds.right &&
      glyphBounds.top >= tileBounds.top &&
      glyphBounds.bottom <= tileBounds.bottom
    );
  });
  expect(fitsTile).toBe(true);
});
