import { Code, ConnectError } from '@connectrpc/connect';
import { act, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, expect, test, vi } from 'vitest';

import { StrokeAnimator } from '@/components/study/stroke-animator';
import { useLocaleStore } from '@/i18n/locale';

const mocks = vi.hoisted(() => {
  let onAnimationComplete:
    | ((result: { canceled: boolean }) => void)
    | undefined;
  let onLoadError: (() => void) | undefined;
  let onLoadSuccess: (() => void) | undefined;
  let autoLoad = true;
  const writer = {
    animateCharacter: vi.fn(
      (options?: { onComplete?: (result: { canceled: boolean }) => void }) => {
        onAnimationComplete = options?.onComplete;
        return Promise.resolve({ canceled: false });
      },
    ),
    animateStroke: vi.fn(),
    hideCharacter: vi.fn(),
    pauseAnimation: vi.fn(),
    resumeAnimation: vi.fn(),
    showCharacter: vi.fn(),
  };

  return {
    createWriter: vi.fn(
      (
        _target: HTMLElement,
        _glyph: string,
        options?: {
          onLoadCharDataError?: () => void;
          onLoadCharDataSuccess?: () => void;
        },
      ) => {
        onLoadError = options?.onLoadCharDataError;
        onLoadSuccess = options?.onLoadCharDataSuccess;
        if (autoLoad) {
          onLoadSuccess?.();
        }
        return writer;
      },
    ),
    failRenderer: () => onLoadError?.(),
    finishRendererLoad: () => onLoadSuccess?.(),
    finishAnimation: (result: { canceled: boolean }) =>
      onAnimationComplete?.(result),
    resetCallbacks: () => {
      onAnimationComplete = undefined;
      onLoadError = undefined;
      onLoadSuccess = undefined;
    },
    setAutoLoad: (value: boolean) => {
      autoLoad = value;
    },
    useQuery: vi.fn(),
    writer,
  };
});

vi.mock('@connectrpc/connect-query', () => ({ useQuery: mocks.useQuery }));
vi.mock('hanzi-writer', () => ({
  default: { create: mocks.createWriter },
}));

const STROKE_DATA = JSON.stringify({
  strokes: ['M 0 0', 'M 1 1'],
  medians: [
    [
      [0, 0],
      [1, 1],
    ],
    [
      [1, 0],
      [0, 1],
    ],
  ],
});

beforeEach(() => {
  vi.clearAllMocks();
  mocks.resetCallbacks();
  mocks.setAutoLoad(true);
  useLocaleStore.setState({ locale: 'en' });
  mocks.useQuery.mockReturnValue({
    data: { data: STROKE_DATA },
    isError: false,
    isPending: false,
    refetch: vi.fn(),
  });
  mocks.writer.animateStroke.mockResolvedValue({ canceled: false });
  mocks.writer.hideCharacter.mockResolvedValue({ canceled: false });
  mocks.writer.pauseAnimation.mockResolvedValue(undefined);
  mocks.writer.resumeAnimation.mockResolvedValue(undefined);
  mocks.writer.showCharacter.mockResolvedValue({ canceled: false });
});

test('starts static and plays the full stroke order on demand', async () => {
  const user = userEvent.setup();
  const { container } = render(<StrokeAnimator glyph="馬" sizePx={300} />);

  await waitFor(() => expect(mocks.createWriter).toHaveBeenCalledOnce());
  expect(container.firstElementChild).toHaveClass('w-full');
  expect(mocks.writer.animateCharacter).not.toHaveBeenCalled();

  await user.click(screen.getByRole('button', { name: 'Play' }));

  expect(mocks.writer.animateCharacter).toHaveBeenCalledOnce();
});

test('reports when the learner finishes watching the stroke order', async () => {
  const user = userEvent.setup();
  const onComplete = vi.fn();
  render(<StrokeAnimator glyph="馬" sizePx={300} onComplete={onComplete} />);
  await waitFor(() => expect(mocks.createWriter).toHaveBeenCalledOnce());

  await user.click(screen.getByRole('button', { name: 'Play' }));
  act(() => mocks.finishAnimation({ canceled: false }));

  expect(onComplete).toHaveBeenCalledOnce();
});

test('waits for the renderer before enabling playback', async () => {
  mocks.setAutoLoad(false);
  render(<StrokeAnimator glyph="馬" sizePx={300} />);
  await waitFor(() => expect(mocks.createWriter).toHaveBeenCalledOnce());

  const play = screen.getByRole('button', { name: 'Play' });
  expect(play).toBeDisabled();

  act(() => mocks.finishRendererLoad());

  await waitFor(() => expect(play).toBeEnabled());
});

test('pauses and resumes the current animation', async () => {
  const user = userEvent.setup();
  render(<StrokeAnimator glyph="馬" sizePx={300} />);
  await waitFor(() => expect(mocks.createWriter).toHaveBeenCalledOnce());

  await user.click(screen.getByRole('button', { name: 'Play' }));
  await user.click(screen.getByRole('button', { name: 'Pause' }));
  await user.click(screen.getByRole('button', { name: 'Play' }));

  expect(mocks.writer.pauseAnimation).toHaveBeenCalledOnce();
  expect(mocks.writer.resumeAnimation).toHaveBeenCalledOnce();
  expect(mocks.writer.animateCharacter).toHaveBeenCalledOnce();
});

