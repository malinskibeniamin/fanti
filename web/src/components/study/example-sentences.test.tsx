import { create } from '@bufbuild/protobuf';
import { render, screen } from '@testing-library/react';
import { expect, test, vi } from 'vitest';

import { ExampleSentences } from '@/components/study/example-sentences';
import { ExampleSentenceSchema } from '@/gen/fanti/v1/common_pb';

vi.mock('@/lib/speak', () => ({ speak: vi.fn() }));

const examples = [
  create(ExampleSentenceSchema, {
    hskLevel: 4,
    chinese: '雨勢越來越大。',
    english: 'The rain is getting heavier.',
  }),
  create(ExampleSentenceSchema, {
    hskLevel: 0,
    chinese: '我該去睡覺了。',
    english: 'I have to go to sleep.',
  }),
];

test('renders examples as an accessible list', () => {
  render(<ExampleSentences examples={examples} />);

  expect(screen.getByRole('list')).toBeVisible();
  expect(screen.getAllByRole('listitem')).toHaveLength(2);
  expect(screen.getByText('雨勢越來越大。')).toBeVisible();
  expect(screen.getByText('I have to go to sleep.')).toBeVisible();
});

test('labels each pronounce button with its own sentence', () => {
  render(<ExampleSentences examples={examples} />);

  expect(
    screen.getByRole('button', { name: 'Pronounce 雨勢越來越大。' }),
  ).toBeVisible();
  expect(
    screen.getByRole('button', { name: 'Pronounce 我該去睡覺了。' }),
  ).toBeVisible();
});

test('renders duplicate sentence texts as separate rows', () => {
  // Distinct corpus rows can normalize to identical traditional text —
  // both must survive React reconciliation.
  render(<ExampleSentences examples={[examples[1], examples[1]]} />);

  expect(screen.getAllByRole('listitem')).toHaveLength(2);
});

test('shows the HSK pill only for levelled sentences', () => {
  render(<ExampleSentences examples={examples} />);

  expect(screen.getByText('HSK 4')).toBeVisible();
  expect(screen.queryByText('HSK 0')).not.toBeInTheDocument();
});
