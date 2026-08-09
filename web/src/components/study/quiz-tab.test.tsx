import { create } from '@bufbuild/protobuf';
import { act, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, expect, test, vi } from 'vitest';

import { QuizTab } from '@/components/study/quiz-tab';
import {
  QuestionType,
  QuizQuestionSchema,
  QuizSchema,
  type SubmitQuizAnswerResponse,
  SubmitQuizAnswerResponseSchema,
} from '@/gen/fanti/v1/study_pb';
import { useLocaleStore } from '@/i18n/locale';

const mocks = vi.hoisted(() => ({
  submit: vi.fn(),
  submitSuccess: undefined as
    | ((response: SubmitQuizAnswerResponse) => Promise<void>)
    | undefined,
}));

vi.mock('@connectrpc/connect-query', () => ({
  createConnectQueryKey: vi.fn(),
  useMutation: (
    _method: unknown,
    options?: {
      onSuccess?: (response: SubmitQuizAnswerResponse) => Promise<void>;
    },
  ) => {
    mocks.submitSuccess = options?.onSuccess;
    return { isPending: false, mutate: mocks.submit };
  },
  useQuery: vi.fn(),
  useTransport: vi.fn(),
}));
vi.mock('@tanstack/react-query', () => ({
  useQueryClient: () => ({ invalidateQueries: vi.fn() }),
}));
vi.mock('@/components/study/stroke-practice-quiz', () => ({
  StrokePracticeQuiz: ({
    glyph,
    onResult,
  }: {
    glyph: string;
    onResult: (correct: boolean) => void;
  }) => (
    <button type="button" onClick={() => onResult(true)}>
      Grade {glyph} correct
    </button>
  ),
}));

beforeEach(() => {
  vi.clearAllMocks();
  useLocaleStore.setState({ locale: 'en' });
});

test('records the handwriting grade as the write-question result', async () => {
  const user = userEvent.setup();
  const quiz = create(QuizSchema, {
    name: 'quizzes/quiz-1',
    currentIndex: 0,
    questions: [
      create(QuizQuestionSchema, {
        type: QuestionType.WRITE,
        character: '馬',
      }),
    ],
  });
  render(
    <QuizTab
      quiz={quiz}
      onQuizChange={vi.fn()}
      onStart={vi.fn()}
      onRetry={vi.fn()}
      startPending={false}
    />,
  );

  await user.click(screen.getByRole('button', { name: 'Grade 馬 correct' }));

  expect(mocks.submit).toHaveBeenCalledWith({
    name: 'quizzes/quiz-1',
    questionIndex: 0,
    answer: { case: 'selfCorrect', value: true },
  });

  await act(async () => {
    await mocks.submitSuccess?.(
      create(SubmitQuizAnswerResponseSchema, { correct: true, quiz }),
    );
  });

  expect(
    screen.queryByRole('button', { name: 'Grade 馬 correct' }),
  ).not.toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'Next' })).toBeVisible();
});
