import { create } from '@bufbuild/protobuf';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, expect, test } from 'vitest';

import { RadicalAssembly } from '@/components/character/radical-assembly';
import {
  CapabilityStatus,
  type RadicalPart,
  RadicalPartSchema,
} from '@/gen/fanti/v1/dictionary_pb';
import { useLocaleStore } from '@/i18n/locale';

const PARTS: RadicalPart[] = [
  create(RadicalPartSchema, { part: '埶', note: 'planting' }),
  create(RadicalPartSchema, { part: '力', note: 'strength' }),
];

beforeEach(() => {
  useLocaleStore.setState({ locale: 'en' });
});

test('builds a character in component order with recoverable mistakes', async () => {
  const user = userEvent.setup();
  render(<RadicalAssembly glyph="勢" parts={PARTS} />);

  await user.click(screen.getByRole('button', { name: 'Add component 力' }));
  expect(screen.getByRole('status')).toHaveTextContent('Try another component');
  expect(
    screen.getByRole('button', { name: 'Add component 力' }),
  ).toBeEnabled();

  await user.click(screen.getByRole('button', { name: 'Hint next component' }));
  expect(screen.getByRole('status')).toHaveTextContent('Next component: 埶');

  await user.click(screen.getByRole('button', { name: 'Add component 埶' }));
  expect(screen.getByRole('status')).toHaveTextContent(
    '1 of 2 components added',
  );
  expect(
    screen.getByRole('button', { name: 'Add component 埶' }),
  ).toBeDisabled();

  await user.click(screen.getByRole('button', { name: 'Add component 力' }));
  expect(screen.getByRole('status')).toHaveTextContent(
    '勢 built from 2 components',
  );
  expect(screen.getByRole('button', { name: 'Build again' })).toBeVisible();
});

test('shows an unavailable state when no decomposition exists', () => {
  render(<RadicalAssembly glyph="𠮷" parts={[]} />);

  expect(
    screen.getByText('Component data is not available in current sources.'),
  ).toBeVisible();
  expect(screen.queryByRole('button')).not.toBeInTheDocument();
});

test('does not offer component practice when decomposition does not apply', () => {
  render(
    <RadicalAssembly
      glyph="一"
      parts={[create(RadicalPartSchema, { part: '一' })]}
      status={CapabilityStatus.NOT_APPLICABLE}
    />,
  );

  expect(screen.getByText('Does not apply to this glyph')).toBeVisible();
  expect(screen.queryByRole('button')).not.toBeInTheDocument();
});
