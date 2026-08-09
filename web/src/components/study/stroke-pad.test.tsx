import { render, screen } from '@testing-library/react';
import { beforeEach, expect, test, vi } from 'vitest';

import { StrokePad } from '@/components/study/stroke-pad';
import { useLocaleStore } from '@/i18n/locale';

beforeEach(() => {
  useLocaleStore.setState({ locale: 'en' });
  vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(null);
});

test('includes the expected stroke total in the practice counter', () => {
  render(
    <StrokePad
      sizePx={300}
      ariaLabel="Practice strokes 馬"
      expectedStrokeCount={10}
    />,
  );

  expect(screen.getByText('Strokes 0 / 10')).toBeVisible();
});

test('keeps the practice grid square when its container is narrower', () => {
  const { container } = render(
    <StrokePad sizePx={300} ariaLabel="Practice strokes 馬" />,
  );

  const frame = screen.getByRole('img', {
    name: 'Practice strokes 馬',
  }).parentElement;
  expect(frame).toHaveClass('aspect-square', 'w-full');
  expect(frame).toHaveStyle({ maxWidth: '300px' });
  expect(frame).not.toHaveStyle({ height: '300px' });
  expect(container.firstElementChild).toHaveClass('w-full');
});
