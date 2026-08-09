import { create } from '@bufbuild/protobuf';
import { render, screen } from '@testing-library/react';
import { beforeEach, expect, test, vi } from 'vitest';

import { LessonsTab } from '@/components/study/lessons-tab';
import {
  CurriculumProgressSchema,
  LessonSchema,
  StudyProfileSchema,
} from '@/gen/fanti/v1/study_pb';
import { useLocaleStore } from '@/i18n/locale';

const mocks = vi.hoisted(() => ({
  query: vi.fn(),
}));

vi.mock('@connectrpc/connect-query', () => ({
  createConnectQueryKey: vi.fn(),
  useMutation: () => ({ isPending: false, mutate: vi.fn() }),
  useQuery: (...args: unknown[]) => mocks.query(...args),
  useTransport: vi.fn(),
}));
vi.mock('@tanstack/react-query', () => ({
  useQueryClient: () => ({ invalidateQueries: vi.fn() }),
}));
vi.mock('@/components/study/speakable-card', () => ({
  SpeakableCard: () => null,
}));

beforeEach(() => {
  vi.clearAllMocks();
  useLocaleStore.setState({ locale: 'en' });
});

test('shows Core, Complete, and manually learned Reference progress separately', () => {
  const profile = create(StudyProfileSchema, {
    name: 'studyProfile',
    curriculumProgress: create(CurriculumProgressSchema, {
      coreLearned: 1234,
      coreSize: 3000,
      completeLearned: 1250,
      completeSize: 11709,
      referenceLearned: 42,
      referenceSize: 30001,
    }),
  });
  const lesson = create(LessonSchema, {});

  mocks.query
    .mockReturnValueOnce({ data: profile, isError: false })
    .mockReturnValueOnce({ data: lesson, isError: false });

  render(<LessonsTab onStartQuiz={vi.fn()} />);

  expect(screen.getByText('Core curriculum')).toBeVisible();
  expect(screen.getByText('1,234 / 3,000')).toBeVisible();
  expect(screen.getByText('Complete curriculum')).toBeVisible();
  expect(screen.getByText('1,250 / 11,709')).toBeVisible();
  expect(screen.getByText('Reference learned')).toBeVisible();
  expect(screen.getByText('42 / 30,001')).toBeVisible();
});

test('does not start a lesson without a next curriculum character', () => {
  const profile = create(StudyProfileSchema, {
    name: 'studyProfile',
    curriculumProgress: create(CurriculumProgressSchema, {
      coreLearned: 3000,
      coreSize: 3000,
      completeLearned: 11709,
      completeSize: 11709,
    }),
  });

  mocks.query
    .mockReturnValueOnce({ data: profile, isError: false })
    .mockReturnValueOnce({
      data: create(LessonSchema, {}),
      isError: false,
    });

  render(<LessonsTab onStartQuiz={vi.fn()} />);

  expect(screen.getByText('Complete curriculum learned')).toBeVisible();
  expect(
    screen.queryByRole('button', { name: 'Start lesson' }),
  ).not.toBeInTheDocument();
});

test('shows the lesson failure instead of claiming the curriculum is complete', () => {
  const profile = create(StudyProfileSchema, { name: 'studyProfile' });

  mocks.query
    .mockReturnValueOnce({ data: profile, isError: false })
    .mockReturnValueOnce({
      data: undefined,
      error: { rawMessage: 'lesson offline' },
      isError: true,
      refetch: vi.fn(),
    });

  render(<LessonsTab onStartQuiz={vi.fn()} />);

  expect(screen.getByRole('alert')).toHaveTextContent('lesson offline');
  expect(
    screen.queryByText('Complete curriculum learned'),
  ).not.toBeInTheDocument();
});
