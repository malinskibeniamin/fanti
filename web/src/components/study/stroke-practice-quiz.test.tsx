import { ConnectError } from '@connectrpc/connect';
import { act, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, expect, test, vi } from 'vitest';

import { StrokePracticeQuiz } from '@/components/study/stroke-practice-quiz';
import { PracticeDifficulty } from '@/gen/fanti/v1/common_pb';
import { useLocaleStore } from '@/i18n/locale';

const mocks = vi.hoisted(() => ({
  grade: vi.fn(),
  gradePending: false,
  gradeError: undefined as ((error: ConnectError) => void) | undefined,
  gradeSuccess: undefined as
    | ((response: {
        correct: boolean;
        expectedStrokes: number;
        feedback: { en: string; sc: string; tc: string };
        gotStrokes: number;
      }) => void)
    | undefined,
  updateDifficulty: vi.fn(),
  updateSuccess: undefined as
    | ((response: {
        name: string;
        practiceDifficulty: PracticeDifficulty;
      }) => void)
    | undefined,
  reviewRefetch: vi.fn(),
  useMutation: vi.fn(),
  useQuery: vi.fn(),
}));

vi.mock('@connectrpc/connect-query', () => ({
  useMutation: mocks.useMutation,
  useQuery: mocks.useQuery,
}));
vi.mock('@/components/study/stroke-animator', () => ({
  default: ({
    fallbackAction,
    onComplete,
  }: {
    fallbackAction?: React.ReactNode;
    onComplete?: () => void;
  }) => (
    <div>
      <button type="button" onClick={onComplete}>
        Finish watching
      </button>
      {fallbackAction}
    </div>
  ),
}));
vi.mock('@/components/study/hanzi-writer-quiz', () => ({
  default: ({
    fallbackAction,
    onComplete,
    onMistake,
    showOutline,
  }: {
    fallbackAction?: React.ReactNode;
    onComplete: (strokes: { x: number; y: number }[][]) => void;
    onMistake?: () => void;
    showOutline?: boolean;
  }) => (
    <div>
      <span>Memory drawing pad</span>
      <span>{showOutline ? 'Outline visible' : 'Blank grid'}</span>
      <button type="button" onClick={onMistake}>
        Miss stroke
      </button>
      <button
        type="button"
        onClick={() =>
          onComplete([
            [
              { x: 10, y: 20 },
              { x: 30, y: 40 },
            ],
          ])
        }
      >
        Finish drawing
      </button>
      {fallbackAction}
    </div>
  ),
}));

beforeEach(() => {
  vi.clearAllMocks();
  mocks.gradePending = false;
  useLocaleStore.setState({ locale: 'en' });
  vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(null);
  mocks.useQuery.mockReturnValue({
    data: {
      name: 'reviews/馬',
      practiceDifficulty: PracticeDifficulty.GUIDED,
    },
    error: undefined,
    isError: false,
    isPending: false,
    refetch: mocks.reviewRefetch,
  });
  mocks.useMutation.mockImplementation(
    (
      method: { name?: string },
      options?: {
        onError?: (error: ConnectError) => void;
        onSuccess?: (response: never) => void;
      },
    ) => {
      if (method.name === 'UpdateReview') {
        mocks.updateSuccess = options?.onSuccess as typeof mocks.updateSuccess;
        return {
          error: undefined,
          isError: false,
          isPending: false,
          mutate: mocks.updateDifficulty,
        };
      }
      mocks.gradeError = options?.onError;
      mocks.gradeSuccess = options?.onSuccess as typeof mocks.gradeSuccess;
      return {
        error: undefined,
        isError: false,
        isPending: mocks.gradePending,
        mutate: mocks.grade,
      };
    },
  );
});

test('persists a per-character difficulty and skips the preview in recall mode', async () => {
  const user = userEvent.setup();
  render(<StrokePracticeQuiz glyph="馬" sizePx={260} onResult={vi.fn()} />);

  expect(screen.getByRole('radio', { name: 'Guided' })).toHaveAttribute(
    'aria-checked',
    'true',
  );
  await user.click(screen.getByRole('radio', { name: 'Recall' }));

  expect(mocks.updateDifficulty).toHaveBeenCalledWith(
    expect.objectContaining({
      review: expect.objectContaining({
        name: 'reviews/馬',
        practiceDifficulty: PracticeDifficulty.RECALL,
      }),
      updateMask: expect.objectContaining({
        paths: ['practice_difficulty'],
      }),
    }),
  );

  act(() =>
    mocks.updateSuccess?.({
      name: 'reviews/馬',
      practiceDifficulty: PracticeDifficulty.RECALL,
    }),
  );

  expect(await screen.findByText('Memory drawing pad')).toBeVisible();
  expect(screen.getByText('Outline visible')).toBeVisible();
  expect(mocks.reviewRefetch).toHaveBeenCalledOnce();
  expect(
    screen.queryByRole('button', { name: 'Finish watching' }),
  ).not.toBeInTheDocument();
});