test('steps forward one stroke at a time', async () => {
  const user = userEvent.setup();
  render(<StrokeAnimator glyph="馬" sizePx={300} />);
  await waitFor(() => expect(mocks.createWriter).toHaveBeenCalledOnce());

  await user.click(screen.getByRole('button', { name: 'Next stroke' }));
  await waitFor(() =>
    expect(mocks.writer.animateStroke).toHaveBeenLastCalledWith(0),
  );
  await user.click(screen.getByRole('button', { name: 'Next stroke' }));
  await waitFor(() =>
    expect(mocks.writer.animateStroke).toHaveBeenLastCalledWith(1),
  );

  expect(mocks.writer.hideCharacter).toHaveBeenCalledOnce();
  expect(screen.getByRole('button', { name: 'Next stroke' })).toBeDisabled();
});

test('counts stepping through every stroke as watching the character', async () => {
  const user = userEvent.setup();
  const onComplete = vi.fn();
  render(<StrokeAnimator glyph="馬" sizePx={300} onComplete={onComplete} />);
  await waitFor(() => expect(mocks.createWriter).toHaveBeenCalledOnce());

  await user.click(screen.getByRole('button', { name: 'Next stroke' }));
  await waitFor(() => expect(mocks.writer.animateStroke).toHaveBeenCalled());
  await user.click(screen.getByRole('button', { name: 'Next stroke' }));
  await waitFor(() => expect(onComplete).toHaveBeenCalledOnce());
});

test('announces step progress', async () => {
  const user = userEvent.setup();
  render(<StrokeAnimator glyph="馬" sizePx={300} />);
  await waitFor(() => expect(mocks.createWriter).toHaveBeenCalledOnce());

  await user.click(screen.getByRole('button', { name: 'Next stroke' }));

  await waitFor(() =>
    expect(screen.getByRole('status')).toHaveTextContent('Stroke 1 of 2'),
  );
});

test('replays the full stroke order after it finishes', async () => {
  const user = userEvent.setup();
  render(<StrokeAnimator glyph="馬" sizePx={300} />);
  await waitFor(() => expect(mocks.createWriter).toHaveBeenCalledOnce());

  await user.click(screen.getByRole('button', { name: 'Play' }));
  act(() => mocks.finishAnimation({ canceled: false }));
  await user.click(screen.getByRole('button', { name: 'Replay' }));

  expect(mocks.writer.animateCharacter).toHaveBeenCalledTimes(2);
});

test('falls back to a static glyph when stroke data is unavailable', () => {
  mocks.useQuery.mockReturnValue({
    data: undefined,
    error: new ConnectError('missing', Code.NotFound),
    isError: true,
    isPending: false,
    refetch: vi.fn(),
  });

  render(<StrokeAnimator glyph="馬" sizePx={300} />);

  const glyph = screen.getByRole('img', { name: 'Strokes 馬' });
  expect(glyph).toBeVisible();
  expect(glyph).toHaveClass('pointer-events-none');
  expect(glyph.parentElement?.parentElement).toHaveClass('w-full');
  expect(
    screen.getByText('Stroke animation is not available for this character.'),
  ).toBeVisible();
  expect(
    screen.queryByRole('button', { name: 'Play' }),
  ).not.toBeInTheDocument();
});

test('offers a caller-provided fallback action when animation is unavailable', () => {
  mocks.useQuery.mockReturnValue({
    data: undefined,
    error: new ConnectError('missing', Code.NotFound),
    isError: true,
    isPending: false,
    refetch: vi.fn(),
  });

  render(
    <StrokeAnimator
      glyph="馬"
      sizePx={300}
      fallbackAction={<button type="button">Practice without animation</button>}
    />,
  );

  expect(
    screen.getByRole('button', { name: 'Practice without animation' }),
  ).toBeVisible();
});

test('shows a loading state before stroke data arrives', () => {
  mocks.useQuery.mockReturnValue({
    data: undefined,
    isError: false,
    isPending: true,
    refetch: vi.fn(),
  });

  render(<StrokeAnimator glyph="馬" sizePx={300} />);

  expect(
    screen.getByRole('status', { name: 'Loading stroke order' }),
  ).toBeVisible();
  expect(
    screen.queryByRole('button', { name: 'Play' }),
  ).not.toBeInTheDocument();
});

test('shows a retryable error when stroke data cannot load', async () => {
  const refetch = vi.fn();
  mocks.useQuery.mockReturnValue({
    data: undefined,
    error: new ConnectError('offline', Code.Unavailable),
    isError: true,
    isPending: false,
    refetch,
  });
  const user = userEvent.setup();

  render(<StrokeAnimator glyph="馬" sizePx={300} />);
  await user.click(screen.getByRole('button', { name: 'Try again' }));

  expect(screen.getByRole('alert')).toHaveTextContent(
    'Could not load stroke animation',
  );
  expect(refetch).toHaveBeenCalledOnce();
});

test('rejects malformed stroke data instead of passing it to the renderer', () => {
  mocks.useQuery.mockReturnValue({
    data: { data: '{"strokes": [7]}' },
    isError: false,
    isPending: false,
    refetch: vi.fn(),
  });

  render(<StrokeAnimator glyph="馬" sizePx={300} />);

  expect(screen.getByRole('alert')).toHaveTextContent(
    'Could not load stroke animation',
  );
  expect(mocks.createWriter).not.toHaveBeenCalled();
  expect(
    screen.queryByRole('button', { name: 'Play' }),
  ).not.toBeInTheDocument();
});

test('retries when the stroke renderer rejects valid data', async () => {
  const user = userEvent.setup();
  render(<StrokeAnimator glyph="馬" sizePx={300} />);
  await waitFor(() => expect(mocks.createWriter).toHaveBeenCalledOnce());

  act(() => mocks.failRenderer());
  await user.click(screen.getByRole('button', { name: 'Try again' }));

  await waitFor(() => expect(mocks.createWriter).toHaveBeenCalledTimes(2));
});
