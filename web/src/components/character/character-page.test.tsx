import { create } from '@bufbuild/protobuf';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, expect, test, vi } from 'vitest';

import { CharacterPage } from '@/components/character/character-page';
import {
  CapabilityStatus,
  CharacterCapabilitiesSchema,
  CharacterCatalogKind,
  CharacterFormSchema,
  CharacterFormStage,
  CharacterGlyphSchema,
  CharacterHistorySchema,
  CharacterSchema,
  CharacterSenseSchema,
  DictionaryService,
  RadicalPartSchema,
} from '@/gen/fanti/v1/dictionary_pb';
import { useLocaleStore } from '@/i18n/locale';

const mocks = vi.hoisted(() => ({
  invalidateQueries: vi.fn(),
  mutate: vi.fn(),
  useQuery: vi.fn(),
}));

vi.mock('@connectrpc/connect-query', () => ({
  createConnectQueryKey: vi.fn(),
  useMutation: () => ({ isPending: false, mutate: mocks.mutate }),
  useQuery: mocks.useQuery,
  useTransport: () => ({}),
}));
vi.mock('@tanstack/react-query', () => ({
  useQueryClient: () => ({ invalidateQueries: mocks.invalidateQueries }),
}));
vi.mock('@/components/study/stroke-learning-surface', () => ({
  StrokeLearningSurface: ({ glyph }: { glyph: string }) => (
    <div>Stroke learning {glyph}</div>
  ),
}));
vi.mock('@/lib/speak', () => ({ speak: vi.fn() }));

beforeEach(() => {
  vi.clearAllMocks();
  useLocaleStore.setState({ locale: 'en' });
  const character = create(CharacterSchema, {
    name: 'characters/馬',
    traditional: '馬',
    simplified: '马',
    pinyin: 'mǎ',
    meaning: 'horse',
    strokeCount: 10,
  });
  const history = create(CharacterHistorySchema, {
    name: 'characters/馬/history',
    forms: [
      create(CharacterFormSchema, {
        stage: CharacterFormStage.ORACLE,
        available: true,
        svg: new TextEncoder().encode(
          '<svg viewBox="0 0 10 10"><path d="M1 1"/></svg>',
        ),
        sourceTitle: 'File:馬-oracle.svg',
        sourceUrl:
          'https://commons.wikimedia.org/wiki/File:%E9%A6%AC-oracle.svg',
        license: 'Public domain',
      }),
      create(CharacterFormSchema, {
        stage: CharacterFormStage.BRONZE,
      }),
      create(CharacterFormSchema, {
        stage: CharacterFormStage.SEAL,
      }),
      create(CharacterFormSchema, {
        stage: CharacterFormStage.CLERICAL,
      }),
      create(CharacterFormSchema, {
        stage: CharacterFormStage.REGULAR,
        available: true,
      }),
    ],
  });

  mocks.useQuery.mockImplementation((schema) => {
    if (schema === DictionaryService.method.getCharacter) {
      return { data: character, isError: false, isPending: false };
    }
    if (schema === DictionaryService.method.getCharacterHistory) {
      return {
        data: history,
        isError: false,
        isPending: false,
        refetch: vi.fn(),
      };
    }
    throw new Error('Unexpected query');
  });
});

test('opens animated stroke learning from the calligraphy tab', async () => {
  const user = userEvent.setup();
  render(<CharacterPage char="馬" />);

  await user.click(screen.getByRole('tab', { name: 'Calligraphy' }));

  expect(screen.getByText('Stroke learning 馬')).toBeVisible();
});

test('connects attested forms to both modern forms in the evolution timeline', async () => {
  const user = userEvent.setup();
  render(<CharacterPage char="馬" />);

  expect(mocks.useQuery).toHaveBeenCalledTimes(1);
  await user.click(screen.getByRole('tab', { name: 'Evolution' }));

  expect(mocks.useQuery).toHaveBeenCalledWith(
    DictionaryService.method.getCharacterHistory,
    { name: 'characters/馬/history' },
  );

  const timeline = screen.getByRole('region', {
    name: 'Character evolution',
  });
  expect(
    within(timeline).getByRole('img', {
      name: '馬 in oracle bone script',
    }),
  ).toBeVisible();
  expect(
    within(timeline).getAllByText('No attested form found in this source'),
  ).toHaveLength(3);
  expect(
    within(timeline).getByRole('link', { name: 'View source' }),
  ).toHaveAttribute(
    'href',
    'https://commons.wikimedia.org/wiki/File:%E9%A6%AC-oracle.svg',
  );
  expect(within(timeline).getByText('Traditional')).toBeVisible();
  expect(within(timeline).getByText('Simplified')).toBeVisible();
  expect(within(timeline).getByText('马')).toBeVisible();
});