test('shows the visual mnemonic before guided drawing', () => {
  render(<StrokePracticeQuiz glyph="馬" sizePx={260} onResult={vi.fn()} />);

  expect(
    screen.getByRole('img', {
      name: 'See 馬 as a horse with mane and four hooves to remember 馬.',
    }),
  ).toBeVisible();
});

test('reveals a recall mnemonic after two missed strokes', async () => {
  mocks.useQuery.mockReturnValue({
    data: {
      name: 'reviews/馬',
      practiceDifficulty: PracticeDifficulty.RECALL,
    },
    error: undefined,
    isError: false,
    isPending: false,
    refetch: vi.fn(),
  });
  const user = userEvent.setup();
  render(<StrokePracticeQuiz glyph="馬" sizePx={260} onResult={vi.fn()} />);

  const cue = 'See 馬 as a horse with mane and four hooves to remember 馬.';
  expect(screen.queryByRole('img', { name: cue })).not.toBeInTheDocument();

  await user.click(await screen.findByRole('button', { name: 'Miss stroke' }));
  expect(screen.queryByRole('img', { name: cue })).not.toBeInTheDocument();

  await user.click(screen.getByRole('button', { name: 'Miss stroke' }));
  expect(screen.getByRole('img', { name: cue })).toBeVisible();
});

test('temporarily lowers mastery to recall when a visual hint is requested', async () => {
  mocks.useQuery.mockReturnValue({
    data: {
      name: 'reviews/馬',
      practiceDifficulty: PracticeDifficulty.MASTERY,
    },
    error: undefined,
    isError: false,
    isPending: false,
    refetch: vi.fn(),
  });
  const user = userEvent.setup();
  render(<StrokePracticeQuiz glyph="馬" sizePx={260} onResult={vi.fn()} />);

  expect(await screen.findByText('Blank grid')).toBeVisible();
  await user.click(screen.getByRole('button', { name: 'Use a Recall hint' }));

  expect(screen.getByText('Outline visible')).toBeVisible();
  expect(
    screen.getByRole('img', {
      name: 'See 馬 as a horse with mane and four hooves to remember 馬.',
    }),
  ).toBeVisible();
  await user.click(screen.getByRole('button', { name: 'Finish drawing' }));
  expect(mocks.grade).toHaveBeenCalledWith(
    expect.objectContaining({
      hintUsed: true,
      practiceDifficulty: PracticeDifficulty.MASTERY,
    }),
  );
  expect(mocks.updateDifficulty).not.toHaveBeenCalled();
});

test('offers a retry when the saved difficulty cannot load', async () => {
  const refetch = vi.fn();
  mocks.useQuery.mockReturnValue({
    data: undefined,
    error: new ConnectError('preferences offline'),
    isError: true,
    isPending: false,
    refetch,
  });
  const user = userEvent.setup();

  render(<StrokePracticeQuiz glyph="馬" sizePx={260} onResult={vi.fn()} />);
  expect(screen.getByRole('alert')).toHaveTextContent('preferences offline');
  expect(
    screen.queryByRole('button', { name: 'Finish watching' }),
  ).not.toBeInTheDocument();

  await user.click(screen.getByRole('button', { name: 'Try again' }));
  expect(refetch).toHaveBeenCalledOnce();
});

test('waits for the saved difficulty before starting practice', () => {
  mocks.useQuery.mockReturnValue({
    data: undefined,
    error: undefined,
    isError: false,
    isPending: true,
    refetch: vi.fn(),
  });

  render(<StrokePracticeQuiz glyph="馬" sizePx={260} onResult={vi.fn()} />);

  expect(
    screen.getByRole('status', { name: 'Loading writing difficulty' }),
  ).toBeVisible();
  expect(
    screen.queryByRole('button', { name: 'Finish watching' }),
  ).not.toBeInTheDocument();
});

test('keeps practicing with cached difficulty after a refresh error', () => {
  mocks.useQuery.mockReturnValue({
    data: {
      name: 'reviews/馬',
      practiceDifficulty: PracticeDifficulty.GUIDED,
    },
    error: new ConnectError('refresh offline'),
    isError: true,
    isPending: false,
    refetch: vi.fn(),
  });

  render(<StrokePracticeQuiz glyph="馬" sizePx={260} onResult={vi.fn()} />);

  expect(screen.getByRole('button', { name: 'Finish watching' })).toBeVisible();
});

test('requires one completed watch before starting memory practice', async () => {
  const user = userEvent.setup();
  render(<StrokePracticeQuiz glyph="馬" sizePx={260} onResult={vi.fn()} />);

  expect(
    screen.queryByRole('button', { name: 'Start memory practice' }),
  ).not.toBeInTheDocument();

  await user.click(
    await screen.findByRole('button', { name: 'Finish watching' }),
  );
  await user.click(
    screen.getByRole('button', { name: 'Start memory practice' }),
  );

  expect(await screen.findByText('Memory drawing pad')).toBeVisible();
  expect(
    screen.queryByRole('button', { name: 'Finish watching' }),
  ).not.toBeInTheDocument();
});

