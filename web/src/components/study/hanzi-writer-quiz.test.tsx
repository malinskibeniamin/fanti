import { Code, ConnectError } from '@connectrpc/connect';
import { act, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, expect, test, vi } from 'vitest';

import { HanziWriterQuiz } from '@/components/study/hanzi-writer-quiz';
import { useLocaleStore } from '@/i18n/locale';

const mocks = vi.hoisted(() => {
  let onLoadError: (() => void) | undefined;
  let onLoadSuccess: (() => void) | undefined;
  let quizOptions:
    | {
        onComplete?: () => void;
        onCorrectStroke?: (stroke: {
          drawnPath: { pathString: string };
          strokeNum: number;
          strokesRemaining: number;
        }) => void;
        onMistake?: (stroke: { strokeNum: number }) => void;
      }
    | undefined;
  const writer = {
    cancelQuiz: vi.fn(),
    highlightStroke: vi.fn(),
    quiz: vi.fn(
      (options?: {
        onComplete?: () => void;
        onCorrectStroke?: (stroke: {
          drawnPath: { pathString: string };
          strokeNum: number;
          strokesRemaining: number;
        }) => void;
        onMistake?: (stroke: { strokeNum: number }) => void;
      }) => {
        quizOptions = options;
        return Promise.resolve(undefined);
      },
    ),
  };

  return {
    createWriter: vi.fn(
      (
        target: HTMLElement,
        _glyph: string,
        options?: {
          onLoadCharDataError?: () => void;
          onLoadCharDataSuccess?: () => void;
          showOutline?: boolean;
        },
      ) => {
        onLoadError = options?.onLoadCharDataError;
        onLoadSuccess = options?.onLoadCharDataSuccess;
        const svg = document.createElementNS(
          'http://www.w3.org/2000/svg',
          'svg',
        );
        svg.dataset.outline = String(options?.showOutline);
        target.append(svg);
        onLoadSuccess?.();
        return writer;
      },
    ),
    correctStroke: (stroke: {
      drawnPath: { pathString: string };
      strokeNum: number;
      strokesRemaining: number;
    }) => quizOptions?.onCorrectStroke?.(stroke),
    finishQuiz: () => quizOptions?.onComplete?.(),
    failRenderer: () => onLoadError?.(),
    mistake: (strokeNum: number) => quizOptions?.onMistake?.({ strokeNum }),
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
  useLocaleStore.setState({ locale: 'en' });
  mocks.useQuery.mockReturnValue({
    data: { data: STROKE_DATA },
    isError: false,
    isPending: false,
    refetch: vi.fn(),
  });
  mocks.writer.highlightStroke.mockResolvedValue({ canceled: false });
});

test('starts a memory quiz and highlights the next stroke on request', async () => {
  const user = userEvent.setup();
  render(<HanziWriterQuiz glyph="馬" sizePx={260} onComplete={vi.fn()} />);

  await waitFor(() => expect(mocks.writer.quiz).toHaveBeenCalledOnce());
  expect(mocks.createWriter).toHaveBeenCalledOnce();
  await user.click(screen.getByRole('button', { name: 'Hint next stroke' }));

  expect(mocks.writer.highlightStroke).toHaveBeenCalledWith(0);

  act(() =>
    mocks.correctStroke({
      drawnPath: { pathString: 'M 10 20 L 30 40' },
      strokeNum: 0,
      strokesRemaining: 1,
    }),
  );
  await user.click(screen.getByRole('button', { name: 'Hint next stroke' }));

  expect(mocks.writer.highlightStroke).toHaveBeenLastCalledWith(1);
});

test('requires strict stroke matching without showing automatic success', async () => {
  render(<HanziWriterQuiz glyph="馬" sizePx={260} onComplete={vi.fn()} />);

  await waitFor(() => expect(mocks.writer.quiz).toHaveBeenCalledOnce());
  expect(mocks.writer.quiz).toHaveBeenCalledWith(
    expect.objectContaining({
      highlightOnComplete: false,
      leniency: 0.5,
      markStrokeCorrectAfterMisses: false,
      showHintAfterMisses: false,
    }),
  );
});

test('hides the character outline for mastery practice', async () => {
  render(
    <HanziWriterQuiz
      glyph="馬"
      sizePx={260}
      showOutline={false}
      onComplete={vi.fn()}
    />,
  );

  await waitFor(() => expect(mocks.createWriter).toHaveBeenCalledOnce());
  expect(mocks.createWriter).toHaveBeenCalledWith(
    expect.any(HTMLElement),
    '馬',
    expect.objectContaining({ showOutline: false }),
  );
  expect(
    screen.queryByRole('button', { name: 'Hint next stroke' }),
  ).not.toBeInTheDocument();
});

test('restarts practice when mastery temporarily enables recall hints', async () => {
  const onComplete = vi.fn();
  const { rerender } = render(
    <HanziWriterQuiz
      glyph="馬"
      sizePx={260}
      showOutline={false}
      onComplete={onComplete}
    />,
  );
  await waitFor(() => expect(mocks.writer.quiz).toHaveBeenCalledOnce());

  rerender(
    <HanziWriterQuiz
      glyph="馬"
      sizePx={260}
      showOutline
      onComplete={onComplete}
    />,
  );

  await waitFor(() => expect(mocks.createWriter).toHaveBeenCalledTimes(2));
  expect(mocks.writer.quiz).toHaveBeenCalledTimes(2);
  expect(
    screen
      .getByRole('img', { name: 'Write it from memory 馬' })
      .querySelectorAll('svg'),
  ).toHaveLength(1);
});

test('returns the accepted drawing in canvas coordinates when complete', async () => {
  const onComplete = vi.fn();
  render(<HanziWriterQuiz glyph="馬" sizePx={260} onComplete={onComplete} />);
  await waitFor(() => expect(mocks.writer.quiz).toHaveBeenCalledOnce());

  act(() => {
    mocks.correctStroke({
      drawnPath: { pathString: 'M 10 20 L 30 40' },
      strokeNum: 0,
      strokesRemaining: 1,
    });
    mocks.correctStroke({
      drawnPath: { pathString: 'M 50 60 L 70 80' },
      strokeNum: 1,
      strokesRemaining: 0,
    });
    mocks.finishQuiz();
  });

  expect(onComplete).toHaveBeenCalledWith([
    [
      { x: 10, y: 20 },
      { x: 30, y: 40 },
    ],
    [
      { x: 50, y: 60 },
      { x: 70, y: 80 },
    ],
  ]);
});

test('shows a loading state while memory-practice data loads', () => {
  mocks.useQuery.mockReturnValue({
    data: undefined,
    isError: false,
    isPending: true,
    refetch: vi.fn(),
  });

  render(<HanziWriterQuiz glyph="馬" sizePx={260} onComplete={vi.fn()} />);

  expect(
    screen.getByRole('status', { name: 'Loading stroke order' }),
  ).toBeVisible();
  expect(
    screen.queryByRole('button', { name: 'Hint next stroke' }),
  ).not.toBeInTheDocument();
});

test('offers fallback practice when stroke data is unavailable', () => {
  mocks.useQuery.mockReturnValue({
    data: undefined,
    error: new ConnectError('missing', Code.NotFound),
    isError: true,
    isPending: false,
    refetch: vi.fn(),
  });

  render(
    <HanziWriterQuiz
      glyph="馬"
      sizePx={260}
      onComplete={vi.fn()}
      fallbackAction={<button type="button">Practice without hints</button>}
    />,
  );

  expect(screen.getByText('Memory practice is unavailable.')).toBeVisible();
  expect(
    screen.getByRole('button', { name: 'Practice without hints' }),
  ).toBeVisible();
});

test('shows a retryable error when memory-practice data cannot load', async () => {
  const refetch = vi.fn();
  mocks.useQuery.mockReturnValue({
    data: undefined,
    error: new ConnectError('offline', Code.Unavailable),
    isError: true,
    isPending: false,
    refetch,
  });
  const user = userEvent.setup();

  render(<HanziWriterQuiz glyph="馬" sizePx={260} onComplete={vi.fn()} />);
  await user.click(screen.getByRole('button', { name: 'Try again' }));

  expect(screen.getByRole('alert')).toHaveTextContent(
    'Could not load memory practice',
  );
  expect(refetch).toHaveBeenCalledOnce();
});

test('rejects malformed stroke data before starting memory practice', () => {
  mocks.useQuery.mockReturnValue({
    data: { data: '{"strokes": [7]}' },
    isError: false,
    isPending: false,
    refetch: vi.fn(),
  });

  render(<HanziWriterQuiz glyph="馬" sizePx={260} onComplete={vi.fn()} />);

  expect(screen.getByRole('alert')).toHaveTextContent(
    'The stroke data has an invalid format.',
  );
  expect(mocks.createWriter).not.toHaveBeenCalled();
});

test('stops instead of grading an invalid renderer drawing path', async () => {
  const onComplete = vi.fn();
  render(<HanziWriterQuiz glyph="馬" sizePx={260} onComplete={onComplete} />);
  await waitFor(() => expect(mocks.writer.quiz).toHaveBeenCalledOnce());

  act(() => {
    mocks.correctStroke({
      drawnPath: { pathString: 'invalid' },
      strokeNum: 0,
      strokesRemaining: 1,
    });
  });

  expect(screen.getByRole('alert')).toHaveTextContent(
    'Memory practice stopped',
  );
  expect(onComplete).not.toHaveBeenCalled();
});

test('announces mistakes and accepted-stroke progress', async () => {
  render(<HanziWriterQuiz glyph="馬" sizePx={260} onComplete={vi.fn()} />);
  await waitFor(() => expect(mocks.writer.quiz).toHaveBeenCalledOnce());

  act(() => mocks.mistake(0));
  expect(screen.getByRole('status')).toHaveTextContent('Try stroke 1 again');

  act(() =>
    mocks.correctStroke({
      drawnPath: { pathString: 'M 10 20 L 30 40' },
      strokeNum: 0,
      strokesRemaining: 1,
    }),
  );
  expect(screen.getByRole('status')).toHaveTextContent(
    'Stroke 1 of 2 complete',
  );
});

test('notifies the practice flow when a stroke is missed', async () => {
  const onMistake = vi.fn();
  render(
    <HanziWriterQuiz
      glyph="馬"
      sizePx={260}
      onComplete={vi.fn()}
      onMistake={onMistake}
    />,
  );
  await waitFor(() => expect(mocks.writer.quiz).toHaveBeenCalledOnce());

  act(() => mocks.mistake(0));

  expect(onMistake).toHaveBeenCalledOnce();
});

test('uses simplified Chinese practice instructions', async () => {
  useLocaleStore.setState({ locale: 'sc' });
  render(<HanziWriterQuiz glyph="马" sizePx={260} onComplete={vi.fn()} />);

  await waitFor(() => expect(mocks.writer.quiz).toHaveBeenCalledOnce());
  expect(screen.getByRole('status')).toHaveTextContent('请写第 1 笔，共 2 笔');

  act(() => mocks.mistake(0));
  expect(screen.getByRole('status')).toHaveTextContent('请重试第 1 笔');
});

test('prevents overlapping hint animations', async () => {
  let finishHint: ((result: { canceled: boolean }) => void) | undefined;
  mocks.writer.highlightStroke.mockImplementationOnce(
    () =>
      new Promise<{ canceled: boolean }>((resolve) => {
        finishHint = resolve;
      }),
  );
  const user = userEvent.setup();
  render(<HanziWriterQuiz glyph="馬" sizePx={260} onComplete={vi.fn()} />);
  await waitFor(() => expect(mocks.writer.quiz).toHaveBeenCalledOnce());

  const hint = screen.getByRole('button', { name: 'Hint next stroke' });
  await user.click(hint);

  expect(hint).toBeDisabled();
  expect(mocks.writer.highlightStroke).toHaveBeenCalledOnce();

  act(() => finishHint?.({ canceled: false }));
  await waitFor(() => expect(hint).toBeEnabled());
});

test('offers a retry when the memory-practice renderer fails', async () => {
  const user = userEvent.setup();
  render(<HanziWriterQuiz glyph="馬" sizePx={260} onComplete={vi.fn()} />);
  await waitFor(() => expect(mocks.createWriter).toHaveBeenCalledOnce());

  act(() => mocks.failRenderer());
  await user.click(screen.getByRole('button', { name: 'Try again' }));

  await waitFor(() => expect(mocks.createWriter).toHaveBeenCalledTimes(2));
});
