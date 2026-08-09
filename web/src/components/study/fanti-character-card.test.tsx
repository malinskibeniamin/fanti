import { create } from '@bufbuild/protobuf';
import { render, screen } from '@testing-library/react';
import { beforeEach, expect, test } from 'vitest';

import { FantiCharacterCard } from '@/components/study/fanti-character-card';
import {
  CharacterCatalogKind,
  CharacterSchema,
} from '@/gen/fanti/v1/dictionary_pb';
import { useLocaleStore } from '@/i18n/locale';

beforeEach(() => {
  useLocaleStore.setState({ locale: 'en' });
});

test('names a manually studied reference entry and its missing meaning', () => {
  const character = create(CharacterSchema, {
    name: 'characters/㐀',
    traditional: '㐀',
    pinyin: 'qiū',
    catalogKind: CharacterCatalogKind.REFERENCE,
  });

  render(<FantiCharacterCard character={character} />);

  expect(screen.getByText('Reference entry')).toBeVisible();
  expect(screen.getByText('Not available in current sources')).toBeVisible();
});