test('exposes character sections as an accessible tab set', async () => {
  const user = userEvent.setup();
  render(<CharacterPage char="馬" />);

  const tablist = screen.getByRole('tablist', { name: '馬' });
  const origin = within(tablist).getByRole('tab', { name: 'Origin' });
  const evolution = within(tablist).getByRole('tab', { name: 'Evolution' });

  expect(origin).toHaveAttribute('aria-selected', 'true');
  expect(evolution).toHaveAttribute('aria-selected', 'false');

  await user.click(evolution);

  expect(origin).toHaveAttribute('aria-selected', 'false');
  expect(evolution).toHaveAttribute('aria-selected', 'true');
  expect(screen.getByRole('tabpanel')).toHaveAccessibleName('Evolution');
});

test('offers a retry when character history cannot load', async () => {
  const user = userEvent.setup();
  const refetch = vi.fn();
  const character = create(CharacterSchema, {
    name: 'characters/馬',
    traditional: '馬',
    simplified: '马',
  });

  mocks.useQuery.mockImplementation((schema) => {
    if (schema === DictionaryService.method.getCharacter) {
      return { data: character, isError: false, isPending: false };
    }
    return {
      data: undefined,
      error: { rawMessage: 'History service is unavailable' },
      isError: true,
      isPending: false,
      refetch,
    };
  });

  render(<CharacterPage char="馬" />);
  await user.click(screen.getByRole('tab', { name: 'Evolution' }));

  expect(screen.getByRole('alert')).toHaveTextContent(
    'History service is unavailable',
  );
  await user.click(screen.getByRole('button', { name: 'Try again' }));
  expect(refetch).toHaveBeenCalledOnce();
});

test('offers ordered component assembly from the origin tab', async () => {
  mocks.useQuery.mockReturnValue({
    data: create(CharacterSchema, {
      name: 'characters/勢',
      traditional: '勢',
      simplified: '势',
      meaning: 'power',
      radicalParts: [
        create(RadicalPartSchema, { part: '埶', note: 'planting' }),
        create(RadicalPartSchema, { part: '力', note: 'strength' }),
      ],
    }),
    isError: false,
    isPending: false,
  });
  const user = userEvent.setup();

  render(<CharacterPage char="勢" />);
  await user.click(screen.getByRole('button', { name: 'Hint next component' }));

  expect(screen.getByRole('status')).toHaveTextContent('Next component: 埶');
});

test('resets component assembly when navigating to another character', async () => {
  const user = userEvent.setup();
  mocks.useQuery.mockReturnValue({
    data: create(CharacterSchema, {
      name: 'characters/勢',
      traditional: '勢',
      radicalParts: [
        create(RadicalPartSchema, { part: '埶' }),
        create(RadicalPartSchema, { part: '力' }),
      ],
    }),
    isError: false,
    isPending: false,
  });
  const view = render(<CharacterPage char="勢" />);
  await user.click(screen.getByRole('button', { name: 'Add component 埶' }));
  await user.click(screen.getByRole('button', { name: 'Add component 力' }));
  expect(screen.getByRole('status')).toHaveTextContent(
    '勢 built from 2 components',
  );

  mocks.useQuery.mockReturnValue({
    data: create(CharacterSchema, {
      name: 'characters/森',
      traditional: '森',
      radicalParts: [
        create(RadicalPartSchema, { part: '木' }),
        create(RadicalPartSchema, { part: '木' }),
        create(RadicalPartSchema, { part: '木' }),
      ],
    }),
    isError: false,
    isPending: false,
  });
  view.rerender(<CharacterPage char="森" />);

  expect(screen.getByRole('status')).toHaveTextContent(
    'Choose the first component',
  );
});