test('grades the completed memory drawing and submits its pass or fail', async () => {
  const onResult = vi.fn();
  const user = userEvent.setup();
  render(<StrokePracticeQuiz glyph="馬" sizePx={260} onResult={onResult} />);
  await user.click(
    await screen.findByRole('button', { name: 'Finish watching' }),
  );
  await user.click(
    screen.getByRole('button', { name: 'Start memory practice' }),
  );
  await user.click(
    await screen.findByRole('button', { name: 'Finish drawing' }),
  );

  expect(mocks.grade).toHaveBeenCalledWith({
    canvasHeight: 260,
    canvasWidth: 260,
    character: '馬',
    hintUsed: false,
    practiceDifficulty: PracticeDifficulty.GUIDED,
    strokes: [
      {
        points: [
          { x: 10, y: 20 },
          { x: 30, y: 40 },
        ],
      },
    ],
  });

  act(() =>
    mocks.gradeSuccess?.({
      correct: true,
      expectedStrokes: 1,
      feedback: { en: 'Every stroke matches.', tc: '', sc: '' },
      gotStrokes: 1,
    }),
  );
  expect(screen.getByText('Correct')).toBeVisible();
  expect(screen.getByText('Every stroke matches.')).toBeVisible();

  await user.click(screen.getByRole('button', { name: 'Continue' }));
  expect(onResult).toHaveBeenCalledWith(true);
});

test('starts a fresh attempt after changing difficulty', async () => {
  const user = userEvent.setup();
  render(<StrokePracticeQuiz glyph="馬" sizePx={260} onResult={vi.fn()} />);
  await user.click(
    await screen.findByRole('button', { name: 'Finish watching' }),
  );
  await user.click(
    screen.getByRole('button', { name: 'Start memory practice' }),
  );
  await user.click(
    await screen.findByRole('button', { name: 'Finish drawing' }),
  );
  act(() =>
    mocks.gradeSuccess?.({
      correct: true,
      expectedStrokes: 1,
      feedback: { en: 'Every stroke matches.', tc: '', sc: '' },
      gotStrokes: 1,
    }),
  );
  expect(screen.getByText('Every stroke matches.')).toBeVisible();

  await user.click(screen.getByRole('radio', { name: 'Mastery' }));
  act(() =>
    mocks.updateSuccess?.({
      name: 'reviews/馬',
      practiceDifficulty: PracticeDifficulty.MASTERY,
    }),
  );

  expect(screen.queryByText('Every stroke matches.')).not.toBeInTheDocument();
  expect(await screen.findByText('Blank grid')).toBeVisible();
});

test('prevents difficulty changes while grading is in progress', () => {
  mocks.gradePending = true;

  render(<StrokePracticeQuiz glyph="馬" sizePx={260} onResult={vi.fn()} />);

  expect(screen.getByRole('radio', { name: 'Guided' })).toBeDisabled();
  expect(screen.getByRole('radio', { name: 'Recall' })).toBeDisabled();
  expect(screen.getByRole('radio', { name: 'Mastery' })).toBeDisabled();
});

test('keeps the completed drawing available when grading needs a retry', async () => {
  const user = userEvent.setup();
  render(<StrokePracticeQuiz glyph="馬" sizePx={260} onResult={vi.fn()} />);
  await user.click(
    await screen.findByRole('button', { name: 'Finish watching' }),
  );
  await user.click(
    screen.getByRole('button', { name: 'Start memory practice' }),
  );
  await user.click(
    await screen.findByRole('button', { name: 'Finish drawing' }),
  );

  act(() => mocks.gradeError?.(new ConnectError('grading offline')));
  expect(screen.getByRole('alert')).toHaveTextContent(
    'Could not grade your strokes',
  );

  await user.click(screen.getByRole('button', { name: 'Try again' }));
  expect(mocks.grade).toHaveBeenCalledTimes(2);
  expect(mocks.grade.mock.calls[1]).toEqual(mocks.grade.mock.calls[0]);
});

test('falls back to manual self-assessment when animation is unavailable', async () => {
  const onResult = vi.fn();
  const user = userEvent.setup();
  render(<StrokePracticeQuiz glyph="馬" sizePx={260} onResult={onResult} />);

  await user.click(
    await screen.findByRole('button', {
      name: 'Practice without animation',
    }),
  );
  expect(
    screen.getByRole('img', { name: 'Write it from memory 馬' }),
  ).toBeVisible();

  await user.click(screen.getByRole('button', { name: 'Show model' }));
  await user.click(screen.getByRole('button', { name: 'Got it wrong' }));

  expect(onResult).toHaveBeenCalledWith(false);
});
