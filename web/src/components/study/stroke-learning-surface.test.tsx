import { act, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, expect, test, vi } from 'vitest';

import { StrokeLearningSurface } from '@/components/study/stroke-learning-surface';
import { useLocaleStore } from '@/i18n/locale';
import { useThemeStore } from '@/stores/theme';

const mocks = vi.hoisted(() => ({ animatorUnmount: vi.fn() }));

vi.mock('@/components/study/stroke-animator', async () => {
  const { useEffect } = await import('react');
  return {
    default: ({ glyph }: { glyph: string }) => {
      useEffect(function trackUnmount() {
        return mocks.animatorUnmount;
      }, []);
      return <div>Animated {glyph}</div>;
    },
  };
});

beforeEach(() => {
  useLocaleStore.setState({ locale: 'en' });
  useThemeStore.setState({ theme: 'light' });
  mocks.animatorUnmount.mockClear();
  vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(null);
});

test('switches one learning surface between animation and freehand practice', async () => {
  const user = userEvent.setup();
  const { container } = render(
    <StrokeLearningSurface
      glyph="馬"
      sizePx={300}
      practiceAriaLabel="Practice strokes 馬"
    />,
  );

  expect(
    screen.getByRole('button', { name: 'Watch stroke order' }),
  ).toHaveAttribute('aria-pressed', 'true');
  expect(container.firstElementChild).toHaveClass('w-full');
  expect(await screen.findByText('Animated 馬')).toBeVisible();

  await user.click(screen.getByRole('button', { name: 'Practice writing' }));

  expect(
    screen.getByRole('img', { name: 'Practice strokes 馬' }),
  ).toBeVisible();
  await waitFor(() =>
    expect(screen.queryByText('Animated 馬')).not.toBeInTheDocument(),
  );
});

test('remounts the renderer after a theme change', async () => {
  render(
    <StrokeLearningSurface
      glyph="馬"
      sizePx={300}
      practiceAriaLabel="Practice strokes 馬"
    />,
  );
  expect(await screen.findByText('Animated 馬')).toBeVisible();

  act(() => useThemeStore.setState({ theme: 'dark' }));

  await waitFor(() => expect(mocks.animatorUnmount).toHaveBeenCalledOnce());
  expect(screen.getByText('Animated 馬')).toBeVisible();
});
