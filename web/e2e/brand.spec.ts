import { expect, test } from '@playwright/test';

test('header logo has no light padding around its edges', async ({ page }) => {
  await page.goto('/');

  const edgeLightness = await page
    .getByRole('img', { name: 'Fanti' })
    .evaluate(async (image) => {
      await image.decode();
      const canvas = document.createElement('canvas');
      canvas.width = image.naturalWidth;
      canvas.height = image.naturalHeight;
      const context = canvas.getContext('2d');
      if (!context) {
        throw new Error('Could not inspect the header logo');
      }
      context.drawImage(image, 0, 0);

      const edges = [
        [0, canvas.height / 2],
        [canvas.width - 1, canvas.height / 2],
        [canvas.width / 2, 0],
        [canvas.width / 2, canvas.height - 1],
      ];
      return edges.map(([x, y]) => {
        const [red, green, blue] = context.getImageData(x, y, 1, 1).data;
        return (red + green + blue) / 3;
      });
    });

  expect(Math.max(...edgeLightness)).toBeLessThan(180);
});