test('makes missing source data explicit for a reference entry', async () => {
  const user = userEvent.setup();
  mocks.useQuery.mockReturnValue({
    data: create(CharacterSchema, {
      name: 'characters/㐀',
      traditional: '㐀',
      simplified: '㐀',
      pinyin: 'qiū',
      catalogKind: CharacterCatalogKind.REFERENCE,
      entryCapabilities: create(CharacterCapabilitiesSchema, {
        reading: CapabilityStatus.AVAILABLE,
        meaning: CapabilityStatus.UNAVAILABLE,
      }),
      glyphs: [
        create(CharacterGlyphSchema, {
          glyph: '㐀',
          primary: true,
          capabilities: create(CharacterCapabilitiesSchema, {
            strokes: CapabilityStatus.UNAVAILABLE,
            components: CapabilityStatus.NOT_APPLICABLE,
            history: CapabilityStatus.UNAVAILABLE,
          }),
        }),
      ],
    }),
    isError: false,
    isPending: false,
  });

  render(<CharacterPage char="㐀" />);

  expect(screen.getByText('Reference entry')).toBeVisible();
  const coverage = screen.getByRole('region', { name: 'Source coverage' });
  expect(within(coverage).getByText('Reading')).toBeVisible();
  expect(within(coverage).getByText('Available')).toBeVisible();
  expect(
    within(coverage).getAllByText('Not available in current sources'),
  ).toHaveLength(3);
  expect(
    within(coverage).getByText('Does not apply to this glyph'),
  ).toBeVisible();

  await user.click(screen.getByRole('tab', { name: 'Calligraphy' }));
  expect(screen.getByRole('tabpanel')).toHaveTextContent(
    'Stroke data is not available in current sources',
  );
  expect(screen.queryByText('Stroke learning 㐀')).not.toBeInTheDocument();
});

test('shows every dictionary sense and related glyph form', () => {
  mocks.useQuery.mockReturnValue({
    data: create(CharacterSchema, {
      name: 'characters/馬',
      traditional: '馬',
      simplified: '马',
      pinyin: 'mǎ',
      meaning: 'horse',
      catalogKind: CharacterCatalogKind.CURRICULUM,
      senses: [
        create(CharacterSenseSchema, {
          simplified: '马',
          pinyin: 'mǎ',
          definitions: ['horse', 'chess piece'],
        }),
        create(CharacterSenseSchema, {
          simplified: '马',
          pinyin: 'Mǎ',
          definitions: ['surname Ma'],
        }),
      ],
      glyphs: [
        create(CharacterGlyphSchema, {
          glyph: '馬',
          primary: true,
          capabilities: create(CharacterCapabilitiesSchema, {
            strokes: CapabilityStatus.AVAILABLE,
            components: CapabilityStatus.AVAILABLE,
            history: CapabilityStatus.AVAILABLE,
          }),
        }),
        create(CharacterGlyphSchema, {
          glyph: '马',
          capabilities: create(CharacterCapabilitiesSchema, {
            strokes: CapabilityStatus.AVAILABLE,
            components: CapabilityStatus.AVAILABLE,
            history: CapabilityStatus.UNAVAILABLE,
          }),
        }),
      ],
      entryCapabilities: create(CharacterCapabilitiesSchema, {
        reading: CapabilityStatus.AVAILABLE,
        meaning: CapabilityStatus.AVAILABLE,
      }),
    }),
    isError: false,
    isPending: false,
  });

  render(<CharacterPage char="馬" />);

  const records = screen.getByRole('region', { name: 'Dictionary records' });
  expect(within(records).getByText('horse; chess piece')).toBeVisible();
  expect(within(records).getByText('surname Ma')).toBeVisible();
  expect(within(records).getByText('Mǎ')).toBeVisible();

  const forms = screen.getByRole('region', { name: 'Related forms' });
  expect(within(forms).getByText('馬')).toBeVisible();
  expect(within(forms).getByText('马')).toBeVisible();
  expect(within(forms).getByText('Primary entry')).toBeVisible();
  expect(within(forms).getByText('Traditional form')).toBeVisible();
  expect(within(forms).getByText('Simplified form')).toBeVisible();
});
