import { create } from '@bufbuild/protobuf';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, expect, test, vi } from 'vitest';

import { DiscoverChars } from '@/components/discover/discover-chars';
import {
  CapabilityStatus,
  CharacterCapabilitiesSchema,
  CharacterCatalogKind,
  CharacterCoverageSchema,
  CharacterGlyphSchema,
  CharacterSchema,
  DictionaryService,
  ListCharactersResponseSchema,
} from '@/gen/fanti/v1/dictionary_pb';
import { useLocaleStore } from '@/i18n/locale';

const mocks = vi.hoisted(() => ({
  fetchNextPage: vi.fn(),
  useInfiniteQuery: vi.fn(),
  useQuery: vi.fn(),
}));

vi.mock('@connectrpc/connect-query', () => ({
  useInfiniteQuery: mocks.useInfiniteQuery,
  useQuery: mocks.useQuery,
}));
vi.mock('@tanstack/react-router', () => ({
  Link: ({
    children,
    params,
    ...props
  }: React.ComponentProps<'a'> & { params: { char: string } }) => (
    <a href={`/characters/${params.char}`} {...props}>
      {children}
    </a>
  ),
}));

beforeEach(() => {
  vi.clearAllMocks();
  useLocaleStore.setState({ locale: 'en' });

  mocks.useQuery.mockReturnValue({
    data: create(CharacterCoverageSchema, {
      name: 'characterCoverage',
      totalEntries: 41_710,
      totalGlyphs: 44_402,
      curriculumEntries: 11_709,
      referenceEntries: 30_001,
      coreEntries: 3_000,
    }),
    isError: false,
    isPending: false,
  });

  const character = create(CharacterSchema, {
    name: 'characters/馬',
    traditional: '馬',
    simplified: '马',
    pinyin: 'mǎ',
    meaning: 'horse',
    catalogKind: CharacterCatalogKind.CURRICULUM,
    entryCapabilities: create(CharacterCapabilitiesSchema, {
      reading: CapabilityStatus.AVAILABLE,
      meaning: CapabilityStatus.AVAILABLE,
    }),
    glyphs: [
      create(CharacterGlyphSchema, {
        glyph: '馬',
        primary: true,
        capabilities: create(CharacterCapabilitiesSchema, {
          strokes: CapabilityStatus.AVAILABLE,
          components: CapabilityStatus.AVAILABLE,
          history: CapabilityStatus.UNAVAILABLE,
        }),
      }),
    ],
  });

  mocks.useInfiniteQuery.mockReturnValue({
    data: {
      pages: [
        create(ListCharactersResponseSchema, {
          characters: [character],
          nextPageToken: '50',
          totalSize: 41_710,
        }),
      ],
    },
    error: undefined,
    fetchNextPage: mocks.fetchNextPage,
    hasNextPage: true,
    isError: false,
    isFetchingNextPage: false,
    isPending: false,
    refetch: vi.fn(),
  });
});

test('shows the complete pinned catalog coverage', () => {
  render(<DiscoverChars />);

  expect(mocks.useQuery).toHaveBeenCalledWith(
    DictionaryService.method.getCharacterCoverage,
    { name: 'characterCoverage' },
  );
  expect(
    screen.getByRole('region', { name: 'Catalog coverage' }),
  ).toHaveTextContent('41,710');
  expect(
    screen.getByRole('region', { name: 'Catalog coverage' }),
  ).toHaveTextContent('44,402');
  expect(screen.getByText('3,000')).toBeVisible();
  expect(screen.getByText('30,001')).toBeVisible();
});

test('combines catalog and missing-data filters on the server query', async () => {
  const user = userEvent.setup();
  render(<DiscoverChars />);

  await user.click(screen.getByRole('button', { name: 'Curriculum' }));
  await user.click(screen.getByRole('button', { name: 'History' }));

  expect(mocks.useInfiniteQuery).toHaveBeenLastCalledWith(
    DictionaryService.method.listCharacters,
    expect.objectContaining({
      filter: 'catalog_kind = "curriculum" AND missing_capability = "history"',
      pageToken: '',
    }),
    expect.objectContaining({ pageParamKey: 'pageToken' }),
  );
});

test('loads the next catalog page and names source gaps on rows', async () => {
  const user = userEvent.setup();
  render(<DiscoverChars />);

  expect(screen.getByRole('status')).toHaveTextContent(
    'Showing 1 of 41,710 characters',
  );
  expect(screen.getByText('Missing: history')).toBeVisible();

  await user.click(screen.getByRole('button', { name: 'Load more' }));

  expect(mocks.fetchNextPage).toHaveBeenCalledOnce();
});

test('offers a retry when catalog coverage cannot load', async () => {
  const user = userEvent.setup();
  const refetch = vi.fn();
  mocks.useQuery.mockReturnValue({
    data: undefined,
    error: { rawMessage: 'Coverage source is unavailable' },
    isError: true,
    isPending: false,
    refetch,
  });

  render(<DiscoverChars />);

  expect(screen.getByRole('alert')).toHaveTextContent(
    'Coverage source is unavailable',
  );
  await user.click(screen.getByRole('button', { name: 'Try again' }));
  expect(refetch).toHaveBeenCalledOnce();
});

test('offers a retry when the filtered catalog cannot load', async () => {
  const user = userEvent.setup();
  const refetch = vi.fn();
  mocks.useInfiniteQuery.mockReturnValue({
    data: undefined,
    error: { rawMessage: 'Character search is unavailable' },
    fetchNextPage: mocks.fetchNextPage,
    hasNextPage: false,
    isError: true,
    isFetchingNextPage: false,
    isPending: false,
    refetch,
  });

  render(<DiscoverChars />);

  expect(screen.getByRole('alert')).toHaveTextContent(
    'Character search is unavailable',
  );
  await user.click(screen.getByRole('button', { name: 'Try again' }));
  expect(refetch).toHaveBeenCalledOnce();
});
