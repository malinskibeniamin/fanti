import { render, screen } from '@testing-library/react';
import { expect, test } from 'vitest';

import {
  TOP_100_GLYPHS,
  VisualMnemonic,
  visualMnemonicFor,
} from '@/components/study/visual-mnemonic';
import { useLocaleStore } from '@/i18n/locale';

test('ships a visual mnemonic for every top-100 character', () => {
  expect(new Set(TOP_100_GLYPHS).size).toBe(100);
  expect(TOP_100_GLYPHS.every((glyph) => visualMnemonicFor(glyph))).toBe(true);
});

test('renders an accessible component-aware illustration', () => {
  useLocaleStore.setState({ locale: 'en' });

  render(<VisualMnemonic glyph="的" />);

  expect(
    screen.getByRole('img', {
      name: 'A white spoon points to the target: the linking particle 的.',
    }),
  ).toBeVisible();
  expect(screen.getByText('白')).toBeVisible();
  expect(screen.getByText('勺')).toBeVisible();
});

test('renders nothing when a character has no authored illustration', () => {
  const { container } = render(<VisualMnemonic glyph="龘" />);

  expect(container).toBeEmptyDOMElement();
});
